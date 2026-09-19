import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import path from "node:path";

const repositoryRoot = path.resolve(import.meta.dirname, "..");

function read(relativePath) {
  return readFileSync(path.join(repositoryRoot, relativePath), "utf8");
}

test("AWS-009 creates one low-cost dashboard and alarms only with the API runtime", () => {
  const observability = read(
    "infra/terraform/modules/application/observability.tf",
  );

  assert.match(observability, /resource\s+"aws_cloudwatch_dashboard"\s+"operations"/);
  assert.match(observability, /count\s*=\s*var\.deployment_enabled\s*\?\s*1\s*:\s*0/);
  assert.match(observability, /AWS\/ApiGateway/);
  assert.match(observability, /AWS\/Lambda/);
  assert.match(observability, /AgentPay\/Operations/);
  assert.match(observability, /treat_missing_data\s*=\s*"notBreaching"/);
  assert.doesNotMatch(observability, /default_value\s*=\s*"0"/);
  assert.doesNotMatch(observability, /aws_xray|aws_opensearch|aws_elasticsearch/);
});

test("AWS-009 covers seller launch failure categories and budget protection", () => {
  const observability = read(
    "infra/terraform/modules/application/observability.tf",
  );
  const root = read("infra/terraform/main.tf");
  const variables = read("infra/terraform/variables.tf");
  const costControls = read("infra/terraform/cost-controls.tf");

  for (const category of [
    "mcp_failure",
    "checkout_failure",
    "facilitator_failure",
    "evidence_failure",
    "seller_forwarding_failure",
    "payment_replay",
    "webhook_retry_scheduled",
    "webhook_dead_letter",
  ]) {
    assert.match(observability, new RegExp(category));
  }
  assert.match(root, /alarm_action_arns\s*=\s*var\.operational_alarm_action_arns/);
  assert.match(variables, /variable\s+"operational_alarm_action_arns"/);
  assert.match(costControls, /aws_budgets_budget/);
});
