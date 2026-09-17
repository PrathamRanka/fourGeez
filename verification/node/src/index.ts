import { createHash, createHmac, timingSafeEqual } from "node:crypto";

const SIGNATURE_DOMAIN = "agentpay.seller-request.v1";
const DEFAULT_MAXIMUM_AGE_MS = 5 * 60 * 1000;

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

  // Claims a transaction once within the current process.
  async claim(transactionId: string): Promise<boolean> {
    if (this.#transactions.has(transactionId)) {
      return false;
    }
    this.#transactions.add(transactionId);
    return true;
  }
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
  const bodyHash = createHash("sha256").update(options.request.body).digest("hex");
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
  if (provided.length !== expected.length || !timingSafeEqual(provided, expected)) {
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
