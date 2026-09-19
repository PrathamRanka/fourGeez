# Clean-account seller onboarding rehearsal

Status: **Harness implemented; deployed execution remains pending after issues 1-3 merge.**

This runbook covers issue 4 from `issues.md`. It creates a genuinely new test
seller and follows the production-shaped seller path through publication and
dashboard readiness. It does not open buyer checkout, create a purchase intent,
authorize a buyer wallet, settle funds, deploy code, or mutate AWS/Vercel
configuration.

## Covered path

The harness reports one named pass/fail result for each boundary:

1. seller sign-up through the deployed web BFF and Cognito email verification;
2. Cognito authentication and seller API authorization;
3. seller/storefront creation against a supplied HTTPS service origin;
4. ES256 seller-service activation;
5. seller payout-destination ownership verification with `personal_sign`;
6. separately operated launch-entitlement readiness;
7. reveal-once project-key creation;
8. local connector project-key exchange and authenticated MCP initialization;
9. maintained-stack detection and bounded OpenAPI route analysis;
10. seller-confirmed MCP route configuration;
11. deterministic route validation and signed sandbox validation;
12. seller-confirmed MCP publication;
13. signed public storefront discovery containing the published route; and
14. authenticated dashboard API and browser readiness.

The seller wallet proves only ownership of the payout destination. The script
contains no buyer-wallet logic and calls no checkout, intent, `/pay`, payment
proof, transaction, receipt, or dispute endpoint.

## Prerequisites

- Issues 1-3 are merged and deployed to the environment under test.
- Node.js 24 or newer and npm 11 or newer are installed.
- `npm ci` has completed, followed by `npm run build:mcp-connector`.
- A unique approved private email inbox is available. Reusing an existing
  Cognito account is an intentional failure because this is a clean-account
  rehearsal.
- A seller-controlled Base Sepolia payout wallet is available. Its private key
  is provided only through the process environment and must never be pasted
  into a command argument, terminal transcript, issue, screenshot, or evidence
  file.
- A public HTTPS seller fixture is already deployed. It must use the version-2
  AgentPay execution-capability verifier, expose
  `POST /.well-known/agentpay/sandbox`, and expose the selected route from the
  supplied OpenAPI document. The fixture and its OpenAPI must describe a
  side-effect-free rehearsal product.
- The launch-entitlement operator is available to follow
  `LAUNCH_ENTITLEMENT.md` after the harness prints the new non-secret
  `sellerId`. The harness polls for the result; it never invokes AWS or grants
  its own entitlement.
- The deployed Cognito app client permits `USER_PASSWORD_AUTH`, matching the
  web application's production adapter.

## Environment

Set values in the current shell or an ignored local secret manager. Do not put
them in a tracked `.env` file.

```powershell
$env:AGENTPAY_REHEARSAL_ACK = "create-clean-test-account"
$env:AGENTPAY_REHEARSAL_WEB_ORIGIN = "https://<deployed-web-origin>"
$env:AGENTPAY_REHEARSAL_API_ORIGIN = "https://<deployed-api-origin>"
$env:AWS_REGION = "ap-south-1"
$env:AGENTPAY_REHEARSAL_COGNITO_CLIENT_ID = "<deployed-web-client-id>"
$env:AGENTPAY_REHEARSAL_EMAIL = "<unique-approved-private-test-recipient>"
$env:AGENTPAY_REHEARSAL_PAYOUT_PRIVATE_KEY = Read-Host "Seller payout testnet private key" -MaskInput
$env:AGENTPAY_REHEARSAL_SERVICE_ORIGIN = "https://<seller-fixture-origin>"
$env:AGENTPAY_REHEARSAL_MANIFEST_PATH = "<path-to-committed-package.json-or-other-supported-manifest>"
$env:AGENTPAY_REHEARSAL_OPENAPI_PATH = "<path-to-bounded-seller-openapi.yaml>"
$env:AGENTPAY_REHEARSAL_FRAMEWORK = "node"
$env:AGENTPAY_REHEARSAL_EXPECTED_STACK = "express"
$env:AGENTPAY_REHEARSAL_ROUTE_METHOD = "POST"
$env:AGENTPAY_REHEARSAL_ROUTE_PATH = "/research"
$env:AGENTPAY_REHEARSAL_ROUTE_DISPLAY_NAME = "Research report"
$env:AGENTPAY_REHEARSAL_ROUTE_SLUG = "research-report"
$env:AGENTPAY_REHEARSAL_ROUTE_AMOUNT = "100000"
$env:AGENTPAY_REHEARSAL_EVIDENCE_PATH = ".cache/rehearsals/clean-seller.json"
```

`AGENTPAY_REHEARSAL_FRAMEWORK` is the repository-analysis family (`node`,
`go`, `python`, `dotnet`, `java`, `ruby`, or `php`).
`AGENTPAY_REHEARSAL_EXPECTED_STACK` is the exact maintained detection such as
`express`, `nextjs`, `go-net-http`, or `fastapi`. The route method and path must
match one proposal in the supplied OpenAPI document.

The default entitlement wait is 30 minutes. Override it with
`AGENTPAY_REHEARSAL_ENTITLEMENT_WAIT_MS` when the operator window is known.
`AGENTPAY_REHEARSAL_VERIFICATION_CODE` may be injected by an approved secret
runner; otherwise the harness prompts without recording the code.

## Execute

Run the focused contract test first:

```powershell
npm run test:seller-onboarding-rehearsal
```

Then start the rehearsal:

```powershell
npm run rehearse:seller:clean
```

When the script prints the new `sellerId`, the entitlement operator performs a
fresh dry run and reviewed apply using `LAUNCH_ENTITLEMENT.md`. No other manual
data repair is permitted. The harness resumes automatically when the
authoritative entitlement reports active network access.

Before `configure_route` and `publish_route`, the harness displays the bounded
non-secret route terms and requires an exact typed confirmation. It mints each
five-minute confirmation grant only after that review and never prints the
grant.

## Evidence and failure handling

The evidence file is created with exclusive-create semantics and restrictive
file permissions. Use a path under ignored `.cache/rehearsals/`. It contains
only allowlisted operational facts such as seller, credential, destination,
route, version, publication revision, public origins, and named pass results.
The recorder rejects fields or values that resemble tokens, cookies, project
keys, confirmation grants, signatures, proofs, passwords, or authorization
headers.

The generated password, Cognito access token, seller payout private key,
ownership challenge/signature, project key, MCP access token, and confirmation
grants stay in process memory and are never printed or written to evidence.
Connector diagnostics are already reduced to status, stable error code, and
request ID. Each failure is prefixed with the exact failed step.

On failure, record the named failing step and diagnose the authoritative
service before rerunning with another clean email address. Do not edit database
records to make the rehearsal pass. The harness intentionally performs no
destructive account cleanup; retention or deletion must follow an approved
operator procedure.
