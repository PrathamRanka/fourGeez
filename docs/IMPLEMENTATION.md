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
- [x] **STK-003** Add verified package and setup-bundle support for ASP.NET Core, Spring Boot, Rails, and Laravel before advertising those ecosystems as supported.
- [x] **WEB-001** Implement design tokens, typography, responsive shell, keyboard focus, and reduced-motion behavior.
- [x] **SITE-001** Implement the complete public AgentPay website for ordinary visitors: responsive navigation, landing hero, live commerce demonstration, seller and buyer explanations, supported-stack showcase, trust and payment sections, pricing preview, FAQ, documentation links, and sign-up/sign-in entry points. Use the shared design system, selected Aceternity/21st.dev/shadcn components, meaningful branded feature names with plain-language descriptions, progressive enhancement, semantic server-rendered content, viewport checks at 360/768/1280/1440, and reduced-motion behavior. Depends on WEB-001.
- [x] **WEB-002** Implement seller onboarding for storefront creation, verified wallet setup, project credentials, MCP configuration, setup-prompt copy, and sandbox status.
- [x] **WEB-003** Implement product-route draft, price, validation, publish, pause, archive, and emergency-disable controls with version history.
- [x] **WEB-004** Implement the seller dashboard for gross verified payments, fulfilled sales, failures, disputes, route performance, and asset/network-separated totals.
- [x] **WEB-005** Implement transaction detail, evidence verification, webhook delivery history, and receipt download views.
- [x] **WEB-006** Implement seller-branded storefront and product-detail pages with wallet/x402 purchase instructions and generated SEO/AEO metadata.
- [x] **WEB-007** Implement buyer chat/tool activity view with deterministic fallback indicator.
- [x] **WEB-008** Implement live approval page, decision controls, REST fallback, and expiration state.
- [x] **WEB-009** Implement evidence verification and dispute creation/resolution views.
- [x] **WEB-010** Add loading, empty, retryable error, terminal error, disabled, permission-denied, quota, and suspended-seller states.

M7 acceptance: a seller can verify a payment destination, connect a supported coding agent, publish and pause products, receive x402 funds directly, reconcile every payment, receive signed notifications, and view asset-separated sales and evidence in an accessible dashboard. Generated storefronts expose validated technical SEO, AEO, manifest, and `llms.txt` output without making ranking guarantees.

## Milestone M7.1 — Lean V1 launch core

This milestone is required before AWS deployment or release verification. The
active launch program is deliberately limited to LCH-001 through LCH-014. It
turns the completed M7 feature surfaces into one secure, production-shaped x402
product without making every desirable platform feature a V1 blocker.

The Go modular monolith remains authoritative for identity, subscription,
authorization, payment, transaction, evidence, and publication. Seller-hosted
code remains untrusted. To move quickly without weakening cancellation or key
revocation, Lean V1 performs authoritative persistence checks on every
transaction-critical request; Redis and distributed invalidation are deferred
until scale requires them. The Node implementation remains limited to the local
MCP connector and seller request-verification package.

- [x] **LCH-001** Reconcile `README.md`, `PRODUCT.md`, `ARCHITECTURE.md`, `DATA_MODEL.md`, `SECURITY.md`, `TEST_PLAN.md`, `DECISIONS.md`, the API contracts, and this ledger so that launch status, authentication, subscription enforcement, buyer channels, storefront routes, and current limitations agree. Depends on WEB-010.
- [x] **LCH-002** Lock the public information architecture: the public site explains AgentPay, the authenticated dashboard serves sellers, `/store/{sellerSlug}` represents one seller, `/store/{sellerSlug}/products/{productSlug}` represents one product, human checkout lives on the product page, and the current buyer page becomes the clearly labeled `/demo/agent-checkout` agent-channel demonstration. Depends on LCH-001.
- [x] **LCH-003** Extend the catalog contract and data model with a seller-approved human product display name and readable public product slug while preserving `routeId`, `pathPattern`, method, MIME type, and upstream details as explicit technical fields. Define uniqueness, normalization, immutable references, migration, SEO metadata, manifest, and historical receipt behavior. Depends on LCH-001.
- [x] **LCH-004** Update OpenAPI, MCP, AsyncAPI when affected, seller verification, discovery, receipt, and webhook contracts for short-lived capabilities, entitlement revisions, inactive discovery responses, execution authorization, revocation, and the complete documented error taxonomy. The contracts preserve the M7 development migration baseline, use `/v1/integration-access-tokens` for project-key bootstrap, require the connector for Lean V1 MCP installations, and define durable browser/approval authorization and retry-safe credential rotation. Depends on LCH-001–LCH-003.
- [x] **LCH-005** Define the seller subscription and entitlement lifecycle independently from buyer settlement: `active`, bounded read-only `grace`, `suspended`, `cancelled` with exact `accessEndsAt`, and `closed`; document Stripe Billing event reconciliation, cancellation-at-period-end, non-payment suspension, reactivation, credential rotation, fraud quarantine, historical read-only access, and finalized-payment obligations. Stripe Billing is the verified initial seller-subscription provider; provider callbacks are implemented in LCH-009 and AWS-013. Depends on LCH-001.
- [x] **LCH-006** Define the non-bypassable seller-MCP security boundary: seller-hosted code is untrusted, the official MCP remains cloud-authoritative, the seller API key is only a bootstrap credential, discovery never authorizes transactions, and no seller-side fork receives signing, payment, publication, or transaction authority. Production MCP mutations require a cloud-issued, one-time confirmation grant bound to the seller, credential, tool, target, canonical arguments, resource version, and expiry; caller-asserted confirmation remains development-only. Depends on LCH-004–LCH-005.
- [x] **LCH-007** Provide one supervised local launcher for Next.js, the real Go API, demo seller, mock x402 facilitator, approval WebSocket, deterministic buyer fallback, and disposable in-memory persistence. It validates configuration and readiness, prints useful URLs, surfaces failures, and terminates every child after one Ctrl+C on Windows. Depends on LCH-004.
- [x] **LCH-008** Finish the guarded `launch-ready` development seed and dependency health checks. Include active and incomplete sellers, below-threshold and approval-required products, successful/pending/failed/disputed transactions, evidence and webhook examples, reset support, and fail-closed readiness for transaction-critical dependencies. Depends on LCH-007.
- [ ] **LCH-009** Implement launch subscription enforcement and credential revocation: authoritative `SellerEntitlement`, authenticated/idempotent Stripe events, exact `accessEndsAt`, hardened `apc2` project keys, rotation/revocation, quotas, and append-only security audit. Missing or unavailable entitlement state must fail closed. Lean V1 does not cache transaction-critical authorization. Depends on LCH-005–LCH-006.
- [ ] **LCH-010** Implement the required local MCP connector and cloud authorization path: project-key bootstrap, 2–5 minute ES256 access capabilities, JWKS rotation, exact per-tool scopes, authoritative entitlement and credential checks on every request, seller-issued mutation confirmation, ownership, idempotency, quota, and bounded JSON-RPC. Project keys are never accepted by `/mcp`; direct OAuth MCP clients are deferred. Depends on LCH-009.
- [ ] **LCH-011** Implement seller identity and the essential operating journey: Cognito production authentication plus a contract-equivalent local adapter, secure cookie/BFF sessions and CSRF, claim-derived ownership, deterministic redirects, persisted resumable onboarding, Stripe plan/portal state, and an authenticated dashboard for products, transactions, evidence, webhooks, billing, credentials, and settings. One seller owner per account is the V1 limit. Depends on LCH-009–LCH-010.
- [ ] **LCH-012** Implement authoritative publication and discovery: verified payment destination and service endpoint, publication readiness, short-lived signed AgentPay-hosted seller/product manifests, inactive tombstones, readable storefront URLs, and fresh entitlement/product checks before intent or challenge creation. Seller-hosted metadata is only a discovery hint; the global marketplace directory and ranking are deferred. Depends on LCH-009–LCH-011.
- [ ] **LCH-013** Complete the shared browser and agent x402 commerce path: immutable intent and quote, optional approval, official SDK challenge/verification/settlement, exact amount/asset/network/destination/resource checks, replay protection, cancellation race policy, atomic finalized-to-forwarding claim, short-lived ES256 execution capability, maintained seller verification packages, exactly-once fulfillment, receipt/evidence, and a bounded manual remediation/refund-recording flow. Depends on LCH-009–LCH-012.
- [ ] **LCH-014** Complete the launch surface and release gate: accessible responsive public pages, authentication, onboarding, dashboard, storefront checkout, agent demo, approval, payment, fulfillment, receipts, evidence, disputes, billing and support; consistent documented error states; required security notifications; an audited operator suspension/replay runbook; privacy/terms/security pages; full real-runtime browser E2E; cancellation/fork/replay/concurrency tests; screenshots at 360/768/1280/1440; Lighthouse budgets; and the full lint/typecheck/test/build baseline. Depends on LCH-008–LCH-013.

### Deferred post-launch backlog

The former LCH-015 through LCH-051 items below are retained as design history
and backlog input only. They are not Lean V1 launch blockers unless explicitly
absorbed by LCH-008 through LCH-014 above.
- [ ] **LCH-015** Add Redis as a non-authoritative entitlement, revocation, replay, idempotency, rate-limit, and discovery cache using the key model in `scripts/flaws.md`. Transaction-critical authorization must fail closed when current state cannot be established; public discovery may only fail soft to an expired or inactive result. Depends on LCH-010 and LCH-014.
- [ ] **LCH-016** Implement durable database/outbox-driven invalidation for subscription changes, API-key revocation, account closure, and administrative suspension: commit the authoritative state and epoch increment atomically, publish cache invalidation after commit, recover missed Pub/Sub events by revision, and remove active discovery listings. Depends on LCH-015.
- [ ] **LCH-017** Keep the official MCP server at the AgentPay cloud endpoint and derive seller identity only from the authenticated capability. Revalidate the exact scope, plan feature, quota, target ownership, confirmation, idempotency, and domain rules for every resource, prompt, and tool; a top-level `read` capability must never authorize mutation tools. Depends on LCH-014–LCH-016.
- [ ] **LCH-018** Implement an optional TypeScript/Node.js local MCP connector matching the reference design in `scripts/flaws.md`. It may exchange the project key, cache a short-lived access token with a safety window, retry once after `401`, and proxy bounded JSON-RPC requests; it may not cache authorization decisions, verify payments, publish locally, sign transactions, or continue when cloud access is revoked. Depends on LCH-012 and LCH-017.
- [ ] **LCH-019** Separate discovery from authorization. Add short-lived signed AgentPay-hosted manifests with seller/product IDs, display names, public slugs, frozen public pricing metadata, publication revision, availability, issued/expiry timestamps, and canonical origin; treat seller-hosted manifests and `llms.txt` as untrusted candidate discovery input. Depends on LCH-003–LCH-004 and LCH-016.
- [ ] **LCH-020** On cancellation, suspension, or closure, invalidate CDN and Redis discovery entries, remove the seller from the AgentPay network index, publish a signed inactive tombstone, and reject new intent creation even when a stale external manifest remains readable. Depends on LCH-016 and LCH-019.
- [ ] **LCH-021** Enforce fresh subscription, seller, route, verified destination, publication, intent, policy, and exact frozen-quote checks before purchase-intent creation and x402 challenge issuance. Discovery data alone must never produce a payment challenge. Depends on LCH-010 and LCH-019.
- [ ] **LCH-022** Recheck authoritative subscription status immediately before x402 verification and settlement. Use the pinned official x402 SDK for canonical payloads and `PAYMENT-REQUIRED`, `PAYMENT-SIGNATURE`, and `PAYMENT-RESPONSE` handling; reject underpayment, overpayment, wrong asset/network/destination/resource, modified intent, duplicate proof, expiry, and replay. Depends on LCH-021.
- [ ] **LCH-023** Lock the cancellation race policy: cancellation before settlement blocks the transaction; a payment finalized before normal cancellation creates a bounded buyer-facing fulfillment obligation that may execute exactly once; emergency fraud quarantine requires an explicit refund/incident path and may never silently retain buyer funds without delivery. Depends on LCH-005 and LCH-022.
- [ ] **LCH-024** Implement cloud-only transaction execution capabilities issued only after successful settlement and an atomic `FINALIZED` to `FORWARDING` claim. Bind each 30–60 second one-time token to seller, route, transaction, HTTP method, path, canonical body hash, audience, JTI, issuer, and expiry. Only the conditional-write winner may call the seller. Depends on LCH-022–LCH-023.
- [ ] **LCH-025** Extend the maintained Node seller-verification package to verify AgentPay execution capabilities against rotated JWKS, preserve and hash raw request bytes before parsing, enforce audience/method/path/body binding, reject expiry and replay, and use `transactionId` as seller-side idempotency input. Keep the existing Go and Python verification packages contract-equivalent. Depends on LCH-024.
- [ ] **LCH-026** Implement one-time execution-JTI consumption and replay protection with Redis `SET NX` plus authoritative cloud transaction state. Removing or forking seller middleware may weaken only the seller endpoint; it must not mint an AgentPay token, create an official transaction, or alter the cloud exactly-once claim. Depends on LCH-015 and LCH-024–LCH-025.
- [ ] **LCH-027** Add append-only, allowlisted security audit events for API-key lifecycle and exchange, token issuance/denial, subscription transitions, epoch changes, MCP decisions, discovery publication/invalidation, payment challenge/verification/settlement, execution grant/consumption/replay, forwarding, and administrative quarantine. Record request/seller/credential/transaction references and reason codes without raw keys, JWTs, cookies, wallet signatures, payment proofs, or seller secrets. Depends on LCH-010–LCH-026.
- [ ] **LCH-028** Implement Cognito-backed production seller registration, verification, sign-in, refresh, recovery, sign-out, expiry, and revocation plus a contract-compatible local development identity adapter. Use secure HTTP-only same-site cookies, CSRF protection, validated relative return paths, and no tokens in URLs or browser storage. Depends on LCH-004 and must align with AWS-006.
- [ ] **LCH-029** Protect every dashboard page, server action, route handler, and backend mutation with authenticated seller ownership derived from claims. Remove `sellerId` query parameters as an authorization source; if a future multi-store account is supported, use an authorized resource selector rather than trusting URL identity. Depends on LCH-028.
- [ ] **LCH-030** Implement redirect and account-state policy: a new seller reaches the first incomplete onboarding step, an established seller reaches the dashboard overview, an incomplete seller resumes deterministically, and an expired session returns to sign-in while preserving only a validated relative destination. Depends on LCH-028–LCH-029.
- [x] **LCH-031** Replace internal-first terminology throughout the UI: storefront name, public storefront URL, service API URL, product name, product URL, decimal price, verified payment destination, project connection key, test purchase, publish product, and payment/delivery proof. Put route ID, path pattern, HTTP method, MIME type, network internals, timeout, and atomic amounts under an advanced technical section. Completed before the Lean V1 consolidation and retained as part of LCH-014 frontend quality. Depends on LCH-002–LCH-003.
- [ ] **LCH-032** Add an authenticated seller dashboard overview and coherent links among onboarding, products, storefront preview, analytics, transactions, evidence, receipts, webhooks, disputes, audit history, plans, documentation, and settings. Remove or implement controls that currently appear disabled or unfinished. Depends on LCH-029 and LCH-031.
- [ ] **LCH-033** Rebuild onboarding as a persisted, resumable server-authoritative journey: account verification; storefront/service connection; payment-destination verification; project-key creation and one-time save; coding-agent connection verification; product discovery/review/naming/pricing; sandbox purchase; storefront preview; and explicit publication. Block publication until all mandatory conditions pass. Depends on LCH-017–LCH-018 and LCH-028–LCH-032.
- [ ] **LCH-034** Rename and complete the agent buyer demonstration so it performs bounded discovery, eligibility/ranking against need and budget, immutable intent creation, optional approval, x402 payment, fulfillment, receipt, and evidence through the same Go services as browser checkout. It must not select the first catalog route without evaluating the request. Depends on LCH-019–LCH-025.
- [ ] **LCH-035** Complete human storefront checkout on the product page with clear value, exact decimal price, output format, trust state, wallet instructions, approval state, payment progress, fulfillment, receipt, evidence, and eligible dispute actions. Browser and agent channels must share the authoritative quote and commerce pipeline. Depends on LCH-021–LCH-026 and LCH-031.
- [ ] **LCH-036** Verify every seller dashboard operation against authenticated Go API state: product lifecycle, wallet verification/rotation, credentials, analytics, filters/pagination, evidence, receipts, webhook delivery/redelivery, disputes, audit events, plans, quotas, suspension, sandbox readiness, and publication. Refreshing or revisiting must restore authoritative state, and every visible control must work or be removed. Depends on LCH-027 and LCH-029–LCH-035.
- [ ] **LCH-037** Implement one consistent accessible error system for field validation and documented `401`, `403`, `404`, `409`, `422`, `428`, `429`, and `503` responses, including loading, empty, disabled, permission-denied, quota, suspended, retryable, terminal, confirmation, success, and dependency-readiness states without leaking protected data. Depends on LCH-029–LCH-036.
- [ ] **LCH-038** Revamp public documentation and establish the frontend quality baseline: lead with a clear API-to-paid-product thesis, present setup as a visual commerce rail, include copyable clean-checkout/install/configure/start/seed/verify/stop/reset/troubleshooting instructions, distinguish local mock/testnet/production behavior, define consistent page hierarchy/copy/tokens/forms/tables/dialogs/toasts/navigation/status treatments, use existing Aceternity/21st.dev/shadcn components before custom UI, use maintained shadcn/21st.dev charts, preserve Server Components, remove data waterfalls and unnecessary client bundles, support keyboard/reduced-motion use, and verify 360/768/1280/1440 layouts. Depends on LCH-031–LCH-037.
- [ ] **LCH-039** Add full-system browser E2E coverage using the real local Go API, mock facilitator, demo seller, WebSocket service, and disposable seed profile for sign-up, verification, sign-in, resumable onboarding, wallet verification, coding-agent connection, product discovery/review, sandbox purchase, publication, browser purchase, agent purchase, two-person approval, fulfillment, evidence, receipt, analytics, webhooks, dispute, and sign-out. Depends on LCH-007–LCH-038.
- [ ] **LCH-040** Add subscription and fork-resistance integration tests: revoked key blocks exchange; old-epoch access token is denied; period-end cancellation changes at the exact boundary; suspension blocks MCP/intent/challenge/settlement; stale discovery cannot authorize; wrong audience/seller/scope/route is denied; missed Pub/Sub is recovered from outbox; Redis failure fails transaction authorization closed; cancellation before settlement blocks; finalized payment fulfills once after normal cancellation; replayed payment IDs/JTIs fail; concurrent claims invoke the seller once; closed sellers receive discovery tombstones; reactivation rotates credentials; and audit output contains no protected material. Depends on LCH-010–LCH-027.
- [ ] **LCH-041** Implement the seller subscription-billing experience independently from buyer x402 settlement: plan selection, hosted checkout or approved equivalent, upgrades, downgrades, cancellation-at-period-end, reactivation, billing portal, invoices, receipts, payment-method management, failed-payment retries, bounded grace state, and clear effective dates. Provider callbacks and reconciliation must drive the authoritative entitlement projection idempotently; success redirects alone never activate access. Depends on LCH-005, LCH-010, LCH-016, and LCH-028.
- [ ] **LCH-042** Implement a separately authorized internal operator console for seller, subscription, credential, route, transaction, payment, evidence, webhook, dispute, and audit investigation; support credential revocation, seller suspension, transaction quarantine, webhook replay, and documented break-glass actions with reason capture and immutable audit. If team RBAC is deferred, enforce and clearly disclose one seller owner per account. Depends on LCH-027, LCH-028, and LCH-041.
- [ ] **LCH-043** Implement the central AgentPay discovery directory and API so buyers and agents can find unknown active sellers and products by capability, category, price, asset, network, and availability with bounded filters, pagination, canonical links, signed status, deterministic ordering, abuse controls, and no ranking guarantees. Only cloud-authoritative active listings may appear; per-seller manifests remain detailed discovery documents rather than the only entry point. Depends on LCH-019–LCH-020 and LCH-031.
- [ ] **LCH-044** Define and implement the V1 buyer-remediation workflow for non-delivery, duplicate payment, invalid fulfillment, and quality complaints: eligibility windows, required evidence, seller response deadlines, support escalation, deterministic recommendation status, manual or provider-supported reimbursement recording, and an explicit distinction among `refund_recommended`, `refund_pending`, `refund_completed`, and `refund_unavailable`. Do not claim that a refund occurred without an authoritative payment reference. Depends on LCH-023–LCH-027 and LCH-035.
- [ ] **LCH-045** Add seller trust and abuse controls: verified email, verified service-domain ownership, acceptable-use acceptance, report-abuse intake, product/storefront takedown, malicious-content and prompt-injection review, new-seller risk limits, repeated-failure thresholds, suspension appeals, and documented legal/compliance review before accepting real stablecoin payments. Trust decisions must be explainable, seller-scoped, auditable, and unable to weaken payment or SSRF validation. Depends on LCH-027–LCH-030 and LCH-043.
- [ ] **LCH-046** Implement the customer data lifecycle: seller data export, account closure, deletion requests, entity-specific retention schedules, evidence and financial-record retention exceptions, legal hold, backup expiration, credential destruction, discovery removal, and auditable completion. Deletion must never remove records required for unresolved transactions, disputes, security investigations, or binding retention obligations. Depends on LCH-005, LCH-027, and LCH-042.
- [ ] **LCH-047** Complete production reliability operations: define RTO/RPO, automated backups, point-in-time recovery where supported, restore drills, dependency and regional failure procedures, status-page updates, on-call ownership, incident severity and communication rules, dead-letter recovery, safe replay tools, capacity/load tests, and post-incident review. A restore test and dependency-failure rehearsal are release gates, not documentation-only tasks. Depends on LCH-009, LCH-016, LCH-027, and LCH-042.
- [ ] **LCH-048** Implement transactional notifications and customer support flows for account verification, password/session security, subscription payment failure, grace and suspension, credential creation/rotation/revocation, approval requests, publication results, payment and fulfillment failures, disputes, incidents, and recovery. Add seller-controlled notification preferences where optional, preserve mandatory security messages, prevent secret leakage, and provide a visible support/contact path with request correlation. Depends on LCH-027–LCH-030, LCH-041, and LCH-044.
- [ ] **LCH-049** Define and enforce the public API, MCP, discovery, webhook, setup-bundle, and verification-package lifecycle: semantic/versioned contracts, compatibility matrix, supported-version window, deprecation headers and notices, migration guides, changelog, sunset process, fixture gates, and rollback behavior. No deployed seller integration may silently break because AgentPay changed an undocumented payload, token claim, signature, or middleware order. Depends on LCH-004, LCH-017–LCH-019, and LCH-025.
- [ ] **LCH-050** Add tenant cost protection and startup product analytics without weakening financial accounting: attribute API, MCP, Bedrock, facilitator, webhook, evidence-storage, and seller-forwarding costs per seller; enforce budgets and noisy-neighbor limits; alert on abnormal usage; and measure sign-up-to-publish conversion, onboarding abandonment, time to first product, sandbox failures, purchase success, fulfillment failure, seller retention, and cancellation reasons. Product telemetry must be privacy-minimized and remain separate from authoritative transaction, settlement, invoice, and audit records. Depends on LCH-015, LCH-027, LCH-036, and LCH-041.
- [ ] **LCH-051** Perform the final end-to-end frontend product polish across the public site, authentication, onboarding, dashboard overview, products, storefront, agent demo, checkout, approvals, transactions, evidence, receipts, webhooks, disputes, billing, directory, operator console, support, privacy, terms, and security pages. Use one intentional AgentPay visual system and plain-language vocabulary; remove placeholder/dead controls; add polished loading/empty/error/success/confirmation states; verify forms, tables, filters, charts, dialogs, navigation, copy, focus management, screen-reader announcements, contrast, touch targets, reduced motion, responsive behavior, hydration, bundle size, and Lighthouse budgets. Capture and review screenshots at 360, 768, 1280, and 1440 pixels before completion. Depends on LCH-038 and LCH-041–LCH-050.

M7.1 acceptance: one clean command starts and seeds the production-shaped local
system and one Ctrl+C removes it; a seller can register, subscribe, resume
onboarding, connect a coding agent, publish readable products, and operate the
essential dashboard against the real Go backend; browser and agent buyers can
complete the same x402 commerce pipeline; cancellation or key revocation blocks
new MCP, discovery, intent, challenge, verification, and settlement operations
even when seller code remains running or forked; stale discovery cannot
authorize a transaction; finalized payments fulfill exactly once; and all
required security, accessibility, responsive, contract, integration, and E2E
checks pass.

## Milestone M8 — AWS infrastructure and operations

M8 begins only after M7.1 acceptance. Infrastructure must preserve the same
authorization, revocation, transaction, and failure semantics proven locally.

- [ ] **AWS-000** Replace the existing CDK placeholder with Terraform modules, remote state, environment configuration, and CI validation.
- [ ] **AWS-001** Bootstrap the development AWS account and Terraform state backend.
- [ ] **AWS-002** Create DynamoDB tables and required secondary indexes.
- [ ] **AWS-003** Create versioned Object Lock evidence bucket and KMS signing key.
- [ ] **AWS-004** Create Secrets Manager entries, KMS/HSM-backed capability-signing keys, API-key digest pepper storage, JWKS publication/rotation support, and least-privilege IAM roles without exposing signing material to application configuration or sellers.
- [ ] **AWS-005** Deploy Lambda, HTTP API, WebSocket API, MCP endpoint, stages, throttles, and access logs.
- [ ] **AWS-006** Configure Cognito seller authentication.
- [ ] **AWS-007** Configure Bedrock model access and runtime permissions.
- [ ] **AWS-008** Deploy Next.js and configure environment-specific API origins.
- [ ] **AWS-009** Add CloudWatch dashboards and alarms for API, MCP, checkout, facilitator, evidence, and seller failures.
- [ ] **AWS-010** Verify teardown behavior while retaining protected evidence resources.
- [ ] **AWS-011** Provision a private TLS-protected managed Redis-compatible cache for entitlement epochs, bounded entitlement caching, revocation, replay, idempotency, rate limits, and discovery invalidation; configure authentication, subnet/security boundaries, metrics, alarms, and failure testing while retaining DynamoDB as source of truth.
- [ ] **AWS-012** Deploy the durable subscription/credential outbox processor and cache/CDN invalidation consumers with retry, dead-letter handling, revision recovery, and observability.
- [ ] **AWS-013** Configure and verify authenticated subscription-provider callbacks, event replay protection, environment separation, cancellation scheduling, entitlement projection, and secret rotation independently from buyer x402 settlement.

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
- [ ] **REL-011** Revoke a project API key while its MCP access token is still unexpired and demonstrate that the next cloud MCP request fails through the entitlement-epoch check.
- [ ] **REL-012** Cancel or suspend a seller while its local connector and a deliberately modified fork remain running; demonstrate that neither can mint capabilities, publish, create intents, settle payments, or produce official AgentPay receipts/evidence.
- [ ] **REL-013** Load a stale seller-hosted manifest after cancellation and demonstrate that authoritative intent creation and checkout fail while AgentPay discovery returns an inactive tombstone.
- [ ] **REL-014** Exercise cancellation immediately before settlement and immediately after finalized settlement; verify that the first transaction is blocked and the second fulfills exactly once according to the documented buyer-obligation rule.
- [ ] **REL-015** Complete the authenticated browser journey from clean sign-up through resumable onboarding, publication, browser purchase, agent purchase, dashboard reconciliation, dispute, sign-out, and expired-session recovery using the deployed production-shaped environment.

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
