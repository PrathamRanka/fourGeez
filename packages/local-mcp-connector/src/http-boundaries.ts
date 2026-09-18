import {
  ConnectorAbortError,
  ConnectorConfigurationError,
  ConnectorHttpError,
  ConnectorMessageTooLargeError,
  ConnectorProtocolError,
  ConnectorTimeoutError,
} from "./errors.js";

export type FetchImplementation = (
  input: string | URL | Request,
  init?: RequestInit,
) => Promise<Response>;

export const DEFAULT_REQUEST_TIMEOUT_MS = 10_000;
export const DEFAULT_MAX_MESSAGE_BYTES = 1024 * 1024;
export const MAXIMUM_MCP_MESSAGE_BYTES = 1024 * 1024;
export const TOKEN_RESPONSE_LIMIT_BYTES = 64 * 1024;

export interface RequestSignal {
  signal: AbortSignal;
  timeoutSignal: AbortSignal;
}

export function createAgentPayEndpoint(baseUrl: string, path: string): URL {
  let endpoint: URL;
  try {
    endpoint = new URL(path, baseUrl.endsWith("/") ? baseUrl : `${baseUrl}/`);
  } catch {
    throw new ConnectorConfigurationError("AGENTPAY_API_BASE_URL is invalid");
  }
  if (endpoint.username || endpoint.password) {
    throw new ConnectorConfigurationError(
      "AgentPay API URL must not contain credentials",
    );
  }
  const loopbackHosts = new Set(["localhost", "127.0.0.1", "[::1]", "::1"]);
  if (
    endpoint.protocol !== "https:" &&
    !(endpoint.protocol === "http:" && loopbackHosts.has(endpoint.hostname))
  ) {
    throw new ConnectorConfigurationError(
      "AgentPay API URL must use HTTPS outside loopback development",
    );
  }
  return endpoint;
}

export function validatePositiveInteger(
  value: number,
  name: string,
  maximum = Number.MAX_SAFE_INTEGER,
): number {
  if (!Number.isSafeInteger(value) || value <= 0 || value > maximum) {
    throw new ConnectorConfigurationError(
      `${name} must be a positive integer no greater than ${maximum}`,
    );
  }
  return value;
}

export function createRequestSignal(
  timeoutMs: number,
  callerSignal?: AbortSignal,
): RequestSignal {
  const timeoutSignal = AbortSignal.timeout(timeoutMs);
  return {
    signal: callerSignal
      ? AbortSignal.any([callerSignal, timeoutSignal])
      : timeoutSignal,
    timeoutSignal,
  };
}

export function translateRequestFailure(
  error: unknown,
  callerSignal: AbortSignal | undefined,
  timeoutSignal: AbortSignal,
): never {
  if (callerSignal?.aborted) {
    throw new ConnectorAbortError();
  }
  if (timeoutSignal.aborted) {
    throw new ConnectorTimeoutError();
  }
  if (error instanceof Error && error.name === "AbortError") {
    throw new ConnectorAbortError();
  }
  throw new ConnectorProtocolError(
    "AgentPay cloud request could not be completed",
  );
}

export async function readBoundedText(
  response: Response,
  maximumBytes: number,
): Promise<string> {
  const declaredLength = response.headers.get("content-length");
  if (declaredLength !== null) {
    const parsedLength = Number.parseInt(declaredLength, 10);
    if (Number.isFinite(parsedLength) && parsedLength > maximumBytes) {
      await response.body?.cancel();
      throw new ConnectorMessageTooLargeError();
    }
  }

  if (!response.body) {
    return "";
  }

  const reader = response.body.getReader();
  const chunks: Uint8Array[] = [];
  let totalBytes = 0;
  try {
    while (true) {
      const { done, value } = await reader.read();
      if (done) {
        break;
      }
      totalBytes += value.byteLength;
      if (totalBytes > maximumBytes) {
        await reader.cancel();
        throw new ConnectorMessageTooLargeError();
      }
      chunks.push(value);
    }
  } finally {
    reader.releaseLock();
  }

  const body = new Uint8Array(totalBytes);
  let offset = 0;
  for (const chunk of chunks) {
    body.set(chunk, offset);
    offset += chunk.byteLength;
  }
  return new TextDecoder("utf-8", { fatal: true }).decode(body);
}

interface ErrorEnvelope {
  error?: {
    code?: unknown;
    requestId?: unknown;
  };
}

export async function createHttpError(
  response: Response,
  maximumBytes: number,
): Promise<ConnectorHttpError> {
  let code: string | undefined;
  let requestId = response.headers.get("x-request-id") ?? undefined;
  const contentType = response.headers.get("content-type")?.toLowerCase() ?? "";
  if (contentType.includes("application/json")) {
    try {
      const text = await readBoundedText(response, maximumBytes);
      const envelope = JSON.parse(text) as ErrorEnvelope;
      if (typeof envelope.error?.code === "string") {
        code = envelope.error.code;
      }
      if (typeof envelope.error?.requestId === "string") {
        requestId = envelope.error.requestId;
      }
    } catch {
      // Error bodies are optional metadata and are never echoed to the caller.
    }
  } else {
    await response.body?.cancel();
  }

  const retryAfterHeader = response.headers.get("retry-after");
  const retryAfter = retryAfterHeader
    ? Number.parseInt(retryAfterHeader, 10)
    : Number.NaN;
  return new ConnectorHttpError({
    status: response.status,
    code,
    requestId,
    retryAfterSeconds:
      Number.isInteger(retryAfter) && retryAfter >= 0 ? retryAfter : undefined,
  });
}
