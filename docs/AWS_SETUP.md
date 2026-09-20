# AWS setup and deployment runbook

Status: **Locked for development and hackathon environments**. Production review is required before handling real funds.

## Environment model

Use separate AWS accounts when available:

- `agentpay-dev`: developer and integration testing.
- `agentpay-demo`: stable hackathon demonstration.
- `agentpay-prod`: deferred until security and legal gates pass.

The default development region is Mumbai, `ap-south-1`. Confirm that every required service is available in the selected region. Never hardcode account IDs, URLs, addresses, or secrets in source. Bedrock is disabled for the seller-first V1 deployment.

## Verified environment snapshot — September 20, 2026

Read-only checks against the configured `agentpay-india` profile established:

- the caller is the non-root IAM user `agentpay-deployer` in the intended account;
- the configured region is `ap-south-1`;
- Terraform applied **37 additions, 0 changes, and 0 destroys**, and the
  post-apply plan reports no changes;
- a September 20 drift audit detected the emergency Lambda upload and
  `dynamodb:ConditionCheckItem` IAM patch; the reviewed reconciliation applied
  **0 additions, 3 in-place changes, and 0 destroys**, deployed the current
  Lambda artifact, tightened the launch-entitlement operator role, and ended
  with a no-change plan;
- the applied Lambda regional concurrency quota is `400`, and
  `agentpay-dev-api` is `Active` on `provided.al2023`, ARM64, with reserved
  concurrency `5`;
- HTTP API `sadmp7j94e` is live at
  `https://sadmp7j94e.execute-api.ap-south-1.amazonaws.com`;
- `GET /health/live` and `GET /health/ready` return `200`; readiness reports
  `dynamodb`, `evidence_store`, `kms`, `secrets`, and `x402_facilitator` ready;
- `GET /.well-known/agentpay` and `GET /v1/payment-capabilities` return `200`;
- both Secrets Manager pepper containers have an `AWSCURRENT` version; no
  secret values were read;
- thirteen CloudWatch alarms and the seller operations dashboard are deployed;
  SNS alarm notification actions and delivery are not configured;
- the Terraform-managed development budget is healthy at `$10/month` and
  reported `$0` actual spend at the time of inspection;
- Vercel production variables use the Terraform HTTP API origin as a
  server-only upstream; commit `eb9bb19` deployed the branded
  `/api/backend/*` BFF through Git integration, and public platform discovery
  no longer emits the generated API Gateway origin; and
- `GET /api/auth/csrf`, `GET /sign-in`, and `GET /docs` return `200` on the
  canonical Vercel domain.

The current deployer user still has AWS-managed `AdministratorAccess`. That is
an external production blocker: use it only to create narrow Terraform roles,
then remove broad standing access before public launch. Manual entitlement
changes are rejected unless the command is running through the
Terraform-managed assumed operator role.

## Local prerequisites

- Node.js 24 or the repository-pinned version once added.
- Go 1.26 or the repository-pinned version once added.
- AWS CLI v2 authenticated through IAM Identity Center or another short-lived credential flow.
- Terraform 1.16.3, matching the version pinned by `infra/terraform/versions.tf`.
- Docker only if Lambda bundling requires it.
- A dedicated testnet wallet containing no production assets.

Verify the caller before any deployment:

```powershell
aws sts get-caller-identity
aws configure get region
```

Do not continue if the returned account is not the intended development or demo account.

## Required configuration

Local and CI configuration names:

```text
AGENTPAY_ENV=dev
AGENTPAY_IDENTITY_MODE=cognito
AWS_REGION=ap-south-1
AGENTPAY_WEB_ORIGIN=https://agentpay.prathamranka.in
AGENTPAY_API_ORIGIN=<Terraform http_api_url output>
AGENTPAY_TABLE_NAME=<Terraform output>
AGENTPAY_EVIDENCE_BUCKET=<Terraform output>
AGENTPAY_EVIDENCE_KMS_KEY_ID=<Terraform output>
AGENTPAY_CAPABILITY_SIGNING_KEY_ID=<Terraform output>
AGENTPAY_CAPABILITY_VERIFICATION_KEY_IDS=<Terraform output>
AGENTPAY_APPLICATION_SECRETS_KMS_KEY_ID=<Terraform output>
AGENTPAY_CREDENTIAL_PEPPER_SECRET_ARN=<Terraform output>
AGENTPAY_CONFIRMATION_GRANT_PEPPER_SECRET_ARN=<Terraform output>
AGENTPAY_SELLER_USER_POOL_ID=<Terraform output>
AGENTPAY_SELLER_USER_POOL_CLIENT_ID=<Terraform output>
AGENTPAY_SESSION_ENCRYPTION_KEY=<Vercel-only base64url 32-byte secret>
AGENTPAY_HTTP_API_URL=<Terraform output; private server-only upstream>
AGENTPAY_PUBLIC_API_ORIGIN=https://agentpay.prathamranka.in/api/backend
AGENTPAY_MCP_URL=<Terraform output>
AGENTPAY_BUYER_MAXIMUM_PRICE_ATOMIC=<positive atomic-unit amount>
AGENTPAY_PAYMENT_MODE=x402
AGENTPAY_FACILITATOR_URL=https://x402.org/facilitator
AGENTPAY_X402_NETWORK=eip155:84532
AGENTPAY_X402_ASSET=0x036CbD53842c5426634e7929541eC2318f3dCF7e
```

Only names, local mock values, and non-sensitive URLs belong in `.env.example`. Actual values are environment configuration; secrets belong in Secrets Manager.

The seller-first deployment is locked to the official credential-free x402
test facilitator, Base Sepolia, and Base Sepolia USDC. Mainnet, card checkout,
platform transaction fees, and custody are disabled. A seller supplies a public
payment address through onboarding and proves control with the documented
one-time ownership challenge; there is no global seller payout address and no
seller private key in AgentPay configuration. A funded buyer wallet is required
only for the REL-003/REL-008 testnet release exercises. If automation stores
that test wallet, `AGENTPAY_REL003_BUYER_WALLET_SECRET_ARN` belongs only to the
isolated release-test runner, never the API Lambda or seller signup path.

## Terraform module layout

### Foundation module

- One DynamoDB table using `PK` and `SK` strings, on-demand billing, point-in-time recovery, AWS-owned encryption for the hackathon, and deletion protection in demo/prod.
- GSIs exactly as defined in `DATA_MODEL.md`.
- One S3 evidence bucket with Block Public Access, bucket-owner-enforced ownership, versioning, Object Lock enabled at creation, and AWS-managed KMS encryption with bucket keys. This avoids a second idle customer-managed-key charge while retaining KMS-backed storage encryption.
- One asymmetric KMS signing key for evidence signatures. The application requires `kms:Sign`; verification paths require `kms:GetPublicKey` and `kms:Verify` where supported.
- One separate CloudWatch log bucket or log groups. Do not use the Object Lock evidence bucket as an S3 server-access-log destination.

Object Lock cannot be treated as a later toggle. Terraform must create the evidence bucket with Object Lock enabled, and lifecycle rules must prevent routine destruction.

The development table is named `agentpay-dev-main`; application processes use
the Terraform `table_name` output rather than hardcoding that value. Development
keeps deletion protection off for controlled teardown, while demo and production
enable it.

### Identity module

- Cognito user pool for seller accounts.
- Email sign-in for the hackathon.
- Self-registration enabled in development and disabled in the demo environment;
  seed approved demo sellers through an operator runbook.
- App client without a client secret for browser use.
- API Gateway JWT authorizer accepting only the configured pool, client, issuer, and audience.
- The BFF uses Cognito service APIs rather than Hosted UI redirects, so there
  is no Cognito callback URL to configure. Exact-Origin and CSRF checks remain
  pinned to `https://agentpay.prathamranka.in`.
- Vercel stores only an AES-256-GCM-sealed, Secure, HttpOnly,
  SameSite=Strict session cookie. Cognito tokens are never available to browser
  JavaScript. Access tokens refresh only inside the trusted BFF, and the seller
  session has an eight-hour absolute lifetime.

Historical buyer-approval participants and invitation-token flows are disabled
for Lean V1. AWS deployment must not expose their REST or WebSocket surfaces.

### Application module

- Go Lambda using ARM64 when all dependencies support it.
- Reserved concurrency set to a small non-zero demo value and adjusted through load testing.
- API Gateway HTTP API with JWT-protected seller routes and Lambda authorization/validation for agent credentials.
- No API Gateway WebSocket API is deployed for Lean V1. The historical buyer-approval channel remains disabled under ADR-043.
- A remote HTTPS MCP endpoint using seller-scoped credentials and the same application-domain services as the seller control API.
- DynamoDB, API invocation, and log permissions scoped to exact resources.
- Lambda environment variables contain references and identifiers, not secret values.
- CloudWatch structured JSON logs with request IDs and redaction.

AWS-005 is deployed. On September 19, 2026, Terraform created the reviewed
application resources with 37 additions, 0 changes, and 0 destroys; the
post-apply plan reported no changes. The Mumbai `agentpay-dev-api` Lambda is
Active on the `provided.al2023` ARM64 runtime with reserved concurrency 5
under the applied regional quota of 400. HTTP API `sadmp7j94e` serves REST and
`/mcp` from `https://sadmp7j94e.execute-api.ap-south-1.amazonaws.com`. The
production composition uses DynamoDB
repositories, Secrets Manager/KMS-backed cryptography and webhook secrets,
protected S3 evidence storage, KMS signing, and the Lambda HTTP adapter. Do not
apply an empty shell or bypass the required non-zero reserved-concurrency guard.

On September 20, 2026, CloudWatch attribution identified one legacy invocation
that exhausted the former 15-second Lambda deadline. A reviewed Terraform apply
updated the Lambda and HTTP API integration to 29 seconds, restricted seller
upstream contracts to 25 seconds, reserved two seconds before seller dispatch,
and enriched access logs with request path, integration latency, integration
error, and response latency. The apply changed three resources in place with no
additions or destroys; the post-apply plan reported no changes and both health
checks passed.

New AWS accounts can have an applied regional Lambda concurrency quota of 10,
even though the documented default quota is higher. Lambda requires at least 10
executions to remain unreserved, so such an account cannot assign any positive
reserved concurrency. Verify the applied quota before the AWS-005 plan:

```powershell
aws lambda get-account-settings --profile agentpay-india --region ap-south-1
aws service-quotas list-service-quotas --service-code lambda --profile agentpay-india --region ap-south-1
```

Do not remove `reserved_concurrent_executions` to work around this gate. If the
applied quota is not greater than `10 + api_reserved_concurrency`, submit a
quota request, keep `api_deployment_enabled = false`, and leave no partial
Lambda or HTTP API deployed. After any quota change, regenerate and review the
plan; never reuse an older plan. Terraform also enforces this condition as a
Lambda resource precondition so an apply fails before runtime creation when
the account cannot preserve AWS's ten unreserved executions.

A fresh plan generated on September 19, 2026 with the API runtime enabled and
the launch-entitlement role configured contained **37 additions, 0 changes,
and 0 destroys**. That plan was applied successfully, and the post-apply plan
reported no changes. Regenerate the plan after every code, quota,
configuration, or state change.

### Publication outbox stream

AWS-012 is deployed for the authoritative no-cache V1 profile. The
`agentpay-dev-main` table publishes `NEW_IMAGE` stream records. A filtered
event-source mapping sends only immutable `publicationOutbox` inserts to the
API Lambda event multiplexer. Processing uses partial-batch responses, three
bounded retries, batch bisection, a one-hour maximum record age, and the
managed-encryption `agentpay-dev-publication-outbox-dlq`.

The consumer refreshes the durable signed publication snapshot before writing
an idempotent `publicationCompletion` record. On September 20, 2026, a live
runtime invocation returned no batch failures and a consistent DynamoDB read
verified the completion record. Because AWS-011 is intentionally deferred, no
Redis or CDN cache exists to invalidate and credential authorization continues
to read DynamoDB directly.

### Bedrock application permissions

Bedrock remains disabled and is non-blocking for the seller-first V1
deployment. The deterministic buyer path remains available without it.

- Do not create a model resource.
- Configure `AGENTPAY_BEDROCK_MODEL_ID` after checking regional availability and account access.
- Grant `bedrock:InvokeModel` only for the selected model/inference profile ARN.
- Complete any provider-specific first-time-use or Marketplace access process required by AWS before the demo.
- Do not grant Bedrock-related code access to Secrets Manager wallet secrets, payment records, or seller signing secrets.

### Web module

- Vercel production deployment connected to the intended project and canonical domain.
- Server-side environment variables contain the private API Gateway origin and
  Cognito identifiers. Public discovery and seller connector instructions use
  `https://agentpay.prathamranka.in/api/backend`; the generated `execute-api`
  URL must not be rendered into public responses.
- No AWS secret or wallet material is exposed through `NEXT_PUBLIC_*` variables.
- Configure CSP, frame ancestors, referrer policy, and secure cookies.

## IAM role matrix

| Role              | Required access                                                                                                                       | Explicitly denied/not granted                                            |
| ----------------- | ------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------ |
| API Lambda        | DynamoDB item operations, append evidence objects, evidence/capability signing, application-envelope encryption, and selected secrets | Table scans, evidence deletion, KMS administration, IAM changes, Bedrock |
| Evidence verifier | Read evidence objects, KMS public-key/verification operations                                                                         | S3 writes/deletes, KMS signing                                           |
| Web frontend      | Public API access and Cognito browser flows                                                                                           | DynamoDB, S3 evidence bucket, KMS, Secrets Manager                       |
| CI deploy         | Terraform deployment permissions scoped to project resources and remote state                                                         | Organization/account administration                                      |
| Human developer   | Assume deployment/read-only roles through short-lived credentials                                                                     | Long-lived access keys in repository or CI variables                     |

Before production, split the API Lambda role into payment/proxy, evidence writer, verifier, and asynchronous worker roles.

## Secrets setup

Terraform creates the two pepper containers without a secret version. Before
the API is activated, generate two independent values with at least 256 bits of
cryptographic randomness and inject each value from a protected local file:

```powershell
aws secretsmanager put-secret-value --secret-id agentpay/dev/credential-pepper --secret-string file://<protected-credential-pepper-file> --profile agentpay-india --region ap-south-1
aws secretsmanager put-secret-value --secret-id agentpay/dev/confirmation-grant-pepper --secret-string file://<protected-confirmation-pepper-file> --profile agentpay-india --region ap-south-1
```

Secret values must not be passed on the command line, committed, pasted into
chat, Terraform variables, plans, outputs, logs, or shell history. Remove the
protected input files after injection according to the operator workstation's
secure-deletion policy. The containers intentionally remain empty until an
authorized operator supplies these values.

Required controls:

- Seller HMAC secrets are generated server-side and displayed only once or delivered through an authenticated rotation flow.
- Logs redact `Authorization`, `Cookie`, `PAYMENT-SIGNATURE`, invitation tokens, approval tokens, and all secret values.
- Rotate demo secrets after every public event.

Capability-key rotation is additive. Add the next version label to
`capability_signing_key_versions`, apply to create the new P-256 key, publish
both key IDs through JWKS, and only then change
`active_capability_signing_key_version`. Retain the old verification key beyond
the maximum capability lifetime and cache window. Protected keys require an
explicit reviewed retirement change; routine teardown cannot destroy them.

## Deployment order

### 1. Bootstrap remote state

Confirm `aws sts get-caller-identity` returns the intended development account.
Then copy the bootstrap example, replace both placeholders locally, and review
the plan before applying it:

```powershell
Copy-Item infra/bootstrap/environments/dev.tfvars.example infra/bootstrap/environments/dev.tfvars
terraform -chdir=infra/bootstrap init
terraform -chdir=infra/bootstrap fmt -check -recursive
terraform -chdir=infra/bootstrap validate
terraform -chdir=infra/bootstrap plan -var-file=environments/dev.tfvars -out=bootstrap.tfplan
terraform -chdir=infra/bootstrap apply bootstrap.tfplan
terraform -chdir=infra/bootstrap output
```

The bootstrap root uses local state because the remote backend cannot create
itself. Never commit that state. Preserve it in an encrypted operator-controlled
location until the bucket has been independently verified. The bucket has
versioning, default encryption, public-access blocking, TLS-only access, and
deletion protection. Terraform uses S3 native lock files; no DynamoDB lock table
is required.

### 2. Configure the environment backend

Copy the committed environment templates and replace their placeholders with
the verified account ID and bootstrap outputs:

```powershell
Copy-Item infra/terraform/environments/dev.backend.hcl.example infra/terraform/environments/dev.backend.hcl
Copy-Item infra/terraform/environments/dev.tfvars.example infra/terraform/environments/dev.tfvars
```

Initialize the main root. Use `-migrate-state` if it has ever been initialized
with local state; use `-reconfigure` for a clean checkout with no state to move:

```powershell
terraform -chdir=infra/terraform init -migrate-state -backend-config=environments/dev.backend.hcl
terraform -chdir=infra/terraform fmt -check -recursive
terraform -chdir=infra/terraform validate
terraform -chdir=infra/terraform plan -var-file=environments/dev.tfvars -out=dev.tfplan
terraform -chdir=infra/terraform apply dev.tfplan
```

For AWS-005, set `api_deployment_enabled = true` only in the ignored
environment tfvars after the concurrency check passes. Build the reviewed
artifact immediately before planning:

```powershell
npm run build:lambda
terraform -chdir=infra/terraform plan -var-file=environments/dev.tfvars -out=aws-005.tfplan
terraform -chdir=infra/terraform apply aws-005.tfplan
```

The reviewed plan must contain no destruction and must retain ARM64, the
configured non-zero reserved concurrency, API throttles, seven-day logs, the
canonical web-origin CORS allowlist, and no WebSocket API.

The September 19, 2026 AWS-005 apply met those requirements: 37 additions, no
changes, no destroys, followed by a no-change plan. The deployed Lambda is
`agentpay-dev-api` on `provided.al2023` ARM64 with reserved concurrency 5, and
HTTP API `sadmp7j94e` is reachable at the recorded Terraform origin.

After initialization, verify that the state object and its `.tflock` companion
can be created only through authenticated TLS requests and that a second
Terraform process cannot acquire the same lock. Do not delete the bootstrap
state until the remote state bucket and version history have been verified.

AWS-001 verification requires a versioned state object, no current `.tflock`
object after Terraform exits, and a concurrent operation failing to acquire the
same native S3 lock. Keep the ignored bootstrap state in encrypted operator
storage because it remains the recovery record for the protected backend.

Deploy the web application only after recording the HTTP API and Cognito outputs.

For the Vercel production project, configure these server-only environment
variables after AWS-005 has produced a real API origin:

```text
AGENTPAY_ENV=dev
AGENTPAY_IDENTITY_MODE=cognito
AGENTPAY_WEB_ORIGIN=https://agentpay.prathamranka.in
AGENTPAY_API_ORIGIN=<http_api_url; server-only and never NEXT_PUBLIC_*>
AWS_REGION=ap-south-1
AGENTPAY_SELLER_USER_POOL_CLIENT_ID=<seller_user_pool_client_id>
AGENTPAY_SESSION_ENCRYPTION_KEY=<independently generated 32-byte base64url value>
```

Generate the session key locally and paste it directly into the encrypted
Vercel environment-variable prompt. Do not write it to a file, commit it, pass
it through Terraform, or paste it into issue or chat history:

```powershell
node -e "console.log(require('node:crypto').randomBytes(32).toString('base64url'))"
```

The repository script validates Terraform outputs and CORS alignment before it
changes Vercel. Its default mode is read-only:

```powershell
npm run ops:vercel:configure
npm run ops:vercel:configure -- --apply --project web --scope prathams-projects-077823c3
```

On the first deployment only, add `--initialize-session-key`. This generates
the key in memory and passes it to Vercel as a sensitive value without printing
or storing it:

```powershell
npm run ops:vercel:configure -- --apply --initialize-session-key --project web --scope prathams-projects-077823c3
```

Do not use `--initialize-session-key` during ordinary redeployments because
rotating it invalidates current seller sessions. The linked Vercel project is
`web`. On September 19, 2026 its production variables used the Terraform API
origin, deployment `dpl_ExWeM7fxdqtUoLK2HUA1cki2X3Ti` was `READY` and aliased
to `https://agentpay.prathamranka.in`, and `/api/auth/csrf`, `/sign-in`, and
`/docs` each returned `200`.

The branded public API is implemented by the Vercel catch-all route at
`/api/backend/*`. Terraform passes that branded origin to Lambda for public
manifest, JWKS, purchase-session, and connector links while Vercel's
server-only `AGENTPAY_API_ORIGIN` remains the raw API Gateway upstream. Deploy
Vercel before applying a Lambda configuration change to the branded origin.

No step may require an undocumented console change except initial account/Bedrock provider access. If a console action is unavoidable, add it here with the exact verification command.

### Deployed seller-auth smoke

Run the smoke only after AWS-005 and AWS-008 are complete and the canonical
Vercel deployment is using Cognito. Supply the approved private test recipient
through the terminal environment; it is never rendered by the public site or
committed to source. The runner generates a temporary password in memory and
prompts locally for the six-digit Cognito email code.

```powershell
$env:AGENTPAY_SMOKE_EMAIL = "<approved-private-test-recipient>"
npm run smoke:auth:deployed
Remove-Item Env:AGENTPAY_SMOKE_EMAIL
```

The standard run verifies sign-up, email-code verification, sign-in, the
unauthenticated-to-onboarding redirect, the Secure HttpOnly session cookie,
authenticated onboarding access, coordinated sign-out, and rejection after
sign-out. Set `AGENTPAY_SMOKE_WAIT_FOR_REFRESH=true` to retain the process until
the 60-minute Cognito access-token refresh boundary. Set
`AGENTPAY_SMOKE_WAIT_FOR_EXPIRY=true` only for the scheduled eight-hour
absolute-session-expiry release run. These long-running checks use the deployed
clock and production cookie path; no test-only endpoint or production lifetime
override is permitted.

## Seed and smoke test

The seed command must create:

- One demo seller.
- `POST /research/basic`, below approval threshold.
- `POST /research/board`, at or above approval threshold.
- Two named demo approver invitations created only when a purchase intent requests them.

Smoke checks:

```text
GET  /health -> 200
GET  /store/demo-research/manifest.json -> 200 with two routes
POST /v1/intents -> 201
Paid route without approval -> 428 for board route
Paid route without payment -> 402 with PAYMENT-REQUIRED
```

## Observability

Emit metrics for:

- API 4xx/5xx count and p95 latency.
- Payment challenge and verification outcomes.
- Facilitator latency/failure rate.
- Approval completion, veto, and expiration counts.
- Evidence append and verification failures.
- Seller upstream timeout and non-success counts.
- Duplicate/replay attempts.
- Bedrock latency, errors, and fallback usage.

Terraform creates one low-cost dashboard, native API Gateway/Lambda alarms, and
log-derived alarms for MCP, checkout, facilitator, evidence, seller forwarding,
webhook retry, and webhook dead-letter failures. Missing data is healthy, and
the dashboard/alarms exist only when the API runtime exists. Configure reviewed
SNS topic ARNs through `operational_alarm_action_arns`; an empty list creates
alarms without notifications and must not be used for a public launch.

On September 19, 2026, all thirteen CloudWatch alarms and the seller operations
dashboard were deployed. AWS-009 remains in progress because
`operational_alarm_action_arns` has no configured SNS notification action and
alarm delivery has not been verified.

Manual launch entitlement operations are available only through the
environment-bound Terraform role and deterministic dry-run/apply workflow; no
seller-authenticated route grants entitlement. The procedure is documented in
[`runbooks/LAUNCH_ENTITLEMENT.md`](runbooks/LAUNCH_ENTITLEMENT.md). Webhook live
verification is documented in
[`runbooks/WEBHOOK_CANARY.md`](runbooks/WEBHOOK_CANARY.md).

## Cost controls

- The development account uses a Terraform-managed monthly cost budget. The
  recipient is supplied only through the ignored environment tfvars; committed
  examples keep it null. Alerts fire at 50%, 80%, and 100% actual spend and at
  100% forecasted spend.
- Use DynamoDB on-demand capacity and Lambda reserved concurrency.
- Limit CloudWatch log retention in development.
- Set Bedrock maximum output tokens and per-request tool-call limits.
- Disable unused NAT gateways; the planned serverless deployment does not require a VPC for the hackathon.
- Tag every resource with `Project=AgentPay`, `Environment`, `Owner`, and `ManagedBy=Terraform`.

## Teardown

Terraform application and identity resources may be destroyed only from their environment-specific state after reviewing the destroy plan. Evidence storage uses deletion protection and `prevent_destroy`; removing it requires a separate, explicitly approved evidence-destruction procedure and must never be part of routine teardown.
