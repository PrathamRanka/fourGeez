import assert from "node:assert/strict";
import test from "node:test";

import { AccessTokenClient, ConnectorHttpError } from "../dist/token-client.js";

const projectKey = "apc2.key_secret.never-print-this";
const accessToken = "eyJhbGciOiJFUzI1NiJ9.eyJzdWIiOiJrZXlfZGVtbyJ9.signature";

function jsonResponse(status, body, headers = {}) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "content-type": "application/json", ...headers },
  });
}

test("exchanges the project key with the proprietary AgentPay request contract", async () => {
  const calls = [];
  const client = new AccessTokenClient({
    baseUrl: "https://api.agentpay.test",
    projectKey,
    scopes: ["read", "validate"],
    now: () => 1_000,
    fetch: async (url, init) => {
      calls.push({ url, init });
      return jsonResponse(200, {
        accessToken,
        tokenType: "Bearer",
        expiresIn: 120,
        scope: "read validate",
        sellerId: "sel_demo",
        credentialId: "key_demo",
        entitlementEpoch: 3,
      });
    },
  });

  assert.equal(await client.getAccessToken(), accessToken);
  assert.equal(calls.length, 1);
  assert.equal(
    calls[0].url,
    "https://api.agentpay.test/v1/integration-access-tokens",
  );
  assert.equal(calls[0].init.method, "POST");
  assert.equal(calls[0].init.headers["X-AgentPay-Project-Key"], projectKey);
  assert.deepEqual(JSON.parse(calls[0].init.body), {
    audience: "urn:agentpay:mcp",
    scopes: ["read", "validate"],
  });
  assert.equal("grant_type" in JSON.parse(calls[0].init.body), false);
});

test("caches a token only while it remains outside the safety window", async () => {
  let now = 10_000;
  let exchanges = 0;
  const client = new AccessTokenClient({
    baseUrl: "https://api.agentpay.test/",
    projectKey,
    scopes: ["read"],
    safetyWindowMs: 30_000,
    now: () => now,
    fetch: async () => {
      exchanges += 1;
      return jsonResponse(200, {
        accessToken: `header.payload${exchanges}.signature`,
        tokenType: "Bearer",
        expiresIn: 120,
        scope: "read",
        sellerId: "sel_demo",
        credentialId: "key_demo",
        entitlementEpoch: 1,
      });
    },
  });

  assert.equal(await client.getAccessToken(), "header.payload1.signature");
  now += 89_999;
  assert.equal(await client.getAccessToken(), "header.payload1.signature");
  now += 2;
  assert.equal(await client.getAccessToken(), "header.payload2.signature");
  assert.equal(exchanges, 2);
});

test("coalesces concurrent exchanges and supports explicit invalidation", async () => {
  let exchanges = 0;
  let releaseExchange;
  const exchangeGate = new Promise((resolve) => {
    releaseExchange = resolve;
  });
  const client = new AccessTokenClient({
    baseUrl: "https://api.agentpay.test",
    projectKey,
    scopes: ["read"],
    fetch: async () => {
      exchanges += 1;
      await exchangeGate;
      return jsonResponse(200, {
        accessToken: `header.payload${exchanges}.signature`,
        tokenType: "Bearer",
        expiresIn: 120,
        scope: "read",
        sellerId: "sel_demo",
        credentialId: "key_demo",
        entitlementEpoch: 1,
      });
    },
  });

  const first = client.getAccessToken();
  const second = client.getAccessToken();
  releaseExchange();
  assert.deepEqual(await Promise.all([first, second]), [
    "header.payload1.signature",
    "header.payload1.signature",
  ]);
  assert.equal(exchanges, 1);

  client.invalidate();
  assert.equal(await client.getAccessToken(), "header.payload2.signature");
});

test("aborts a timed-out exchange without exposing the project key", async () => {
  const client = new AccessTokenClient({
    baseUrl: "https://api.agentpay.test",
    projectKey,
    scopes: ["read"],
    timeoutMs: 5,
    fetch: async (_url, init) =>
      new Promise((_resolve, reject) => {
        init.signal.addEventListener(
          "abort",
          () => reject(init.signal.reason),
          { once: true },
        );
      }),
  });

  await assert.rejects(client.getAccessToken(), (error) => {
    assert.equal(error.name, "ConnectorTimeoutError");
    assert.equal(String(error).includes(projectKey), false);
    return true;
  });
});

test("returns sanitized cloud error metadata without response bodies or secrets", async () => {
  const responseSecret = "server-body-secret";
  const client = new AccessTokenClient({
    baseUrl: "https://api.agentpay.test",
    projectKey,
    scopes: ["read"],
    fetch: async () =>
      jsonResponse(403, {
        error: {
          code: "subscription_inactive",
          message: responseSecret,
          requestId: "req_safe",
        },
      }),
  });

  await assert.rejects(client.getAccessToken(), (error) => {
    assert.equal(error instanceof ConnectorHttpError, true);
    assert.equal(error.status, 403);
    assert.equal(error.code, "subscription_inactive");
    assert.equal(error.requestId, "req_safe");
    assert.equal(String(error).includes(projectKey), false);
    assert.equal(String(error).includes(responseSecret), false);
    return true;
  });
});

test("rejects malformed successful exchange payloads", async () => {
  const client = new AccessTokenClient({
    baseUrl: "https://api.agentpay.test",
    projectKey,
    scopes: ["read"],
    fetch: async () =>
      jsonResponse(200, { accessToken: projectKey, expiresIn: 9999 }),
  });

  await assert.rejects(client.getAccessToken(), (error) => {
    assert.equal(error.name, "ConnectorProtocolError");
    assert.equal(String(error).includes(projectKey), false);
    return true;
  });
});

test("rejects a non-JWT token so a project key can never become an MCP bearer", async () => {
  const client = new AccessTokenClient({
    baseUrl: "https://api.agentpay.test",
    projectKey,
    scopes: ["read"],
    fetch: async () =>
      jsonResponse(200, {
        accessToken: "not-a-jwt",
        tokenType: "Bearer",
        expiresIn: 120,
        scope: "read",
        sellerId: "sel_demo",
        credentialId: "key_demo",
        entitlementEpoch: 1,
      }),
  });

  await assert.rejects(client.getAccessToken(), {
    name: "ConnectorProtocolError",
  });
});

test("rejects an echoed project key even when it resembles a compact JWT", async () => {
  const client = new AccessTokenClient({
    baseUrl: "https://api.agentpay.test",
    projectKey,
    scopes: ["read"],
    fetch: async () =>
      jsonResponse(200, {
        accessToken: projectKey,
        tokenType: "Bearer",
        expiresIn: 120,
        scope: "read",
        sellerId: "sel_demo",
        credentialId: "key_demo",
        entitlementEpoch: 1,
      }),
  });

  await assert.rejects(client.getAccessToken(), {
    name: "ConnectorProtocolError",
  });
});

test("requires HTTPS except for loopback local development", () => {
  assert.throws(
    () =>
      new AccessTokenClient({
        baseUrl: "http://api.agentpay.test",
        projectKey,
        scopes: ["read"],
      }),
    { name: "ConnectorConfigurationError" },
  );
  assert.doesNotThrow(
    () =>
      new AccessTokenClient({
        baseUrl: "http://127.0.0.1:8080",
        projectKey,
        scopes: ["read"],
      }),
  );
  assert.doesNotThrow(
    () =>
      new AccessTokenClient({
        baseUrl: "http://localhost:8080",
        projectKey,
        scopes: ["read"],
      }),
  );
});
