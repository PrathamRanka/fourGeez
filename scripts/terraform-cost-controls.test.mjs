import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import path from "node:path";

const repositoryRoot = path.resolve(import.meta.dirname, "..");

function read(relativePath) {
  return readFileSync(path.join(repositoryRoot, relativePath), "utf8");
}

test("development cost controls create an account budget without committing an email", () => {
  const costControls = read("infra/terraform/cost-controls.tf");
  const variables = read("infra/terraform/variables.tf");
  const example = read("infra/terraform/environments/dev.tfvars.example");

  assert.match(costControls, /resource\s+"aws_budgets_budget"\s+"monthly"/);
  assert.match(costControls, /budget_type\s*=\s*"COST"/);
  assert.match(costControls, /time_unit\s*=\s*"MONTHLY"/);
  assert.match(costControls, /notification_type\s*=\s*notification\.value\.notification_type/);
  assert.match(variables, /variable\s+"budget_alert_email"/);
  assert.match(variables, /variable\s+"monthly_budget_limit_usd"/);
  assert.match(example, /budget_alert_email\s*=\s*null/);
  assert.doesNotMatch(example, /pranka0789@gmail\.com/);
});
