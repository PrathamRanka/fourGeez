import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import path from "node:path";

const repositoryRoot = path.resolve(import.meta.dirname, "..");

function read(relativePath) {
  return readFileSync(path.join(repositoryRoot, relativePath), "utf8");
}

test("AWS-003 defines protected Object Lock evidence storage", () => {
  const main = read("infra/terraform/modules/foundation/main.tf");

  assert.match(main, /resource\s+"aws_s3_bucket"\s+"evidence"/);
  assert.match(main, /object_lock_enabled\s*=\s*true/);
  assert.match(main, /resource\s+"aws_s3_bucket_versioning"\s+"evidence"/);
  assert.match(main, /status\s*=\s*"Enabled"/);
  assert.match(main, /resource\s+"aws_s3_bucket_object_lock_configuration"\s+"evidence"/);
  assert.match(main, /mode\s*=\s*"GOVERNANCE"/);
  assert.match(main, /days\s*=\s*var\.evidence_retention_days/);
  assert.match(main, /resource\s+"aws_s3_bucket_public_access_block"\s+"evidence"/);
  assert.match(main, /restrict_public_buckets\s*=\s*true/);
  assert.match(main, /aws:SecureTransport/);
  assert.match(main, /prevent_destroy\s*=\s*true/);
});

test("AWS-003 uses a non-exportable asymmetric KMS signing key", () => {
  const main = read("infra/terraform/modules/foundation/main.tf");

  assert.match(main, /resource\s+"aws_kms_key"\s+"evidence_signing"/);
  assert.match(main, /key_usage\s*=\s*"SIGN_VERIFY"/);
  assert.match(main, /customer_master_key_spec\s*=\s*"ECC_NIST_P256"/);
  assert.match(main, /deletion_window_in_days\s*=\s*30/);
  assert.match(main, /sse_algorithm\s*=\s*"aws:kms"/);
  assert.match(main, /bucket_key_enabled\s*=\s*true/);
});

test("AWS-003 exports evidence runtime identifiers without secret material", () => {
  const moduleOutputs = read("infra/terraform/modules/foundation/outputs.tf");
  const rootOutputs = read("infra/terraform/outputs.tf");

  for (const output of ["evidence_bucket_name", "evidence_kms_key_id", "evidence_kms_key_arn"]) {
    assert.match(moduleOutputs, new RegExp(`output\\s+"${output}"`));
    assert.match(rootOutputs, new RegExp(`output\\s+"${output}"`));
  }
});

test("AWS-003 is marked complete only with protected evidence infrastructure", () => {
  const implementation = read("docs/IMPLEMENTATION.md");

  assert.match(
    implementation,
    /- \[x\] \*\*AWS-003\*\* Create versioned Object Lock evidence bucket and KMS signing key\./,
  );
});
