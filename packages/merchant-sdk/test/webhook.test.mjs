import assert from "node:assert/strict";
import { createHash, createHmac } from "node:crypto";
import { readFileSync } from "node:fs";
import test from "node:test";
import {
  MemoryWebhookReplayStore,
  MerchantSdkError,
  parseAgentPayWebhookEvent,
  verifyAgentPayWebhook,
} from "../dist/index.js";

const secret = "0123456789abcdef0123456789abcdef";
const now = new Date("2026-09-19T12:00:00Z");
const timestamp = "2026-09-19T12:00:00Z";
const eventId = "evt_01K5D09YJ0C0M7RJM4FWQ0K9H7";
const body = Buffer.from(
  JSON.stringify(
    JSON.parse(
      readFileSync(
        new URL("./fixtures/webhook-v2.json", import.meta.url),
        "utf8",
      ),
    ),
  ),
);

test("verifies exact webhook bytes, parses the envelope, and blocks replay", async () => {
  const replayStore = new MemoryWebhookReplayStore();
  const options = {
    secret,
    rawBody: body,
    headers: signedHeaders(body),
    replayStore,
    now,
  };

  await verifyAgentPayWebhook(options);
  const event = parseAgentPayWebhookEvent(body);
  assert.equal(event.eventType, "fulfillment.succeeded");
  await assert.rejects(verifyAgentPayWebhook(options), (error) => {
    assert.ok(error instanceof MerchantSdkError);
    assert.equal(error.code, "replay");
    assert.equal(error.status, 409);
    return true;
  });
});

test("rejects modified, stale, and malformed webhook input", async (t) => {
  await t.test("modified body", async () => {
    await assert.rejects(
      verifyAgentPayWebhook({
        secret,
        rawBody: Buffer.from(`${body.toString("utf8")} `),
        headers: signedHeaders(body),
        replayStore: new MemoryWebhookReplayStore(),
        now,
      }),
      (error) =>
        error instanceof MerchantSdkError && error.code === "invalid_signature",
    );
  });

  await t.test("stale timestamp", async () => {
    const staleBody = Buffer.from("{}");
    const staleTimestamp = "2026-09-19T11:00:00Z";
    await assert.rejects(
      verifyAgentPayWebhook({
        secret,
        rawBody: staleBody,
        headers: signedHeaders(staleBody, staleTimestamp),
        replayStore: new MemoryWebhookReplayStore(),
        now,
      }),
      (error) =>
        error instanceof MerchantSdkError && error.code === "stale_request",
    );
  });

  await t.test("unknown event type", () => {
    assert.throws(
      () =>
        parseAgentPayWebhookEvent(
          Buffer.from(
            JSON.stringify({
              schemaVersion: "2",
              eventId,
              sellerId: "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7",
              eventType: "merchant.deleted",
              occurredAt: timestamp,
              payload: {},
            }),
          ),
        ),
      (error) =>
        error instanceof MerchantSdkError && error.code === "invalid_event",
    );
  });
});

test("fails closed when the webhook replay store is unavailable", async () => {
  await assert.rejects(
    verifyAgentPayWebhook({
      secret,
      rawBody: body,
      headers: signedHeaders(body),
      replayStore: {
        async claim() {
          throw new Error("database unavailable");
        },
      },
      now,
    }),
    (error) =>
      error instanceof MerchantSdkError &&
      error.code === "dependency_unavailable" &&
      error.status === 503,
  );
});

function signedHeaders(signedBody, signedTimestamp = timestamp) {
  const digest = createHash("sha256").update(signedBody).digest("hex");
  const canonical = [
    "agentpay.webhook.v1",
    eventId,
    signedTimestamp,
    digest,
  ].join("\n");
  return {
    "X-AgentPay-Webhook-Id": eventId,
    "X-AgentPay-Webhook-Timestamp": signedTimestamp,
    "X-AgentPay-Webhook-Signature": createHmac("sha256", secret)
      .update(canonical)
      .digest("base64"),
  };
}
