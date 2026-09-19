import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { test } from "node:test";
import path from "node:path";

const repositoryRoot = path.resolve(import.meta.dirname, "..");

function read(relativePath) {
  return readFileSync(path.join(repositoryRoot, relativePath), "utf8");
}

test("AWS-001 provisions a protected S3 backend with native state locking", () => {
  const requiredFiles = [
    "infra/bootstrap/versions.tf",
    "infra/bootstrap/providers.tf",
    "infra/bootstrap/variables.tf",
    "infra/bootstrap/main.tf",
    "infra/bootstrap/outputs.tf",
    "infra/bootstrap/environments/dev.tfvars.example",
  ];

  for (const relativePath of requiredFiles) {
    assert.equal(existsSync(path.join(repositoryRoot, relativePath)), true, relativePath);
  }

  const main = read("infra/bootstrap/main.tf");
  assert.match(main, /resource\s+"aws_s3_bucket"\s+"terraform_state"/);
  assert.match(main, /prevent_destroy\s*=\s*true/);
  assert.match(main, /status\s*=\s*"Enabled"/);
  assert.match(main, /sse_algorithm\s*=\s*"AES256"/);
  assert.match(main, /block_public_acls\s*=\s*true/);
  assert.match(main, /restrict_public_buckets\s*=\s*true/);
  assert.match(main, /aws:SecureTransport/);
  assert.doesNotMatch(main, /aws_dynamodb_table/);
});

test("state bootstrap is pinned, account-scoped, and validated by CI", () => {
  const versions = read("infra/bootstrap/versions.tf");
  const provider = read("infra/bootstrap/providers.tf");
  const workflow = read(".github/workflows/ci.yml");

  assert.match(versions, /required_version\s*=\s*"= 1\.16\.3"/);
  assert.match(versions, /version\s*=\s*"= 6\.65\.0"/);
  assert.match(provider, /allowed_account_ids\s*=\s*\[var\.aws_account_id\]/);
  assert.match(workflow, /working-directory:\s*infra\/bootstrap/);
});

test("runbook documents bootstrap and backend migration without committing account values", () => {
  const runbook = read("docs/AWS_SETUP.md");
  const bootstrapExample = read(
    "infra/bootstrap/environments/dev.tfvars.example",
  );

  assert.match(runbook, /terraform -chdir=infra\/bootstrap init/);
  assert.match(runbook, /terraform -chdir=infra\/bootstrap apply/);
  assert.match(runbook, /terraform -chdir=infra\/terraform init -migrate-state/);
  assert.match(bootstrapExample, /aws_account_id\s*=\s*"000000000000"/);
  assert.match(bootstrapExample, /state_bucket_name\s*=\s*"replace-with-/);
});
