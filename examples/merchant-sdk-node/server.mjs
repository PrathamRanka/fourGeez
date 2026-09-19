import { createServer } from "node:http";
import {
  createHttpsMerchantAdapter,
  createRemoteJwksResolver,
  MemoryExecutionReplayStore,
  MemoryFulfillmentStore,
  MemoryWebhookReplayStore,
  MerchantSdkError,
  processAgentPayFulfillment,
  verifyAgentPayWebhook,
} from "../../packages/merchant-sdk/dist/index.js";

const apiOrigin = requiredEnvironment("AGENTPAY_API_ORIGIN");
const sellerId = requiredEnvironment("AGENTPAY_SELLER_ID");
const routeId = requiredEnvironment("AGENTPAY_ROUTE_ID");
const webhookSecret = requiredEnvironment("AGENTPAY_WEBHOOK_SECRET");
const adapter = createHttpsMerchantAdapter({
  endpoint: requiredEnvironment("MERCHANT_FULFILLMENT_URL"),
});
const keyResolver = createRemoteJwksResolver({
  jwksUrl: `${apiOrigin.replace(/\/$/, "")}/.well-known/jwks.json`,
});

// Local example only. Production deployments require shared atomic stores.
const executionReplayStore = new MemoryExecutionReplayStore();
const fulfillmentStore = new MemoryFulfillmentStore();
const webhookReplayStore = new MemoryWebhookReplayStore();

createServer(async (request, response) => {
  try {
    const rawBody = await readRawBody(request, 1024 * 1024);
    const headers = normalizeNodeHeaders(request.headers);

    if (request.url === "/webhooks/agentpay") {
      const event = await verifyAgentPayWebhook({
        secret: webhookSecret,
        rawBody,
        headers,
        replayStore: webhookReplayStore,
        expectedSellerId: sellerId,
      });
      response.writeHead(204, { "Cache-Control": "no-store" });
      response.end();
      process.stdout.write(`accepted ${event.eventType} ${event.eventId}\n`);
      return;
    }

    if (request.url === "/fulfill" && request.method === "POST") {
      const outcome = await processAgentPayFulfillment({
        verification: {
          issuer: apiOrigin,
          sellerId,
          routeId,
          method: request.method,
          path: "/fulfill",
          rawBody,
          headers,
          keyResolver,
          replayStore: executionReplayStore,
        },
        fulfillmentStore,
        parseInput: (body) => JSON.parse(Buffer.from(body).toString("utf8")),
        adapter,
      });
      response.writeHead(200, {
        "Cache-Control": "no-store",
        "Content-Type": "application/json",
      });
      response.end(JSON.stringify(outcome));
      return;
    }

    response.writeHead(404).end();
  } catch (error) {
    const status = error instanceof MerchantSdkError ? error.status : 500;
    response.writeHead(status, {
      "Cache-Control": "no-store",
      "Content-Type": "application/json",
    });
    response.end(JSON.stringify({ error: "AgentPay request failed" }));
  }
}).listen(3001, "127.0.0.1", () => {
  process.stdout.write("merchant example listening on http://127.0.0.1:3001\n");
});

async function readRawBody(request, maximumBytes) {
  const chunks = [];
  let total = 0;
  for await (const chunk of request) {
    total += chunk.length;
    if (total > maximumBytes) throw new Error("request too large");
    chunks.push(chunk);
  }
  return Buffer.concat(chunks);
}

function normalizeNodeHeaders(headers) {
  return Object.fromEntries(
    Object.entries(headers).map(([name, value]) => [
      name,
      Array.isArray(value) ? value.join(", ") : value,
    ]),
  );
}

function requiredEnvironment(name) {
  const value = process.env[name]?.trim();
  if (!value) throw new Error(`${name} is required`);
  return value;
}
