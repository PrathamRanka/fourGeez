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
- [x] **REP-004** Add CI jobs for Go tests, web lint/typecheck/build, OpenAPI validation, and the infrastructure validation used at that milestone (CDK synthesis, superseded by AWS-000 Terraform validation).
- [x] **REP-005** Add local development configuration with a mock payment mode and demo seller.

M1 acceptance: a clean checkout can install dependencies and run all empty-project checks using documented commands.

## Milestone M2 — Go domain and persistence

Historical note: BE-004 and BE-005 record completed hackathon/M2 work. Their
buyer-side threshold and multi-person approval runtime is deferred and disabled
for Lean V1 under ADR-043. The code and records may remain for compatibility
tests, but no active V1 route, WebSocket, challenge, or transaction depends on
them.

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

Historical note: API-005 and API-006 record completed M3 approval work. Those
approval REST and WebSocket surfaces are not part of the active Lean V1
OpenAPI/AsyncAPI runtime contract and must not be started by the launch stack.

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
- [x] **PAY-005** Implement paid-route resolution and historical approval precondition handling. The approval branch is disabled for Lean V1.
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

M6 acceptance: a seller can connect a supported coding agent, review generated changes, approve route publication, and pass automated non-payment sandbox verification without manually implementing AgentPay protocols, operating buyer checkout, or exposing credentials.

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
- [x] **WEB-008** Implement the historical live approval page, decision controls, REST fallback, and expiration state. This surface is disabled and excluded from Lean V1.
- [x] **WEB-009** Implement evidence verification and dispute creation/resolution views.
- [x] **WEB-010** Add loading, empty, retryable error, terminal error, disabled, permission-denied, quota, and suspended-seller states.
- [x] **WEB-011** Replace the public landing hero's layered shader blobs with a reusable animated radial-gradient background in `components/ui`. Preserve the AgentPay headline, launch actions, and purchase-flow illustration; keep the gradient decorative, responsive, theme-aware, and fully static when reduced motion is requested. Depends on SITE-001.
- [x] **WEB-012** Refine the landing hero to keep the vivid blue, pink, and orange gradient bands outside the central copy area and add a small decorative, pointer-responsive AgentPay mascot animation above the headline. The mascot must not carry meaning, must stop autoplay and interaction motion for reduced-motion users, and must not displace the primary launch actions at mobile widths. Depends on WEB-011.
- [x] **WEB-013** Restore the existing cloud-call-to-action footer on public routes through the shared application shell while keeping authenticated dashboard routes footer-free. Depends on SITE-001.
- [x] **WEB-014** Consolidate public decorative gradients and non-footer shader accents on the landing palette: ink black, electric blue, signal pink, and payment orange. Preserve the restored cloud CTA footer's existing shader colors and CSS, and keep semantic success, warning, error, and chart colors independent. Depends on WEB-013.
- [x] **WEB-015** Make the complete web experience dark-only, remove appearance controls and persisted theme state, align seller authentication with the black product tone, label the repository-to-MCP integration flow, and present the exact maintained stack matrix as a high-contrast developer-platform proof strip. Depends on WEB-014 and STK-003.
- [x] **WEB-016** Focus the landing hero on one seller sign-up action, publish creator and source attribution in the footer and machine-readable discovery, add a generated social-sharing image, and expand canonical search/social metadata without making ranking guarantees. Depends on WEB-015 and SEO-003.
- [x] **WEB-017** Add a public Developers page and footer entry with verified maintainer attribution, public profile links, accessible profile imagery, canonical metadata, and sitemap discovery. Depends on WEB-016.
- [x] **WEB-018** Expand the public landing canvas edge-to-edge, simplify the maintained-stack proof into a large borderless icon rail, and refine the Developers page with direct return navigation, verified founder profiles, and color-treated imagery. Depends on WEB-017.
- [x] **WEB-019** Group the 21 maintained stack recipes into distinct public ecosystem marks and render each mark in its official brand color without repeating language/runtime logos for sibling frameworks. Depends on WEB-018 and STK-003.
- [x] **WEB-020** Replace the landing ticker with the shared shadcn-style Marquee primitive, add soft edge fading and reduced-motion behavior, and publish inherited Open Graph, Twitter, and App Router favicon metadata across public pages. Depends on WEB-019.
- [x] **WEB-021** Refine the public stack proof around seller outcomes, remove non-brand surface tinting, top-align bento copy with its product preview, and add a persistent reduced-motion-safe hero headline reveal without delaying the page behind a preloader. Preserve server-rendered headline semantics. Depends on WEB-020.
- [x] **WEB-022** Slow and split the landing headline reveal, number the seller outcome proof, simplify the maintained-stack claim, sharpen the integration journey, publish truthful $6/$10/$15 launch pricing against implemented limits, and add an indexed Contact page with direct founder email and phone links. Keep Stripe checkout visibly disabled in the development preview. Depends on WEB-021.
- [x] **WEB-023** Restore the landing headline's original gradual Dia text reveal without a permanently visible duplicate layer, keep “Settle on-chain.” on its own line, slow both reveal passes, and ensure React development remounts cannot cancel the animation before completion. The final white headline must remain visible and reduced-motion users must receive it immediately. Depends on WEB-022.
- [x] **WEB-024** Redesign the seller onboarding and dashboard entry around the five founder actions: connect service, confirm payout, review detected products, publish, and monitor sales. Keep buyer checkout absent, map the existing server-authoritative onboarding and integration checks to plain pass/fail health, and present one precise next action. Verified locally with focused web tests, lint, typecheck, build, and responsive screenshots at 360, 768, 1280, and 1440 pixels; deployed verification was not performed. Depends on LCH-011–LCH-014.

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
- [x] **LCH-004** Update OpenAPI, MCP, AsyncAPI when affected, seller verification, discovery, receipt, and webhook contracts for short-lived capabilities, entitlement revisions, inactive discovery responses, execution authorization, revocation, and the complete documented error taxonomy. The contracts preserve the M7 development migration baseline, use `/v1/integration-access-tokens` for project-key bootstrap, require the connector for Lean V1 MCP installations, define durable browser purchase authorization and retry-safe credential rotation, and retain the historical approval AsyncAPI only as explicitly disabled design documentation. Depends on LCH-001–LCH-003.
- [x] **LCH-005** Define the seller subscription and entitlement lifecycle independently from buyer settlement: `active`, bounded read-only `grace`, `suspended`, `cancelled` with exact `accessEndsAt`, and `closed`; document Stripe Billing event reconciliation, cancellation-at-period-end, non-payment suspension, reactivation, credential rotation, fraud quarantine, historical read-only access, and finalized-payment obligations. Stripe Billing is the verified initial seller-subscription provider; provider callbacks are implemented in LCH-009 and AWS-013. Depends on LCH-001.
- [x] **LCH-006** Define the non-bypassable seller-MCP security boundary: seller-hosted code is untrusted, the official MCP remains cloud-authoritative, the seller API key is only a bootstrap credential, discovery never authorizes transactions, and no seller-side fork receives signing, payment, publication, or transaction authority. Production MCP mutations require a cloud-issued, one-time confirmation grant bound to the seller, credential, tool, target, canonical arguments, resource version, and expiry; caller-asserted confirmation remains development-only. Depends on LCH-004–LCH-005.
- [x] **LCH-007** Provide one supervised local launcher for Next.js, the real Go API, demo seller, mock x402 facilitator, deterministic buyer fallback, and disposable in-memory persistence. The Lean V1 launcher does not start the historical approval WebSocket. It validates configuration and readiness, prints useful URLs, surfaces failures, and terminates every child after one Ctrl+C on Windows. Depends on LCH-004.
- [x] **LCH-008** Finish the guarded `launch-ready` development seed and dependency health checks. Include active and incomplete sellers, fixed-price products, successful/pending/failed/disputed transactions, evidence and webhook examples, reset support, and fail-closed readiness for transaction-critical dependencies. Depends on LCH-007.
- [x] **LCH-009** Implement launch subscription enforcement and credential revocation: authoritative `SellerEntitlement`, authenticated/idempotent Stripe events, exact `accessEndsAt`, hardened `apc2` project keys, rotation/revocation, quotas, and append-only security audit. Missing or unavailable entitlement state must fail closed. Lean V1 does not cache transaction-critical authorization. Depends on LCH-005–LCH-006.
- [x] **LCH-010** Implement the required local MCP connector and cloud authorization path: project-key bootstrap, 2–5 minute ES256 access capabilities, JWKS rotation, exact per-tool scopes, authoritative entitlement and credential checks on every request, seller-issued mutation confirmation, ownership, idempotency, quota, and bounded JSON-RPC. Project keys are never accepted by `/mcp`; direct OAuth MCP clients are deferred. A September 20, 2026 regression proves that revoking a project key blocks its already-issued unexpired MCP token before quota consumption or JSON-RPC dispatch. Depends on LCH-009.
- [x] **LCH-011** Implement seller identity and the essential operating journey: Cognito production authentication plus a contract-equivalent local adapter, secure cookie/BFF sessions and CSRF, claim-derived ownership, deterministic redirects, persisted resumable onboarding, Stripe plan/portal state, and an authenticated dashboard for products, transactions, evidence, webhooks, billing, credentials, and settings. One seller owner per account is the V1 limit. Depends on LCH-009–LCH-010.
- [x] **LCH-012** Implement authoritative publication and discovery: verified payment destination and service endpoint, publication readiness, short-lived signed AgentPay-hosted seller/product manifests, inactive tombstones, readable storefront URLs, and fresh entitlement/product checks before intent or challenge creation. Seller-hosted metadata is only a discovery hint; the global marketplace directory and ranking are deferred. Depends on LCH-009–LCH-011.
- [x] **LCH-013** Complete the shared browser and agent x402 commerce path: immutable intent and fixed seller quote, buyer-maximum enforcement without buyer-side approval, official SDK challenge/verification/settlement, exact amount/asset/network/destination/resource checks, replay protection, cancellation race policy, atomic finalized-to-forwarding claim, short-lived ES256 execution capability, maintained seller verification packages, exactly-once fulfillment, receipt/evidence, and the seller-authenticated idempotent `POST /v1/sellers/{sellerId}/disputes/{disputeId}/refund-records` flow. The endpoint records one external full refund only for a finalized `refund_recommended` dispute and never moves funds. Depends on LCH-009–LCH-012.
- [x] **LCH-014** Complete the launch surface and release gate: accessible responsive public pages, authentication, onboarding, dashboard, storefront checkout, agent demo, wallet authorization, payment, fulfillment, receipts, evidence, disputes, manual refund recording, billing and support; consistent documented error states; required security notifications; an audited operator suspension/replay runbook; privacy/terms/security pages; full real-runtime browser E2E; cancellation/fork/replay/concurrency tests; screenshots at 360/768/1280/1440; Lighthouse budgets; and the full lint/typecheck/test/build baseline. Buyer-side approval UI, REST, and WebSocket runtime are excluded. Depends on LCH-008–LCH-013.

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

## Milestone M7.2 — Seller MCP release hardening

This milestone closes evidenced first-time seller integration gaps without
changing the cloud-authoritative authorization boundary completed by LCH-010.
Each task remains independently tested and committed.

- [x] **DX-000** Reconcile the seller MCP security and transport documents with the implemented local LCH-010 runtime: project-key bootstrap, short-lived bearer access at `/mcp`, cloud-issued one-time mutation confirmation, and the remaining AWS/package-distribution release blockers.
- [x] **DX-001** Expose deterministic maintained-stack detection through the authenticated MCP and allow repository route analysis for every language family represented by the exact 21-stack matrix. The coding agent must use bounded committed evidence and cannot select an unevidenced stack.
- [x] **DX-002** Add a Windows/PowerShell-first connector preflight and actionable secret-safe diagnostics, then publish exact Claude Code, Codex, and generic-host setup steps that do not place a project key in committed configuration.
- [x] **DX-003** Gate seller MCP setup and project-key issuance on authoritative seller ownership, verified account/profile state, active launch entitlement, a verified platform-supported testnet payment destination, and HTTPS service readiness. Ineligible sellers receive a prerequisite checklist with completion links and no project-key or MCP invitation. Eligible sellers receive reveal-once credential handling, exact Claude Code/Codex/generic local-connector configuration, Windows PowerShell preflight, credential and connector lifecycle diagnostics (`disconnected`, `connected`, `expired`, or `revoked`), validation results with retry guidance, and canonical storefront/product URLs after publication. Record the first authenticated connector authorization idempotently and fail closed when it cannot be recorded. Keep prices, payout destinations, publication, credential rotation, and deployment seller-confirmed. Stripe subscription checkout remains disabled for this launch.
  - September 20, 2026 production-flow correction: an authenticated, version-bound service-activation mutation enables the ES256 execution-capability integration for an HTTPS origin before project-key issuance. It creates no shared seller HMAC secret; publication still requires the cloud sandbox to verify the generated endpoint.
- [ ] **DX-004** Produce reproducible, self-contained proprietary release artifacts for `@agentpay/local-mcp-connector` and `@agentpay/merchant-sdk`. Validate exact packed contents and clean installation, include the proprietary license and reviewed notice, generate SHA-256 checksums and source-commit provenance, and publish a Windows-first installation runbook for Claude Code, Codex, and generic stdio MCP hosts. Registry publication remains blocked; a release owner may distribute the artifacts only from a protected immutable release after approving customer-use terms and third-party provenance.
- [x] **DX-005** Replace seller-funded onboarding proof with an authoritative automated integration verification result. Reuse the existing sandbox validator and protected seller forwarder; report bounded redacted pass/fail checks for reachability, signed request/response compatibility, closed input/output contracts, fulfillment readiness, payment gating, and replay/idempotency readiness. Persist only the latest route-version-bound result for onboarding, append a seller-scoped audit event for every completed run, and never create an intent or transaction, settle funds, invoke a paid business route, accept a caller-supplied URL, or expose secrets. Issue-register item 2.
- [x] **DX-006** Replace the runnable seller prototype's process-local execution,
  webhook, and fulfillment state with seller-owned DynamoDB adapters. Replay
  claims must be atomic and TTL-bounded; fulfillment claims and completed
  serializable results must be transaction-keyed, conditionally written, and
  retained without TTL so restarts and concurrent instances cannot rerun a
  completed business action. Keep memory adapters explicitly limited to tests
  and single-process local development, fail closed on storage errors, and do
  not add an AgentPay-managed seller-state table or mutate AWS. Issue-register
  item 11.

M7.2 acceptance: an eligible first-time seller can identify an evidenced
maintained stack, select the matching setup workflow, diagnose local connector
configuration safely on Windows, and reach the cloud-authorized MCP boundary.
Ineligible sellers see every missing prerequisite and no project-key or MCP
setup invitation. Prices and payout destinations remain seller-controlled, and
published products expose their canonical storefront URLs in the dashboard.
Package-registry publication and deployed AWS reachability remain explicit
release dependencies rather than being represented as complete.

DX-006 acceptance: two store instances sharing one DynamoDB table admit one
execution JTI, legacy request identifier, or webhook event claim; replay items
carry numeric DynamoDB TTL values; one transaction conditionally starts
fulfillment and later replays the exact stored result; overlapping attempts are
rejected; failed business execution may release its own claim; uncertain
completion remains claimed; the runnable seller prototype selects DynamoDB by
default and permits memory state only through an explicit local-only setting;
focused package tests, type checks, builds, and package-distribution checks
pass.

DX-004 acceptance: two independently generated artifact sets from the same
clean commit have identical package checksums; each tarball installs into an
empty Node.js project without registry access; the connector exposes only its
documented executable and the merchant SDK has no unresolved AgentPay runtime
dependency; packed files contain no source maps, tests, local configuration,
credentials, or secret values; checksum and provenance verification fail
closed; and every documented seller command uses a downloaded immutable
artifact rather than an unsupported Git workspace-subdirectory install.

## Milestone M7.3 — Public discovery and commerce extensions

- [-] **EXT-001** Add `/.well-known/agentpay`, buyer/agent-readable capability metadata, and a deterministic public product directory/search API plus responsive public discovery UI. Maintain a query-only DynamoDB projection atomically with published-route lifecycle changes, use stable lexical ordering and exact normalized-term filtering without ranking claims, and freshly revalidate seller entitlement, publication readiness, route state, and payment destination before returning every result. Update Product, Decisions, Data Model, OpenAPI, web copy, and focused backend/frontend tests. Do not change checkout, payment adapters, SDK packages, or Terraform.

EXT-001 acceptance: people and software agents can start from one public
capability manifest, list or exactly filter currently eligible published
products without a production table scan, follow canonical storefront/product
links, and receive truthful supported-channel and x402 testnet capability
metadata. Directory and manifest data remain non-authoritative for purchases.

EXT-001 implementation status: the machine-readable runtime slice is implemented
and verified locally. `/.well-known/agentpay` and
`GET /v1/discovery/products` are wired into the Go API; published-route writes
maintain bounded listing and exact-term DynamoDB projections atomically; and
focused domain, persistence, service, transport, and OpenAPI tests cover stable
ordering, cursor/filter binding, stale-candidate rejection, and fresh
eligibility checks. The responsive public directory/search UI and its focused
frontend tests remain pending, so EXT-001 stays in progress. AWS deployment and
release verification remain M8/M9 work.

- [x] **EXT-002** Preserve `purchaseIntent` and `transaction` as the canonical
  lifecycle while adding buyer-owned pre-checkout cancellation with an
  expiration-first conditional claim, deterministic single-product exact-price
  breakdowns, an order-compatible derived transaction lifecycle/recovery
  projection, readable seller-reported refund remediation, stronger replay and
  transition coverage, and matching seller dashboard copy. Do not add carts,
  physical inventory, shipping, tax, custody, Stripe, automated refunds, a
  generic `Order` entity, discovery changes, payment-adapter changes, SDK work,
  or Terraform. Depends on LCH-013–LCH-014.

EXT-002 acceptance: cancellation, expiration, and checkout claim are mutually
exclusive under optimistic concurrency; cancelled or expired intents never
produce a payment challenge; exact-price totals contain no invented charges;
transaction lifecycle and recovery projections are deterministic; refund
records remain append-only, seller-scoped, idempotent, readable, and labeled as
seller-reported rather than network-verified; focused backend, persistence,
OpenAPI, and dashboard tests pass.

- [x] **EXT-003** Implement a pinned, server-only TypeScript merchant SDK that
  composes the Node execution verifier, legacy sandbox request verifier,
  webhook signature verification, typed AgentPay merchant contracts, and a
  durable idempotent-fulfillment helper. Formalize one small merchant-adapter
  interface and add tested generic HTTPS, Shopify, and WooCommerce reference
  adapters that accept seller-owned runtime credentials without embedding
  secrets. Add fixtures, runnable examples, setup documentation, and only the
  supported-integration claims proven by focused tests. The SDK and adapters
  never receive payment custody, wallet keys, payment verification authority,
  AgentPay signing authority, publication authority, or core transaction
  persistence. Depends on AUT-006, EVT-002, and LCH-013.

M7.3 acceptance: a TypeScript seller can verify an execution capability and a
signed webhook over exact raw bytes, coordinate exactly-once local fulfillment
through a seller-owned atomic store, and call each documented reference adapter
under focused transport tests. Memory stores remain explicitly local-only,
registry publication remains a release dependency, and Shopify/WooCommerce are
described as tested reference adapters rather than managed or certified
integrations.

- [x] **EXT-004** Publish the runtime-enabled payment capability catalog and deterministic compatibility contract; negotiate only the enabled adapter (currently exact x402 on Base Sepolia USDC in the testnet runtime); provide actionable unsupported-wallet, expired, rejected, facilitator-unavailable, and unknown-settlement recovery responses; and expose the same behavior in browser checkout and payment-specific public copy. Browser wallet checkout remains a presentation path over the existing purchase intent and `/pay` operation. Card checkout, automatic refunds, mainnet, additional networks, and additional assets remain disabled. Acceptance requires contract tests, payment-domain tests for exact amount/network/asset/expiry/nonce/replay/idempotency/safe retry, and accessible checkout tests for every recovery action.

EXT-004 acceptance: every buyer can discover the payment capabilities enabled by the running environment, determine compatibility without guessing, and follow one bounded recovery action after a payment failure without creating a second charge or bypassing the immutable purchase intent.

- [x] **EXT-005** Harden seller-first V1 interoperability with canonical closed
  input/output schemas on route drafts, deterministic route-version contract
  hashes, seller-confirmed publish binding, signed versioned public product
  contracts, discovery revision refresh after approved changes, closed MCP tool
  output schemas with structured content, stable machine-readable errors, and
  truthful capability metadata. Do not add or advertise an AgentPay buyer
  runtime, A2A execution, negotiation, mainnet, multi-currency, ranking, or
  external Bazaar publication. Preserve negotiation analysis as deferred V2
  documentation only. Depends on EXT-001, EXT-004, and LCH-013. No new
  buyer-facing UI or infrastructure changes.

EXT-005 acceptance: a seller can draft a route with bounded closed schemas,
receive a validation result bound to the exact route version and contract hash,
approve and publish only that validated version, and observe the approved
contract in a short-lived signed product document and refreshed discovery
revision. MCP tool listings expose closed output schemas and successful calls
return matching structured content. Unknown schema keywords/object fields,
stale versions/hashes, and malformed requests fail with stable codes. Public
capabilities explicitly keep AgentPay buyer execution, A2A, and negotiation
disabled. Focused domain, transport, persistence, MCP, and OpenAPI checks pass.

## Milestone M8 — AWS infrastructure and operations

M8 begins only after M7.1 acceptance. Infrastructure must preserve the same
authorization, revocation, transaction, and failure semantics proven locally.

- [x] **AWS-000** Replace the existing CDK placeholder with Terraform modules, remote state, environment configuration, and CI validation.
- [x] **AWS-001** Bootstrap the development AWS account and Terraform state backend. The development backend is deployed in Mumbai with encrypted, versioned S3 state, public access blocked, TLS-only access, and verified native S3 lock-file contention.
- [x] **AWS-002** Create DynamoDB tables and required secondary indexes. The Mumbai development table uses on-demand billing, AWS-owned encryption, point-in-time recovery, and the four documented seller, payment, storefront, and route indexes.
- [x] **AWS-003** Create versioned Object Lock evidence bucket and KMS signing key. The Mumbai development bucket uses KMS encryption, versioning, public-access blocking, TLS-only access, and 30-day governance retention; its protected asymmetric P-256 key successfully signs with ECDSA-SHA256.
- [x] **AWS-004** Create Secrets Manager entries, KMS/HSM-backed capability-signing keys, API-key digest pepper storage, JWKS publication/rotation support, and least-privilege IAM roles without exposing signing material to application configuration or sellers. The development account now has KMS-encrypted credential-pepper and confirmation-grant-pepper containers with `AWSCURRENT` versions verified without reading secret values, additive versioned ES256 capability keys, a rotating envelope-encryption key, and independently verified API and evidence-verifier permissions.
- [x] **AWS-005** Deploy the Go Lambda behind one HTTP API serving REST and the `/mcp` endpoint, with stages, throttles, and access logs. The historical approval WebSocket remains disabled and is not deployed for Lean V1. On September 19, 2026, Terraform applied 37 additions, 0 changes, and 0 destroys, and the post-apply plan reported no changes. Mumbai Lambda `agentpay-dev-api` is Active on `provided.al2023` ARM64 with reserved concurrency 5 under the regional quota of 400. HTTP API `sadmp7j94e` is live at `https://sadmp7j94e.execute-api.ap-south-1.amazonaws.com`; `/health/live` and `/health/ready` return `200`, readiness reports `dynamodb`, `evidence_store`, `kms`, `secrets`, and `x402_facilitator` ready, and `/.well-known/agentpay` plus `/v1/payment-capabilities` return `200`. Runtime payment configuration remains locked to credential-free x402 testnet (`https://x402.org/facilitator`, Base Sepolia `eip155:84532`, USDC `0x036CbD53842c5426634e7929541eC2318f3dCF7e`).
- [x] **AWS-006** Configure Cognito seller authentication. The Mumbai
      development environment now has an email-verified seller user pool and a
      no-secret, revocable web/BFF client. The production web adapter implements
      registration, verification, password recovery, sign-in, Cognito
      revalidation, bounded access-token refresh, global sign-out, exact-Origin
      CSRF protection, claim-derived seller hydration, and versioned,
      size-bounded compressed AES-256-GCM-sealed Secure HttpOnly sessions that
      stay below browser cookie limits without exposing Cognito tokens to browser
      JavaScript. API Gateway JWT protection is active for the deployed
      seller-only route families.
- [ ] **AWS-007** Configure Bedrock model access and runtime permissions. Bedrock is disabled and non-blocking for the seller-first V1 deployment.
- [x] **AWS-008** Deploy Next.js and configure environment-specific API origins. On September 19, 2026, Vercel production variables used the Terraform API origin, deployment `dpl_ExWeM7fxdqtUoLK2HUA1cki2X3Ti` was `READY` and aliased to `https://agentpay.prathamranka.in`, and `/api/auth/csrf`, `/sign-in`, and `/docs` returned `200`.
- [-] **AWS-009** Add CloudWatch dashboards and alarms for API, MCP, checkout, facilitator, evidence, seller forwarding, webhook retry/dead-letter, Lambda errors/throttles, API latency, and 5xx responses. Thirteen alarms and the seller operations dashboard were deployed on September 19, 2026 with missing-data-safe low-cost settings. SNS alarm notification actions are not configured and delivery is unverified, so this task remains in progress.
- [ ] **AWS-010** Verify teardown behavior while retaining protected evidence resources.
- [ ] **AWS-011** Provision a private TLS-protected managed Redis-compatible cache for entitlement epochs, bounded entitlement caching, revocation, replay, idempotency, rate limits, and discovery invalidation; configure authentication, subnet/security boundaries, metrics, alarms, and failure testing while retaining DynamoDB as source of truth.
- [ ] **AWS-012** Deploy the durable subscription/credential outbox processor and cache/CDN invalidation consumers with retry, dead-letter handling, revision recovery, and observability.
- [-] **AWS-013** Configure and verify authenticated subscription-provider callbacks, event replay protection, environment separation, cancellation scheduling, entitlement projection, and secret rotation independently from buyer x402 settlement. While Stripe is disabled, cloud environments require an explicit dry-run-first, environment- and version-bound, assumed-role-only launch-entitlement operation. The reviewed plan digest and exact confirmation bind the seller and complete operation; an immutable seller-scoped operation claim makes exact replay idempotent and conflicting replay fail closed; the claim, reconciliation, projection, and administrator audit record commit atomically. No seller API or normal UI can grant entitlement. Webhook signature/history canary tooling is implemented. Stripe callbacks, the AWS-012 worker, live retry/DLQ proof, and seller-facing webhook replacement/disable remain pending.

M8 acceptance: a new development environment can be deployed from committed Terraform without console-only changes or dual ownership with CDK.

## Milestone M9 — Demo and release gate

- [ ] **REL-001** Seed the demo seller with fixed-price active and inactive routes.
- [ ] **REL-002** Run unit, integration, contract, web accessibility, and end-to-end suites.
- [ ] **REL-003** Complete one real x402 testnet transaction and preserve its evidence bundle.
- [ ] **REL-004** Demonstrate buyer-maximum rejection and exact wallet authorization without a buyer-approval step.
- [ ] **REL-005** Demonstrate duplicate, non-delivery, and quality dispute outcomes.
- [ ] **REL-006** Rehearse deterministic fallback and AWS dependency failures.
- [ ] **REL-007** Connect a coding agent, generate a seller integration, approve publication, and pass the sandbox validator.
- [ ] **REL-008** Complete one browser-wallet purchase and one agent/x402 purchase for the same storefront and show both in the seller dashboard without double counting.
- [ ] **REL-009** Run the complete demo three consecutive times without manual data repair.
- [ ] **REL-010** Verify generated storefront metadata, structured data, sitemap, `llms.txt`, and manifest output across the supported stack fixtures.
- [ ] **REL-011** Revoke a project API key while its MCP access token is still unexpired and demonstrate that the next cloud MCP request fails through the entitlement-epoch check.
- [ ] **REL-012** Cancel or suspend a seller while its local connector and a deliberately modified fork remain running; demonstrate that neither can mint capabilities, publish, create intents, settle payments, or produce official AgentPay receipts/evidence.
  Local MCP coverage completed on September 20, 2026: automated regressions
  prove that `suspended` and `cancelled` sellers cannot mint a fresh capability
  through the official connector path or reuse an unexpired capability through
  a modified client. REL-012 remains open for deployed publication, intent,
  settlement, receipt, and evidence verification.
- [ ] **REL-013** Load a stale seller-hosted manifest after cancellation and demonstrate that authoritative intent creation and checkout fail while AgentPay discovery returns an inactive tombstone.
- [ ] **REL-014** Exercise cancellation immediately before settlement and immediately after finalized settlement; verify that the first transaction is blocked and the second fulfills exactly once according to the documented buyer-obligation rule.
- [ ] **REL-015** Complete the authenticated browser journey from clean sign-up through resumable onboarding, publication, browser purchase, agent purchase, dashboard reconciliation, dispute, sign-out, and expired-session recovery using the deployed production-shaped environment.
- [x] **REL-016** Historical implementation of a seller-owned onboarding test-purchase journey. Superseded on September 20, 2026 by issues 1 and 2: seller onboarding and dashboard surfaces expose no buyer checkout, wallet authorization, or seller-funded purchase action, and instead show the route-version-bound automated non-payment verification result. Public storefront checkout remains available only to human buyers and external buyer agents.

M9 acceptance: all release gates in `TEST_PLAN.md` pass, generated changes are reviewable, browser-wallet and agent purchase paths work, dashboard totals reconcile by asset and network, and mocked behavior is visibly labeled.

## Deferred milestone H1 — Card checkout

- [ ] **HUM-001** Verify a human card-checkout provider and lock merchant-of-record, settlement, refund, tax, chargeback, and platform-fee responsibilities.
- [ ] **HUM-002** Implement seller payment-account onboarding without exposing financial credentials to AgentPay agents or browser code.
- [ ] **HUM-003** Implement hosted card checkout against the same immutable purchase-intent rules used by x402.
- [ ] **HUM-004** Implement authenticated, idempotent provider callbacks with replay protection and no success-redirect trust.
- [ ] **HUM-005** Persist card payment-rail metadata so card and x402 sales use the same fulfillment, evidence, receipt, and dispute behavior.

## Post-hackathon backlog

- SDKs for languages other than the compact TypeScript merchant SDK and the
  existing maintained verification packages.
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
test(payments): reject a quote above the buyer maximum
fix(proxy): prevent duplicate upstream forwarding
```

Each commit must leave relevant checks passing. Do not mark a task complete until its acceptance tests and documentation are included.
