import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import path from "node:path";

const repositoryRoot = path.resolve(import.meta.dirname, "..");

function read(relativePath) {
  return readFileSync(path.join(repositoryRoot, relativePath), "utf8");
}

test("AWS-002 defines the documented single-table DynamoDB layout", () => {
  const main = read("infra/terraform/modules/foundation/main.tf");

  assert.match(main, /resource\s+"aws_dynamodb_table"\s+"agentpay"/);
  assert.match(main, /billing_mode\s*=\s*"PAY_PER_REQUEST"/);
  assert.match(main, /hash_key\s*=\s*"PK"/);
  assert.match(main, /range_key\s*=\s*"SK"/);
  assert.match(main, /point_in_time_recovery\s*{[\s\S]*enabled\s*=\s*true/);
  assert.match(main, /server_side_encryption\s*{[\s\S]*enabled\s*=\s*true/);
  assert.match(main, /deletion_protection_enabled\s*=\s*var\.environment\s*!=\s*"dev"/);

  for (const index of [1, 2, 3, 4]) {
    assert.match(
      main,
      new RegExp(
        `name\\s*=\\s*"GSI${index}"[\\s\\S]*attribute_name\\s*=\\s*"GSI${index}PK"[\\s\\S]*key_type\\s*=\\s*"HASH"[\\s\\S]*attribute_name\\s*=\\s*"GSI${index}SK"[\\s\\S]*key_type\\s*=\\s*"RANGE"[\\s\\S]*projection_type\\s*=\\s*"ALL"`,
      ),
    );
  }
});

test("AWS-002 exports the table name for runtime configuration", () => {
  const moduleOutputs = read("infra/terraform/modules/foundation/outputs.tf");
  const rootOutputs = read("infra/terraform/outputs.tf");

  assert.match(moduleOutputs, /output\s+"table_name"/);
  assert.match(moduleOutputs, /aws_dynamodb_table\.agentpay\.name/);
  assert.match(rootOutputs, /output\s+"table_name"/);
  assert.match(rootOutputs, /module\.foundation\.table_name/);
});

test("AWS-002 is marked complete only with the deployed table contract", () => {
  const implementation = read("docs/IMPLEMENTATION.md");

  assert.match(
    implementation,
    /- \[x\] \*\*AWS-002\*\* Create DynamoDB tables and required secondary indexes\./,
  );
});
