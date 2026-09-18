import "server-only";

const maximumResponseBytes = 1_048_576;
const requestTimeoutMilliseconds = 10_000;

type APIErrorResponse = {
  error?: {
    message?: string;
  };
};

export type ActionResult<Value> =
  | { ok: true; value: Value }
  | { ok: false; error: string };

type AgentPayRequest = {
  body?: unknown;
  method: "GET" | "PATCH" | "POST";
};

export type AgentPayFile = {
  body: string;
  contentType: string;
};

// getAPIErrorMessage extracts only the documented public API error shape.
function getAPIErrorMessage(responseBody: unknown): string | null {
  if (typeof responseBody !== "object" || responseBody === null) {
    return null;
  }

  const apiError = responseBody as APIErrorResponse;
  return typeof apiError.error?.message === "string"
    ? apiError.error.message
    : null;
}

// getAPIConfiguration returns server-only credentials without serializing them to React.
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

// readBoundedResponse prevents an upstream response from exhausting the web process.
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

// requestAgentPay invokes one authenticated backend operation with bounded I/O.
export async function requestAgentPay<Value>(
  path: string,
  request: AgentPayRequest,
): Promise<ActionResult<Value>> {
  const configuration = getAPIConfiguration();
  if (!configuration.ok) {
    return configuration;
  }

  try {
    const isMutation = request.method !== "GET";
    const response = await fetch(`${configuration.value.apiOrigin}${path}`, {
      method: request.method,
      headers: {
        Authorization: `Bearer ${configuration.value.sellerToken}`,
        ...(request.body === undefined
          ? {}
          : { "Content-Type": "application/json" }),
        ...(isMutation ? { "Idempotency-Key": crypto.randomUUID() } : {}),
      },
      body:
        request.body === undefined ? undefined : JSON.stringify(request.body),
      cache: "no-store",
      signal: AbortSignal.timeout(requestTimeoutMilliseconds),
    });
    const responseText = await readBoundedResponse(response);
    const responseBody: unknown = responseText ? JSON.parse(responseText) : {};

    if (!response.ok) {
      return {
        ok: false,
        error:
          getAPIErrorMessage(responseBody) ??
          "AgentPay could not complete this request.",
      };
    }

    return { ok: true, value: responseBody as Value };
  } catch {
    return {
      ok: false,
      error: "AgentPay API is unavailable. Check the API and try again.",
    };
  }
}

// downloadAgentPayFile returns one bounded authenticated file response.
export async function downloadAgentPayFile(
  path: string,
): Promise<ActionResult<AgentPayFile>> {
  const configuration = getAPIConfiguration();
  if (!configuration.ok) {
    return configuration;
  }
  try {
    const response = await fetch(`${configuration.value.apiOrigin}${path}`, {
      method: "GET",
      headers: {
        Authorization: `Bearer ${configuration.value.sellerToken}`,
      },
      cache: "no-store",
      signal: AbortSignal.timeout(requestTimeoutMilliseconds),
    });
    const body = await readBoundedResponse(response);
    if (!response.ok) {
      let responseBody: unknown = {};
      try {
        responseBody = body ? JSON.parse(body) : {};
      } catch {
        responseBody = {};
      }
      return {
        ok: false,
        error:
          getAPIErrorMessage(responseBody) ??
          "AgentPay could not prepare this download.",
      };
    }
    return {
      ok: true,
      value: {
        body,
        contentType:
          response.headers.get("Content-Type") ?? "application/octet-stream",
      },
    };
  } catch {
    return {
      ok: false,
      error: "AgentPay API is unavailable. Check the API and try again.",
    };
  }
}

// requestPublicAgentPay reads one bounded unauthenticated storefront response.
export async function requestPublicAgentPay<Value>(
  path: string,
): Promise<ActionResult<Value>> {
  const apiOrigin = process.env.AGENTPAY_API_ORIGIN ?? "http://localhost:8080";
  try {
    const response = await fetch(`${apiOrigin}${path}`, {
      method: "GET",
      cache: "no-store",
      signal: AbortSignal.timeout(requestTimeoutMilliseconds),
    });
    const responseText = await readBoundedResponse(response);
    const responseBody: unknown = responseText ? JSON.parse(responseText) : {};
    if (!response.ok) {
      return {
        ok: false,
        error:
          getAPIErrorMessage(responseBody) ?? "This storefront is unavailable.",
      };
    }
    return { ok: true, value: responseBody as Value };
  } catch {
    return { ok: false, error: "This storefront is unavailable." };
  }
}

// requestPublicAgentPayText reads one bounded public discovery document.
export async function requestPublicAgentPayText(
  path: string,
): Promise<ActionResult<string>> {
  const apiOrigin = process.env.AGENTPAY_API_ORIGIN ?? "http://localhost:8080";
  try {
    const response = await fetch(`${apiOrigin}${path}`, {
      method: "GET",
      cache: "no-store",
      signal: AbortSignal.timeout(requestTimeoutMilliseconds),
    });
    const body = await readBoundedResponse(response);
    return response.ok
      ? { ok: true, value: body }
      : { ok: false, error: "This discovery document is unavailable." };
  } catch {
    return { ok: false, error: "This discovery document is unavailable." };
  }
}
