import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import {
  createHttpsMerchantAdapter,
  createShopifyAdminAdapter,
  createWooCommerceAdapter,
  MerchantAdapterError,
} from "../dist/index.js";

const execution = {
  iss: "https://api.agentpay.test",
  aud: "urn:agentpay:seller:sel_test",
  sub: "txn_test",
  sellerId: "sel_test",
  routeId: "rte_test",
  transactionId: "txn_test",
  method: "POST",
  path: "/fulfill",
  bodySha256: "a".repeat(64),
  paymentFinality: "finalized",
  jti: "xec_test",
  iat: 1,
  exp: 46,
};

const shopifyOperation = readFixture("shopify-operation.json");
const wooCommerceOrder = readFixture("woocommerce-order.json");

test("generic HTTPS adapter sends bounded JSON with the transaction id", async () => {
  const requests = [];
  const adapter = createHttpsMerchantAdapter({
    endpoint: "https://merchant.example/internal/fulfill",
    fetch: recordingFetch(requests, { deliveryId: "del_1" }),
  });
  const result = await adapter.fulfill(context({ sku: "report" }));

  assert.deepEqual(result, { deliveryId: "del_1" });
  assert.equal(requests[0].url, "https://merchant.example/internal/fulfill");
  assert.equal(requests[0].init.headers["Idempotency-Key"], "txn_test");
});

test("Shopify adapter keeps the token in a server-only header", async () => {
  const requests = [];
  const adapter = createShopifyAdminAdapter({
    shopDomain: "seller-shop.myshopify.com",
    apiVersion: "2026-07",
    accessToken: "shopify-server-token",
    fetch: recordingFetch(requests, {
      data: { orderCreate: { order: { id: "gid://order/1" }, userErrors: [] } },
    }),
  });
  const result = await adapter.fulfill(context(shopifyOperation));

  assert.equal(result.data.orderCreate.order.id, "gid://order/1");
  assert.equal(
    requests[0].url,
    "https://seller-shop.myshopify.com/admin/api/2026-07/graphql.json",
  );
  assert.equal(
    requests[0].init.headers["X-Shopify-Access-Token"],
    "shopify-server-token",
  );
  assert.ok(!requests[0].init.body.includes("shopify-server-token"));
});

test("WooCommerce adapter uses server-side basic authentication and creates an order", async () => {
  const requests = [];
  const adapter = createWooCommerceAdapter({
    storeOrigin: "https://shop.example",
    consumerKey: "ck_server_only",
    consumerSecret: "cs_server_only",
    fetch: recordingFetch(requests, { id: 42, status: "processing" }),
  });
  const result = await adapter.fulfill(context(wooCommerceOrder));

  assert.equal(result.id, 42);
  assert.equal(requests[0].url, "https://shop.example/wp-json/wc/v3/orders");
  assert.equal(
    requests[0].init.headers.Authorization,
    `Basic ${Buffer.from("ck_server_only:cs_server_only").toString("base64")}`,
  );
  assert.ok(!requests[0].init.body.includes("cs_server_only"));
});

test("adapters reject insecure origins and bounded response failures", async () => {
  assert.throws(
    () =>
      createHttpsMerchantAdapter({
        endpoint: "http://merchant.example/fulfill",
      }),
    (error) =>
      error instanceof MerchantAdapterError &&
      error.code === "invalid_configuration",
  );

  const adapter = createHttpsMerchantAdapter({
    endpoint: "https://merchant.example/fulfill",
    maximumResponseBytes: 8,
    fetch: async () => new Response("response-too-large", { status: 200 }),
  });
  await assert.rejects(
    adapter.fulfill(context({})),
    (error) =>
      error instanceof MerchantAdapterError &&
      error.code === "response_too_large",
  );
});

function context(input) {
  return { transactionId: "txn_test", execution, input };
}

function recordingFetch(requests, responseBody) {
  return async (url, init) => {
    requests.push({ url: String(url), init });
    return new Response(JSON.stringify(responseBody), {
      status: 200,
      headers: { "content-type": "application/json" },
    });
  };
}

function readFixture(name) {
  return JSON.parse(
    readFileSync(new URL(`./fixtures/${name}`, import.meta.url), "utf8"),
  );
}
