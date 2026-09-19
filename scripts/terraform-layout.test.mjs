import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { test } from "node:test";
import path from "node:path";

const repositoryRoot = path.resolve(import.meta.dirname, "..");
const terraformRoot = path.join(repositoryRoot, "infra", "terraform");

function read(relativePath) {
  return readFileSync(path.join(repositoryRoot, relativePath), "utf8");
}

test("Terraform replaces the CDK placeholder with pinned infrastructure tooling", () => {
  const requiredFiles = [
    "infra/terraform/main.tf",
    "infra/terraform/providers.tf",
    "infra/terraform/variables.tf",
    "infra/terraform/versions.tf",
    "infra/terraform/modules/foundation/main.tf",
    "infra/terraform/environments/dev.backend.hcl.example",
    "infra/terraform/environments/dev.tfvars.example",
    "infra/terraform/environments/demo.backend.hcl.example",
    "infra/terraform/environments/demo.tfvars.example",
  ];

  for (const relativePath of requiredFiles) {
    assert.equal(existsSync(path.join(repositoryRoot, relativePath)), true, relativePath);
  }

  assert.equal(existsSync(path.join(repositoryRoot, "infra", "cdk.json")), false);
  assert.equal(existsSync(path.join(repositoryRoot, "infra", "package.json")), false);

  const versions = read("infra/terraform/versions.tf");
  assert.match(versions, /required_version\s*=\s*"= 1\.16\.3"/);
  assert.match(versions, /version\s*=\s*"= 6\.65\.0"/);
  assert.match(versions, /backend\s+"s3"/);
  assert.match(versions, /use_lockfile\s*=\s*true/);
});

test("CI validates Terraform without applying infrastructure", () => {
  const workflow = read(".github/workflows/ci.yml");

  assert.match(workflow, /hashicorp\/setup-terraform@v4\.0\.1/);
  assert.match(workflow, /terraform fmt -check -recursive/);
  assert.match(workflow, /terraform init -backend=false/);
  assert.match(workflow, /terraform validate/);
  assert.doesNotMatch(workflow, /cdk:synth/);
});

test("environment templates keep account-specific values out of committed defaults", () => {
  for (const environment of ["dev", "demo"]) {
    const backend = read(
      `infra/terraform/environments/${environment}.backend.hcl.example`,
    );
    const variables = read(
      `infra/terraform/environments/${environment}.tfvars.example`,
    );

    assert.match(backend, /bucket\s*=\s*"replace-with-/);
    assert.match(backend, /encrypt\s*=\s*true/);
    assert.match(backend, /use_lockfile\s*=\s*true/);
    assert.match(variables, new RegExp(`environment\\s*=\\s*"${environment}"`));
    assert.match(variables, /aws_region\s*=\s*"ap-south-1"/);
    assert.match(variables, /aws_account_id\s*=\s*"000000000000"/);
  }

  assert.equal(existsSync(terraformRoot), true);
});

test("the locked AWS region decision matches the Mumbai deployment", () => {
  const decisions = read("docs/DECISIONS.md");

  assert.match(
    decisions,
    /ADR-013 \| Default development\/demo region is Mumbai, `ap-south-1`/,
  );
  assert.doesNotMatch(decisions, /ADR-013[^\n]*`us-east-1`/);
});
