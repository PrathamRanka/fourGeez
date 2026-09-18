import {
  ConnectorHttpError,
  ConnectorMessageTooLargeError,
  ConnectorProtocolError,
} from "./errors.js";
import {
  createAgentPayEndpoint,
  createHttpError,
  createRequestSignal,
  DEFAULT_MAX_MESSAGE_BYTES,
  DEFAULT_REQUEST_TIMEOUT_MS,
  type FetchImplementation,
  readBoundedText,
  MAXIMUM_MCP_MESSAGE_BYTES,
  translateRequestFailure,
  validatePositiveInteger,
} from "./http-boundaries.js";
import type { AccessTokenProvider } from "./token-client.js";

const MCP_PATH = "/mcp";

export interface CloudMcpTransportOptions {
  baseUrl: string;
  tokenClient: AccessTokenProvider;
  maxMessageBytes?: number;
  timeoutMs?: number;
  fetch?: FetchImplementation;
}

export interface McpTransport {
  send(message: string, signal?: AbortSignal): Promise<string | undefined>;
}

export class CloudMcpTransport implements McpTransport {
  readonly #mcpUrl: URL;
  readonly #tokenClient: AccessTokenProvider;
  readonly #maxMessageBytes: number;
  readonly #timeoutMs: number;
  readonly #fetch: FetchImplementation;

  constructor(options: CloudMcpTransportOptions) {
    this.#mcpUrl = createAgentPayEndpoint(options.baseUrl, MCP_PATH);
    this.#tokenClient = options.tokenClient;
    this.#maxMessageBytes = validatePositiveInteger(
      options.maxMessageBytes ?? DEFAULT_MAX_MESSAGE_BYTES,
      "maxMessageBytes",
      MAXIMUM_MCP_MESSAGE_BYTES,
    );
    this.#timeoutMs = validatePositiveInteger(
      options.timeoutMs ?? DEFAULT_REQUEST_TIMEOUT_MS,
      "timeoutMs",
    );
    this.#fetch = options.fetch ?? fetch;
  }

  async send(
    message: string,
    signal?: AbortSignal,
  ): Promise<string | undefined> {
    validateJsonRpcMessage(message, this.#maxMessageBytes);
    const firstResponse = await this.#sendOnce(message, signal);
    if (firstResponse.status !== 401) {
      return this.#readResponse(firstResponse);
    }

    await firstResponse.body?.cancel();
    this.#tokenClient.invalidate();
    const retryResponse = await this.#sendOnce(message, signal);
    return this.#readResponse(retryResponse);
  }

  async #sendOnce(
    message: string,
    callerSignal?: AbortSignal,
  ): Promise<Response> {
    const accessToken = await this.#tokenClient.getAccessToken(callerSignal);
    const requestSignal = createRequestSignal(this.#timeoutMs, callerSignal);
    try {
      return await this.#fetch(this.#mcpUrl.href, {
        method: "POST",
        headers: {
          Accept: "application/json, text/event-stream",
          Authorization: `Bearer ${accessToken}`,
          "Content-Type": "application/json",
        },
        body: message,
        cache: "no-store",
        redirect: "error",
        signal: requestSignal.signal,
      });
    } catch (error) {
      translateRequestFailure(error, callerSignal, requestSignal.timeoutSignal);
    }
  }

  async #readResponse(response: Response): Promise<string | undefined> {
    if (response.status === 204) {
      return undefined;
    }
    if (!response.ok) {
      throw await createHttpError(response, this.#maxMessageBytes);
    }

    const contentType =
      response.headers.get("content-type")?.toLowerCase() ?? "";
    const responseText = await readBoundedText(response, this.#maxMessageBytes);
    if (contentType.includes("application/json")) {
      return JSON.stringify(
        parseJsonRpcMessage(responseText, this.#maxMessageBytes),
      );
    }
    if (contentType.includes("text/event-stream")) {
      return decodeJsonRpcServerEvents(responseText);
    }
    throw new ConnectorProtocolError(
      "AgentPay returned an unsupported MCP content type",
    );
  }
}

function validateJsonRpcMessage(message: string, maximumBytes: number): void {
  parseJsonRpcMessage(message, maximumBytes);
}

function parseJsonRpcMessage(message: string, maximumBytes: number): unknown {
  if (Buffer.byteLength(message, "utf8") > maximumBytes) {
    throw new ConnectorMessageTooLargeError();
  }
  let parsed: unknown;
  try {
    parsed = JSON.parse(message);
  } catch {
    throw new ConnectorProtocolError("MCP message is not valid JSON");
  }
  if (typeof parsed !== "object" || parsed === null) {
    throw new ConnectorProtocolError(
      "MCP message must be a JSON object or batch",
    );
  }
  return parsed;
}

function decodeJsonRpcServerEvents(eventStream: string): string | undefined {
  const messages: string[] = [];
  for (const event of eventStream.split(/\r?\n\r?\n/u)) {
    const payload = event
      .split(/\r?\n/u)
      .filter((line) => line.startsWith("data:"))
      .map((line) => line.slice(5).trimStart())
      .join("\n");
    if (!payload) {
      continue;
    }
    let parsedPayload: unknown;
    try {
      parsedPayload = JSON.parse(payload);
    } catch {
      throw new ConnectorProtocolError(
        "AgentPay returned invalid MCP event data",
      );
    }
    messages.push(JSON.stringify(parsedPayload));
  }
  return messages.length === 0 ? undefined : messages.join("\n");
}

export { ConnectorHttpError } from "./errors.js";
