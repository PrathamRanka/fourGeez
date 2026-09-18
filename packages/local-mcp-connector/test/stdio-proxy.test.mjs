import assert from "node:assert/strict";
import { PassThrough } from "node:stream";
import test from "node:test";

import { runStdioProxy } from "../dist/stdio-proxy.js";

async function runProxy(inputText, transport, options = {}) {
  const input = new PassThrough();
  const output = new PassThrough();
  const diagnostics = new PassThrough();
  let stdout = "";
  let stderr = "";
  output.setEncoding("utf8");
  diagnostics.setEncoding("utf8");
  output.on("data", (chunk) => {
    stdout += chunk;
  });
  diagnostics.on("data", (chunk) => {
    stderr += chunk;
  });
  input.end(inputText);
  await runStdioProxy({ input, output, diagnostics, transport, ...options });
  return { stdout, stderr };
}

test("proxies newline-delimited JSON-RPC without inspecting or authorizing methods", async () => {
  const requests = [];
  const request =
    '{"jsonrpc":"2.0","id":7,"method":"publish_route","params":{"opaque":true}}';
  const result = await runProxy(`${request}\n`, {
    send: async (message) => {
      requests.push(message);
      return '{"jsonrpc":"2.0","id":7,"result":{"cloudAuthorized":true}}';
    },
  });

  assert.deepEqual(requests, [request]);
  assert.equal(
    result.stdout,
    '{"jsonrpc":"2.0","id":7,"result":{"cloudAuthorized":true}}\n',
  );
  assert.equal(result.stderr, "");
});

test("does not write a response for an HTTP 204 notification", async () => {
  const result = await runProxy(
    '{"jsonrpc":"2.0","method":"notifications/initialized"}\n',
    {
      send: async () => undefined,
    },
  );

  assert.equal(result.stdout, "");
  assert.equal(result.stderr, "");
});

test("returns a JSON-RPC parse error locally for malformed input", async () => {
  let requests = 0;
  const result = await runProxy("not-json\n", {
    send: async () => {
      requests += 1;
      return "{}";
    },
  });

  assert.equal(requests, 0);
  assert.deepEqual(JSON.parse(result.stdout), {
    jsonrpc: "2.0",
    id: null,
    error: { code: -32700, message: "Parse error" },
  });
});

test("discards oversized input and continues with the next bounded message", async () => {
  const forwarded = [];
  const oversized = JSON.stringify({
    jsonrpc: "2.0",
    id: 1,
    payload: "x".repeat(100),
  });
  const valid = '{"jsonrpc":"2.0","id":2,"method":"tools/list"}';
  const result = await runProxy(
    `${oversized}\n${valid}\n`,
    {
      send: async (message) => {
        forwarded.push(message);
        return '{"jsonrpc":"2.0","id":2,"result":{}}';
      },
    },
    { maxMessageBytes: 64 },
  );

  const lines = result.stdout.trim().split("\n").map(JSON.parse);
  assert.equal(lines[0].error.code, -32600);
  assert.equal(lines[0].error.message, "Request exceeds connector size limit");
  assert.deepEqual(lines[1], { jsonrpc: "2.0", id: 2, result: {} });
  assert.deepEqual(forwarded, [valid]);
});

test("diagnostics remain secret-safe", async () => {
  const projectKey = "apc2.key_secret.never-print-this";
  const bearer = "signed-access-token-never-print-this";
  const result = await runProxy(
    '{"jsonrpc":"2.0","id":1,"method":"tools/list"}\n',
    {
      send: async () => {
        const error = new Error(`unsafe ${projectKey} ${bearer}`);
        error.name = "ConnectorHttpError";
        error.status = 503;
        error.code = "dependency_unavailable";
        error.requestId = "req_safe";
        throw error;
      },
    },
  );

  assert.equal(result.stderr.includes(projectKey), false);
  assert.equal(result.stderr.includes(bearer), false);
  assert.match(result.stderr, /dependency_unavailable/);
  assert.match(result.stderr, /req_safe/);
  assert.deepEqual(JSON.parse(result.stdout), {
    jsonrpc: "2.0",
    id: 1,
    error: { code: -32000, message: "AgentPay cloud request failed" },
  });
});
