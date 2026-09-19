import { createHash, createHmac, timingSafeEqual } from "node:crypto";

import { MerchantSdkError } from "./errors.js";
import { normalizeHeaders, type MerchantHeaders } from "./headers.js";

const WEBHOOK_DOMAIN = "agentpay.webhook.v1";
const DEFAULT_MAXIMUM_AGE_MS = 5 * 60 * 1000;
const RFC3339_UTC = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z$/;
const BASE64 =
  /^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$/;
const EVENT_ID = /^evt_[A-Za-z0-9]+$/;
const SELLER_ID = /^sel_[A-Za-z0-9]+$/;

export type AgentPayWebhookEventType =
  | "payment.verified"
  | "fulfillment.succeeded"
  | "fulfillment.failed"
  | "dispute.changed";

const eventTypes = new Set<AgentPayWebhookEventType>([
  "payment.verified",
  "fulfillment.succeeded",
  "fulfillment.failed",
  "dispute.changed",
]);

export interface AgentPayWebhookEvent<
  TPayload extends Record<string, unknown> = Record<string, unknown>,
> {
  schemaVersion: "1" | "2";
  eventId: string;
  sellerId: string;
  eventType: AgentPayWebhookEventType;
  occurredAt: string;
  payload: TPayload;
}

export interface WebhookReplayStore {
  claim(eventId: string, expiresAt: Date): Promise<boolean>;
}

export class MemoryWebhookReplayStore implements WebhookReplayStore {
  readonly #eventIds = new Set<string>();

  async claim(eventId: string): Promise<boolean> {
    if (this.#eventIds.has(eventId)) return false;
    this.#eventIds.add(eventId);
    return true;
  }
}

export interface VerifyAgentPayWebhookOptions {
  secret: string | Uint8Array;
  rawBody: Uint8Array;
  headers: MerchantHeaders;
  replayStore: WebhookReplayStore;
  expectedSellerId: string;
  now?: Date;
  maximumAgeMs?: number;
}

export async function verifyAgentPayWebhook(
  options: VerifyAgentPayWebhookOptions,
): Promise<AgentPayWebhookEvent> {
  const secret =
    typeof options.secret === "string"
      ? Buffer.from(options.secret)
      : Buffer.from(options.secret);
  if (secret.length < 32) {
    throw new MerchantSdkError("invalid_configuration", 500);
  }
  if (!SELLER_ID.test(options.expectedSellerId)) {
    throw new MerchantSdkError("invalid_configuration", 500);
  }

  const headers = normalizeHeaders(options.headers);
  const eventId = headers["x-agentpay-webhook-id"];
  const timestampValue = headers["x-agentpay-webhook-timestamp"];
  const signatureValue = headers["x-agentpay-webhook-signature"];
  if (
    !eventId ||
    !EVENT_ID.test(eventId) ||
    !timestampValue ||
    !RFC3339_UTC.test(timestampValue) ||
    !signatureValue ||
    !BASE64.test(signatureValue)
  ) {
    throw new MerchantSdkError("invalid_signature", 401);
  }

  const timestamp = new Date(timestampValue);
  if (Number.isNaN(timestamp.getTime())) {
    throw new MerchantSdkError("invalid_signature", 401);
  }
  const now = options.now ?? new Date();
  const maximumAgeMs = options.maximumAgeMs ?? DEFAULT_MAXIMUM_AGE_MS;
  if (
    !Number.isFinite(maximumAgeMs) ||
    maximumAgeMs <= 0 ||
    Math.abs(now.getTime() - timestamp.getTime()) > maximumAgeMs
  ) {
    throw new MerchantSdkError("stale_request", 401);
  }

  const bodyHash = createHash("sha256")
    .update(Buffer.from(options.rawBody))
    .digest("hex");
  const canonical = [WEBHOOK_DOMAIN, eventId, timestampValue, bodyHash].join(
    "\n",
  );
  const expected = createHmac("sha256", secret).update(canonical).digest();
  const provided = Buffer.from(signatureValue, "base64");
  if (
    provided.length !== expected.length ||
    !timingSafeEqual(provided, expected)
  ) {
    throw new MerchantSdkError("invalid_signature", 401);
  }

  const event = parseAgentPayWebhookEvent(options.rawBody);
  if (
    event.eventId !== eventId ||
    event.sellerId !== options.expectedSellerId
  ) {
    throw new MerchantSdkError("binding_mismatch", 403);
  }

  try {
    if (
      !(await options.replayStore.claim(
        eventId,
        new Date(timestamp.getTime() + maximumAgeMs),
      ))
    ) {
      throw new MerchantSdkError("replay", 409);
    }
  } catch (error) {
    if (error instanceof MerchantSdkError) throw error;
    throw new MerchantSdkError("dependency_unavailable", 503);
  }
  return event;
}

export function parseAgentPayWebhookEvent(
  rawBody: Uint8Array,
): AgentPayWebhookEvent {
  let value: unknown;
  try {
    value = JSON.parse(Buffer.from(rawBody).toString("utf8"));
  } catch {
    throw new MerchantSdkError("invalid_event", 400);
  }
  if (!isPlainObject(value)) {
    throw new MerchantSdkError("invalid_event", 400);
  }
  const allowedKeys = new Set([
    "schemaVersion",
    "eventId",
    "sellerId",
    "eventType",
    "occurredAt",
    "payload",
  ]);
  if (Object.keys(value).some((key) => !allowedKeys.has(key))) {
    throw new MerchantSdkError("invalid_event", 400);
  }
  const schemaVersion = value.schemaVersion;
  const eventType = value.eventType;
  if (
    (schemaVersion !== "1" && schemaVersion !== "2") ||
    typeof value.eventId !== "string" ||
    !EVENT_ID.test(value.eventId) ||
    typeof value.sellerId !== "string" ||
    !SELLER_ID.test(value.sellerId) ||
    typeof eventType !== "string" ||
    !eventTypes.has(eventType as AgentPayWebhookEventType) ||
    typeof value.occurredAt !== "string" ||
    !RFC3339_UTC.test(value.occurredAt) ||
    Number.isNaN(new Date(value.occurredAt).getTime()) ||
    !isPlainObject(value.payload)
  ) {
    throw new MerchantSdkError("invalid_event", 400);
  }
  return {
    schemaVersion,
    eventId: value.eventId,
    sellerId: value.sellerId,
    eventType: eventType as AgentPayWebhookEventType,
    occurredAt: value.occurredAt,
    payload: value.payload,
  };
}

function isPlainObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
