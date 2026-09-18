import {
  ConnectorConfigurationError,
  ConnectorHttpError,
  ConnectorProtocolError,
} from "./errors.js";
import {
  createAgentPayEndpoint,
  createHttpError,
  createRequestSignal,
  DEFAULT_REQUEST_TIMEOUT_MS,
  type FetchImplementation,
  readBoundedText,
  TOKEN_RESPONSE_LIMIT_BYTES,
  translateRequestFailure,
  validatePositiveInteger,
} from "./http-boundaries.js";

const ACCESS_TOKEN_PATH = "/v1/integration-access-tokens";
const MCP_AUDIENCE = "urn:agentpay:mcp";
const MINIMUM_TOKEN_LIFETIME_SECONDS = 120;
const MAXIMUM_TOKEN_LIFETIME_SECONDS = 300;
const DEFAULT_SAFETY_WINDOW_MS = 30_000;
const ACCESS_TOKEN_MAXIMUM_LENGTH = 16_384;
const COMPACT_JWT_PATTERN = /^[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+$/u;
const PROJECT_KEY_PREFIX_PATTERN = /^apc[12]\./u;
const ALLOWED_SCOPES = new Set(["read", "configure", "publish", "validate"]);

export interface AccessTokenProvider {
  getAccessToken(signal?: AbortSignal): Promise<string>;
  invalidate(): void;
}

export interface AccessTokenClientOptions {
  baseUrl: string;
  projectKey: string;
  scopes: readonly string[];
  safetyWindowMs?: number;
  timeoutMs?: number;
  now?: () => number;
  fetch?: FetchImplementation;
}

interface CachedAccessToken {
  value: string;
  expiresAtMs: number;
}

interface IntegrationAccessTokenResponse {
  accessToken: string;
  tokenType: "Bearer";
  expiresIn: number;
  scope: string;
  sellerId: string;
  credentialId: string;
  entitlementEpoch: number;
}

export class AccessTokenClient implements AccessTokenProvider {
  readonly #exchangeUrl: URL;
  readonly #projectKey: string;
  readonly #scopes: readonly string[];
  readonly #safetyWindowMs: number;
  readonly #timeoutMs: number;
  readonly #now: () => number;
  readonly #fetch: FetchImplementation;
  #cachedAccessToken?: CachedAccessToken;
  #exchangeInFlight?: Promise<string>;

  constructor(options: AccessTokenClientOptions) {
    this.#exchangeUrl = createAgentPayEndpoint(
      options.baseUrl,
      ACCESS_TOKEN_PATH,
    );
    if (options.projectKey.length === 0) {
      throw new ConnectorConfigurationError("AGENTPAY_PROJECT_KEY is required");
    }
    this.#projectKey = options.projectKey;
    this.#scopes = validateScopes(options.scopes);
    this.#safetyWindowMs = validatePositiveInteger(
      options.safetyWindowMs ?? DEFAULT_SAFETY_WINDOW_MS,
      "safetyWindowMs",
    );
    this.#timeoutMs = validatePositiveInteger(
      options.timeoutMs ?? DEFAULT_REQUEST_TIMEOUT_MS,
      "timeoutMs",
    );
    this.#now = options.now ?? Date.now;
    this.#fetch = options.fetch ?? fetch;
  }

  async getAccessToken(signal?: AbortSignal): Promise<string> {
    if (
      this.#cachedAccessToken &&
      this.#now() < this.#cachedAccessToken.expiresAtMs - this.#safetyWindowMs
    ) {
      return this.#cachedAccessToken.value;
    }

    if (!this.#exchangeInFlight) {
      this.#exchangeInFlight = this.#exchange(signal).finally(() => {
        this.#exchangeInFlight = undefined;
      });
    }
    return this.#exchangeInFlight;
  }

  invalidate(): void {
    this.#cachedAccessToken = undefined;
  }

  async #exchange(callerSignal?: AbortSignal): Promise<string> {
    const requestSignal = createRequestSignal(this.#timeoutMs, callerSignal);
    let response: Response;
    try {
      response = await this.#fetch(this.#exchangeUrl.href, {
        method: "POST",
        headers: {
          Accept: "application/json",
          "Content-Type": "application/json",
          "X-AgentPay-Project-Key": this.#projectKey,
        },
        body: JSON.stringify({ audience: MCP_AUDIENCE, scopes: this.#scopes }),
        cache: "no-store",
        redirect: "error",
        signal: requestSignal.signal,
      });
    } catch (error) {
      translateRequestFailure(error, callerSignal, requestSignal.timeoutSignal);
    }

    if (!response.ok) {
      throw await createHttpError(response, TOKEN_RESPONSE_LIMIT_BYTES);
    }

    const contentType =
      response.headers.get("content-type")?.toLowerCase() ?? "";
    if (!contentType.includes("application/json")) {
      await response.body?.cancel();
      throw new ConnectorProtocolError();
    }

    let payload: unknown;
    try {
      payload = JSON.parse(
        await readBoundedText(response, TOKEN_RESPONSE_LIMIT_BYTES),
      );
    } catch (error) {
      if (error instanceof ConnectorProtocolError) {
        throw error;
      }
      throw new ConnectorProtocolError();
    }
    const accessToken = validateAccessTokenResponse(payload);
    if (
      accessToken.accessToken === this.#projectKey ||
      PROJECT_KEY_PREFIX_PATTERN.test(accessToken.accessToken)
    ) {
      throw new ConnectorProtocolError();
    }
    this.#cachedAccessToken = {
      value: accessToken.accessToken,
      expiresAtMs: this.#now() + accessToken.expiresIn * 1_000,
    };
    return accessToken.accessToken;
  }
}

function validateScopes(scopes: readonly string[]): readonly string[] {
  if (scopes.length === 0) {
    throw new ConnectorConfigurationError("At least one MCP scope is required");
  }
  const uniqueScopes = [...new Set(scopes)];
  if (
    uniqueScopes.length !== scopes.length ||
    uniqueScopes.some((scope) => !ALLOWED_SCOPES.has(scope))
  ) {
    throw new ConnectorConfigurationError("MCP scopes are invalid");
  }
  return Object.freeze(uniqueScopes);
}

function validateAccessTokenResponse(
  payload: unknown,
): IntegrationAccessTokenResponse {
  if (!isRecord(payload)) {
    throw new ConnectorProtocolError();
  }
  const response = payload as Partial<IntegrationAccessTokenResponse>;
  if (
    typeof response.accessToken !== "string" ||
    response.accessToken.length === 0 ||
    response.accessToken.length > ACCESS_TOKEN_MAXIMUM_LENGTH ||
    !COMPACT_JWT_PATTERN.test(response.accessToken) ||
    response.tokenType !== "Bearer" ||
    !Number.isInteger(response.expiresIn) ||
    response.expiresIn! < MINIMUM_TOKEN_LIFETIME_SECONDS ||
    response.expiresIn! > MAXIMUM_TOKEN_LIFETIME_SECONDS ||
    typeof response.scope !== "string" ||
    typeof response.sellerId !== "string" ||
    typeof response.credentialId !== "string" ||
    !Number.isInteger(response.entitlementEpoch) ||
    response.entitlementEpoch! < 1
  ) {
    throw new ConnectorProtocolError();
  }
  return response as IntegrationAccessTokenResponse;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

export { ConnectorHttpError } from "./errors.js";
