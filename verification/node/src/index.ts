import {
  createHash,
  createHmac,
  createPublicKey,
  timingSafeEqual,
  verify as verifySignature,
  type JsonWebKey,
} from "node:crypto";

const SIGNATURE_DOMAIN = "agentpay.seller-request.v1";
const DEFAULT_MAXIMUM_AGE_MS = 5 * 60 * 1000;
const EXECUTION_CAPABILITY_TYPE = "agentpay-execution+jwt";
const MINIMUM_EXECUTION_LIFETIME_SECONDS = 30;
const MAXIMUM_EXECUTION_LIFETIME_SECONDS = 60;
const MAXIMUM_EXECUTION_TOKEN_BYTES = 16 * 1024;

export interface ReplayStore {
  claim(transactionId: string, timestamp: Date): Promise<boolean>;
}

export interface VerificationRequest {
  method: string;
  path: string;
  body: Buffer;
  headers: Record<string, string | undefined>;
}

export interface VerificationOptions {
  secret: string | Buffer;
  request: VerificationRequest;
  replayStore: ReplayStore;
  now?: Date;
  maximumAgeMs?: number;
}

export interface MiddlewareOptions {
  secret: string | Buffer;
  replayStore: ReplayStore;
  now?: () => Date;
  maximumAgeMs?: number;
}

export interface NodeMiddlewareRequest {
  method: string;
  path: string;
  rawBody?: Buffer;
  headers: Record<string, string | undefined>;
}

export interface NodeMiddlewareResponse {
  status(code: number): NodeMiddlewareResponse;
  json(body: { error: string }): void;
}

export type NodeNext = () => void;

export class VerificationError extends Error {
  // Creates a stable machine-readable verification failure.
  constructor(public readonly code: string) {
    super(code);
    this.name = "VerificationError";
  }
}

export class MemoryReplayStore implements ReplayStore {
  readonly #transactions = new Set<string>();

  // Local/test only: claims a transaction once within the current process.
  async claim(transactionId: string): Promise<boolean> {
    if (this.#transactions.has(transactionId)) {
      return false;
    }
    this.#transactions.add(transactionId);
    return true;
  }
}

export interface ExecutionReplayStore {
  claim(jti: string, expiresAt: Date): Promise<boolean>;
}

export class MemoryExecutionReplayStore implements ExecutionReplayStore {
  readonly #identifiers = new Set<string>();

  // Local/test only: this does not coordinate multiple processes.
  async claim(jti: string): Promise<boolean> {
    if (this.#identifiers.has(jti)) {
      return false;
    }
    this.#identifiers.add(jti);
    return true;
  }
}

export interface ExecutionClaims {
  iss: string;
  aud: string;
  sub: string;
  sellerId: string;
  routeId: string;
  transactionId: string;
  method: string;
  path: string;
  bodySha256: string;
  paymentFinality: "finalized";
  jti: string;
  iat: number;
  exp: number;
}

export interface ExecutionVerificationOptions {
  issuer: string;
  sellerId: string;
  routeId: string;
  request: VerificationRequest;
  keyResolver: (keyId: string) => Promise<JsonWebKey>;
  replayStore: ExecutionReplayStore;
  now?: Date;
}

export async function verifyAgentPayExecution(
  options: ExecutionVerificationOptions,
): Promise<ExecutionClaims> {
  const rawToken = options.request.headers["x-agentpay-execution-capability"];
  const transactionHeader =
    options.request.headers["x-agentpay-transaction-id"];
  if (
    !rawToken ||
    !transactionHeader ||
    Buffer.byteLength(rawToken) > MAXIMUM_EXECUTION_TOKEN_BYTES
  ) {
    throw new VerificationError("invalid_capability");
  }
  const segments = rawToken.split(".");
  if (segments.length !== 3) {
    throw new VerificationError("invalid_capability");
  }
  let header: Record<string, unknown>;
  let claims: ExecutionClaims;
  let signature: Buffer;
  try {
    header = JSON.parse(
      Buffer.from(segments[0]!, "base64url").toString("utf8"),
    );
    claims = JSON.parse(
      Buffer.from(segments[1]!, "base64url").toString("utf8"),
    );
    signature = Buffer.from(segments[2]!, "base64url");
  } catch {
    throw new VerificationError("invalid_capability");
  }
  if (
    header.typ !== EXECUTION_CAPABILITY_TYPE ||
    header.alg !== "ES256" ||
    typeof header.kid !== "string" ||
    signature.length !== 64
  ) {
    throw new VerificationError("invalid_capability");
  }
  let publicKey: JsonWebKey;
  try {
    publicKey = await options.keyResolver(header.kid);
  } catch {
    throw new VerificationError("key_unavailable");
  }
  const signingInput = Buffer.from(`${segments[0]}.${segments[1]}`);
  let signatureValid = false;
  try {
    signatureValid = verifySignature(
      "sha256",
      signingInput,
      {
        key: createPublicKey({ key: publicKey, format: "jwk" }),
        dsaEncoding: "ieee-p1363",
      },
      signature,
    );
  } catch {
    throw new VerificationError("key_unavailable");
  }
  if (!signatureValid || !validExecutionClaims(claims)) {
    throw new VerificationError("invalid_capability");
  }
  const nowSeconds = Math.floor((options.now ?? new Date()).getTime() / 1000);
  if (
    claims.iss !== options.issuer.replace(/\/$/, "") ||
    claims.aud !== `urn:agentpay:seller:${options.sellerId}` ||
    claims.iat > nowSeconds ||
    nowSeconds >= claims.exp ||
    claims.exp - claims.iat < MINIMUM_EXECUTION_LIFETIME_SECONDS ||
    claims.exp - claims.iat > MAXIMUM_EXECUTION_LIFETIME_SECONDS
  ) {
    throw new VerificationError("invalid_capability");
  }
  const bodyHash = createHash("sha256")
    .update(options.request.body)
    .digest("hex");
  if (
    claims.sub !== transactionHeader ||
    claims.transactionId !== transactionHeader ||
    claims.sellerId !== options.sellerId ||
    claims.routeId !== options.routeId ||
    claims.method !== options.request.method.toUpperCase() ||
    claims.path !== options.request.path ||
    !safeTextEqual(claims.bodySha256, bodyHash) ||
    claims.paymentFinality !== "finalized"
  ) {
    throw new VerificationError("binding_mismatch");
  }
  if (
    !(await options.replayStore.claim(claims.jti, new Date(claims.exp * 1000)))
  ) {
    throw new VerificationError("replay");
  }
  return claims;
}

function validExecutionClaims(value: unknown): value is ExecutionClaims {
  if (typeof value !== "object" || value === null) {
    return false;
  }
  const claims = value as Record<string, unknown>;
  const strings = [
    "iss",
    "aud",
    "sub",
    "sellerId",
    "routeId",
    "transactionId",
    "method",
    "path",
    "bodySha256",
    "paymentFinality",
    "jti",
  ];
  return (
    strings.every(
      (name) => typeof claims[name] === "string" && claims[name] !== "",
    ) &&
    typeof claims.iat === "number" &&
    Number.isInteger(claims.iat) &&
    typeof claims.exp === "number" &&
    Number.isInteger(claims.exp) &&
    /^xec_[A-Za-z0-9_-]+$/.test(String(claims.jti)) &&
    /^[a-f0-9]{64}$/.test(String(claims.bodySha256))
  );
}

function safeTextEqual(left: string, right: string): boolean {
  const leftBytes = Buffer.from(left);
  const rightBytes = Buffer.from(right);
  return (
    leftBytes.length === rightBytes.length &&
    timingSafeEqual(leftBytes, rightBytes)
  );
}

// Verifies one raw Node.js request before seller fulfillment.
export async function verifyAgentPayRequest(
  options: VerificationOptions,
): Promise<void> {
  const secret = Buffer.isBuffer(options.secret)
    ? options.secret
    : Buffer.from(options.secret);
  if (secret.length < 32) {
    throw new VerificationError("invalid_configuration");
  }
  const signatureValue = options.request.headers["x-agentpay-signature"];
  const timestampValue = options.request.headers["x-agentpay-timestamp"];
  const transactionId = options.request.headers["x-agentpay-transaction-id"];
  if (!signatureValue || !timestampValue || !transactionId) {
    throw new VerificationError("invalid_signature");
  }
  const timestamp = new Date(timestampValue);
  if (Number.isNaN(timestamp.getTime())) {
    throw new VerificationError("invalid_signature");
  }
  const now = options.now ?? new Date();
  const maximumAgeMs = options.maximumAgeMs ?? DEFAULT_MAXIMUM_AGE_MS;
  if (Math.abs(now.getTime() - timestamp.getTime()) > maximumAgeMs) {
    throw new VerificationError("stale_request");
  }
  const bodyHash = createHash("sha256")
    .update(options.request.body)
    .digest("hex");
  const canonical = [
    SIGNATURE_DOMAIN,
    timestampValue,
    options.request.method.toUpperCase(),
    options.request.path,
    bodyHash,
    transactionId,
  ].join("\n");
  const expected = createHmac("sha256", secret).update(canonical).digest();
  let provided: Buffer;
  try {
    provided = Buffer.from(signatureValue, "base64");
  } catch {
    throw new VerificationError("invalid_signature");
  }
  if (
    provided.length !== expected.length ||
    !timingSafeEqual(provided, expected)
  ) {
    throw new VerificationError("invalid_signature");
  }
  if (!(await options.replayStore.claim(transactionId, timestamp))) {
    throw new VerificationError("replay");
  }
}

// Creates Express-compatible middleware that requires the exact raw body.
export function createAgentPayMiddleware(options: MiddlewareOptions) {
  return async function agentPayMiddleware(
    request: NodeMiddlewareRequest,
    response: NodeMiddlewareResponse,
    next: NodeNext,
  ): Promise<void> {
    if (!Buffer.isBuffer(request.rawBody)) {
      response.status(401).json({ error: "AgentPay verification failed" });
      return;
    }
    try {
      await verifyAgentPayRequest({
        secret: options.secret,
        request: {
          method: request.method,
          path: request.path,
          body: request.rawBody,
          headers: request.headers,
        },
        replayStore: options.replayStore,
        now: options.now?.(),
        maximumAgeMs: options.maximumAgeMs,
      });
      next();
    } catch (error) {
      let status = 503;
      if (error instanceof VerificationError) {
        status = error.code === "replay" ? 409 : 401;
      }
      response.status(status).json({ error: "AgentPay verification failed" });
    }
  };
}
