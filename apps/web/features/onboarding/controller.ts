"use server";

import type {
  ActionResult,
  CreateIntegrationCredentialInput,
  CreateStorefrontInput,
  CredentialCreated,
  IntegrationCredential,
  PaymentDestination,
  PreparePaymentDestinationInput,
  PreparedPaymentDestination,
  Seller,
  VerifyPaymentDestinationInput,
} from "@/features/onboarding/model";

const maximumResponseBytes = 1_048_576;
const requestTimeoutMilliseconds = 10_000;

type APIError = {
  message?: string;
};

// getAPIErrorMessage extracts only the documented public error message shape.
function getAPIErrorMessage(responseBody: unknown): string | null {
  if (
    typeof responseBody === "object" &&
    responseBody !== null &&
    "message" in responseBody &&
    typeof (responseBody as APIError).message === "string"
  ) {
    return (responseBody as APIError).message ?? null;
  }
  return null;
}

// getAPIConfiguration returns server-only API credentials or a safe configuration error.
function getAPIConfiguration(): ActionResult<{
  apiOrigin: string;
  sellerToken: string;
}> {
  const apiOrigin = process.env.AGENTPAY_API_ORIGIN ?? "http://localhost:8080";
  const sellerToken = process.env.AGENTPAY_SELLER_BEARER_TOKEN;

  if (!sellerToken) {
    return {
      ok: false,
      error: "Seller authentication is not configured for this environment.",
    };
  }

  return { ok: true, value: { apiOrigin, sellerToken } };
}

// readBoundedResponse prevents an upstream API response from exhausting the web process.
async function readBoundedResponse(response: Response): Promise<string> {
  const reader = response.body?.getReader();
  if (!reader) {
    return "";
  }

  const chunks: Uint8Array[] = [];
  let receivedBytes = 0;

  while (true) {
    const result = await reader.read();
    if (result.done) {
      break;
    }

    receivedBytes += result.value.byteLength;
    if (receivedBytes > maximumResponseBytes) {
      await reader.cancel();
      throw new Error("AgentPay API response exceeded the allowed size.");
    }
    chunks.push(result.value);
  }

  const responseBytes = new Uint8Array(receivedBytes);
  let offset = 0;
  for (const chunk of chunks) {
    responseBytes.set(chunk, offset);
    offset += chunk.byteLength;
  }
  return new TextDecoder().decode(responseBytes);
}

// requestAgentPay invokes one authenticated backend operation without exposing its bearer token.
async function requestAgentPay<Value>(
  path: string,
  method: "GET" | "POST",
  body?: unknown,
): Promise<ActionResult<Value>> {
  const configuration = getAPIConfiguration();
  if (!configuration.ok) {
    return configuration;
  }

  try {
    const response = await fetch(`${configuration.value.apiOrigin}${path}`, {
      method,
      headers: {
        Authorization: `Bearer ${configuration.value.sellerToken}`,
        "Content-Type": "application/json",
        ...(method === "POST"
          ? { "Idempotency-Key": crypto.randomUUID() }
          : {}),
      },
      body: body === undefined ? undefined : JSON.stringify(body),
      cache: "no-store",
      signal: AbortSignal.timeout(requestTimeoutMilliseconds),
    });
    const responseText = await readBoundedResponse(response);
    const responseBody: unknown = responseText ? JSON.parse(responseText) : {};

    if (!response.ok) {
      const apiErrorMessage = getAPIErrorMessage(responseBody);
      return {
        ok: false,
        error:
          apiErrorMessage ??
          "AgentPay could not complete this onboarding step.",
      };
    }

    return { ok: true, value: responseBody as Value };
  } catch {
    return {
      ok: false,
      error: "AgentPay API is unavailable. Check the local API and try again.",
    };
  }
}

// createStorefront creates the seller record that owns every later onboarding resource.
export async function createStorefront(
  input: CreateStorefrontInput,
): Promise<ActionResult<Seller>> {
  return requestAgentPay<Seller>("/v1/sellers", "POST", input);
}

// preparePaymentDestination creates a pending destination and its one-time ownership challenge.
export async function preparePaymentDestination(
  input: PreparePaymentDestinationInput,
): Promise<ActionResult<PreparedPaymentDestination>> {
  const destinationResult = await requestAgentPay<PaymentDestination>(
    `/v1/sellers/${encodeURIComponent(input.sellerId)}/payment-destinations`,
    "POST",
    {
      asset: input.asset,
      network: input.network,
      address: input.address,
    },
  );
  if (!destinationResult.ok) {
    return destinationResult;
  }

  const challengeResult = await requestAgentPay<{ challenge: string }>(
    `/v1/sellers/${encodeURIComponent(input.sellerId)}/payment-destinations/${encodeURIComponent(destinationResult.value.destinationId)}/ownership-challenges`,
    "POST",
    {},
  );
  if (!challengeResult.ok) {
    return challengeResult;
  }

  return {
    ok: true,
    value: {
      destination: destinationResult.value,
      challenge: challengeResult.value.challenge,
    },
  };
}

// verifyPaymentDestination submits the browser-wallet proof and activates the destination.
export async function verifyPaymentDestination(
  input: VerifyPaymentDestinationInput,
): Promise<ActionResult<PaymentDestination>> {
  return requestAgentPay<PaymentDestination>(
    `/v1/sellers/${encodeURIComponent(input.sellerId)}/payment-destinations/${encodeURIComponent(input.destinationId)}/verify`,
    "POST",
    {
      challenge: input.challenge,
      signature: input.signature,
      confirmRotation: false,
    },
  );
}

// createIntegrationCredential issues the project key once with the minimum onboarding scopes.
export async function createIntegrationCredential(
  input: CreateIntegrationCredentialInput,
): Promise<ActionResult<CredentialCreated>> {
  return requestAgentPay<CredentialCreated>(
    `/v1/sellers/${encodeURIComponent(input.sellerId)}/integration-credentials`,
    "POST",
    {
      label: "Primary coding agent",
      scopes: ["read", "configure", "validate", "publish"],
      expiresAt: null,
    },
  );
}

// listOnboardingResources restores non-secret wallet and credential status for a known seller.
export async function listOnboardingResources(sellerId: string): Promise<{
  paymentDestinations: PaymentDestination[];
  credentials: IntegrationCredential[];
}> {
  const [paymentResult, credentialResult] = await Promise.all([
    requestAgentPay<{ items: PaymentDestination[] }>(
      `/v1/sellers/${encodeURIComponent(sellerId)}/payment-destinations`,
      "GET",
    ),
    requestAgentPay<{ items: IntegrationCredential[] }>(
      `/v1/sellers/${encodeURIComponent(sellerId)}/integration-credentials`,
      "GET",
    ),
  ]);

  return {
    paymentDestinations: paymentResult.ok ? paymentResult.value.items : [],
    credentials: credentialResult.ok ? credentialResult.value.items : [],
  };
}
