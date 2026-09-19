import type { JsonWebKey } from "node:crypto";

import {
  MemoryExecutionReplayStore,
  MemoryReplayStore,
  VerificationError,
  verifyAgentPayExecution,
  verifyAgentPayRequest,
  type ExecutionClaims,
  type ExecutionReplayStore,
  type ReplayStore,
} from "@agentpay/verify-node";

import type { MerchantAdapter } from "./adapters.js";
import { MerchantSdkError } from "./errors.js";
import {
  fulfillOnce,
  type FulfillmentOutcome,
  type FulfillmentStore,
} from "./fulfillment.js";
import { normalizeHeaders, type MerchantHeaders } from "./headers.js";

const DEFAULT_JWKS_MAXIMUM_BYTES = 64 * 1024;

export { MemoryExecutionReplayStore, MemoryReplayStore };
export type { ExecutionClaims, ExecutionReplayStore, ReplayStore };

export interface VerifyExecutionRequestOptions {
  issuer: string;
  sellerId: string;
  routeId: string;
  method: string;
  path: string;
  rawBody: Uint8Array;
  headers: MerchantHeaders;
  keyResolver: (keyId: string) => Promise<JsonWebKey>;
  replayStore: ExecutionReplayStore;
  now?: Date;
}

export async function verifyExecutionRequest(
  options: VerifyExecutionRequestOptions,
): Promise<ExecutionClaims> {
  try {
    return await verifyAgentPayExecution({
      issuer: options.issuer,
      sellerId: options.sellerId,
      routeId: options.routeId,
      request: {
        method: options.method,
        path: options.path,
        body: Buffer.from(options.rawBody),
        headers: normalizeHeaders(options.headers),
      },
      keyResolver: options.keyResolver,
      replayStore: options.replayStore,
      now: options.now,
    });
  } catch (error) {
    throw mapVerificationError(error);
  }
}

export interface VerifyLegacySandboxRequestOptions {
  secret: string | Uint8Array;
  method: string;
  path: string;
  rawBody: Uint8Array;
  headers: MerchantHeaders;
  replayStore: ReplayStore;
  now?: Date;
  maximumAgeMs?: number;
}

export async function verifyLegacySandboxRequest(
  options: VerifyLegacySandboxRequestOptions,
): Promise<void> {
  try {
    await verifyAgentPayRequest({
      secret:
        typeof options.secret === "string"
          ? options.secret
          : Buffer.from(options.secret),
      request: {
        method: options.method,
        path: options.path,
        body: Buffer.from(options.rawBody),
        headers: normalizeHeaders(options.headers),
      },
      replayStore: options.replayStore,
      now: options.now,
      maximumAgeMs: options.maximumAgeMs,
    });
  } catch (error) {
    throw mapVerificationError(error);
  }
}

export interface ProcessAgentPayFulfillmentOptions<TInput, TResult> {
  verification: VerifyExecutionRequestOptions;
  fulfillmentStore: FulfillmentStore<TResult>;
  parseInput: (rawBody: Uint8Array) => TInput;
  adapter: MerchantAdapter<TInput, TResult>;
}

export async function processAgentPayFulfillment<TInput, TResult>(
  options: ProcessAgentPayFulfillmentOptions<TInput, TResult>,
): Promise<FulfillmentOutcome<TResult>> {
  const execution = await verifyExecutionRequest(options.verification);
  let input: TInput;
  try {
    input = options.parseInput(options.verification.rawBody);
  } catch {
    throw new MerchantSdkError("invalid_event", 400);
  }
  return fulfillOnce({
    transactionId: execution.transactionId,
    store: options.fulfillmentStore,
    execute: () =>
      options.adapter.fulfill({
        transactionId: execution.transactionId,
        execution,
        input,
      }),
  });
}

export interface RemoteJwksResolverOptions {
  jwksUrl: string;
  fetch?: typeof fetch;
  now?: () => Date;
  maximumResponseBytes?: number;
}

export function createRemoteJwksResolver(
  options: RemoteJwksResolverOptions,
): (keyId: string) => Promise<JsonWebKey> {
  const jwksUrl = requireHttpsUrl(options.jwksUrl);
  const fetchImplementation = options.fetch ?? globalThis.fetch;
  const now = options.now ?? (() => new Date());
  const maximumResponseBytes =
    options.maximumResponseBytes ?? DEFAULT_JWKS_MAXIMUM_BYTES;
  let cachedKeys = new Map<string, JsonWebKey>();
  let expiresAt = 0;

  return async (keyId: string): Promise<JsonWebKey> => {
    const nowMs = now().getTime();
    if (nowMs < expiresAt && cachedKeys.has(keyId)) {
      return cachedKeys.get(keyId)!;
    }
    const refreshed = await fetchJwks(
      jwksUrl,
      fetchImplementation,
      maximumResponseBytes,
    );
    cachedKeys = refreshed.keys;
    expiresAt = nowMs + refreshed.maximumAgeMs;
    const key = cachedKeys.get(keyId);
    if (!key) {
      throw new MerchantSdkError("dependency_unavailable", 503);
    }
    return key;
  };
}

async function fetchJwks(
  jwksUrl: URL,
  fetchImplementation: typeof fetch,
  maximumResponseBytes: number,
): Promise<{ keys: Map<string, JsonWebKey>; maximumAgeMs: number }> {
  try {
    const response = await fetchImplementation(jwksUrl, {
      method: "GET",
      headers: { Accept: "application/json" },
      redirect: "error",
    });
    if (!response.ok) throw new Error("JWKS response was not successful");
    const bytes = new Uint8Array(await response.arrayBuffer());
    if (bytes.byteLength > maximumResponseBytes) {
      throw new Error("JWKS response exceeded its limit");
    }
    const document = JSON.parse(Buffer.from(bytes).toString("utf8")) as unknown;
    if (!isJwksDocument(document)) throw new Error("JWKS response was invalid");
    const keys = new Map<string, JsonWebKey>();
    for (const key of document.keys) {
      if (
        key.kty === "EC" &&
        key.crv === "P-256" &&
        key.alg === "ES256" &&
        typeof key.kid === "string" &&
        typeof key.x === "string" &&
        typeof key.y === "string" &&
        key.d === undefined
      ) {
        keys.set(key.kid, key);
      }
    }
    if (keys.size === 0) throw new Error("JWKS had no supported keys");
    return {
      keys,
      maximumAgeMs: cacheMaximumAgeMs(response.headers.get("cache-control")),
    };
  } catch (error) {
    if (error instanceof MerchantSdkError) throw error;
    throw new MerchantSdkError("dependency_unavailable", 503);
  }
}

function cacheMaximumAgeMs(cacheControl: string | null): number {
  if (!cacheControl || /(?:^|,)\s*no-store\b/i.test(cacheControl)) return 0;
  const match = /(?:^|,)\s*max-age=(\d+)\b/i.exec(cacheControl);
  return match ? Number(match[1]) * 1000 : 0;
}

function isJwksDocument(value: unknown): value is { keys: JsonWebKey[] } {
  return (
    typeof value === "object" &&
    value !== null &&
    Array.isArray((value as { keys?: unknown }).keys)
  );
}

function requireHttpsUrl(rawUrl: string): URL {
  let url: URL;
  try {
    url = new URL(rawUrl);
  } catch {
    throw new MerchantSdkError("invalid_configuration", 500);
  }
  if (url.protocol !== "https:" || url.username || url.password) {
    throw new MerchantSdkError("invalid_configuration", 500);
  }
  return url;
}

function mapVerificationError(error: unknown): MerchantSdkError {
  if (error instanceof MerchantSdkError) return error;
  if (!(error instanceof VerificationError)) {
    return new MerchantSdkError("dependency_unavailable", 503);
  }
  switch (error.code) {
    case "replay":
      return new MerchantSdkError("replay", 409);
    case "binding_mismatch":
      return new MerchantSdkError("binding_mismatch", 403);
    case "key_unavailable":
      return new MerchantSdkError("dependency_unavailable", 503);
    case "invalid_configuration":
      return new MerchantSdkError("invalid_configuration", 500);
    case "stale_request":
      return new MerchantSdkError("stale_request", 401);
    case "invalid_signature":
      return new MerchantSdkError("invalid_signature", 401);
    default:
      return new MerchantSdkError("invalid_capability", 401);
  }
}
