import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import path from "node:path";

const repositoryRoot = path.resolve(import.meta.dirname, "..");

function read(relativePath) {
  return readFileSync(path.join(repositoryRoot, relativePath), "utf8");
}

test("manual launch entitlement role is optional and seller-partition scoped", () => {
  const operations = read("infra/terraform/operator-access.tf");
  const variables = read("infra/terraform/variables.tf");

  assert.match(variables, /variable\s+"launch_entitlement_operator_principal_arns"/);
  assert.match(operations, /resource\s+"aws_iam_role"\s+"launch_entitlement_operator"/);
  assert.match(operations, /dynamodb:PutItem/);
  assert.match(operations, /dynamodb:GetItem/);
  assert.match(operations, /dynamodb:TransactWriteItems/);
  assert.match(operations, /dynamodb:LeadingKeys/);
  assert.match(operations, /SELLER#\*/);
  assert.doesNotMatch(operations, /dynamodb:Scan/);
  assert.doesNotMatch(operations, /dynamodb:DeleteItem/);
});
