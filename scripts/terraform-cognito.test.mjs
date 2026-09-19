import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { test } from "node:test";
import path from "node:path";

const repositoryRoot = path.resolve(import.meta.dirname, "..");

function read(relativePath) {
  return readFileSync(path.join(repositoryRoot, relativePath), "utf8");
}

test("AWS-006 provisions email-based Cognito seller identity", () => {
  const identityModule = "infra/terraform/modules/identity/main.tf";
  assert.equal(existsSync(path.join(repositoryRoot, identityModule)), true);

  const identity = read(identityModule);
  assert.match(identity, /resource\s+"aws_cognito_user_pool"\s+"seller"/);
  assert.match(identity, /username_attributes\s*=\s*\["email"\]/);
  assert.match(identity, /auto_verified_attributes\s*=\s*\["email"\]/);
  assert.match(identity, /minimum_length\s*=\s*12/);
  assert.match(identity, /account_recovery_setting/);
  assert.match(identity, /verified_email/);
  assert.match(identity, /allow_admin_create_user_only\s*=\s*!var\.self_registration_enabled/);
});

test("AWS-006 creates a public BFF client with revocation and no client secret", () => {
  const identity = read("infra/terraform/modules/identity/main.tf");

  assert.match(identity, /resource\s+"aws_cognito_user_pool_client"\s+"seller_web"/);
  assert.match(identity, /generate_secret\s*=\s*false/);
  assert.match(identity, /enable_token_revocation\s*=\s*true/);
  assert.match(identity, /prevent_user_existence_errors\s*=\s*"ENABLED"/);
  assert.match(identity, /"ALLOW_USER_PASSWORD_AUTH"/);
  assert.match(identity, /"ALLOW_REFRESH_TOKEN_AUTH"/);
  assert.doesNotMatch(identity, /client_secret/);
});

test("AWS-006 wires Cognito outputs to the application runtime", () => {
  const root = read("infra/terraform/main.tf");
  const outputs = read("infra/terraform/outputs.tf");
  const application = read("infra/terraform/modules/application/main.tf");
  const runbook = read("docs/AWS_SETUP.md");

  assert.match(root, /module\s+"identity"/);
  assert.match(root, /seller_user_pool_id\s*=\s*module\.identity\.user_pool_id/);
  assert.match(root, /seller_user_pool_client_id\s*=\s*module\.identity\.user_pool_client_id/);
  assert.match(outputs, /output\s+"seller_user_pool_id"/);
  assert.match(outputs, /output\s+"seller_user_pool_client_id"/);
  assert.match(application, /AGENTPAY_SELLER_USER_POOL_ID\s*=\s*var\.seller_user_pool_id/);
  assert.match(application, /AGENTPAY_SELLER_USER_POOL_CLIENT_ID\s*=\s*var\.seller_user_pool_client_id/);
  assert.match(application, /resource\s+"aws_apigatewayv2_authorizer"\s+"seller"/);
  assert.match(application, /authorization_type\s*=\s*"JWT"/);
  assert.match(application, /"ANY \/v1\/me\/\{proxy\+\}"/);
  assert.match(application, /"ANY \/v1\/sellers\/\{sellerId\}\/\{proxy\+\}"/);
  assert.match(runbook, /AGENTPAY_SESSION_ENCRYPTION_KEY/);
  assert.match(runbook, /AGENTPAY_IDENTITY_MODE=cognito/);
});
