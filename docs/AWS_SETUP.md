# AWS setup and deployment runbook

Status: **Locked for development and hackathon environments**. Production review is required before handling real funds.

## Environment model

Use separate AWS accounts when available:

- `agentpay-dev`: developer and integration testing.
- `agentpay-demo`: stable hackathon demonstration.
- `agentpay-prod`: deferred until security and legal gates pass.

The default development region is Mumbai, `ap-south-1`. Confirm that every required service is available in the selected region. Never hardcode account IDs, URLs, addresses, or secrets in source. Bedrock is disabled for the seller-first V1 deployment.

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
AGENTPAY_HTTP_API_URL=<Terraform output>
AGENTPAY_MCP_URL=<Terraform output>
AGENTPAY_BUYER_MAXIMUM_PRICE_ATOMIC=<positive atomic-unit amount>
AGENTPAY_FACILITATOR_URL=<verified testnet facilitator URL>
AGENTPAY_X402_NETWORK=<verified SDK network identifier>
AGENTPAY_X402_ASSET=<verified testnet asset identifier>
```

Only names, local mock values, and non-sensitive URLs belong in `.env.example`. Actual values are environment configuration; secrets belong in Secrets Manager.

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

AWS-005 application resources are reproducible but remain disabled by
`api_deployment_enabled = false` until the production composition uses durable
DynamoDB repositories, Secrets Manager/KMS-backed cryptography and webhook
secrets, protected evidence storage/signing, and a Lambda HTTP adapter. The
current local server intentionally rejects every non-local composition. Do not
enable or apply the application module merely to deploy an empty shell.

### Bedrock application permissions

- Do not create a model resource.
- Configure `AGENTPAY_BEDROCK_MODEL_ID` after checking regional availability and account access.
- Grant `bedrock:InvokeModel` only for the selected model/inference profile ARN.
- Complete any provider-specific first-time-use or Marketplace access process required by AWS before the demo.
- Do not grant Bedrock-related code access to Secrets Manager wallet secrets, payment records, or seller signing secrets.

### Web module

- Amplify Hosting connected to the intended branch or deployed through a documented artifact flow.
- Server-side environment variables contain API origins and Cognito identifiers.
- No AWS secret or wallet material is exposed through `NEXT_PUBLIC_*` variables.
- Configure CSP, frame ancestors, referrer policy, and secure cookies.

## IAM role matrix

| Role | Required access | Explicitly denied/not granted |
|---|---|---|
| API Lambda | DynamoDB item operations, append evidence objects, evidence/capability signing, application-envelope encryption, and selected secrets | Table scans, evidence deletion, KMS administration, IAM changes, Bedrock |
| Evidence verifier | Read evidence objects, KMS public-key/verification operations | S3 writes/deletes, KMS signing |
| Web frontend | Public API access and Cognito browser flows | DynamoDB, S3 evidence bucket, KMS, Secrets Manager |
| CI deploy | Terraform deployment permissions scoped to project resources and remote state | Organization/account administration |
| Human developer | Assume deployment/read-only roles through short-lived credentials | Long-lived access keys in repository or CI variables |

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
AGENTPAY_API_ORIGIN=<http_api_url>
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

No step may require an undocumented console change except initial account/Bedrock provider access. If a console action is unavoidable, add it here with the exact verification command.

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

Create alarms for any evidence-write failure, repeated payment replay, 5xx spikes, facilitator failure rate, and seller timeout rate. Alarms notify a configured development channel; the notification integration is supplied at deployment time.

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
