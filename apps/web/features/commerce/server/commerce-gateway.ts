import "server-only";

import { createHash, randomUUID } from "node:crypto";
import { browserPurchaseCookieNames } from "@/features/commerce/commerce-cookie-policy";
import type {
  BrowserDispute,
  BuyerPurchaseSnapshot,
  CheckoutCompleteResult,
  CheckoutStartResult,
  CommerceChannel,
  PurchaseReceipt,
  PurchaseIntent,
} from "@/features/commerce/model";
import type { DisputeReason } from "@/features/disputes/model";

const requestTimeoutMilliseconds = 12_000;
const maximumRequestBodyBytes = 65_536;
const slugPattern = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;
const routeIDPattern = /^rte_[A-Za-z0-9]+$/;
const intentIDPattern = /^int_[A-Za-z0-9]+$/;
const transactionIDPattern = /^txn_[A-Za-z0-9]+$/;
const requestIDPattern = /^[\x20-\x7e]{1,128}$/;
const disputeIDPattern = /^dsp_[A-Za-z0-9]+$/;
const paidPathPattern = /^\/[A-Za-z0-9/_-]+$/;
const disputeReasons = new Set<DisputeReason>([
  "unauthorized",
  "duplicate",
  "wrong_amount",
  "not_delivered",
  "quality_or_output",
]);

type GatewayResult<Value> =
  | { ok: true; value: Value; setCookies?: string[] }
  | {
      ok: false;
      status: number;
      code: string;
      message: string;
      recoveryAction?: string;
    };

type StartInput = {
  channel: CommerceChannel;
  sellerSlug: string;
  productSlug: string;
  routeId: string;
  maximumAmount: string;
  requestBody: string;
};

type CompleteInput = {
  channel: CommerceChannel;
  sellerSlug: string;
  requestBody: string;
  purchaseIntent: PurchaseIntent;
  paymentSignature: string;
};

type CreateBrowserDisputeInput = {
  transactionId: string;
  reason: DisputeReason;
  statement: string;
};

export type BrowserReceiptFile = {
  body: string;
  contentType: string;
  contentDisposition: string;
  receipt: PurchaseReceipt;
};

export async function startCommerce(
  input: unknown,
): Promise<GatewayResult<CheckoutStartResult>> {
  const parsed = parseStartInput(input);
  if (!parsed.ok) {
    return parsed;
  }
  const apiOrigin = getAPIOrigin();
  const canonicalRequest = canonicalRequestBody(parsed.value.requestBody);
  if (!canonicalRequest.ok) {
    return canonicalRequest;
  }
  const requestBodyHash = sha256(canonicalRequest.value.body);
  let cookieHeader = "";
  let csrfToken = "";
  let setCookies: string[] = [];
  const browserCookies = browserPurchaseCookieNames();

  if (parsed.value.channel === "browser") {
    const sessionResponse = await fetchBounded(
      `${apiOrigin}/v1/storefronts/${parsed.value.sellerSlug}/products/${parsed.value.productSlug}/purchase-sessions`,
      {
        method: "POST",
        headers: jsonHeaders({ "Idempotency-Key": randomUUID() }),
        body: JSON.stringify({
          requestBodyHash,
          maximumAmount: parsed.value.maximumAmount,
        }),
      },
    );
    if (!sessionResponse.ok) {
      return sessionResponse.error;
    }
    setCookies = readSetCookies(sessionResponse.response.headers);
    cookieHeader = cookieHeaderFromSetCookies(setCookies);
    csrfToken = cookieValue(cookieHeader, browserCookies.csrf);
    if (!cookieValue(cookieHeader, browserCookies.purchase) || !csrfToken) {
      return gatewayFailure(
        503,
        "dependency_unavailable",
        "AgentPay did not establish a secure purchase session.",
      );
    }
  }

  const authorizationHeaders = commerceAuthorization(
    parsed.value.channel,
    cookieHeader,
    csrfToken,
  );
  if (!authorizationHeaders.ok) {
    return authorizationHeaders;
  }
  const intentResponse = await fetchBounded(`${apiOrigin}/v1/intents`, {
    method: "POST",
    headers: jsonHeaders({
      ...authorizationHeaders.value,
      "Idempotency-Key": randomUUID(),
    }),
    body: JSON.stringify({
      routeId: parsed.value.routeId,
      requestBodyHash,
      maximumAmount: parsed.value.maximumAmount,
    }),
  });
  if (!intentResponse.ok) {
    return intentResponse.error;
  }
  const purchaseIntent = intentResponse.value as PurchaseIntent;
  if (!validIntent(purchaseIntent, parsed.value)) {
    return gatewayFailure(
      502,
      "invalid_upstream_response",
      "AgentPay returned an invalid or mismatched purchase intent.",
    );
  }
  const challengeResponse = await fetchBounded(
    `${apiOrigin}/pay/${parsed.value.sellerSlug}${purchaseIntent.requestPath}`,
    {
      method: purchaseIntent.requestMethod,
      headers: {
        ...authorizationHeaders.value,
        "X-AgentPay-Intent-Id": purchaseIntent.intentId,
        ...(canonicalRequest.value.contentType
          ? { "Content-Type": canonicalRequest.value.contentType }
          : {}),
      },
      body:
        purchaseIntent.requestMethod === "POST"
          ? canonicalRequest.value.body
          : undefined,
    },
  );
  if (challengeResponse.response?.status !== 402) {
    return challengeResponse.ok
      ? gatewayFailure(
          502,
          "invalid_upstream_response",
          "AgentPay did not return the required x402 payment challenge.",
        )
      : challengeResponse.error;
  }
  const paymentRequired =
    challengeResponse.response.headers.get("PAYMENT-REQUIRED");
  if (!paymentRequired) {
    return gatewayFailure(
      502,
      "invalid_upstream_response",
      "AgentPay returned an incomplete x402 payment challenge.",
    );
  }
  const transactionId = challengeResponse.response.headers.get(
    "X-AgentPay-Transaction-Id",
  );
  const requestId = challengeResponse.response.headers.get(
    "X-AgentPay-Request-Id",
  );
  if (
    (transactionId !== null && !transactionIDPattern.test(transactionId)) ||
    (!transactionId &&
      (requestId === null || !requestIDPattern.test(requestId)))
  ) {
    return gatewayFailure(
      502,
      "invalid_upstream_response",
      "AgentPay returned a payment challenge without a trace identifier.",
    );
  }

  return {
    ok: true,
    value: {
      purchaseIntent,
      ...(transactionId ? { transactionId } : {}),
      traceId: transactionId ?? requestId!,
      paymentRequired,
      paymentMode: paymentChallengeMode(paymentRequired),
    },
    setCookies,
  };
}

export async function completeCommerce(
  input: unknown,
  incomingCookieHeader: string,
): Promise<GatewayResult<CheckoutCompleteResult>> {
  const parsed = parseCompleteInput(input);
  if (!parsed.ok) {
    return parsed;
  }
  const { purchaseIntent } = parsed.value;
  const canonicalRequest = canonicalRequestBody(parsed.value.requestBody);
  if (!canonicalRequest.ok) {
    return canonicalRequest;
  }
  if (sha256(canonicalRequest.value.body) !== purchaseIntent.requestBodyHash) {
    return gatewayFailure(
      409,
      "request_changed",
      "The request body changed after the quote was frozen. Start a new checkout.",
    );
  }
  const browserCookies = browserPurchaseCookieNames();
  const csrfToken = cookieValue(incomingCookieHeader, browserCookies.csrf);
  const authorizationHeaders = commerceAuthorization(
    parsed.value.channel,
    incomingCookieHeader,
    csrfToken,
  );
  if (!authorizationHeaders.ok) {
    return authorizationHeaders;
  }
  const paidResponse = await fetchBounded(
    `${getAPIOrigin()}/pay/${parsed.value.sellerSlug}${purchaseIntent.requestPath}`,
    {
      method: purchaseIntent.requestMethod,
      headers: {
        ...authorizationHeaders.value,
        "X-AgentPay-Intent-Id": purchaseIntent.intentId,
        "PAYMENT-SIGNATURE": parsed.value.paymentSignature,
        ...(canonicalRequest.value.contentType
          ? { "Content-Type": canonicalRequest.value.contentType }
          : {}),
      },
      body:
        purchaseIntent.requestMethod === "POST"
          ? canonicalRequest.value.body
          : undefined,
    },
  );
  if (!paidResponse.ok) {
    return paidResponse.error;
  }
  const contentType =
    paidResponse.response.headers.get("Content-Type") ??
    "application/octet-stream";
  return {
    ok: true,
    value: {
      status: "fulfilled",
      transactionId:
        paidResponse.response.headers.get("X-AgentPay-Transaction-Id") ??
        "unknown",
      settlementReference:
        paidResponse.response.headers.get("PAYMENT-RESPONSE") ?? undefined,
      contentType,
      fulfillment: parseFulfillment(paidResponse.body, contentType),
    },
  };
}

export async function loadBrowserPurchase(
  transactionId: string,
  cookieHeader: string,
): Promise<GatewayResult<BuyerPurchaseSnapshot>> {
  if (!transactionIDPattern.test(transactionId)) {
    return gatewayFailure(404, "not_found", "Purchase record not found.");
  }
  const authorization = browserReadAuthorization(cookieHeader);
  if (!authorization.ok) {
    return authorization;
  }
  const response = await fetchBounded(
    `${getAPIOrigin()}/v1/transactions/${encodeURIComponent(transactionId)}`,
    { method: "GET", headers: authorization.value },
  );
  return response.ok
    ? { ok: true, value: response.value as BuyerPurchaseSnapshot }
    : response.error;
}

export async function loadBrowserPurchaseReceipt(
  transactionId: string,
  cookieHeader: string,
): Promise<GatewayResult<BrowserReceiptFile>> {
  if (!transactionIDPattern.test(transactionId)) {
    return gatewayFailure(404, "not_found", "Receipt not found.");
  }
  const authorization = browserReadAuthorization(cookieHeader);
  if (!authorization.ok) {
    return authorization;
  }
  const response = await fetchBounded(
    `${getAPIOrigin()}/v1/transactions/${encodeURIComponent(transactionId)}/receipt`,
    { method: "GET", headers: authorization.value },
  );
  if (!response.ok) {
    return response.error;
  }
  if (!isRecord(response.value)) {
    return gatewayFailure(
      502,
      "invalid_upstream_response",
      "AgentPay returned an invalid receipt.",
    );
  }
  return {
    ok: true,
    value: {
      body: response.body,
      contentType:
        response.response.headers.get("Content-Type") ??
        "application/vnd.agentpay.receipt+json",
      contentDisposition:
        response.response.headers.get("Content-Disposition") ??
        `attachment; filename="${transactionId}-receipt.json"`,
      receipt: response.value as PurchaseReceipt,
    },
  };
}

export async function createBrowserDispute(
  input: unknown,
  cookieHeader: string,
): Promise<GatewayResult<BrowserDispute>> {
  const parsed = parseBrowserDisputeInput(input);
  if (!parsed.ok) {
    return parsed;
  }
  const browserCookies = browserPurchaseCookieNames();
  const authorization = commerceAuthorization(
    "browser",
    cookieHeader,
    cookieValue(cookieHeader, browserCookies.csrf),
  );
  if (!authorization.ok) {
    return authorization;
  }
  const response = await fetchBounded(`${getAPIOrigin()}/v1/disputes`, {
    method: "POST",
    headers: jsonHeaders({
      ...authorization.value,
      "Idempotency-Key": randomUUID(),
    }),
    body: JSON.stringify(parsed.value),
  });
  return response.ok
    ? { ok: true, value: response.value as BrowserDispute }
    : response.error;
}

export async function loadBrowserDispute(
  disputeId: string,
  cookieHeader: string,
): Promise<GatewayResult<BrowserDispute>> {
  if (!disputeIDPattern.test(disputeId)) {
    return gatewayFailure(404, "not_found", "Dispute record not found.");
  }
  const authorization = browserReadAuthorization(cookieHeader);
  if (!authorization.ok) {
    return authorization;
  }
  const response = await fetchBounded(
    `${getAPIOrigin()}/v1/disputes/${encodeURIComponent(disputeId)}`,
    { method: "GET", headers: authorization.value },
  );
  return response.ok
    ? { ok: true, value: response.value as BrowserDispute }
    : response.error;
}

function parseStartInput(input: unknown): GatewayResult<StartInput> {
  if (!isRecord(input)) {
    return gatewayFailure(400, "invalid_request", "Invalid checkout request.");
  }
  const value = input as Partial<StartInput>;
  if (
    (value.channel !== "browser" && value.channel !== "agent") ||
    typeof value.sellerSlug !== "string" ||
    !slugPattern.test(value.sellerSlug) ||
    typeof value.productSlug !== "string" ||
    !slugPattern.test(value.productSlug) ||
    typeof value.routeId !== "string" ||
    !routeIDPattern.test(value.routeId) ||
    typeof value.maximumAmount !== "string" ||
    !/^\d+$/.test(value.maximumAmount) ||
    typeof value.requestBody !== "string" ||
    byteLength(value.requestBody) > maximumRequestBodyBytes
  ) {
    return gatewayFailure(400, "invalid_request", "Invalid checkout request.");
  }
  return { ok: true, value: value as StartInput };
}

function parseCompleteInput(input: unknown): GatewayResult<CompleteInput> {
  if (!isRecord(input)) {
    return gatewayFailure(400, "invalid_request", "Invalid payment request.");
  }
  const value = input as Partial<CompleteInput>;
  if (
    (value.channel !== "browser" && value.channel !== "agent") ||
    typeof value.sellerSlug !== "string" ||
    !slugPattern.test(value.sellerSlug) ||
    typeof value.requestBody !== "string" ||
    byteLength(value.requestBody) > maximumRequestBodyBytes ||
    typeof value.paymentSignature !== "string" ||
    value.paymentSignature.length < 8 ||
    value.paymentSignature.length > 16_384 ||
    !isRecord(value.purchaseIntent) ||
    typeof value.purchaseIntent.intentId !== "string" ||
    !intentIDPattern.test(value.purchaseIntent.intentId) ||
    (value.purchaseIntent.requestMethod !== "GET" &&
      value.purchaseIntent.requestMethod !== "POST") ||
    typeof value.purchaseIntent.requestPath !== "string" ||
    !paidPathPattern.test(value.purchaseIntent.requestPath)
  ) {
    return gatewayFailure(400, "invalid_request", "Invalid payment request.");
  }
  return { ok: true, value: value as CompleteInput };
}

function parseBrowserDisputeInput(
  input: unknown,
): GatewayResult<CreateBrowserDisputeInput> {
  if (!isRecord(input)) {
    return gatewayFailure(400, "invalid_request", "Invalid dispute request.");
  }
  const keys = Object.keys(input);
  const value = input as Partial<CreateBrowserDisputeInput>;
  if (
    keys.some(
      (key) =>
        key !== "transactionId" && key !== "reason" && key !== "statement",
    ) ||
    typeof value.transactionId !== "string" ||
    !transactionIDPattern.test(value.transactionId) ||
    typeof value.reason !== "string" ||
    !disputeReasons.has(value.reason as DisputeReason) ||
    typeof value.statement !== "string" ||
    value.statement.length > 2000
  ) {
    return gatewayFailure(400, "invalid_request", "Invalid dispute request.");
  }
  return { ok: true, value: value as CreateBrowserDisputeInput };
}

function validIntent(intent: PurchaseIntent, input: StartInput): boolean {
  return (
    isRecord(intent) &&
    intent.routeId === input.routeId &&
    intent.productSlug === input.productSlug &&
    intent.purchaseChannel === input.channel &&
    intent.maximumAmount === input.maximumAmount &&
    typeof intent.intentId === "string" &&
    intentIDPattern.test(intent.intentId) &&
    (intent.requestMethod === "GET" || intent.requestMethod === "POST") &&
    paidPathPattern.test(intent.requestPath) &&
    /^\d+$/.test(intent.amount) &&
    intent.amount.length <= input.maximumAmount.length + 1 &&
    BigInt(intent.amount) <= BigInt(input.maximumAmount)
  );
}

function commerceAuthorization(
  channel: CommerceChannel,
  cookieHeader: string,
  csrfToken: string,
): GatewayResult<Record<string, string>> {
  if (channel === "agent") {
    const buyerAgentKey = (
      process.env.AGENTPAY_BUYER_AGENT_KEY ??
      process.env.AGENTPAY_LOCAL_AGENT_KEY
    )?.trim();
    return buyerAgentKey
      ? { ok: true, value: { "X-AgentPay-Agent-Key": buyerAgentKey } }
      : gatewayFailure(
          503,
          "agent_key_unavailable",
          "The agent checkout demo is not configured in this environment.",
        );
  }
  const browserCookies = browserPurchaseCookieNames();
  if (!cookieValue(cookieHeader, browserCookies.purchase) || !csrfToken) {
    return gatewayFailure(
      401,
      "purchase_session_missing",
      "The secure purchase session expired. Start checkout again.",
    );
  }
  return {
    ok: true,
    value: {
      Cookie: cookieHeader,
      "X-AgentPay-CSRF": csrfToken,
      Origin: process.env.AGENTPAY_WEB_ORIGIN ?? "http://localhost:3000",
      "Sec-Fetch-Site": "same-origin",
    },
  };
}

function browserReadAuthorization(
  cookieHeader: string,
): GatewayResult<Record<string, string>> {
  const browserCookies = browserPurchaseCookieNames();
  if (!cookieValue(cookieHeader, browserCookies.purchase)) {
    return gatewayFailure(
      401,
      "purchase_session_missing",
      "Purchase access expired. Recover it with the paying wallet.",
    );
  }
  return { ok: true, value: { Cookie: cookieHeader } };
}

function canonicalRequestBody(
  requestBody: string,
): GatewayResult<{ body: string; contentType: string }> {
  if (!requestBody.trim()) {
    return { ok: true, value: { body: "", contentType: "" } };
  }
  try {
    return {
      ok: true,
      value: {
        body: JSON.stringify(sortJSON(JSON.parse(requestBody))),
        contentType: "application/json",
      },
    };
  } catch {
    return gatewayFailure(
      400,
      "invalid_request_body",
      "Request body must be valid JSON.",
    );
  }
}

function sortJSON(value: unknown): unknown {
  if (Array.isArray(value)) {
    return value.map(sortJSON);
  }
  if (isRecord(value)) {
    return Object.fromEntries(
      Object.keys(value)
        .sort()
        .map((key) => [key, sortJSON(value[key])]),
    );
  }
  return value;
}

async function fetchBounded(
  url: string,
  init: RequestInit,
): Promise<
  | { ok: true; response: Response; body: string; value: unknown }
  | {
      ok: false;
      response?: Response;
      body?: string;
      error: Extract<GatewayResult<never>, { ok: false }>;
    }
> {
  try {
    const response = await fetch(url, {
      ...init,
      cache: "no-store",
      signal: AbortSignal.timeout(requestTimeoutMilliseconds),
    });
    const body = await response.text();
    if (byteLength(body) > 1_048_576) {
      return {
        ok: false,
        response,
        error: gatewayFailure(
          502,
          "invalid_upstream_response",
          "AgentPay returned an oversized response.",
        ),
      };
    }
    let value: unknown = {};
    if (body) {
      try {
        value = JSON.parse(body);
      } catch {
        value = body;
      }
    }
    if (!response.ok) {
      const apiError =
        isRecord(value) && isRecord(value.error) ? value.error : null;
      return {
        ok: false,
        response,
        body,
        error: gatewayFailure(
          response.status,
          apiError && typeof apiError.code === "string"
            ? apiError.code
            : "checkout_failed",
          apiError && typeof apiError.message === "string"
            ? apiError.message
            : "AgentPay could not complete checkout.",
          apiError &&
            isRecord(apiError.details) &&
            typeof apiError.details.recoveryAction === "string"
            ? apiError.details.recoveryAction
            : undefined,
        ),
      };
    }
    return { ok: true, response, body, value };
  } catch {
    return {
      ok: false,
      error: gatewayFailure(
        503,
        "dependency_unavailable",
        "AgentPay commerce services are temporarily unavailable.",
      ),
    };
  }
}

function paymentChallengeMode(encodedChallenge: string): "mock" | "x402" {
  try {
    const decoded = JSON.parse(
      Buffer.from(encodedChallenge, "base64").toString("utf8"),
    ) as unknown;
    return isRecord(decoded) && decoded.x402Version === 2 ? "x402" : "mock";
  } catch {
    return "x402";
  }
}

function parseFulfillment(body: string, contentType: string): unknown {
  if (contentType.toLowerCase().includes("json")) {
    try {
      return JSON.parse(body);
    } catch {
      return body;
    }
  }
  return body;
}

function jsonHeaders(headers: Record<string, string>): Record<string, string> {
  return { "Content-Type": "application/json", ...headers };
}

function readSetCookies(headers: Headers): string[] {
  const enhancedHeaders = headers as Headers & {
    getSetCookie?: () => string[];
  };
  const direct = enhancedHeaders.getSetCookie?.() ?? [];
  if (direct.length > 0) {
    return direct;
  }
  const combined = headers.get("set-cookie");
  return combined
    ? combined
        .split(/,(?=\s*(?:__Host-)?agentpay_)/)
        .map((value) => value.trim())
    : [];
}

function cookieHeaderFromSetCookies(setCookies: string[]): string {
  return setCookies.map((cookie) => cookie.split(";", 1)[0]).join("; ");
}

function cookieValue(cookieHeader: string, name: string): string {
  for (const cookie of cookieHeader.split(";")) {
    const [cookieName, ...valueParts] = cookie.trim().split("=");
    if (cookieName === name) {
      return valueParts.join("=");
    }
  }
  return "";
}

function sha256(value: string): string {
  return createHash("sha256").update(value).digest("hex");
}

function byteLength(value: string): number {
  return Buffer.byteLength(value, "utf8");
}

function getAPIOrigin(): string {
  return (process.env.AGENTPAY_API_ORIGIN ?? "http://localhost:8080").replace(
    /\/$/,
    "",
  );
}

function gatewayFailure(
  status: number,
  code: string,
  message: string,
  recoveryAction?: string,
): Extract<GatewayResult<never>, { ok: false }> {
  return { ok: false, status, code, message, recoveryAction };
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}
