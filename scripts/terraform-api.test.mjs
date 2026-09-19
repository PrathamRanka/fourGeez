import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import path from "node:path";

const repositoryRoot = path.resolve(import.meta.dirname, "..");

function read(relativePath) {
  return readFileSync(path.join(repositoryRoot, relativePath), "utf8");
}

test("AWS-005 excludes the deferred approval WebSocket runtime", () => {
  const implementation = read("docs/IMPLEMENTATION.md");
  const runbook = read("docs/AWS_SETUP.md");
  const architecture = read("docs/ARCHITECTURE.md");

  assert.match(
    implementation,
    /AWS-005.*HTTP API.*`\/mcp` endpoint.*approval WebSocket remains disabled/,
  );
  assert.doesNotMatch(implementation, /AWS-005.*Deploy.*WebSocket API/);
  assert.doesNotMatch(runbook, /AGENTPAY_WEBSOCKET_URL/);
  assert.doesNotMatch(runbook, /API Gateway WebSocket API with/);
  assert.match(
    architecture,
    /historical API Gateway approval WebSocket design is not deployed/,
  );
});

test("AWS-005 uses the canonical production frontend origin", () => {
  const variables = read("infra/terraform/variables.tf");
  const development = read("infra/terraform/environments/dev.tfvars.example");
  const runbook = read("docs/AWS_SETUP.md");

  assert.match(variables, /variable\s+"web_origin"/);
  assert.match(
    development,
    /web_origin\s*=\s*"https:\/\/agentpay\.prathamranka\.in"/,
  );
  assert.match(runbook, /https:\/\/agentpay\.prathamranka\.in/);
  assert.doesNotMatch(runbook, /vercel\.app/);
});

test("AWS-005 infrastructure is reproducible and deploys only a reviewed ARM64 artifact", () => {
  const application = read("infra/terraform/modules/application/main.tf");
  const variables = read("infra/terraform/variables.tf");
  const root = read("infra/terraform/main.tf");
  const packageManifest = read("package.json");

  assert.match(application, /resource\s+"aws_lambda_function"\s+"api"/);
  assert.match(application, /resource\s+"aws_apigatewayv2_api"\s+"http"/);
  assert.match(application, /resource\s+"aws_apigatewayv2_stage"\s+"default"/);
  assert.match(application, /resource\s+"aws_cloudwatch_log_group"\s+"api"/);
  assert.match(application, /route_key\s*=\s*"\$default"/);
  assert.match(application, /allow_origins\s*=\s*\[var\.web_origin\]/);
  assert.match(application, /throttling_burst_limit/);
  assert.match(application, /throttling_rate_limit/);
  assert.match(application, /architectures\s*=\s*\["arm64"\]/);
  assert.match(application, /retention_in_days\s*=\s*var\.log_retention_days/);
  assert.match(
    application,
    /AGENTPAY_FACILITATOR_URL\s*=\s*var\.facilitator_url/,
  );
  assert.match(application, /AGENTPAY_X402_NETWORK\s*=\s*var\.x402_network/);
  assert.match(application, /AGENTPAY_X402_ASSET\s*=\s*var\.x402_asset/);
  assert.match(root, /filebase64sha256\(var\.api_lambda_artifact_path\)/);
  assert.match(packageManifest, /"build:lambda"/);
  assert.match(
    variables,
    /variable\s+"api_reserved_concurrency"[^]*default\s*=\s*5/,
  );
  assert.doesNotMatch(
    application,
    /aws_apigatewayv2_api[^]*protocol_type\s*=\s*"WEBSOCKET"/,
  );
  assert.match(variables, /variable\s+"api_deployment_enabled"/);
  assert.match(variables, /default\s*=\s*false/);
});
