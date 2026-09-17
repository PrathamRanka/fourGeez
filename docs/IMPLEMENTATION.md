# Task-by-task implementation plan

Status legend: `[ ]` not started, `[-]` in progress, `[x]` complete, `[!]` blocked.

Tasks must be completed in ID order unless their listed dependencies are already complete. Every implementation task includes tests and relevant documentation updates before it is marked complete.

## Milestone M0 — Documentation contract

- [x] **DOC-001** Create the documentation index and authority rules.
- [x] **DOC-002** Lock product and architecture decisions.
- [x] **DOC-003** Define runtime architecture and trust boundaries.
- [x] **DOC-004** Define persisted entities and state machines.
- [x] **DOC-005** Publish OpenAPI 3.1 REST contract.
- [x] **DOC-006** Publish AsyncAPI WebSocket contract.
- [x] **DOC-007** Publish PlantUML system and sequence diagrams.
- [x] **DOC-008** Publish AWS setup and teardown runbook.
- [x] **DOC-009** Publish security requirements and threat model.
- [x] **DOC-010** Publish test and release gates.
- [x] **DOC-011** Record verified external sources and open protocol questions.

M0 acceptance: all documents exist, cross-reference each other, and contain no unresolved implementation choices for the hackathon scope.

## Milestone M1 — Repository foundations

- [x] **REP-001** Create `apps/web`, `cmd/api`, `internal`, `infra`, and `examples` workspaces. Depends on DOC-005–DOC-010.
- [x] **REP-002** Add root commands for build, test, lint, local development, and contract validation.
- [x] **REP-003** Add `.env.example` files containing names but no secrets.
- [x] **REP-004** Add CI jobs for Go tests, web lint/typecheck/build, OpenAPI validation, and CDK synthesis.
- [x] **REP-005** Add local development configuration with a mock payment mode and demo seller.

M1 acceptance: a clean checkout can install dependencies and run all empty-project checks using documented commands.

## Milestone M2 — Go domain and persistence

- [x] **BE-001** Implement shared IDs, money type, UTC timestamps, validation errors, and idempotency interface.
- [x] **BE-002** Implement seller and paid-route domain models.
- [x] **BE-003** Implement immutable purchase intents and canonical request hashing.
- [x] **BE-004** Implement deterministic policy evaluation for approval thresholds.
- [x] **BE-005** Implement approval sessions, invitations, decisions, expiration, veto, and approval-token validation.
- [x] **BE-006** Implement transaction state machine with guarded transitions.
- [x] **BE-007** Implement evidence event hashing, chaining, signing interface, and chain verification.
- [x] **BE-008** Implement dispute classification rules.
- [x] **BE-009** Implement in-memory repositories for local development and unit tests.
- [x] **BE-010** Implement DynamoDB repositories and conditional writes.

M2 acceptance: domain tests cover valid transitions, invalid transitions, replay, expiration, modified intents, chain tampering, and every dispute rule.

## Milestone M3 — HTTP and WebSocket API

- [x] **API-001** Add request IDs, structured errors, panic recovery, CORS, logging, and authentication middleware.
- [x] **API-002** Implement seller onboarding and route configuration endpoints.
- [x] **API-003** Implement manifest and `llms.txt` generation.
- [x] **API-004** Implement purchase-intent endpoints.
- [x] **API-005** Implement approval-session and decision endpoints.
- [x] **API-006** Implement WebSocket connection registration and approval event publication.
- [x] **API-007** Implement transaction and evidence-read endpoints.
- [x] **API-008** Implement dispute creation and retrieval.
- [x] **API-009** Add OpenAPI conformance tests for every M3 endpoint and its documented error responses. The `/pay` proxy operations remain deferred to PAY-003 through PAY-009 and receive conformance coverage with those tasks.

M3 acceptance: generated requests from the OpenAPI examples pass against the local API, and WebSocket clients can reconnect and recover session state.

## Milestone M4 — x402 payment and seller proxy

- [x] **PAY-001** Verify the selected official x402 Go SDK version and record it in `SOURCES.md` before adding the dependency.
- [x] **PAY-002** Implement a payment adapter interface and deterministic mock adapter.
- [x] **PAY-003** Implement x402 testnet challenge creation using the verified SDK.
- [x] **PAY-004** Implement facilitator verification with timeout, retry classification, and proof replay protection.
- [x] **PAY-005** Implement paid-route resolution and approval precondition handling.
- [x] **PAY-006** Implement SSRF-safe upstream forwarding with method/path allowlisting and response-size limits.
- [x] **PAY-007** Implement per-seller HMAC request signatures.
- [x] **PAY-008** Enforce exactly-once forwarding through conditional transaction claims.
- [x] **PAY-009** Record challenge, verification, forwarding, and delivery evidence without raw credentials.

M4 acceptance: one real testnet payment invokes the demo seller once; invalid, expired, modified, and replayed proofs never invoke it.

## Milestone M5 — Bedrock buyer and fallback client

- [x] **AGT-001** Implement tool schemas matching the documented API operations.
- [x] **AGT-002** Implement Bedrock invocation with model ID supplied through configuration.
- [x] **AGT-003** Validate every model tool call through the same Go domain services used by HTTP clients.
- [x] **AGT-004** Implement a deterministic buyer client that can complete the full flow without Bedrock.
- [x] **AGT-005** Add budget, maximum-price, timeout, and tool-call-count limits.

M5 acceptance: Bedrock cannot execute an unknown route, change an approved intent, exceed the configured maximum, or access wallet secrets; fallback mode completes the same transaction.

## Milestone M6 — Automated seller launch

- [x] **AUT-001** Define the automated seller-launch product contract, security boundaries, buyer channels, and implementation order.
- [x] **AUT-002** Define and implement seller-scoped integration credentials with read, configure, publish, validate, and rotate scopes.
- [x] **AUT-003** Publish a remote MCP server with authenticated read-only seller, storefront, route, transaction-summary, and integration-document resources.
- [x] **AUT-004** Add idempotent MCP mutation tools for storefront configuration, draft-route creation, price changes, validation, and publication with explicit confirmation metadata. Initial seller/storefront creation remains in the seller API because integration credentials are seller-scoped.
- [x] **AUT-005** Implement deterministic OpenAPI and repository-manifest analysis that proposes supported paid routes without publishing them automatically.
- [x] **AUT-006** Implement maintained seller-request verification packages for the initial supported Go, Node.js, and Python server frameworks.
- [x] **AUT-007** Publish versioned Claude Code, Codex, and generic MCP setup bundles that install verification, generate storefront integration code, and run tests.
- [x] **AUT-008** Implement a sandbox validator that verifies discovery, signature handling, payment gating, and exactly-once fulfillment before publication.

M6 acceptance: a seller can connect a supported coding agent, review generated changes, approve route publication, and pass a sandbox purchase without manually implementing AgentPay protocols or exposing credentials.

## Parallel track R1 — Recommendation research

This track may begin after M1 and must not block or modify the payment-critical milestones.

- [x] **RL-000** Scaffold the isolated Python workspace with file-level contracts and TODOs.
- [x] **RL-001** Implement strict versioned recommendation and outcome contracts.
- [x] **RL-002** Implement candidate validation and deterministic feature construction.
- [x] **RL-003** Implement and test the deterministic ranking baseline.
- [x] **RL-004** Implement reproducible synthetic data generation.
- [x] **RL-005** Define configurable, auditable reward components.
- [x] **RL-006** Implement an offline contextual bandit behind the common ranking interface.
- [x] **RL-007** Implement baseline comparison, confidence intervals, and regression thresholds.
- [ ] **RL-008** Implement the bounded JSON service adapter after offline evaluation passes.
- [ ] **RL-009** Add reproducible simulation, training, evaluation, and serving commands.
- [ ] **RL-010** Publish an evaluation report that clearly labels synthetic versus real data.

R1 acceptance: the model ranks only eligible offers, beats or matches the deterministic baseline on predefined synthetic scenarios, reproduces results from fixed seeds, and has no access to authorization or payment capabilities.

## Milestone M7 — Human storefront and unified commerce

- [ ] **HUM-001** Verify the selected human-checkout provider, lock merchant and settlement responsibilities, and update OpenAPI and data-model compatibility notes.
- [ ] **HUM-002** Implement hosted human checkout creation against the same immutable purchase-intent rules used by agents.
- [ ] **HUM-003** Implement authenticated, idempotent payment callbacks with replay protection and no success-redirect trust.
- [ ] **HUM-004** Persist purchase channel and payment rail so human and agent sales share transaction, fulfillment, evidence, and dispute behavior.
- [ ] **BIL-001** Implement seller subscription plans without taking custody of buyer-to-seller x402 funds.
- [ ] **BIL-002** Implement auditable metered billing from successful transaction records.
- [ ] **WEB-001** Implement design tokens, typography, responsive shell, keyboard focus, and reduced-motion behavior.
- [ ] **WEB-002** Implement seller onboarding, project credentials, product-route configuration, validation, and publication controls.
- [ ] **WEB-003** Implement seller-branded storefront and product-detail pages generated from published paid routes.
- [ ] **WEB-004** Implement human checkout, success, cancellation, fulfillment, and purchase-history experiences.
- [ ] **WEB-005** Implement the unified seller dashboard for human and agent transactions, revenue, payment status, and filters.
- [ ] **WEB-006** Implement buyer chat/tool activity view with deterministic fallback indicator.
- [ ] **WEB-007** Implement live approval page, decision controls, REST fallback, and expiration state.
- [ ] **WEB-008** Implement evidence verification and dispute creation/resolution views.
- [ ] **WEB-009** Add loading, empty, retryable error, terminal error, disabled, and permission-denied states.

M7 acceptance: the same published product can be purchased through the hosted human storefront and the agent/x402 path, both produce one normalized transaction history, and all primary screens pass responsive, accessibility, and secret-exposure checks.

## Milestone M8 — AWS infrastructure and operations

- [ ] **AWS-000** Replace the existing CDK placeholder with Terraform modules, remote state, environment configuration, and CI validation.
- [ ] **AWS-001** Bootstrap the development AWS account and Terraform state backend.
- [ ] **AWS-002** Create DynamoDB tables and required secondary indexes.
- [ ] **AWS-003** Create versioned Object Lock evidence bucket and KMS signing key.
- [ ] **AWS-004** Create Secrets Manager entries and least-privilege IAM roles.
- [ ] **AWS-005** Deploy Lambda, HTTP API, WebSocket API, MCP endpoint, stages, throttles, and access logs.
- [ ] **AWS-006** Configure Cognito seller authentication.
- [ ] **AWS-007** Configure Bedrock model access and runtime permissions.
- [ ] **AWS-008** Deploy Next.js and configure environment-specific API origins.
- [ ] **AWS-009** Add CloudWatch dashboards and alarms for API, MCP, checkout, facilitator, evidence, and seller failures.
- [ ] **AWS-010** Verify teardown behavior while retaining protected evidence resources.

M8 acceptance: a new development environment can be deployed from committed Terraform without console-only changes or dual ownership with CDK.

## Milestone M9 — Demo and release gate

- [ ] **REL-001** Seed the demo seller and two routes: below-threshold and approval-required.
- [ ] **REL-002** Run unit, integration, contract, web accessibility, and end-to-end suites.
- [ ] **REL-003** Complete one real x402 testnet transaction and preserve its evidence bundle.
- [ ] **REL-004** Demonstrate two-person approval on separate clients.
- [ ] **REL-005** Demonstrate duplicate, non-delivery, and quality dispute outcomes.
- [ ] **REL-006** Rehearse deterministic fallback and AWS dependency failures.
- [ ] **REL-007** Connect a coding agent, generate a seller integration, approve publication, and pass the sandbox validator.
- [ ] **REL-008** Complete one human checkout and one agent/x402 purchase for the same storefront and show both in the seller dashboard.
- [ ] **REL-009** Run the complete demo three consecutive times without manual data repair.

M9 acceptance: all release gates in `TEST_PLAN.md` pass, generated changes are reviewable, both buyer channels work, and mocked behavior is visibly labeled.

## Post-hackathon backlog

- Full seller SDKs beyond the maintained verification packages.
- Seller-controlled reimbursement adapter.
- Enterprise authentication and configurable N-of-M approval.
- Protocol adapter interface for additional rails.
- Physical-product inventory, shipping, taxation, and returns.
- Data export, retention controls, and incident runbooks.
- External security review and regulatory counsel before real funds.

## Commit convention

Use Conventional Commits with one logical change per commit:

```text
docs: define API and architecture contracts
feat(api): add purchase intent creation
feat(payments): verify x402 testnet proof
test(approvals): cover expired and vetoed sessions
fix(proxy): prevent duplicate upstream forwarding
```

Each commit must leave relevant checks passing. Do not mark a task complete until its acceptance tests and documentation are included.
