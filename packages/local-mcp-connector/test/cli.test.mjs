import assert from "node:assert/strict";
import { execFile } from "node:child_process";
import { createServer } from "node:http";
import { promisify } from "node:util";
import test from "node:test";

const execFileAsync = promisify(execFile);
const projectKey = "apc2.project.secret-never-print";

test("--check exchanges the project key and reports a secret-safe success", async (t) => {
  let receivedProjectKey = "";
  let receivedAuthorization = "";
  const requestPaths = [];
  const server = createServer((request, response) => {
    requestPaths.push(request.url);
    if (request.url === "/mcp") {
      receivedAuthorization = request.headers.authorization ?? "";
      response.writeHead(200, { "content-type": "application/json" });
      response.end(
        JSON.stringify({
          jsonrpc: "2.0",
          id: "agentpay-preflight",
          result: {},
        }),
      );
      return;
    }
    receivedProjectKey = request.headers["x-agentpay-project-key"] ?? "";
    response.writeHead(200, { "content-type": "application/json" });
    response.end(
      JSON.stringify({
        accessToken: "header.payload.signature",
        tokenType: "Bearer",
        expiresIn: 120,
        scope: "read configure publish validate",
        sellerId: "sel_demo",
        credentialId: "key_demo",
        entitlementEpoch: 1,
      }),
    );
  });
  await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));
  t.after(() => server.close());
  const address = server.address();
  assert.notEqual(address, null);

  const result = await execFileAsync(
    process.execPath,
    ["dist/cli.js", "--check"],
    {
      cwd: new URL("..", import.meta.url),
      env: {
        ...process.env,
        AGENTPAY_API_BASE_URL: `http://127.0.0.1:${address.port}`,
        AGENTPAY_PROJECT_KEY: projectKey,
      },
      timeout: 2_000,
    },
  );

  assert.equal(receivedProjectKey, projectKey);
  assert.equal(receivedAuthorization, "Bearer header.payload.signature");
  assert.deepEqual(requestPaths, ["/v1/integration-access-tokens", "/mcp"]);
  assert.match(result.stdout, /preflight passed/i);
  assert.equal(result.stdout.includes(projectKey), false);
  assert.equal(result.stderr.includes(projectKey), false);
});

test("--check identifies a missing project key without echoing secrets", async () => {
  await assert.rejects(
    execFileAsync(process.execPath, ["dist/cli.js", "--check"], {
      cwd: new URL("..", import.meta.url),
      env: {
        ...process.env,
        AGENTPAY_API_BASE_URL: "https://api.agentpay.test",
        AGENTPAY_PROJECT_KEY: "",
      },
      timeout: 2_000,
    }),
    (error) => {
      assert.equal(error.code, 1);
      assert.match(error.stderr, /AGENTPAY_PROJECT_KEY is required/);
      assert.equal(error.stderr.includes(projectKey), false);
      return true;
    },
  );
});
