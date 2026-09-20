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
  assert.match(application, /aws_servicequotas_service_quota/);
  assert.match(
    application,
    /value\s*>\s*10\s*\+\s*var\.lambda_reserved_concurrency/,
  );
});

test("AWS-005 preserves time to record failures before the HTTP integration deadline", () => {
  const application = read("infra/terraform/modules/application/main.tf");
  const variables = read("infra/terraform/variables.tf");

  assert.match(
    variables,
    /variable\s+"api_timeout_seconds"[^]*default\s*=\s*29/,
  );
  assert.match(application, /path\s*=\s*"\$context\.path"/);
  assert.match(
    application,
    /integrationLatency\s*=\s*"\$context\.integrationLatency"/,
  );
  assert.match(
    application,
    /integrationErrorMessage\s*=\s*"\$context\.integrationErrorMessage"/,
  );
  assert.match(
    application,
    /responseLatency\s*=\s*"\$context\.responseLatency"/,
  );
});

test("AWS-012 deploys the publication outbox stream consumer with bounded retries and a DLQ", () => {
  const foundation = read("infra/terraform/modules/foundation/main.tf");
  const foundationOutputs = read("infra/terraform/modules/foundation/outputs.tf");
  const application = read("infra/terraform/modules/application/main.tf");
  const root = read("infra/terraform/main.tf");

  assert.match(foundation, /stream_enabled\s*=\s*true/);
  assert.match(foundation, /stream_view_type\s*=\s*"NEW_IMAGE"/);
  assert.match(foundationOutputs, /output\s+"table_stream_arn"/);
  assert.match(root, /table_stream_arn\s*=\s*module\.foundation\.table_stream_arn/);
  assert.match(application, /resource\s+"aws_sqs_queue"\s+"publication_outbox_dlq"/);
  assert.match(application, /resource\s+"aws_lambda_event_source_mapping"\s+"publication_outbox"/);
  assert.match(application, /function_response_types\s*=\s*\["ReportBatchItemFailures"\]/);
  assert.match(application, /maximum_retry_attempts\s*=\s*3/);
  assert.match(application, /bisect_batch_on_function_error\s*=\s*true/);
  assert.match(application, /destination_arn\s*=\s*aws_sqs_queue\.publication_outbox_dlq\[0\]\.arn/);
});

test("AWS-005 locks Lambda payments to the credential-free Base Sepolia profile", () => {
  const root = read("infra/terraform/main.tf");
  const rootVariables = read("infra/terraform/variables.tf");
  const application = read("infra/terraform/modules/application/main.tf");

  assert.match(root, /payment_mode\s*=\s*var\.payment_mode/);
  assert.match(root, /facilitator_url\s*=\s*var\.x402_facilitator_url/);
  assert.match(root, /x402_network\s*=\s*var\.x402_network/);
  assert.match(root, /x402_asset\s*=\s*var\.x402_asset/);
  assert.match(rootVariables, /default\s*=\s*"x402"/);
  assert.match(
    rootVariables,
    /default\s*=\s*"https:\/\/x402\.org\/facilitator"/,
  );
  assert.match(rootVariables, /default\s*=\s*"eip155:84532"/);
  assert.match(
    rootVariables,
    /default\s*=\s*"0x036CbD53842c5426634e7929541eC2318f3dCF7e"/,
  );
  assert.match(application, /AGENTPAY_PAYMENT_MODE\s*=\s*var\.payment_mode/);
  assert.match(
    application,
    /AGENTPAY_FACILITATOR_URL\s*=\s*var\.facilitator_url/,
  );
  assert.match(application, /AGENTPAY_X402_NETWORK\s*=\s*var\.x402_network/);
  assert.match(application, /AGENTPAY_X402_ASSET\s*=\s*var\.x402_asset/);
  assert.doesNotMatch(application, /TEST_WALLET|FACILITATOR_(API_)?KEY/);
});
