import assert from "node:assert/strict";
import test from "node:test";

import {
  CloudMcpTransport,
  ConnectorHttpError,
} from "../dist/cloud-transport.js";

function response(status, body, headers = {}) {
  return new Response(body, { status, headers });
}

test("posts bounded JSON-RPC with a short-lived bearer capability", async () => {
  const calls = [];
  const tokenClient = {
    getAccessToken: async () => "access-token",
    invalidate: () => assert.fail("token should not be invalidated"),
  };
  const transport = new CloudMcpTransport({
    baseUrl: "https://api.agentpay.test",
    tokenClient,
    fetch: async (url, init) => {
      calls.push({ url, init });
      return response(200, '{"jsonrpc":"2.0","id":1,"result":{"ok":true}}', {
        "content-type": "application/json",
      });
    },
  });
  const request =
    '{"jsonrpc":"2.0","id":1,"method":"custom/cloud_authorized_method"}';

  assert.equal(
    await transport.send(request),
    '{"jsonrpc":"2.0","id":1,"result":{"ok":true}}',
  );
  assert.equal(calls[0].url, "https://api.agentpay.test/mcp");
  assert.equal(calls[0].init.headers.Authorization, "Bearer access-token");
  assert.equal(calls[0].init.body, request);
});

test("normalizes a formatted cloud response to one stdio-safe JSON line", async () => {
  const transport = new CloudMcpTransport({
    baseUrl: "https://api.agentpay.test",
    tokenClient: {
      getAccessToken: async () => "header.payload.signature",
      invalidate: () => {},
    },
    fetch: async () =>
      response(200, '{\n  "jsonrpc": "2.0",\n  "id": 1,\n  "result": {}\n}', {
        "content-type": "application/json",
      }),
  });

  assert.equal(
    await transport.send('{"jsonrpc":"2.0","id":1,"method":"tools/list"}'),
    '{"jsonrpc":"2.0","id":1,"result":{}}',
  );
});

test("invalidates and retries exactly once after 401", async () => {
  const tokens = ["expired-token", "fresh-token"];
  let invalidations = 0;
  let requests = 0;
  const transport = new CloudMcpTransport({
    baseUrl: "https://api.agentpay.test",
    tokenClient: {
      getAccessToken: async () => tokens[Math.min(requests, 1)],
      invalidate: () => {
        invalidations += 1;
      },
    },
    fetch: async (_url, init) => {
      requests += 1;
      if (requests === 1) {
        assert.equal(init.headers.Authorization, "Bearer expired-token");
        return response(401, '{"error":{"code":"token_expired"}}', {
          "content-type": "application/json",
        });
      }
      assert.equal(init.headers.Authorization, "Bearer fresh-token");
      return response(200, '{"jsonrpc":"2.0","id":1,"result":{}}', {
        "content-type": "application/json",
      });
    },
  });

  assert.equal(
    await transport.send('{"jsonrpc":"2.0","id":1,"method":"tools/list"}'),
    '{"jsonrpc":"2.0","id":1,"result":{}}',
  );
  assert.equal(requests, 2);
  assert.equal(invalidations, 1);
});

test("stops after the single permitted 401 retry", async () => {
  let requests = 0;
  let invalidations = 0;
  const transport = new CloudMcpTransport({
    baseUrl: "https://api.agentpay.test",
    tokenClient: {
      getAccessToken: async () => "header.payload.signature",
      invalidate: () => {
        invalidations += 1;
      },
    },
    fetch: async () => {
      requests += 1;
      return response(401, '{"error":{"code":"token_revoked"}}', {
        "content-type": "application/json",
      });
    },
  });

  await assert.rejects(
    transport.send('{"jsonrpc":"2.0","id":1,"method":"tools/list"}'),
    (error) => error instanceof ConnectorHttpError && error.status === 401,
  );
  assert.equal(requests, 2);
  assert.equal(invalidations, 1);
});

test("treats server-authoritative suspension as terminal", async () => {
  let requests = 0;
  let invalidations = 0;
  const transport = new CloudMcpTransport({
    baseUrl: "https://api.agentpay.test",
    tokenClient: {
      getAccessToken: async () => "header.payload.signature",
      invalidate: () => {
        invalidations += 1;
      },
    },
    fetch: async () => {
      requests += 1;
      return response(
        403,
        '{"error":{"code":"subscription_inactive","message":"private lifecycle detail"}}',
        { "content-type": "application/json" },
      );
    },
  });

  await assert.rejects(
    transport.send('{"jsonrpc":"2.0","id":1,"method":"tools/list"}'),
    (error) => {
      assert.equal(error instanceof ConnectorHttpError, true);
      assert.equal(error.status, 403);
      assert.equal(error.code, "subscription_inactive");
      assert.equal(String(error).includes("private lifecycle detail"), false);
      return true;
    },
  );
  assert.equal(requests, 1);
  assert.equal(invalidations, 0);
});

for (const status of [403, 429, 503]) {
  test(`does not retry status ${status}`, async () => {
    let requests = 0;
    const transport = new CloudMcpTransport({
      baseUrl: "https://api.agentpay.test",
      tokenClient: {
        getAccessToken: async () => "access-token",
        invalidate: () => assert.fail("token should not be invalidated"),
      },
      fetch: async () => {
        requests += 1;
        return response(
          status,
          '{"error":{"code":"cloud-denied","message":"do-not-print"}}',
          {
            "content-type": "application/json",
            "retry-after": "9",
            "x-request-id": "req_safe",
          },
        );
      },
    });

    await assert.rejects(
      transport.send('{"jsonrpc":"2.0","id":1,"method":"tools/call"}'),
      (error) => {
        assert.equal(error instanceof ConnectorHttpError, true);
        assert.equal(error.status, status);
        assert.equal(error.retryAfterSeconds, 9);
        assert.equal(String(error).includes("do-not-print"), false);
        return true;
      },
    );
    assert.equal(requests, 1);
  });
}

test("rejects oversized or malformed JSON-RPC before network access", async () => {
  let requests = 0;
  const transport = new CloudMcpTransport({
    baseUrl: "https://api.agentpay.test",
    maxMessageBytes: 32,
    tokenClient: {
      getAccessToken: async () => "access-token",
      invalidate: () => {},
    },
    fetch: async () => {
      requests += 1;
      return response(200, "{}");
    },
  });

  await assert.rejects(transport.send("not-json"), {
    name: "ConnectorProtocolError",
  });
  await assert.rejects(
    transport.send(JSON.stringify({ payload: "x".repeat(64) })),
    {
      name: "ConnectorMessageTooLargeError",
    },
  );
  assert.equal(requests, 0);
});

test("does not allow the configured MCP limit to exceed the one MiB contract", () => {
  assert.throws(
    () =>
      new CloudMcpTransport({
        baseUrl: "https://api.agentpay.test",
        maxMessageBytes: 1024 * 1024 + 1,
        tokenClient: {
          getAccessToken: async () => "header.payload.signature",
          invalidate: () => {},
        },
      }),
    { name: "ConnectorConfigurationError" },
  );
});

test("rejects an oversized cloud response while streaming it", async () => {
  const transport = new CloudMcpTransport({
    baseUrl: "https://api.agentpay.test",
    maxMessageBytes: 32,
    tokenClient: {
      getAccessToken: async () => "access-token",
      invalidate: () => {},
    },
    fetch: async () =>
      response(200, JSON.stringify({ result: "x".repeat(64) }), {
        "content-type": "application/json",
      }),
  });

  await assert.rejects(transport.send('{"jsonrpc":"2.0"}'), {
    name: "ConnectorMessageTooLargeError",
  });
});

test("honors caller abort signals without retrying", async () => {
  let requests = 0;
  const abortController = new AbortController();
  const transport = new CloudMcpTransport({
    baseUrl: "https://api.agentpay.test",
    tokenClient: {
      getAccessToken: async () => "access-token",
      invalidate: () => {},
    },
    fetch: async (_url, init) => {
      requests += 1;
      abortController.abort(new Error("caller stopped"));
      await new Promise((_resolve, reject) => {
        if (init.signal.aborted) reject(init.signal.reason);
        init.signal.addEventListener(
          "abort",
          () => reject(init.signal.reason),
          { once: true },
        );
      });
    },
  });

  await assert.rejects(
    transport.send('{"jsonrpc":"2.0"}', abortController.signal),
    {
      name: "ConnectorAbortError",
    },
  );
  assert.equal(requests, 1);
});

test("times out an MCP request without retrying", async () => {
  let requests = 0;
  const transport = new CloudMcpTransport({
    baseUrl: "https://api.agentpay.test",
    timeoutMs: 5,
    tokenClient: {
      getAccessToken: async () => "header.payload.signature",
      invalidate: () => {},
    },
    fetch: async (_url, init) => {
      requests += 1;
      return new Promise((_resolve, reject) => {
        init.signal.addEventListener(
          "abort",
          () => reject(init.signal.reason),
          { once: true },
        );
      });
    },
  });

  await assert.rejects(
    transport.send('{"jsonrpc":"2.0","id":1,"method":"tools/list"}'),
    { name: "ConnectorTimeoutError" },
  );
  assert.equal(requests, 1);
});
