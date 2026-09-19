import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import path from "node:path";

const repositoryRoot = path.resolve(import.meta.dirname, "..");

function read(relativePath) {
  return readFileSync(path.join(repositoryRoot, relativePath), "utf8");
}

test("AWS-004 creates metadata-only pepper secret containers", () => {
  const security = read("infra/terraform/modules/foundation/security.tf");

  assert.match(security, /resource\s+"aws_secretsmanager_secret"\s+"credential_pepper"/);
  assert.match(security, /resource\s+"aws_secretsmanager_secret"\s+"confirmation_grant_pepper"/);
  assert.doesNotMatch(security, /aws_secretsmanager_secret_version/);
  assert.match(security, /kms_key_id\s*=\s*aws_kms_key\.application_secrets\.arn/);
});

test("AWS-004 creates rotatable KMS-backed ES256 capability keys", () => {
  const security = read("infra/terraform/modules/foundation/security.tf");

  assert.match(security, /resource\s+"aws_kms_key"\s+"capability_signing"/);
  assert.match(security, /for_each\s*=\s*var\.capability_signing_key_versions/);
  assert.match(security, /key_usage\s*=\s*"SIGN_VERIFY"/);
  assert.match(security, /customer_master_key_spec\s*=\s*"ECC_NIST_P256"/);
  assert.match(security, /resource\s+"aws_kms_alias"\s+"capability_signing_current"/);
  assert.match(security, /var\.active_capability_signing_key_version/);
  assert.match(security, /prevent_destroy\s*=\s*true/);
});

test("AWS-004 scopes runtime and verifier roles to AgentPay resources", () => {
  const security = read("infra/terraform/modules/foundation/security.tf");

  assert.match(security, /resource\s+"aws_iam_role"\s+"api_runtime"/);
  assert.match(security, /resource\s+"aws_iam_role"\s+"evidence_verifier"/);
  assert.match(security, /aws_dynamodb_table\.agentpay\.arn/);
  assert.match(security, /aws_s3_bucket\.evidence\.arn/);
  assert.match(security, /aws_kms_key\.evidence_signing\.arn/);
  assert.match(security, /aws_kms_key\.capability_signing/);
  assert.match(security, /secretsmanager:GetSecretValue/);
  assert.doesNotMatch(security, /AdministratorAccess/);
  assert.doesNotMatch(security, /"kms:\*"/);
  assert.doesNotMatch(security, /"s3:\*"/);
  assert.doesNotMatch(security, /"dynamodb:\*"/);
});

test("AWS-004 exports only identifiers required by the runtime", () => {
  const moduleOutputs = read("infra/terraform/modules/foundation/outputs.tf");
  const rootOutputs = read("infra/terraform/outputs.tf");
  const outputs = [
    "capability_signing_key_id",
    "capability_verification_key_ids",
    "credential_pepper_secret_arn",
    "confirmation_grant_pepper_secret_arn",
    "application_secrets_kms_key_id",
    "api_runtime_role_arn",
    "evidence_verifier_role_arn",
  ];

  for (const output of outputs) {
    assert.match(moduleOutputs, new RegExp(`output\\s+"${output}"`));
    assert.match(rootOutputs, new RegExp(`output\\s+"${output}"`));
  }
});

test("AWS-004 documents secret injection and is marked complete", () => {
  const runbook = read("docs/AWS_SETUP.md");
  const implementation = read("docs/IMPLEMENTATION.md");

  assert.match(runbook, /AGENTPAY_CREDENTIAL_PEPPER_SECRET_ARN/);
  assert.match(runbook, /AGENTPAY_CONFIRMATION_GRANT_PEPPER_SECRET_ARN/);
  assert.match(runbook, /aws secretsmanager put-secret-value/);
  assert.match(runbook, /must not be passed on the command line/);
  assert.match(
    implementation,
    /- \[x\] \*\*AWS-004\*\* Create Secrets Manager entries, KMS\/HSM-backed capability-signing keys/,
  );
});
