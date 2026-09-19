import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

test("Lambda runtime can consume confirmation grants with DynamoDB condition checks", async () => {
  const policy = await readFile(
    new URL("../infra/terraform/modules/foundation/security.tf", import.meta.url),
    "utf8",
  );

  assert.match(policy, /"dynamodb:ConditionCheckItem"/);
});
