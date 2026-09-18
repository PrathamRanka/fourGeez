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
- [x] **RL-008** Implement the bounded JSON service adapter after offline evaluation passes.
- [x] **RL-009** Add reproducible simulation, training, evaluation, and serving commands.
- [x] **RL-010** Publish an evaluation report that clearly labels synthetic versus real data.

R1 acceptance: the model ranks only eligible offers, beats or matches the deterministic baseline on predefined synthetic scenarios, reproduces results from fixed seeds, and has no access to authorization or payment capabilities.

## Milestone M7 — Seller operations, storefront, and growth

- [x] **WAL-001** Define the seller payment-destination contract and add seller-scoped wallet create, list, and read operations for explicit asset and network pairs.
- [x] **WAL-002** Implement wallet-ownership challenges, verification, guarded activation, and confirmed rotation without storing private keys or signed challenge material.
- [x] **PAY-010** Add payment reconciliation that distinguishes challenged, verified, finalized, fulfilled, failed, and disputed amounts using safe facilitator or network references.
- [x] **ANL-001** Implement seller sales aggregates grouped by asset, network, route, UTC day, and transaction status without combining unlike currencies.
- [x] **API-010** Add bounded seller transaction filters, dashboard-summary endpoints, and cursor pagination for date, route, status, asset, and network.
- [x] **EVT-001** Define seller webhook subscriptions and signed event contracts for payment verified, fulfillment succeeded, fulfillment failed, and dispute changes.
- [x] **EVT-002** Implement idempotent webhook delivery, bounded retries, dead-letter state, replay-safe redelivery, and seller-visible delivery history.
- [x] **RCP-001** Implement downloadable machine-readable purchase receipts backed by transaction and evidence-chain verification.
- [x] **BIL-001** Implement seller plans, quotas, and feature limits independently from buyer-to-seller payment settlement.
- [x] **BIL-002** Implement immutable usage-meter events and invoice exports from successful AgentPay transactions; payment collection for AgentPay invoices remains a separate adapter.
- [x] **AUD-001** Implement seller-visible audit events for credentials, wallets, prices, route lifecycle, publication, webhook configuration, and administrative suspension.
- [x] **OPS-001** Enforce per-seller API, MCP, route, and webhook quotas with deterministic permission-denied and rate-limit responses.
- [x] **SEO-001** Define stack detection and the supported integration matrix for Next.js, React/Vite, Remix, Nuxt, SvelteKit, Astro, Express, Fastify, NestJS, Go `net/http`, Gin, Echo, Fiber, FastAPI, Starlette, Flask, and Django; keep unsupported ecosystems explicitly labeled.
- [x] **SEO-002** Publish setup bundle v2 for Claude Code, Codex, and generic MCP hosts with technical SEO, AEO, and agent-discovery generation using stack-native conventions.
- [x] **SEO-003** Add deterministic checks for metadata, canonical URLs, robots directives, sitemap output, structured data, semantic content, `llms.txt`, manifest consistency, accessibility, and performance budgets. Never promise or report guaranteed search ranking.
- [x] **STK-001** Add maintained integration recipes and focused tests for the supported JavaScript and TypeScript stacks.
- [x] **STK-002** Add maintained integration recipes and focused tests for the supported Go and Python stacks.
- [ ] **STK-003** Add verified package and setup-bundle support for ASP.NET Core, Spring Boot, Rails, and Laravel before advertising those ecosystems as supported.
- [ ] **WEB-001** Implement design tokens, typography, responsive shell, keyboard focus, and reduced-motion behavior.
- [ ] **WEB-002** Implement seller onboarding for storefront creation, verified wallet setup, project credentials, MCP configuration, setup-prompt copy, and sandbox status.
- [ ] **WEB-003** Implement product-route draft, price, validation, publish, pause, archive, and emergency-disable controls with version history.
- [ ] **WEB-004** Implement the seller dashboard for gross verified payments, fulfilled sales, failures, disputes, route performance, and asset/network-separated totals.
- [ ] **WEB-005** Implement transaction detail, evidence verification, webhook delivery history, and receipt download views.
- [ ] **WEB-006** Implement seller-branded storefront and product-detail pages with wallet/x402 purchase instructions and generated SEO/AEO metadata.
- [ ] **WEB-007** Implement buyer chat/tool activity view with deterministic fallback indicator.
- [ ] **WEB-008** Implement live approval page, decision controls, REST fallback, and expiration state.
- [ ] **WEB-009** Implement evidence verification and dispute creation/resolution views.
- [ ] **WEB-010** Add loading, empty, retryable error, terminal error, disabled, permission-denied, quota, and suspended-seller states.

M7 acceptance: a seller can verify a payment destination, connect a supported coding agent, publish and pause products, receive x402 funds directly, reconcile every payment, receive signed notifications, and view asset-separated sales and evidence in an accessible dashboard. Generated storefronts expose validated technical SEO, AEO, manifest, and `llms.txt` output without making ranking guarantees.

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
- [ ] **REL-008** Complete one browser-wallet purchase and one agent/x402 purchase for the same storefront and show both in the seller dashboard without double counting.
- [ ] **REL-009** Run the complete demo three consecutive times without manual data repair.
- [ ] **REL-010** Verify generated storefront metadata, structured data, sitemap, `llms.txt`, and manifest output across the supported stack fixtures.

M9 acceptance: all release gates in `TEST_PLAN.md` pass, generated changes are reviewable, browser-wallet and agent purchase paths work, dashboard totals reconcile by asset and network, and mocked behavior is visibly labeled.

## Deferred milestone H1 — Card checkout

- [ ] **HUM-001** Verify a human card-checkout provider and lock merchant-of-record, settlement, refund, tax, chargeback, and platform-fee responsibilities.
- [ ] **HUM-002** Implement seller payment-account onboarding without exposing financial credentials to AgentPay agents or browser code.
- [ ] **HUM-003** Implement hosted card checkout against the same immutable purchase-intent rules used by x402.
- [ ] **HUM-004** Implement authenticated, idempotent provider callbacks with replay protection and no success-redirect trust.
- [ ] **HUM-005** Persist card payment-rail metadata so card and x402 sales use the same fulfillment, evidence, receipt, and dispute behavior.

## Post-hackathon backlog

- Full seller SDKs beyond the maintained verification packages.
- Card checkout remains deferred until H1; Stripe or another provider is not required for the agent-first x402 release.
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
