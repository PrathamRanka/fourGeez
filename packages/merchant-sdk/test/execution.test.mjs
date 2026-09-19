import assert from "node:assert/strict";
import {
  createHash,
  generateKeyPairSync,
  sign as signBytes,
} from "node:crypto";
import test from "node:test";
import {
  createRemoteJwksResolver,
  MemoryFulfillmentStore,
  MemoryExecutionReplayStore,
  MemoryReplayStore,
  MerchantSdkError,
  processAgentPayFulfillment,
  verifyExecutionRequest,
  verifyLegacySandboxRequest,
} from "../dist/index.js";

const now = new Date("2026-09-19T12:00:00Z");

test("verifies a production execution request and blocks capability replay", async () => {
  const { privateKey, publicKey } = generateKeyPairSync("ec", {
    namedCurve: "P-256",
  });
  const body = Buffer.from('{"report":"weekly"}');
  const token = signExecutionCapability({ privateKey, body });
  const options = {
    issuer: "https://api.agentpay.test/",
    sellerId: "sel_test",
    routeId: "rte_test",
    method: "POST",
    path: "/fulfill",
    rawBody: body,
    headers: {
      "X-AgentPay-Execution-Capability": token,
      "X-AgentPay-Transaction-Id": "txn_test",
    },
    keyResolver: async () => publicKey.export({ format: "jwk" }),
    replayStore: new MemoryExecutionReplayStore(),
    now,
  };

  const claims = await verifyExecutionRequest(options);
  assert.equal(claims.transactionId, "txn_test");
  await assert.rejects(verifyExecutionRequest(options), (error) => {
    assert.ok(error instanceof MerchantSdkError);
    assert.equal(error.code, "replay");
    assert.equal(error.status, 409);
    return true;
  });
});

test("rejects execution requests whose raw body binding changed", async () => {
  const { privateKey, publicKey } = generateKeyPairSync("ec", {
    namedCurve: "P-256",
  });
  const signedBody = Buffer.from('{"report":"weekly"}');

  await assert.rejects(
    verifyExecutionRequest({
      issuer: "https://api.agentpay.test",
      sellerId: "sel_test",
      routeId: "rte_test",
      method: "POST",
      path: "/fulfill",
      rawBody: Buffer.from('{"report":"changed"}'),
      headers: {
        "x-agentpay-execution-capability": signExecutionCapability({
          privateKey,
          body: signedBody,
        }),
        "x-agentpay-transaction-id": "txn_test",
      },
      keyResolver: async () => publicKey.export({ format: "jwk" }),
      replayStore: new MemoryExecutionReplayStore(),
      now,
    }),
    (error) =>
      error instanceof MerchantSdkError && error.code === "binding_mismatch",
  );
});

test("maps unavailable execution keys to a safe dependency error", async () => {
  const { privateKey } = generateKeyPairSync("ec", { namedCurve: "P-256" });
  const body = Buffer.from("{}");

  await assert.rejects(
    verifyExecutionRequest({
      issuer: "https://api.agentpay.test",
      sellerId: "sel_test",
      routeId: "rte_test",
      method: "POST",
      path: "/fulfill",
      rawBody: body,
      headers: {
        "x-agentpay-execution-capability": signExecutionCapability({
          privateKey,
          body,
        }),
        "x-agentpay-transaction-id": "txn_test",
      },
      keyResolver: async () => {
        throw new Error("private resolver failure");
      },
      replayStore: new MemoryExecutionReplayStore(),
      now,
    }),
    (error) =>
      error instanceof MerchantSdkError &&
      error.code === "dependency_unavailable" &&
      !error.message.includes("private resolver failure"),
  );
});

test("processes verified JSON through one idempotent merchant adapter call", async () => {
  const { privateKey, publicKey } = generateKeyPairSync("ec", {
    namedCurve: "P-256",
  });
  const body = Buffer.from('{"report":"weekly"}');
  let calls = 0;
  const result = await processAgentPayFulfillment({
    verification: {
      issuer: "https://api.agentpay.test",
      sellerId: "sel_test",
      routeId: "rte_test",
      method: "POST",
      path: "/fulfill",
      rawBody: body,
      headers: {
        "x-agentpay-execution-capability": signExecutionCapability({
          privateKey,
          body,
        }),
        "x-agentpay-transaction-id": "txn_test",
      },
      keyResolver: async () => publicKey.export({ format: "jwk" }),
      replayStore: new MemoryExecutionReplayStore(),
      now,
    },
    fulfillmentStore: new MemoryFulfillmentStore(),
    parseInput(rawBody) {
      return JSON.parse(Buffer.from(rawBody).toString("utf8"));
    },
    adapter: {
      async fulfill(context) {
        calls += 1;
        assert.equal(context.input.report, "weekly");
        return { delivered: true };
      },
    },
  });

  assert.equal(result.disposition, "executed");
  assert.equal(calls, 1);
});

test("exposes the legacy HMAC verifier only through an explicit sandbox name", async () => {
  const secret = "0123456789abcdef0123456789abcdef";
  const body = Buffer.from("sandbox");
  const timestamp = "2026-09-19T12:00:00Z";
  const transactionId = "txn_sandbox";
  const canonical = [
    "agentpay.seller-request.v1",
    timestamp,
    "POST",
    "/fulfill",
    createHash("sha256").update(body).digest("hex"),
    transactionId,
  ].join("\n");
  const { createHmac } = await import("node:crypto");

  await verifyLegacySandboxRequest({
    secret,
    method: "POST",
    path: "/fulfill",
    rawBody: body,
    headers: {
      "x-agentpay-signature": createHmac("sha256", secret)
        .update(canonical)
        .digest("base64"),
      "x-agentpay-timestamp": timestamp,
      "x-agentpay-transaction-id": transactionId,
    },
    replayStore: new MemoryReplayStore(),
    now,
  });
});

test("remote JWKS resolver caches only according to response headers", async () => {
  const { publicKey } = generateKeyPairSync("ec", { namedCurve: "P-256" });
  const jwk = publicKey.export({ format: "jwk" });
  let calls = 0;
  const resolver = createRemoteJwksResolver({
    jwksUrl: "https://api.agentpay.test/.well-known/jwks.json",
    fetch: async () => {
      calls += 1;
      return new Response(
        JSON.stringify({ keys: [{ ...jwk, kid: "key-1", alg: "ES256" }] }),
        {
          status: 200,
          headers: {
            "cache-control": "public, max-age=60",
            "content-type": "application/json",
          },
        },
      );
    },
    now: () => now,
  });

  assert.equal((await resolver("key-1")).kid, "key-1");
  assert.equal((await resolver("key-1")).kid, "key-1");
  assert.equal(calls, 1);
});

function signExecutionCapability({ privateKey, body }) {
  const header = { typ: "agentpay-execution+jwt", alg: "ES256", kid: "key-1" };
  const issuedAt = Math.floor(now.getTime() / 1000);
  const claims = {
    iss: "https://api.agentpay.test",
    aud: "urn:agentpay:seller:sel_test",
    sub: "txn_test",
    sellerId: "sel_test",
    routeId: "rte_test",
    transactionId: "txn_test",
    method: "POST",
    path: "/fulfill",
    bodySha256: createHash("sha256").update(body).digest("hex"),
    paymentFinality: "finalized",
    jti: "xec_merchant_test",
    iat: issuedAt,
    exp: issuedAt + 45,
  };
  const signingInput = `${Buffer.from(JSON.stringify(header)).toString("base64url")}.${Buffer.from(JSON.stringify(claims)).toString("base64url")}`;
  const signature = signBytes("sha256", Buffer.from(signingInput), {
    key: privateKey,
    dsaEncoding: "ieee-p1363",
  });
  return `${signingInput}.${signature.toString("base64url")}`;
}
