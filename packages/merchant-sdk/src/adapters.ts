import type { ExecutionClaims } from "@agentpay/verify-node";

const DEFAULT_TIMEOUT_MS = 10_000;
const DEFAULT_MAXIMUM_RESPONSE_BYTES = 1024 * 1024;

export interface MerchantFulfillmentContext<TInput> {
  transactionId: string;
  execution: ExecutionClaims;
  input: TInput;
}

export interface MerchantAdapter<TInput, TResult> {
  fulfill(context: MerchantFulfillmentContext<TInput>): Promise<TResult>;
}

export type MerchantAdapterErrorCode =
  | "invalid_configuration"
  | "invalid_response"
  | "response_too_large"
  | "timeout"
  | "upstream_rejected"
  | "upstream_unavailable";

export class MerchantAdapterError extends Error {
  constructor(public readonly code: MerchantAdapterErrorCode) {
    super(`Merchant adapter failed: ${code}`);
    this.name = "MerchantAdapterError";
  }
}

interface TransportOptions {
  fetch?: typeof fetch;
  timeoutMs?: number;
  maximumResponseBytes?: number;
}

export interface HttpsMerchantAdapterOptions extends TransportOptions {
  endpoint: string;
  headers?: Readonly<Record<string, string>>;
}

export function createHttpsMerchantAdapter<TInput = unknown, TResult = unknown>(
  options: HttpsMerchantAdapterOptions,
): MerchantAdapter<TInput, TResult> {
  const endpoint = requireHttpsUrl(options.endpoint);
  const transport = createJsonTransport(options);
  return {
    fulfill(context) {
      return transport<TResult>(endpoint, {
        method: "POST",
        headers: {
          ...options.headers,
          "Content-Type": "application/json",
          "Idempotency-Key": context.transactionId,
          "X-AgentPay-Transaction-Id": context.transactionId,
        },
        body: JSON.stringify({
          transactionId: context.transactionId,
          execution: context.execution,
          payload: context.input,
        }),
      });
    },
  };
}

export interface ShopifyGraphqlOperation {
  query: string;
  variables?: Record<string, unknown>;
}

export interface ShopifyGraphqlResponse<TData = Record<string, unknown>> {
  data?: TData;
  errors?: ReadonlyArray<{ message: string }>;
}

export interface ShopifyAdminAdapterOptions extends TransportOptions {
  shopDomain: string;
  apiVersion: string;
  accessToken: string;
}

export function createShopifyAdminAdapter<TData = Record<string, unknown>>(
  options: ShopifyAdminAdapterOptions,
): MerchantAdapter<ShopifyGraphqlOperation, ShopifyGraphqlResponse<TData>> {
  if (
    !/^[a-z0-9][a-z0-9-]*\.myshopify\.com$/i.test(options.shopDomain) ||
    !/^\d{4}-(?:01|04|07|10)$/.test(options.apiVersion) ||
    options.accessToken.length === 0
  ) {
    throw new MerchantAdapterError("invalid_configuration");
  }
  const endpoint = new URL(
    `/admin/api/${options.apiVersion}/graphql.json`,
    `https://${options.shopDomain}`,
  );
  const transport = createJsonTransport(options);
  return {
    async fulfill(context) {
      if (!context.input.query.trim()) {
        throw new MerchantAdapterError("invalid_configuration");
      }
      const response = await transport<ShopifyGraphqlResponse<TData>>(
        endpoint,
        {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            "X-Shopify-Access-Token": options.accessToken,
            "X-AgentPay-Transaction-Id": context.transactionId,
          },
          body: JSON.stringify(context.input),
        },
      );
      if (response.errors && response.errors.length > 0) {
        throw new MerchantAdapterError("upstream_rejected");
      }
      return response;
    },
  };
}

export type WooCommerceOrderDraft = Record<string, unknown> & {
  meta_data?: ReadonlyArray<Record<string, unknown>>;
};

export type WooCommerceOrder = Record<string, unknown> & { id: number };

export interface WooCommerceAdapterOptions extends TransportOptions {
  storeOrigin: string;
  consumerKey: string;
  consumerSecret: string;
}

export function createWooCommerceAdapter(
  options: WooCommerceAdapterOptions,
): MerchantAdapter<WooCommerceOrderDraft, WooCommerceOrder> {
  const storeOrigin = requireHttpsUrl(options.storeOrigin);
  if (!options.consumerKey || !options.consumerSecret) {
    throw new MerchantAdapterError("invalid_configuration");
  }
  const endpoint = new URL("/wp-json/wc/v3/orders", storeOrigin);
  const transport = createJsonTransport(options);
  return {
    async fulfill(context) {
      const metaData = Array.isArray(context.input.meta_data)
        ? context.input.meta_data.filter(
            (entry) => entry.key !== "_agentpay_transaction_id",
          )
        : [];
      const response = await transport<unknown>(endpoint, {
        method: "POST",
        headers: {
          Authorization: `Basic ${Buffer.from(
            `${options.consumerKey}:${options.consumerSecret}`,
          ).toString("base64")}`,
          "Content-Type": "application/json",
          "Idempotency-Key": context.transactionId,
          "X-AgentPay-Transaction-Id": context.transactionId,
        },
        body: JSON.stringify({
          ...context.input,
          meta_data: [
            ...metaData,
            { key: "_agentpay_transaction_id", value: context.transactionId },
          ],
        }),
      });
      if (
        typeof response !== "object" ||
        response === null ||
        typeof (response as { id?: unknown }).id !== "number"
      ) {
        throw new MerchantAdapterError("invalid_response");
      }
      return response as WooCommerceOrder;
    },
  };
}

function createJsonTransport(options: TransportOptions) {
  const fetchImplementation = options.fetch ?? globalThis.fetch;
  const timeoutMs = options.timeoutMs ?? DEFAULT_TIMEOUT_MS;
  const maximumResponseBytes =
    options.maximumResponseBytes ?? DEFAULT_MAXIMUM_RESPONSE_BYTES;
  if (
    typeof fetchImplementation !== "function" ||
    !Number.isInteger(timeoutMs) ||
    timeoutMs <= 0 ||
    !Number.isInteger(maximumResponseBytes) ||
    maximumResponseBytes <= 0
  ) {
    throw new MerchantAdapterError("invalid_configuration");
  }

  return async function sendJson<TResult>(
    endpoint: URL,
    init: RequestInit,
  ): Promise<TResult> {
    const abortController = new AbortController();
    const timeout = setTimeout(() => abortController.abort(), timeoutMs);
    try {
      const response = await fetchImplementation(endpoint, {
        ...init,
        redirect: "error",
        signal: abortController.signal,
      });
      const responseBytes = await readBoundedBody(
        response,
        maximumResponseBytes,
      );
      if (!response.ok) {
        throw new MerchantAdapterError("upstream_rejected");
      }
      try {
        return JSON.parse(
          Buffer.from(responseBytes).toString("utf8"),
        ) as TResult;
      } catch {
        throw new MerchantAdapterError("invalid_response");
      }
    } catch (error) {
      if (error instanceof MerchantAdapterError) throw error;
      if (abortController.signal.aborted) {
        throw new MerchantAdapterError("timeout");
      }
      throw new MerchantAdapterError("upstream_unavailable");
    } finally {
      clearTimeout(timeout);
    }
  };
}

async function readBoundedBody(
  response: Response,
  maximumResponseBytes: number,
): Promise<Uint8Array> {
  const contentLength = response.headers.get("content-length");
  if (contentLength && Number(contentLength) > maximumResponseBytes) {
    throw new MerchantAdapterError("response_too_large");
  }
  if (!response.body) return new Uint8Array();

  const reader = response.body.getReader();
  const chunks: Uint8Array[] = [];
  let totalBytes = 0;
  while (true) {
    const { done, value } = await reader.read();
    if (done) break;
    totalBytes += value.byteLength;
    if (totalBytes > maximumResponseBytes) {
      await reader.cancel();
      throw new MerchantAdapterError("response_too_large");
    }
    chunks.push(value);
  }
  const body = new Uint8Array(totalBytes);
  let offset = 0;
  for (const chunk of chunks) {
    body.set(chunk, offset);
    offset += chunk.byteLength;
  }
  return body;
}

function requireHttpsUrl(rawUrl: string): URL {
  let url: URL;
  try {
    url = new URL(rawUrl);
  } catch {
    throw new MerchantAdapterError("invalid_configuration");
  }
  if (url.protocol !== "https:" || url.username || url.password) {
    throw new MerchantAdapterError("invalid_configuration");
  }
  return url;
}
