import assert from "node:assert/strict";
import {
  createHmac,
  createHash,
  generateKeyPairSync,
  sign as signBytes,
} from "node:crypto";
import test from "node:test";
import {
  createAgentPayMiddleware,
  MemoryExecutionReplayStore,
  MemoryReplayStore,
  verifyAgentPayRequest,
  verifyAgentPayExecution,
} from "../dist/index.js";

const secret = "0123456789abcdef0123456789abcdef";
const now = new Date("2026-09-17T10:00:00Z");

// sign creates an independent request signature fixture.
function sign(body, timestamp, transactionId) {
  const bodyHash = createHash("sha256").update(body).digest("hex");
  const canonical = [
    "agentpay.seller-request.v1",
    timestamp,
    "POST",
    "/fulfill",
    bodyHash,
    transactionId,
  ].join("\n");
  return createHmac("sha256", secret).update(canonical).digest("base64");
}

test("verifies signatures and blocks replay", async () => {
  const replayStore = new MemoryReplayStore();
  const timestamp = now.toISOString().replace(".000Z", "Z");
  const request = {
    method: "POST",
    path: "/fulfill",
    body: Buffer.from("hello"),
    headers: {
      "x-agentpay-signature": sign("hello", timestamp, "txn_node"),
      "x-agentpay-timestamp": timestamp,
      "x-agentpay-transaction-id": "txn_node",
    },
  };
  await verifyAgentPayRequest({ secret, request, replayStore, now });
  await assert.rejects(
    verifyAgentPayRequest({ secret, request, replayStore, now }),
    { code: "replay" },
  );
});

test("rejects modified bodies", async () => {
  const timestamp = now.toISOString().replace(".000Z", "Z");
  await assert.rejects(
    verifyAgentPayRequest({
      secret,
      now,
      replayStore: new MemoryReplayStore(),
      request: {
        method: "POST",
        path: "/fulfill",
        body: Buffer.from("changed"),
        headers: {
          "x-agentpay-signature": sign("hello", timestamp, "txn_modified"),
          "x-agentpay-timestamp": timestamp,
          "x-agentpay-transaction-id": "txn_modified",
        },
      },
    }),
    { code: "invalid_signature" },
  );
});

test("preserves the literal fractional timestamp in the signature", async () => {
  const timestamp = "2026-09-17T10:00:00.123456Z";
  await verifyAgentPayRequest({
    secret,
    now,
    replayStore: new MemoryReplayStore(),
    request: {
      method: "POST",
      path: "/fulfill",
      body: Buffer.from("hello"),
      headers: {
        "x-agentpay-signature": sign("hello", timestamp, "txn_fractional"),
        "x-agentpay-timestamp": timestamp,
        "x-agentpay-transaction-id": "txn_fractional",
      },
    },
  });
});

test("Express-compatible middleware verifies raw bodies", async () => {
  const timestamp = now.toISOString().replace(".000Z", "Z");
  const replayStore = new MemoryReplayStore();
  const middleware = createAgentPayMiddleware({
    secret,
    replayStore,
    now: () => now,
  });
  const request = {
    method: "POST",
    path: "/fulfill",
    rawBody: Buffer.from("hello"),
    headers: {
      "x-agentpay-signature": sign("hello", timestamp, "txn_middleware"),
      "x-agentpay-timestamp": timestamp,
      "x-agentpay-transaction-id": "txn_middleware",
    },
  };
  let nextCalls = 0;
  const response = {
    statusCode: 200,
    status(code) {
      this.statusCode = code;
      return this;
    },
    json() {},
  };
  await middleware(request, response, () => {
    nextCalls += 1;
  });
  assert.equal(nextCalls, 1);
  await middleware(request, response, () => {
    nextCalls += 1;
  });
  assert.equal(response.statusCode, 409);
  assert.equal(nextCalls, 1);
});

test("verifies exact ES256 execution bindings and consumes JTI once", async () => {
  const { privateKey, publicKey } = generateKeyPairSync("ec", {
    namedCurve: "P-256",
  });
  const body = Buffer.from('{"topic":"payments"}');
  const token = signExecutionCapability({ privateKey, body });
  const replayStore = new MemoryExecutionReplayStore();
  const options = {
    issuer: "https://api.agentpay.test",
    sellerId: "sel_test",
    routeId: "rte_test",
    request: {
      method: "POST",
      path: "/research",
      body,
      headers: {
        "x-agentpay-execution-capability": token,
        "x-agentpay-transaction-id": "txn_test",
      },
    },
    keyResolver: async () => publicKey.export({ format: "jwk" }),
    replayStore,
    now,
  };

  const claims = await verifyAgentPayExecution(options);
  assert.equal(claims.transactionId, "txn_test");
  await assert.rejects(verifyAgentPayExecution(options), { code: "replay" });
});

test("rejects a modified execution request binding", async () => {
  const { privateKey, publicKey } = generateKeyPairSync("ec", {
    namedCurve: "P-256",
  });
  const signedBody = Buffer.from('{"topic":"payments"}');
  await assert.rejects(
    verifyAgentPayExecution({
      issuer: "https://api.agentpay.test",
      sellerId: "sel_test",
      routeId: "rte_test",
      request: {
        method: "POST",
        path: "/research",
        body: Buffer.from('{"topic":"changed"}'),
        headers: {
          "x-agentpay-execution-capability": signExecutionCapability({
            privateKey,
            body: signedBody,
          }),
          "x-agentpay-transaction-id": "txn_test",
        },
      },
      keyResolver: async () => publicKey.export({ format: "jwk" }),
      replayStore: new MemoryExecutionReplayStore(),
      now,
    }),
    { code: "binding_mismatch" },
  );
});

function signExecutionCapability({ privateKey, body, mutate = () => {} }) {
  const header = { typ: "agentpay-execution+jwt", alg: "ES256", kid: "key-1" };
  const claims = {
    iss: "https://api.agentpay.test",
    aud: "urn:agentpay:seller:sel_test",
    sub: "txn_test",
    sellerId: "sel_test",
    routeId: "rte_test",
    transactionId: "txn_test",
    method: "POST",
    path: "/research",
    bodySha256: createHash("sha256").update(body).digest("hex"),
    paymentFinality: "finalized",
    jti: "xec_test",
    iat: Math.floor(now.getTime() / 1000),
    exp: Math.floor(now.getTime() / 1000) + 45,
  };
  mutate(claims);
  const signingInput = `${Buffer.from(JSON.stringify(header)).toString("base64url")}.${Buffer.from(JSON.stringify(claims)).toString("base64url")}`;
  const signature = signBytes("sha256", Buffer.from(signingInput), {
    key: privateKey,
    dsaEncoding: "ieee-p1363",
  });
  return `${signingInput}.${signature.toString("base64url")}`;
}
