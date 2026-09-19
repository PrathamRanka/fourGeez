# Manual launch-entitlement runbook

Status: **Implemented for the Stripe-disabled seller pilot; cloud execution is pending reviewed Terraform apply.**

This workflow grants, suspends, cancels, or reactivates the seller software
entitlement only. It never reads or changes buyer x402 settlement records,
wallets, payment proofs, routes, or transaction state.

## Safety properties

- Cloud `dev`, `demo`, and `prod` never auto-create an entitlement. Automatic
  Starter access remains local-development-only.
- The command is a dry run unless `--apply` is present.
- Every apply requires the exact phrase
  `<action>:<sellerId>:<expectedVersion>`.
- Apply rejects root and direct administrator sessions. The caller must be an
  assumed session of the Terraform-managed launch-entitlement operator role.
- The IAM role can only read and transact against `SELLER#*` records in the
  environment table. It cannot scan, delete, access evidence, manage secrets,
  invoke Lambda, or modify buyer settlement.
- The entitlement projection, reconciliation history, and administrator audit
  event are committed in one DynamoDB transaction.
- Optimistic version checks prevent overwriting a concurrent billing or
  operator decision.
- Reactivating a non-active seller increments `entitlementEpoch` and requires
  project-key rotation according to the billing domain rules.

## One-time role setup

Put only the approved IAM user or role ARN in the ignored environment tfvars:

```hcl
launch_entitlement_operator_principal_arns = [
  "arn:aws:iam::<account-id>:user/<approved-operator>",
]
```

Review `terraform plan` before applying. After apply, configure a role profile
without creating another access key:

```powershell
$roleArn = terraform -chdir=infra/terraform output -raw launch_entitlement_operator_role_arn
aws configure set role_arn $roleArn --profile agentpay-entitlement-operator
aws configure set source_profile agentpay-india --profile agentpay-entitlement-operator
aws configure set region ap-south-1 --profile agentpay-entitlement-operator
aws sts get-caller-identity --profile agentpay-entitlement-operator
```

The final ARN must contain `assumed-role/agentpay-<environment>-launch-entitlement-operator/`.

## Grant a pilot entitlement

Obtain `sellerId` from the authenticated seller workspace. For a seller with no
entitlement, the expected version is `0`. Choose an explicit UTC expiry; do not
use an unbounded date.

```powershell
$table = terraform -chdir=infra/terraform output -raw table_name
$role = terraform -chdir=infra/terraform output -raw launch_entitlement_operator_role_arn
$seller = "sel_<seller-ulid>"
$expires = "2026-10-19T00:00:00Z"

go run ./cmd/ops-entitlement --action grant --seller-id $seller --plan starter --access-ends-at $expires --expected-version 0 --table-name $table --region ap-south-1 --profile agentpay-entitlement-operator --account-id <account-id> --operator-role-arn $role
```

Review the printed current/proposed snapshots and required confirmation phrase.
Then repeat with the exact confirmation:

```powershell
go run ./cmd/ops-entitlement --action grant --seller-id $seller --plan starter --access-ends-at $expires --expected-version 0 --table-name $table --region ap-south-1 --profile agentpay-entitlement-operator --account-id <account-id> --operator-role-arn $role --apply --confirm "grant:$seller:0"
```

Never reuse a stale expected version. A conditional conflict requires a fresh
dry run and review.

## Suspend, cancel, or reactivate

Suspension blocks network access immediately while retaining the commercial
period for audit. Cancellation records a terminal voluntary cancellation.

```powershell
go run ./cmd/ops-entitlement --action suspend --seller-id $seller --expected-version <current-version> --table-name $table --region ap-south-1 --profile agentpay-entitlement-operator --account-id <account-id> --operator-role-arn $role --apply --confirm "suspend:$seller:<current-version>"
```

Reactivation uses `grant` with a new future boundary and current version. It
requires the seller to rotate the project credential before MCP access resumes.

## Verification

After apply:

1. Read the seller entitlement through the authenticated dashboard/API.
2. Confirm `source=operator`, `provider=operator`, the expected plan and exact
   `accessEndsAt`.
3. Confirm one `entitlement.changed` audit event identifies the assumed-role
   ARN and contains no credentials or wallet material.
4. Confirm suspended/cancelled sellers cannot use MCP, publish, enter discovery,
   create intents, or receive a payment challenge.
