import assert from "node:assert/strict";
import { createHash, createHmac } from "node:crypto";
import { test } from "node:test";

import {
  classifyDeliveryHistory,
  verifyWebhookSignature,
} from "./webhook-canary.mjs";

test("webhook canary verifies the exact raw body without exposing the secret", () => {
  const secret = "s".repeat(32);
  const eventId = "evt_01K5J8QX6T7Z8Y9W0A1B2C3D4E";
  const timestamp = "2026-09-19T12:00:00Z";
  const body = Buffer.from('{"schemaVersion":"1","payload":{"ok":true}}');
  const bodyHash = createHash("sha256").update(body).digest("hex");
  const signature = createHmac("sha256", secret)
    .update(`agentpay.webhook.v1\n${eventId}\n${timestamp}\n${bodyHash}`)
    .digest("base64");

  assert.equal(
    verifyWebhookSignature({
      body,
      eventId,
      timestamp,
      signature,
      secret,
      now: new Date("2026-09-19T12:01:00Z"),
    }),
    true,
  );
  assert.equal(
    verifyWebhookSignature({
      body,
      eventId,
      timestamp,
      signature,
      secret,
      now: new Date("2026-09-19T12:10:01Z"),
    }),
    false,
  );
});

test("webhook canary recognizes retry and dead-letter evidence", () => {
  const result = classifyDeliveryHistory([
    { status: "retry_scheduled", attemptCount: 2 },
    { status: "dead_letter", attemptCount: 5 },
    { status: "delivered", attemptCount: 1 },
  ]);

  assert.deepEqual(result, {
    delivered: 1,
    retryScheduled: 1,
    deadLetter: 1,
    maximumAttempts: 5,
  });
});
