import { createHash, createHmac, timingSafeEqual } from "node:crypto";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

export function verifyWebhookSignature({
  body,
  eventId,
  timestamp,
  signature,
  secret,
  now = new Date(),
  maximumAgeSeconds = 300,
}) {
  if (!Buffer.isBuffer(body) || typeof secret !== "string" || secret.length < 32) {
    return false;
  }
  const deliveredAt = new Date(timestamp);
  if (
    Number.isNaN(deliveredAt.valueOf()) ||
    Math.abs(now.valueOf() - deliveredAt.valueOf()) > maximumAgeSeconds * 1000
  ) {
    return false;
  }
  const bodyHash = createHash("sha256").update(body).digest("hex");
  const expected = createHmac("sha256", secret)
    .update(`agentpay.webhook.v1\n${eventId}\n${timestamp}\n${bodyHash}`)
    .digest();
  let supplied;
  try {
    supplied = Buffer.from(signature, "base64");
  } catch {
    return false;
  }
  return supplied.length === expected.length && timingSafeEqual(supplied, expected);
}

export function classifyDeliveryHistory(deliveries) {
  return deliveries.reduce(
    (summary, delivery) => {
      if (delivery.status === "delivered") summary.delivered += 1;
      if (delivery.status === "retry_scheduled") summary.retryScheduled += 1;
      if (delivery.status === "dead_letter") summary.deadLetter += 1;
      summary.maximumAttempts = Math.max(summary.maximumAttempts, Number(delivery.attemptCount || 0));
      return summary;
    },
    { delivered: 0, retryScheduled: 0, deadLetter: 0, maximumAttempts: 0 },
  );
}

function requiredEnvironment(name) {
  const value = process.env[name]?.trim();
  if (!value) throw new Error(`${name} is required.`);
  return value;
}

function runVerify() {
  const bodyPath = process.argv[3];
  if (!bodyPath) throw new Error("Usage: webhook-canary.mjs verify <raw-body-file>");
  const valid = verifyWebhookSignature({
    body: readFileSync(bodyPath),
    eventId: requiredEnvironment("AGENTPAY_WEBHOOK_ID"),
    timestamp: requiredEnvironment("AGENTPAY_WEBHOOK_TIMESTAMP"),
    signature: requiredEnvironment("AGENTPAY_WEBHOOK_SIGNATURE"),
    secret: requiredEnvironment("AGENTPAY_WEBHOOK_SECRET"),
  });
  process.stdout.write(`${valid ? "valid" : "invalid"}\n`);
  if (!valid) process.exitCode = 1;
}

async function runHistory() {
  const apiOrigin = new URL(requiredEnvironment("AGENTPAY_API_ORIGIN"));
  if (apiOrigin.protocol !== "https:") throw new Error("AGENTPAY_API_ORIGIN must use HTTPS.");
  const sellerID = requiredEnvironment("AGENTPAY_SELLER_ID");
  const bearer = requiredEnvironment("AGENTPAY_SELLER_BEARER");
  const response = await fetch(
    new URL(`/v1/sellers/${encodeURIComponent(sellerID)}/webhook-deliveries`, apiOrigin),
    { headers: { Authorization: `Bearer ${bearer}` } },
  );
  if (!response.ok) throw new Error(`Delivery history request failed with HTTP ${response.status}.`);
  const payload = await response.json();
  process.stdout.write(`${JSON.stringify(classifyDeliveryHistory(payload.items || []), null, 2)}\n`);
}

async function run() {
  const command = process.argv[2];
  if (command === "verify") return runVerify();
  if (command === "history") return runHistory();
  throw new Error("Usage: webhook-canary.mjs <verify|history> [raw-body-file]");
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  run().catch((error) => {
    process.stderr.write(`${error.message}\n`);
    process.exitCode = 1;
  });
}
