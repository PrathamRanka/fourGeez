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
- [ ] **PAY-007** Implement per-seller HMAC request signatures.
- [ ] **PAY-008** Enforce exactly-once forwarding through conditional transaction claims.
- [ ] **PAY-009** Record challenge, verification, forwarding, and delivery evidence without raw credentials.

M4 acceptance: one real testnet payment invokes the demo seller once; invalid, expired, modified, and replayed proofs never invoke it.

## Milestone M5 — Bedrock buyer and fallback client

- [ ] **AGT-001** Implement tool schemas matching the documented API operations.
- [ ] **AGT-002** Implement Bedrock invocation with model ID supplied through configuration.
- [ ] **AGT-003** Validate every model tool call through the same Go domain services used by HTTP clients.
- [ ] **AGT-004** Implement a deterministic buyer client that can complete the full flow without Bedrock.
- [ ] **AGT-005** Add budget, maximum-price, timeout, and tool-call-count limits.

M5 acceptance: Bedrock cannot execute an unknown route, change an approved intent, exceed the configured maximum, or access wallet secrets; fallback mode completes the same transaction.

## Milestone M6 — Web application

- [ ] **WEB-001** Implement design tokens, typography, responsive shell, keyboard focus, and reduced-motion behavior.
- [ ] **WEB-002** Implement seller onboarding and paid-route form.
- [ ] **WEB-003** Implement transaction dashboard and status filters.
- [ ] **WEB-004** Implement the evidence chain-of-custody rail and verification state.
- [ ] **WEB-005** Implement buyer chat/tool activity view with deterministic fallback indicator.
- [ ] **WEB-006** Implement live approval page, decision controls, REST fallback, and expiration state.
- [ ] **WEB-007** Implement dispute creation and resolution explanation.
- [ ] **WEB-008** Add loading, empty, retryable error, terminal error, and permission-denied states.

M6 acceptance: all five primary screens work at 360 px and desktop widths, pass keyboard navigation, and expose no secrets in browser bundles or logs.

## Parallel track R1 — Recommendation research

This track may begin after M1 and must not block or modify the payment-critical milestones.

- [x] **RL-000** Scaffold the isolated Python workspace with file-level contracts and TODOs.
- [ ] **RL-001** Implement strict versioned recommendation and outcome contracts.
- [ ] **RL-002** Implement candidate validation and deterministic feature construction.
- [ ] **RL-003** Implement and test the deterministic ranking baseline.
- [ ] **RL-004** Implement reproducible synthetic data generation.
- [ ] **RL-005** Define configurable, auditable reward components.
- [ ] **RL-006** Implement an offline contextual bandit behind the common ranking interface.
- [ ] **RL-007** Implement baseline comparison, confidence intervals, and regression thresholds.
- [ ] **RL-008** Implement the bounded JSON service adapter after offline evaluation passes.
- [ ] **RL-009** Add reproducible simulation, training, evaluation, and serving commands.
- [ ] **RL-010** Publish an evaluation report that clearly labels synthetic versus real data.

R1 acceptance: the model ranks only eligible offers, beats or matches the deterministic baseline on predefined synthetic scenarios, reproduces results from fixed seeds, and has no access to authorization or payment capabilities.

## Milestone M7 — AWS infrastructure and operations

- [ ] **AWS-001** Bootstrap development AWS account and CDK environment.
- [ ] **AWS-002** Create DynamoDB tables and required secondary indexes.
- [ ] **AWS-003** Create versioned Object Lock evidence bucket and KMS signing key.
- [ ] **AWS-004** Create Secrets Manager entries and least-privilege IAM roles.
- [ ] **AWS-005** Deploy Lambda, HTTP API, WebSocket API, stages, throttles, and access logs.
- [ ] **AWS-006** Configure Cognito seller authentication.
- [ ] **AWS-007** Configure Bedrock model access and runtime permissions.
- [ ] **AWS-008** Deploy Next.js and configure environment-specific API origins.
- [ ] **AWS-009** Add CloudWatch dashboards and alarms for errors, latency, facilitator failures, evidence failures, and seller timeouts.
- [ ] **AWS-010** Verify teardown behavior while retaining protected evidence resources.

M7 acceptance: a new development environment can be deployed from documented commands without console-only changes.

## Milestone M8 — Demo and release gate

- [ ] **REL-001** Seed the demo seller and two routes: below-threshold and approval-required.
- [ ] **REL-002** Run unit, integration, contract, web accessibility, and end-to-end suites.
- [ ] **REL-003** Complete one real x402 testnet transaction and preserve its evidence bundle.
- [ ] **REL-004** Demonstrate two-person approval on separate clients.
- [ ] **REL-005** Demonstrate duplicate, non-delivery, and quality dispute outcomes.
- [ ] **REL-006** Rehearse deterministic fallback and AWS dependency failures.
- [ ] **REL-007** Run the three-minute demo three consecutive times without manual data repair.

M8 acceptance: all release gates in `TEST_PLAN.md` pass and mocked behavior is visibly labeled.

## Post-hackathon backlog

- Design-partner tenant isolation and usage billing.
- Seller-controlled reimbursement adapter.
- Enterprise authentication and configurable N-of-M approval.
- Protocol adapter interface for additional rails.
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
