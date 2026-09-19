# Manual launch-entitlement runbook

Status: **Implemented for the Stripe-disabled seller pilot; the hardened IAM
change requires a reviewed Terraform apply before cloud use.**

This workflow grants, suspends, cancels, or reactivates only the seller software
entitlement. It never reads or changes buyer x402 settlement records, wallets,
payment proofs, routes, or transaction state. The seller dashboard is read-only
for entitlement status and cannot self-grant access.

## Safety properties

- Cloud `dev`, `demo`, and `prod` never auto-create an entitlement. Automatic
  Starter access remains local-development-only.
- Dry run and apply require the same Terraform environment, account, region,
  table, operator role, seller, operation ID, effective time, and expected
  entitlement version.
- The command is a dry run unless `--apply` is present. Apply additionally
  requires the printed `planSha256` and exact `requiredConfirmation` phrase.
- Both dry run and apply reject root, IAM users, direct administrator roles,
  the wrong environment role, and assumed-role sessions without the
  `agentpay-entitlement-` session-name prefix.
- The IAM role can only read and transact against `SELLER#sel_*` records in the
  exact environment table. It cannot scan, call standalone `PutItem`, delete,
  access evidence, manage secrets, invoke Lambda, or modify buyer settlement.
- One immutable seller-scoped operation claim, the entitlement projection,
  reconciliation history, and administrator audit event commit in one DynamoDB
  transaction.
- An exact replay returns the original applied version without another write or
  audit event. Reusing an operation ID with changed inputs or a changed plan
  digest fails closed.
- Optimistic version checks prevent overwriting a concurrent billing or
  operator decision. Reactivation increments `entitlementEpoch` and requires
  project-key rotation according to the billing domain rules.
- No command argument, output, example, Terraform value, or operation record
  contains a secret.

## One-time role setup

Put only approved, unique, same-account IAM user or role ARNs in the ignored
environment tfvars:

```hcl
launch_entitlement_operator_principal_arns = [
  "arn:aws:iam::<account-id>:user/<approved-operator>",
]
```

Run and review Terraform format, validation, and plan. Do not apply as part of
this runbook review. After a separately approved apply, configure a role
profile without creating another access key:

```powershell
$role = terraform -chdir=infra/terraform output -raw launch_entitlement_operator_role_arn
$region = terraform -chdir=infra/terraform output -raw aws_region
$environment = terraform -chdir=infra/terraform output -raw environment

aws configure set role_arn $role --profile agentpay-entitlement-operator
aws configure set source_profile agentpay-india --profile agentpay-entitlement-operator
aws configure set region $region --profile agentpay-entitlement-operator
aws configure set role_session_name "agentpay-entitlement-$environment" --profile agentpay-entitlement-operator
aws sts get-caller-identity --profile agentpay-entitlement-operator
```

The returned ARN must contain
`assumed-role/agentpay-<environment>-launch-entitlement-operator/agentpay-entitlement-`.

## Load the environment binding

Read every non-secret binding from the same initialized Terraform root and
state. Do not hand-type a table or role from another environment.

```powershell
$deployment = terraform -chdir=infra/terraform output -json deployment | ConvertFrom-Json
$project = terraform -chdir=infra/terraform output -raw project_name
$table = terraform -chdir=infra/terraform output -raw table_name
$role = terraform -chdir=infra/terraform output -raw launch_entitlement_operator_role_arn
$environment = $deployment.environment
$account = $deployment.aws_account_id
$region = $deployment.aws_region
```

## Grant a pilot entitlement

Obtain `sellerId` from the authenticated seller workspace. For a seller with no
entitlement, expected version is `0`. Choose a bounded UTC expiry. Generate a
non-secret operation ID and freeze one UTC effective time for both commands.
Apply must begin within 15 minutes of that time.

```powershell
$seller = "sel_<seller-ulid>"
$expectedVersion = 0
$expires = "2026-10-20T00:00:00Z"
$effective = [DateTime]::UtcNow.ToString("yyyy-MM-ddTHH:mm:ssZ")
$operation = "leo_$([guid]::NewGuid().ToString('N'))"

$plan = go run ./cmd/ops-entitlement `
  --operation-id $operation `
  --environment $environment `
  --project-name $project `
  --account-id $account `
  --region $region `
  --table-name $table `
  --operator-role-arn $role `
  --profile agentpay-entitlement-operator `
  --action grant `
  --seller-id $seller `
  --plan starter `
  --effective-at $effective `
  --access-ends-at $expires `
  --expected-version $expectedVersion | ConvertFrom-Json

$plan | ConvertTo-Json -Depth 12
```

Review the complete current and proposed snapshots. Confirm the environment,
seller, plan, exact expiry, expected version, epoch/rotation effects, and both
digests. Apply using the unchanged values printed by that dry run:

```powershell
go run ./cmd/ops-entitlement `
  --operation-id $operation `
  --environment $environment `
  --project-name $project `
  --account-id $account `
  --region $region `
  --table-name $table `
  --operator-role-arn $role `
  --profile agentpay-entitlement-operator `
  --action grant `
  --seller-id $seller `
  --plan starter `
  --effective-at $effective `
  --access-ends-at $expires `
  --expected-version $expectedVersion `
  --plan-sha256 $plan.planSha256 `
  --apply `
  --confirm $plan.requiredConfirmation
```

Running that exact apply command again is a safe idempotent replay. Any changed
seller, environment, version, timestamp, action, plan, expiry, table, role, or
digest is rejected. A stale-version conflict requires a new dry run, effective
time, operation ID, and review.

## Suspend, cancel, or reactivate

Suspension blocks network access immediately while retaining the commercial
period for audit. Cancellation records terminal voluntary cancellation.
Perform the same dry-run/review/apply sequence with `--action suspend` or
`--action cancel`, the current entitlement version, and no
`--access-ends-at`. Reactivation uses `--action grant`, a new future boundary,
and the current version; the seller must rotate the project credential before
MCP access resumes.

## Verification

After apply:

1. Read the seller entitlement through the authenticated dashboard/API.
2. Confirm `source=operator`, `provider=operator`, expected plan, exact
   `accessEndsAt`, and applied version.
3. Confirm one `LaunchEntitlementOperation`, reconciliation record, and
   `entitlement.changed` audit event share the seller and operation ID; confirm
   the audit actor is the expected assumed-role session.
4. Replay the exact apply command and confirm `idempotentReplay=true` with no
   version increment or second audit event.
5. Confirm suspended/cancelled sellers cannot use MCP, publish, enter discovery,
   create intents, or receive a payment challenge.
