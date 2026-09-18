# AgentPay launch remediation plan

Status: Proposed for review before implementation
Last updated: 2026-09-18

## Purpose

This document consolidates the product flaws found during the local review and
the work required before AgentPay can be treated as a launch-ready startup
product. It includes the original review notes plus additional product,
frontend, backend, security, integration, testing, documentation, and
operational gaps discovered while tracing the implementation.

No task in this plan should be marked complete because its screen exists. A
task is complete only when the user-visible flow works against the real Go
backend, its failure paths are covered, the contracts match, and the relevant
tests pass.

## Current verified baseline

- The Next.js component test suite currently passes: 12 files and 35 tests.
- The Go package test suite currently passes.
- The public site, dashboard screens, storefront, and buyer demonstration
  render locally.
- The existing seeded preview is not a full-system integration. It uses a
  small JavaScript read-only demo API instead of the real Go commerce API.
- Sign-up and sign-in are public preview placeholders and do not create or
  authenticate an account.
- The web application currently authenticates backend calls with one static
  server environment token.
- Dashboard seller context is carried through `sellerId` query parameters.
  Query parameters must never be treated as proof of ownership.
- The real Go API uses in-memory repositories locally and exposes the core
  catalog, wallet, integration, intent, approval, payment, transaction,
  evidence, analytics, webhook, billing, audit, and dispute operations.
- The current demo API exposes only selected reads and cannot complete seller
  onboarding, product publication, or a real purchase flow.
- `README.md` and the authoritative documentation now identify M0–M7 as a
  development preview and M7.1 as mandatory launch hardening. Remaining
  contract and implementation work must preserve that distinction.
- OpenAPI 0.4 and its LCH-004 companion documents are the M7.1 target contract,
  not the behavior of the current M7 binary. The legacy and target modes must
  never be partially mixed in production.
- AWS infrastructure, Cognito, deployment, and the M9 release gates are still
  incomplete.

Passing unit tests are a useful foundation, but they do not demonstrate a
working frontend-to-backend product journey.

## Product model that must be clear to users

### Public website

The public website explains AgentPay to potential sellers and buyers. Its main
job is to communicate that an existing API or digital service can become a
paid product for both people and software agents.

### Seller dashboard

The dashboard is the authenticated seller workspace. Sellers use it to finish
setup, verify their wallet, connect a coding agent, review products, publish a
storefront, inspect transactions, verify evidence, and manage disputes.

### Storefront

`/store/[sellerSlug]` is one seller's public storefront. It contains all
published products belonging to that seller.

`/store/[sellerSlug]/products/[productSlug]` is the public detail and purchase
page for one product. Public product URLs should use a readable product slug,
not an internal route identifier.

### Buyer experience

Human buyers should browse and purchase directly from storefront product
pages. Software agents normally discover products through the storefront
manifest, `llms.txt`, MCP/API tools, and paid URLs.

The current `/buyer` page is only a deterministic catalog-inspection
demonstration. It should be renamed to `/demo/agent-checkout`, clearly labeled
as an agent demonstration, and connected to the complete intent, approval,
payment, fulfillment, receipt, and evidence flow. It must not be confused with
a required buyer account or the ordinary human checkout.

## Original reported flaws

1. Revamp the documentation page with a memorable headline and a clear,
   visually polished sequence instead of vague or generic AI-style content.
2. Make sign-up and sign-in functional.
3. Decide and document safe URL and session behavior for authentication.
4. Use access and refresh credentials safely without exposing secrets or
   dirtying Git.
5. Explain the purpose of the buyer route.
6. Explain the purpose of the store route and whether it represents a seller
   or an individual product.
7. Send a new seller into onboarding after sign-up or first sign-in.
8. Replace technical names such as raw `name`, `slug`, route path, route ID,
   atomic amount, and network identifier with understandable product language.
9. Provide proper errors everywhere.
10. Verify that every onboarding step genuinely works.
11. Polish the frontend across public, seller, storefront, and buyer views.
12. Connect every related page and backend action into one coherent flow.
13. Publish a complete local-development guide covering dependencies,
    environment variables, service startup, validation, and troubleshooting.
14. Add further launch-readiness improvements discovered during engineering
    review.

## Additional product and engineering flaws

### Authentication and authorization

- There is no real seller account lifecycle: registration, email verification,
  sign-in, sign-out, refresh, session expiration, recovery, or revocation.
- Dashboard routes are not protected by an authenticated seller session.
- Seller ownership is not derived from the authenticated identity in the web
  application.
- Sign-up currently links directly to onboarding without creating an account.
- Sign-in truthfully says it is unavailable, which means the primary public
  calls to action do not lead to a usable product.
- Redirect behavior after sign-in, first-time onboarding, and session expiry is
  undefined.
- Authentication errors and permission-denied states are not exercised through
  browser E2E tests.

### Frontend-to-backend integration

- The disposable seeded preview bypasses the Go backend.
- The local launcher does not start the real API, mock facilitator, demo seller,
  and web application as one integrated stack.
- Seeded data does not prove domain transitions, exactly-once fulfillment, or
  evidence generation through production code paths.
- Frontend controllers have API wiring, but there is no full browser test
  proving that all actions work together.
- WebSocket approval, REST fallback, seller fulfillment, payment challenge,
  webhook history, receipt download, and dispute creation need system-level
  verification.
- The application has no dependency-readiness page or clear explanation when a
  required local service is unavailable.

### Product naming and navigation

- Public storefront cards use an API `pathPattern` as the product title.
- Product detail pages also use the raw API path as the primary heading and
  structured-data name.
- Product creation exposes atomic price values and raw payout addresses as
  primary fields, which is inappropriate for a simple seller experience.
- `Storefront name`, `Public storefront URL`, and `Service API URL` need clearer
  labels and inline examples.
- Product records need a human-readable display name and public product slug in
  the API and data model before the UI can use them reliably.
- Branded terms such as Launch Rail, Revenue Lens, Proof Stream, Trust Gate,
  Discovery Mesh, and Agent Checkout must always be paired with plain-language
  descriptions.
- The dashboard needs a useful home/overview destination rather than dropping
  users into a disconnected feature page.
- Sellers need an obvious way to preview their public storefront and return to
  the correct dashboard context.
- Disabled settings and incomplete navigation should not look like usable
  launch features.

### Onboarding

- The five visible steps are not yet proven against the full Go stack.
- Progress is calculated in component memory and is not an authoritative,
  resumable launch state.
- Wallet verification depends on a browser wallet but does not provide enough
  recovery guidance for rejection, wrong network, unsupported wallet,
  duplicate destination, or expired challenge.
- Integration credentials need clear one-time-display, copy confirmation,
  revocation, expiry, and recovery behavior.
- The coding-agent connection step must verify the connection instead of only
  showing copyable configuration.
- The sandbox-purchase step must run and report deterministic checks rather
  than only describe them.
- Product discovery, seller review, price confirmation, publication, and
  storefront preview must be connected to onboarding completion.
- A seller should resume at the first incomplete required step after signing
  back in.
- Publication must remain blocked until wallet, route, verification, sandbox,
  and explicit confirmation requirements pass.

### Buyer and checkout

- The buyer demonstration loads the first catalog route rather than ranking by
  the buyer's request.
- The buyer demonstration currently stops before creating an intent or making
  a purchase.
- Storefront product pages explain checkout but do not provide a complete
  human wallet purchase interaction.
- The browser and agent channels must visibly share the same quote, intent,
  payment, transaction, fulfillment, evidence, receipt, and dispute pipeline.
- Approval-required and below-threshold products must both be demonstrated.
- Empty storefronts, unavailable products, changed prices, expired intents,
  payment rejection, payment timeout, approval expiry, and seller failure need
  buyer-facing recovery behavior.
- Mocked and testnet payment behavior must be visibly labeled and must never be
  presented as production settlement.

### Errors and state handling

- Errors should distinguish validation, authentication, authorization, not
  found, conflict, expired state, rate limit, quota, suspension, dependency
  outage, payment failure, and terminal seller failure.
- Form errors must be attached to the relevant field and summarized for screen
  readers.
- Retryable errors need an explicit retry action and, when supplied, a visible
  retry-after duration.
- Destructive actions require clear confirmation and completion feedback.
- Loading, empty, disabled, permission-denied, quota, suspended, retryable, and
  terminal states must be verified on every applicable route.
- Error messages should explain what happened and what the user can do next,
  without leaking internal IDs, secrets, proofs, or infrastructure details.

### Documentation and setup

- The docs page lacks a distinctive product thesis and sufficient practical
  setup detail.
- The root README status is stale and contradicts the implementation ledger.
- Local setup does not yet describe one authoritative integrated command.
- Required Node, npm, Go, wallet, environment, and mock-service prerequisites
  need explicit versions and checks.
- Every environment variable needs a description, example category, required
  mode, and secret-handling rule.
- Setup instructions need clean checkout, install, start, seed, verify, stop,
  reset, and troubleshooting sections.
- The documentation must distinguish local mock behavior, testnet behavior,
  and future production behavior.
- Public documentation must not promise guaranteed revenue, search ranking,
  settlement finality, refunds, cards, or production support that is not live.

### Visual quality, accessibility, and performance

- The public docs should open with a direct headline such as "Turn your API
  into a product agents can buy" and show the setup path as an intentional
  commerce rail rather than a generic card list.
- Product language should be written from the seller's or buyer's point of
  view, not from internal implementation terminology.
- The UI needs consistent spacing, hierarchy, form density, focus treatment,
  status language, and responsive behavior.
- Use existing Aceternity, 21st.dev, or shadcn components before writing custom
  primitives. Use shadcn chart primitives for dashboard charts unless a
  maintained 21st.dev chart is demonstrably more suitable.
- All interactions must work with keyboard-only navigation and visible focus.
- Motion must respect reduced-motion preferences.
- Layouts must be verified at 360, 768, 1280, and 1440 pixels.
- Long transaction lists and optional charts must not create avoidable client
  bundles or rendering waterfalls.
- Read-heavy pages should remain Server Components, with small serializable
  client boundaries only where interaction requires them.

### Security and operational readiness

- Seller sessions, API keys, MCP access tokens, payment proofs, and execution
  tokens must never appear in URLs, logs, client-readable persistent storage,
  repository files, or browser bundles. The current one-time approval
  invitation token is a narrowly documented query-parameter exception with a
  short expiry and `Referrer-Policy: no-referrer`; LCH-004 must either preserve
  that explicit exception or replace it with fragment-to-cookie exchange.
- `.env.local` and generated secret files must remain ignored by Git.
- The API must derive tenant ownership from authenticated claims instead of
  trusting caller-supplied seller IDs.
- Session, callback, redirect, CSRF, cookie, CORS, and logout behavior need
  dedicated tests.
- Local seed/reset functionality must be unavailable in production builds.
- Health currently proves only that the API process responds; launch readiness
  also needs dependency readiness for configured storage, signing, payment,
  and seller-forwarding dependencies.
- Logging and tracing must correlate one browser action through API, payment,
  forwarding, evidence, and webhook operations without logging protected data.
- AWS Terraform, Cognito, deployment, alarms, secret injection, and environment
  separation remain mandatory before production launch.

## Proposed locked product decisions

These decisions should be accepted and copied into the controlling contracts
before implementation.

1. **Authentication:** use Cognito-backed OIDC for production rather than
   creating an independent JWT issuer. Use a compatible local development
   identity adapter so local and deployed authorization semantics match.
2. **Token storage:** keep access and refresh credentials behind secure,
   HTTP-only, same-site cookies. Do not place tokens in URLs or local storage.
3. **Redirects:** allow only validated relative return paths. New sellers go to
   the first incomplete onboarding step; established sellers go to the
   dashboard overview.
4. **Seller ownership:** determine seller identity from the authenticated
   session. A query-string `sellerId` may never authorize access.
5. **Buyer route:** rename `/buyer` to `/demo/agent-checkout` and clearly label
   it as an agent-channel demonstration. Human checkout remains on storefront
   product pages.
6. **Storefront routes:** `/store/[sellerSlug]` represents one seller;
   `/store/[sellerSlug]/products/[productSlug]` represents one product.
7. **Product naming:** add a human-readable product display name and public slug
   to the authoritative API and data model. Preserve HTTP route details as an
   advanced technical section.
8. **Local demo:** seed the real Go in-memory backend. Do not use a parallel
   JavaScript API as evidence of integration.
9. **Payment scope:** retain x402 mock/testnet checkout for the current launch
   scope. Card checkout remains deferred until its commercial and regulatory
   contract is approved.

## Target user-facing language

| Internal or unclear term | Preferred user-facing term |
|---|---|
| `name` | Storefront name or Product name |
| `slug` | Public storefront URL or Product URL |
| `upstreamBaseUrl` | Service API URL |
| `pathPattern` | Fulfillment endpoint, shown under Advanced API details |
| `routeId` | Hidden internal identifier |
| atomic amount | Editable decimal price with validated atomic conversion |
| `payTo` | Verified payment destination; select rather than retype |
| integration credential | Project connection key |
| sandbox validation | Test purchase |
| route publication | Publish product |
| transaction evidence | Payment and delivery proof |

## Ordered implementation plan

### Phase 0 — Contracts, task ledger, and clean baseline

1. Add numbered launch-stabilization tasks to `docs/IMPLEMENTATION.md`.
2. Update `PRODUCT.md`, `ARCHITECTURE.md`, `DATA_MODEL.md`, OpenAPI, security,
   and test-plan documents for authentication, session ownership, product
   display names, public slugs, redirects, and integrated local development.
3. Reconcile README and implementation status so completed means verified.
4. Record the buyer-route and storefront-route decisions.
5. Establish a clean Git baseline with one logical Conventional Commit per
   numbered task.

Acceptance:

- No documentation conflicts remain.
- Every planned behavior has one controlling contract.
- The worktree contains no generated output, secrets, caches, or unrelated
  changes.

### Phase 1 — Production-shaped integrated local runtime

Create one supported local command that starts and supervises:

- the Next.js application;
- the real Go API;
- the demo seller fulfillment service;
- the mock x402 facilitator;
- the API's approval WebSocket service;
- deterministic buyer fallback mode; and
- the Go in-memory repositories with a named disposable seed profile.

The launcher must propagate environment configuration, wait for readiness,
print useful URLs, report failed dependencies, and terminate every child
process on one Ctrl+C on Windows. All in-memory seed data must disappear when
the stack exits.

Seed at least:

- one launch-ready seller;
- one incomplete seller for onboarding recovery;
- one below-approval-threshold product;
- one approval-required product;
- fulfilled, failed, pending, and disputed transactions;
- valid and invalid evidence chains;
- webhook success, retry, and dead-letter attempts; and
- asset/network-separated analytics.

Use two verification layers:

- a visual seed builder inside the Go local runtime for stable demonstrations;
- full E2E setup that creates and purchases records through public APIs for
  release proof.

Acceptance:

- No frontend page depends on the JavaScript demo API.
- Health and readiness checks pass for every required local service.
- Storefront, onboarding, product management, checkout, analytics, evidence,
  receipts, webhooks, and disputes read from the Go backend.
- Ctrl+C leaves no web, API, facilitator, or seller process listening.

### Phase 2 — Seller authentication and sessions

1. Implement Cognito-backed production authentication and a contract-compatible
   local development adapter.
2. Implement registration, verification, sign-in, sign-out, refresh, recovery,
   expiration, invalid-session, and revoked-session flows.
3. Store the session only in secure server-managed cookies.
4. Add route protection for every dashboard page and mutation.
5. Resolve the seller and account state from authenticated claims.
6. Remove query-string seller IDs as an authorization source.
7. Add safe return-path handling and onboarding redirects.
8. Add CSRF, cookie, callback, CORS, and cross-tenant tests.

Acceptance:

- Editing a URL cannot expose another seller's resources.
- New accounts reach onboarding.
- Returning completed accounts reach the dashboard overview.
- Expired sessions return to sign-in without losing a safe intended route.
- Access and refresh credentials never appear in browser-visible URLs,
  JavaScript storage, logs, or Git.

### Phase 3 — Information architecture and product terminology

1. Add an authenticated dashboard overview.
2. Introduce product display names and readable public product slugs through
   contract-first changes.
3. Replace technical-first forms with guided seller language.
4. Generate storefront slugs from business names while allowing advanced edits.
5. Accept decimal prices in the UI and convert them safely to atomic-unit
   strings at the validated boundary.
6. Select verified payment destinations rather than asking users to retype an
   address.
7. Place route method, endpoint, MIME type, timeout, network, and internal IDs
   under Advanced API details.
8. Add consistent links among dashboard overview, onboarding, products,
   storefront preview, transactions, documentation, and settings.
9. Remove or implement controls that currently appear disabled or unfinished.

Acceptance:

- A first-time seller can explain every navigation item and form field without
  understanding AgentPay's internal data model.
- Storefront product cards and metadata use product names, not API paths.

### Phase 4 — Resumable end-to-end onboarding

The authoritative onboarding sequence is:

1. Create and verify seller account.
2. Name the storefront and connect the HTTPS service.
3. Verify an asset/network payment destination.
4. Create and securely save a project connection key.
5. Connect and verify the coding-agent MCP client.
6. Discover, review, name, price, and confirm proposed products.
7. Run a complete test purchase, review results, and explicitly publish.

Requirements:

- Persist progress and compute completion server-side.
- Resume at the first incomplete mandatory step.
- Provide clear recovery for wallet, credential, connection, validation, and
  publication failures.
- Verify the coding-agent connection instead of treating copied text as
  success.
- Execute the sandbox validator and expose each named check.
- Block publication until every required condition passes.
- Provide a storefront preview before final publication.

Acceptance:

- A clean seller can complete onboarding without editing URLs, database data,
  source files, or environment variables mid-flow.

### Phase 5 — Buyer, storefront, approval, and checkout

1. Redesign storefront cards and product details around human-readable value,
   price, output, trust, and availability.
2. Implement browser-wallet checkout through immutable purchase intents.
3. Support the below-threshold direct path and approval-required path.
4. Connect the agent demonstration to discovery, candidate selection, intent,
   limits, approval, payment, fulfillment, receipt, and evidence.
5. Improve deterministic offer selection so it respects the stated need and
   budget instead of selecting the first catalog route.
6. Keep browser and agent purchases on the same backend commerce pipeline.
7. Expose receipts, evidence verification, and eligible dispute actions after
   purchase.
8. Add recovery for expired intent, changed availability, rejected payment,
   facilitator timeout, approval expiry, seller timeout, and duplicate retry.

Acceptance:

- One browser-wallet purchase and one agent purchase for the same storefront
  appear once each in the seller dashboard with correct asset/network totals.
- Duplicate retries never invoke the seller twice.

### Phase 6 — Seller dashboard integration

Verify and complete the following against authenticated Go API requests:

- storefront status and preview;
- product draft, validation, publication, pause, archive, and emergency stop;
- wallet verification and rotation;
- project key issuance, listing, revocation, and expiry;
- asset/network-separated analytics;
- transaction filtering and pagination;
- transaction detail and evidence verification;
- receipt download;
- webhook subscription, delivery history, redelivery, and dead-letter state;
- dispute creation, detail, classification, and resolution state;
- audit history;
- plan, quota, rate-limit, and suspension states; and
- sandbox and publication readiness.

Acceptance:

- Every visible control either completes its documented backend operation or is
  removed from the launch interface.
- Refreshing or revisiting a page restores authoritative backend state.

### Phase 7 — Documentation, errors, accessibility, and visual polish

#### Documentation redesign

- Lead with a clear headline: "Turn your API into a product agents can buy."
- Show the setup as one visual commerce rail:
  Connect service -> Verify wallet -> Connect agent -> Review products -> Test
  purchase -> Publish.
- Include copyable setup commands, MCP configuration, environment guidance,
  expected output, and troubleshooting.
- Clearly separate seller setup, human buying, and agent buying.
- Explain security, payment boundaries, supported stacks, and mock/testnet
  limitations in plain language.

#### Error system

- Map documented API codes to consistent page, form, inline, toast, and retry
  treatments.
- Cover validation, 401, 403, 404, 409, 422, 428, 429, and 503 behavior.
- Preserve actionable server messages while preventing sensitive-data leakage.
- Add accessible announcements and focus movement for failed submissions.

#### Visual and interaction quality

- Establish an AgentPay-specific visual signature based on a visible commerce
  rail and proof timeline rather than generic SaaS cards.
- Use registry components directly and avoid unnecessary wrappers.
- Use shadcn or suitable maintained 21st.dev chart components for analytics.
- Verify responsive layouts at 360, 768, 1280, and 1440 pixels.
- Verify keyboard-only use, semantic HTML, visible focus, labels, contrast, and
  reduced motion.
- Eliminate avoidable data-fetching waterfalls and excessive client bundles.

Acceptance:

- A new visitor understands the product, the two buyer channels, what is live,
  and how to begin without protocol knowledge.
- All applicable loading, empty, disabled, permission, quota, suspension,
  retryable, and terminal states are demonstrated and tested.

### Phase 8 — Full-system release verification

Add browser-driven E2E coverage for this launch journey:

```text
sign up
-> verify account
-> sign in
-> resume onboarding
-> verify wallet
-> connect coding agent
-> discover and review products
-> run sandbox purchase
-> publish storefront
-> complete human purchase
-> complete agent purchase
-> approve a gated purchase
-> verify fulfillment and evidence
-> download receipt
-> inspect analytics and webhook history
-> create and review dispute
-> sign out
```

Required failure coverage:

- invalid, expired, revoked, and cross-tenant sessions;
- unknown fields and invalid form input;
- expired intent and approval;
- underpayment, overpayment, rejection, timeout, unavailable facilitator, and
  replayed proof;
- duplicate paid retry and exactly-once seller invocation;
- seller 4xx, 5xx, timeout, redirect, private-address SSRF attempt, and
  oversized response;
- evidence append or verification failure;
- webhook retry and dead-letter behavior;
- quota exhaustion and seller suspension;
- Bedrock timeout with deterministic fallback; and
- local service startup failure and clean shutdown.

Release gates:

- formatting, lint, typecheck, unit, race, integration, contract, accessibility,
  E2E, production build, secret scan, and dependency audit pass;
- no undocumented mock or console configuration is required;
- one real x402 testnet transaction has a valid evidence bundle;
- two-client approval survives reconnect;
- one browser and one agent purchase reconcile without double counting;
- three complete rehearsals succeed from a clean environment without manual
  data repair;
- Lighthouse meets the documented accessibility and best-practice targets;
- Terraform, Cognito, deployment, alarms, secrets, and environment isolation
  pass M8 before production launch; and
- final privacy, terms, support, incident, financial, and regulatory review is
  completed before accepting real customer funds.

## Test-driven implementation rules

For every numbered implementation task:

1. Write the user-visible or domain regression test first.
2. Run it and confirm the expected failure.
3. Implement the smallest contract-compliant behavior.
4. Run focused tests and refactor.
5. Run all affected quality gates.
6. Update the controlling documentation and task status.
7. Commit one logical change using Conventional Commits.

UI tests must assert user behavior, not implementation details. Backend domain
rules must be tested before handlers. Browser E2E tests must use the real local
Go API, mock facilitator, and demo seller rather than mocked frontend actions.

## Proposed commit sequence

1. `docs: define launch stabilization tasks and product decisions`
2. `feat(dev): run the complete disposable local stack`
3. `feat(auth): add seller registration and secure sessions`
4. `feat(web): protect seller routes and restore onboarding progress`
5. `feat(catalog): add product display names and public slugs`
6. `feat(web): clarify dashboard and storefront navigation`
7. `feat(onboarding): complete verified seller launch flow`
8. `feat(checkout): connect browser and agent purchase journeys`
9. `feat(dashboard): integrate seller operations with the Go API`
10. `feat(docs): publish the complete seller setup guide`
11. `fix(web): complete error and accessibility states`
12. `test(e2e): verify the full AgentPay launch journey`
13. `feat(infra): deploy the production-shaped AWS environment`
14. `chore(release): pass AgentPay launch gates`

Each commit must include its tests and documentation and leave all relevant
checks passing.

## Launch definition of done

AgentPay is ready for startup launch only when:

- a new seller can register, authenticate, onboard, reconnect, and resume;
- seller identity and authorization cannot be changed through a URL;
- the coding-agent integration produces reviewable changes and cannot publish
  without confirmation;
- the seller can verify a wallet, create products, test, publish, pause, and
  inspect the storefront;
- human and agent buyers can purchase the same published products through the
  shared commerce pipeline;
- payment, fulfillment, evidence, receipt, webhook, analytics, audit, quota,
  and dispute records reconcile;
- retries cannot double-charge or double-fulfill;
- errors are clear, recoverable where appropriate, and safe;
- documentation can take a clean machine from checkout to a working local
  environment;
- the complete system passes accessibility, responsive, performance, security,
  integration, and E2E gates;
- the AWS environment is reproducible from Terraform and uses Cognito,
  least-privilege IAM, protected secrets, monitoring, and environment
  separation; and
- all mock, deferred, preview, and testnet behavior is labeled truthfully.

## Subscription enforcement for a seller-controlled MCP

### Security conclusion

AgentPay cannot prevent a seller from continuing to run, modify, or fork code
that is already deployed in the seller's infrastructure. The architecture must
therefore avoid placing any authoritative commercial capability in that code.

Cancellation enforcement comes from removing the seller's ability to
participate in the AgentPay network, not from remotely disabling local files.
After access ends, the seller may still run a local MCP process, but it must be
unable to:

- authenticate to AgentPay's MCP or control APIs;
- mint a fresh short-lived capability token;
- appear as active in AgentPay-hosted discovery;
- create a new purchase intent through AgentPay;
- receive a valid AgentPay x402 payment challenge;
- have a payment verified or settled through AgentPay;
- receive an AgentPay transaction-execution authorization;
- obtain an official receipt, evidence chain, webhook, or dashboard record; or
- display a valid AgentPay-issued availability or verification badge.

A forked MCP may imitate screens, accept direct requests, or implement an
independent payment flow, because the seller controls its own infrastructure.
It cannot forge AgentPay signatures, cloud records, transaction identifiers,
receipts, discovery status, or payment verification. Such traffic is outside
the AgentPay network and must not be represented as an AgentPay transaction.

### Correct interpretation of "install our MCP"

AgentPay should not ship the authoritative MCP server as a self-contained
seller-hosted service. The preferred installation consists of:

1. project configuration that points the seller's coding agent to AgentPay's
   remote cloud MCP endpoint;
2. either a project API key stored in a server-side secret store for the
   mandatory AgentPay connector, or a direct standards-compatible OAuth grant
   that never exposes a project key to the MCP host;
3. repository-local generated integration files and public discovery metadata;
4. a small AgentPay request-verification package in the seller's application;
   and
5. the AgentPay local MCP connector when project-key bootstrap is used; direct
   OAuth-capable MCP clients may omit it.

The connector must not contain subscription policy, signing keys, payment
verification authority, publication authority, or transaction state. Every
privileged operation still calls AgentPay's cloud control plane.

Project-key bootstrap makes the connector mandatory for that installation
mode. A standards-compatible host may instead connect directly to `/mcp` using
OAuth authorization discovered through
`/.well-known/oauth-protected-resource/mcp`; it never receives a seller project
key. Missing or invalid MCP bearer authentication returns `401` with a
`WWW-Authenticate` challenge naming that protected-resource metadata URL.

### Non-negotiable invariants

1. The seller API key is a bootstrap credential, not a permanent transaction
   capability.
2. The API key is accepted only by the cloud
   `/v1/integration-access-tokens` bootstrap endpoint; key-management endpoints
   require the authenticated seller session.
3. MCP and control-plane requests use short-lived signed access tokens.
4. Every privileged request validates the current seller entitlement, not only
   the token signature and expiry.
5. Every new purchase checks subscription state before intent creation, before
   challenge issuance, and immediately before x402 verification/settlement.
6. A finalized buyer payment creates a bounded fulfillment obligation. Normal
   subscription cancellation must not strand a buyer after funds are finalized.
7. Exactly-once seller forwarding is claimed in AgentPay's cloud database before
   the seller is called.
8. The seller fulfillment endpoint accepts only AgentPay-signed execution
   requests for official transactions.
9. Discovery is advisory. A cached product listing never authorizes payment or
   execution.
10. Redis accelerates revocation and replay checks but is not the source of
    truth for subscriptions, credentials, or transaction state.
11. Transaction authorization fails closed when current entitlement or
    revocation state cannot be established.
12. Raw API keys, access tokens, payment signatures, and execution tokens are
    never written to logs or audit records.

### Production MCP confirmation cannot be caller asserted

The implemented M7 Go MCP models currently accept
`confirmation.approved`, `confirmation.summary`, and
`confirmation.confirmedAt` from the same caller requesting the mutation. A
modified connector can fabricate all three fields. This is a development-only
compatibility shape and must be disabled in production rather than treated as
seller authority.

Production state-changing MCP tools require an opaque 256-bit
`MCPConfirmationGrant` created only through the authenticated seller
browser/BFF endpoint
`POST /v1/sellers/{sellerId}/mcp-confirmation-grants`. The dashboard displays
the exact canonical mutation before issuance. The grant is bound to seller,
credential, tool, target, `agentpay.mcp-mutation.v1` canonical arguments hash,
expected seller or route version, unique grant ID, issue time, and an exclusive
expiry no more than five minutes later.

AgentPay returns the raw grant once with `Cache-Control: no-store` and stores
only a keyed digest plus binding metadata. Reissuing the same binding revokes
the earlier unconsumed grant. The MCP server atomically consumes the grant with
the mutation idempotency decision. Exact retry returns the stored redacted
result; a missing, expired, revoked, consumed, wrong-seller, wrong-credential,
wrong-tool, wrong-target, wrong-version, or wrong-arguments grant fails before
mutation. A project key, MCP bearer, model, repository, connector, or fork
cannot mint or approve a grant.

### Cloud and seller responsibility split

| Capability | AgentPay cloud/control plane | Seller infrastructure |
|---|---|---|
| Seller identity and account status | Authoritative | No authority |
| Subscription and entitlements | Authoritative database plus cache | May display last-known state only |
| API-key registry and revocation | Authoritative | Stores its own raw key securely |
| Short-lived access-token signing | KMS/HSM-backed cloud signer | Verifies public key only when necessary |
| Official MCP endpoint and tools | Authoritative remote server | Optional untrusted proxy/connector |
| Public network directory | Authoritative active listing | May host generated local metadata |
| Product publication state and price | Authoritative | May propose configuration |
| Purchase intents and policy | Authoritative | No mutation authority |
| x402 challenge, verification, settlement | Authoritative gateway and verified facilitator | Receives funds at its verified destination |
| Transaction claim and execution grant | Authoritative conditional write and signer | Verifies exact signed request and fulfills |
| Evidence, receipts, audit, webhooks | Authoritative append-only records | Receives signed requests/events |
| Seller business implementation | No ownership | Authoritative for the digital service output |

The existing Go modular monolith should remain the authoritative implementation
of these cloud responsibilities. TypeScript examples below define the required
behavior and can be used for a Node edge service, local connector, and seller
verification package. They must not create a second independent source of
payment or authorization rules.

### Subscription state and cancellation policy

The billing domain needs a documented entitlement state separate from provider
invoice terminology:

- `active`: new MCP operations and transactions are allowed;
- `grace`: fixed 72-hour billing-recovery and historical-read-only period;
  it grants no MCP, discovery, publication, or new-commerce authority;
- `suspended`: new privileged operations and transactions are denied;
- `cancelled`: voluntary cancellation is effective and network access is denied;
  scheduled cancellation remains `active` with `cancelAtPeriodEnd=true` until
  `accessEndsAt`;
- `closed`: all credentials are revoked and no new access is allowed.

`cancelAtPeriodEnd` does not stop access immediately. The entitlement remains
active until the exact UTC `accessEndsAt` timestamp. Non-payment, fraud, policy
abuse, or administrator suspension can stop new operations immediately.

Cancellation checkpoints:

| Operation | Active before `accessEndsAt` | Grace/suspended/cancelled/closed |
|---|---:|---:|
| Read public discovery | Allowed | Return inactive/tombstone state; remove from network index |
| Read private MCP resources | Allowed | Denied |
| Mutate configuration or publish | Allowed by scope/quota | Denied |
| Create purchase intent | Allowed | Denied |
| Issue x402 challenge | Allowed | Denied |
| Verify or settle a new payment | Allowed after fresh check | Denied |
| Fulfill payment finalized before suspension | Allowed exactly once | Allowed exactly once unless fraud quarantine policy requires refund handling |
| Read historical invoices/exports | Optional restricted account access | Read-only according to retention policy |

The important race rule is: **before settlement, subscription status can block
the transaction; after settlement, AgentPay has a buyer-facing fulfillment
obligation.** A normal cancellation must allow that already-finalized
transaction to complete through a short-lived, transaction-specific execution
grant. An emergency fraud suspension may quarantine it, but then AgentPay needs
an explicit refund or incident procedure rather than silently retaining buyer
funds without delivery.

### Discovery must be separate from authorization

Discovery answers, "What might this seller offer?" Authorization answers,
"May this exact transaction happen now?" They must never be conflated.

Public discovery may be cached for performance and search indexing. Each
AgentPay-hosted manifest should include:

- seller and product identifiers;
- human-readable product name and public slug;
- price, asset, network, and verified payment destination reference;
- publication revision;
- `available` or `inactive` status;
- `issuedAt` and short `expiresAt` timestamps; and
- an AgentPay signature or canonical URL to the authoritative manifest.

On cancellation, AgentPay removes the seller from the network directory,
invalidates CDN/Redis entries, publishes an inactive tombstone, and refuses new
intent creation. A stale copy may remain on the internet, but it has no power:
the cloud transaction endpoint performs a fresh entitlement and route check.

Seller-hosted `llms.txt`, manifests, or forked MCP resources are claims made by
the seller. Agents should treat them as candidate discovery input and require a
fresh AgentPay intent or capability response before considering the product an
active AgentPay offer.

### Production request flow

```text
Seller MCP host or local connector
  -> exchange project API key with AgentPay cloud
  -> receive 2-5 minute seller-scoped access token
  -> call AgentPay remote MCP with token
  -> cloud verifies signature, audience, scope, credential, entitlement epoch,
     current subscription, quota, and idempotency
  -> for a state-changing tool, authenticated seller dashboard mints a one-time
     grant bound to the exact credential, tool, target, arguments, and version
  -> cloud atomically consumes that grant with the mutation decision
  -> cloud performs bounded read/mutation and appends audit decision

Buyer agent or browser
  -> read public discovery
  -> buyer agent authenticates with its own key, or browser receives an opaque
     HttpOnly purchase grant plus double-submit CSRF cookie
  -> create immutable intent in AgentPay cloud
  -> cloud checks active seller, published route, frozen quote, wallet, policy
  -> optional approval completes; approvers receive no payment authority
  -> the authenticated purchase owner claims the short-lived approval token
  -> AgentPay paid URL returns the x402 payment requirement
  -> buyer retries with PAYMENT-SIGNATURE
  -> cloud rechecks seller entitlement and frozen intent
  -> official x402 adapter verifies and settles exact payment
  -> cloud stores unique payment identifier and finalized transaction
  -> cloud atomically claims transaction for forwarding
  -> cloud signs a one-time execution capability bound to transaction, method,
     path, body hash, seller, route, destination, and expiry
  -> cloud calls seller fulfillment endpoint
  -> seller middleware verifies AgentPay signature and exact request binding
  -> cloud records response hash, evidence, receipt, analytics, and webhook
```

The buyer must call an AgentPay-owned paid URL, not the seller MCP, for an
official transaction. Buyer funds can still settle directly to the seller's
verified wallet; AgentPay controls the challenge, verification, transaction
state, and execution authorization without taking custody.

### Token model

Use distinct tokens for distinct purposes:

| Token | Audience | Lifetime | Purpose |
|---|---|---:|---|
| Project API key | Token endpoint only | Long-lived until rotation/revocation | Bootstrap machine identity |
| MCP access token | `urn:agentpay:mcp` | 2-5 minutes | Scoped MCP/control API access |
| Seller web session | `agentpay-dashboard` | Short access plus managed refresh | Human dashboard access |
| Browser purchase grant | Server-side opaque cookie | 10-minute commerce window; read access until 30 days after terminal outcome or dispute resolution | One browser purchase with reload-safe receipt/dispute access |
| Approval grant set | Server-side opaque cookie | Per invitation/session expiry | Concurrent approval sessions with CSRF-protected decisions |
| Discovery signature | Public verifiers | 1-5 minutes for availability | Authentic but non-authoritative catalog hint |
| Execution capability | Exact seller service | 30-60 seconds, one use | Fulfill one finalized transaction |
| Webhook signature | Seller webhook endpoint | Timestamp-bounded | Authenticate one event payload |

An MCP access token should contain only bounded claims:

```json
{
  "iss": "https://api.agentpay.example",
  "aud": "urn:agentpay:mcp",
  "sub": "key_...",
  "sellerId": "sel_...",
  "credentialId": "key_...",
  "scope": "read configure",
  "entitlementEpoch": 42,
  "jti": "cap_...",
  "iat": 1789728000,
  "exp": 1789728300
}
```

The signature and expiry alone are insufficient for immediate revocation.
Middleware must compare `entitlementEpoch` with the current cloud value. API-key
revocation, subscription suspension, account closure, and administrator lock
increment that epoch, causing already-issued access tokens to fail immediately.

Browser and approval authorization are not JavaScript-readable JWTs. They use
opaque Secure, HttpOnly, SameSite=Strict cookies backed by server-side grants,
plus separate non-HttpOnly CSRF cookies whose values must match
`X-AgentPay-CSRF` and an exact allowlisted Origin on every state-changing
request. A browser grant may contain multiple approval sessions. Finalized x402
payment binds the payer wallet to the purchase session, allowing a one-time
wallet-signature challenge to recover only receipt/evidence/dispute access after
cookie loss; recovery never restores intent or payment authority.

### API-key design

- Generate at least 256 bits of random secret material.
- Format keys so the public lookup identifier is separate from the secret. New
  production keys use `apc2.<credentialId>.<randomSecret>`; existing
  `apc1.<sellerId>.<credentialId>.<randomSecret>` keys are migration-only and
  must be replaced by explicit rotation before production activation.
- Show the raw key once.
- Store only a keyed digest of the secret and metadata such as seller, scopes,
  status, expiry, created time, last-used time, and version.
- Make rotation retry-safe: atomically create the successor and revoke the
  predecessor, then retain the committed response secret only as seller- and
  request-bound KMS envelope-encrypted idempotency data for at most ten minutes.
  After that window, a lost-response retry returns the successor ID and requires
  rotating that successor; it never revives the predecessor.
- Use a server-side pepper from KMS/Secrets Manager and constant-time digest
  comparison.
- Do not accept the API key directly as a transaction or execution credential.
- Rate-limit exchanges by credential, seller, IP risk signal, and account.
- Rotate credentials after reactivation or suspected exposure.

### Redis cache and invalidation model

Suggested keys:

```text
agentpay:entitlement:{sellerId}                 JSON, TTL 15-30 seconds
agentpay:entitlement-epoch:{sellerId}           integer, no opportunistic TTL
agentpay:credential-revoked:{credentialId}      1, retained through max token TTL
agentpay:token-jti-used:{jti}                   1, SET NX with token-expiry TTL
agentpay:execution-jti-used:{jti}               1, SET NX with execution TTL
agentpay:idempotency:{principal}:{operation}:{key}
agentpay:rate:{sellerId}:{operation}:{period}
agentpay:discovery:{sellerSlug}:{revision}
```

Use a durable database transaction and outbox for subscription/key changes:

1. update the authoritative subscription or credential row;
2. increment the seller entitlement epoch;
3. write an outbox event in the same transaction;
4. commit;
5. update/delete Redis cache keys;
6. publish `entitlement.changed` for process-local cache invalidation;
7. remove active discovery listings; and
8. append the audit event.

Redis Pub/Sub alone is not durable. Consumers must be able to recover from the
database/outbox revision after a restart. Transaction authorization and token
exchange fail closed if authoritative state cannot be obtained. Public
discovery may fail soft by returning an expired/inactive result, but it cannot
authorize a purchase.

### Audit requirements

Append an allowlisted audit event for every security-relevant decision:

- API key created, exchanged, rotated, revoked, or rejected;
- access token issued or denied;
- subscription activated, grace-started, suspended, cancelled, expired,
  reactivated, or closed;
- entitlement epoch incremented;
- MCP operation allowed or denied;
- discovery listing published, invalidated, or tombstoned;
- payment challenge issued or denied;
- payment verification and settlement result;
- transaction authorization issued, consumed, replayed, expired, or denied;
- seller forwarding claimed, completed, failed, or skipped; and
- administrative override or fraud quarantine.

Record request ID, seller ID, credential ID, operation, decision, reason code,
subscription revision, entitlement epoch, transaction ID when applicable,
timestamp, and bounded metadata. Store a hash of a token JTI when correlation
is required. Never log raw credentials, JWTs, payment signatures, wallet
signatures, approval tokens, or seller secrets.

## TypeScript/Node.js reference implementation

### Repository placement

The authoritative AgentPay backend remains Go. The following TypeScript layout
is a concrete reference for an optional Node edge gateway, local MCP connector,
and the existing seller verification package:

```text
reference/node-control-plane/
  src/domain/types.ts
  src/auth/api-key-service.ts
  src/auth/capability-token-service.ts
  src/auth/authorization-middleware.ts
  src/billing/entitlement-service.ts
  src/cache/redis-keys.ts
  src/mcp/cloud-mcp-handler.ts
  src/payments/x402-service.ts
  src/transactions/transaction-authorizer.ts
  src/revocation/revocation-service.ts
  src/audit/audit-service.ts

packages/local-mcp-connector/
  src/token-client.ts
  src/mcp-proxy.ts

verification/node/src/
  execution-capability.ts
  middleware.ts
```

Production dependencies should be pinned only when this design is implemented.
Use the official x402 SDK at the payment adapter boundary, an audited JOSE/JWT
library for verification and local tests, the official Redis client, and the
existing AWS/KMS strategy for production signing keys.

### Shared domain contracts

```ts
// reference/node-control-plane/src/domain/types.ts
export type SubscriptionStatus =
  | "active"
  | "grace"
  | "suspended"
  | "cancelled"
  | "closed";

export type IntegrationScope =
  | "read"
  | "configure"
  | "validate"
  | "publish"
  | "rotate";

export type McpScope = Exclude<IntegrationScope, "rotate">;

export type Entitlement = {
  sellerId: string;
  status: SubscriptionStatus;
  accessEndsAt: string | null;
  entitlementEpoch: number;
  revision: number;
};

export type ApiCredential = {
  credentialId: string;
  sellerId: string;
  secretDigest: string;
  scopes: IntegrationScope[];
  expiresAt: string | null;
  revokedAt: string | null;
};

export type CapabilityClaims = {
  sellerId: string;
  credentialId: string;
  scope: string;
  entitlementEpoch: number;
  jti: string;
};

export type AuthorizationPrincipal = CapabilityClaims & {
  subject: string;
};

export class AuthorizationError extends Error {
  constructor(
    readonly code:
      | "invalid_credential"
      | "token_expired"
      | "token_revoked"
      | "subscription_inactive"
      | "insufficient_scope"
      | "dependency_unavailable",
    message: string,
  ) {
    super(message);
  }
}
```

### API-key authentication

```ts
// reference/node-control-plane/src/auth/api-key-service.ts
import {
  createHmac,
  randomBytes,
  timingSafeEqual,
} from "node:crypto";
import type { ApiCredential } from "../domain/types.js";
import { AuthorizationError } from "../domain/types.js";

export interface CredentialRepository {
  findById(credentialId: string): Promise<ApiCredential | null>;
  touchLastUsed(credentialId: string, usedAt: string): Promise<void>;
}

const prefix = "apc2.";

export function generateApiKey(
  credentialId: string,
): {
  rawKey: string;
  secretDigestInput: string;
} {
  const secret = randomBytes(32).toString("base64url");
  return {
    rawKey: `${prefix}${credentialId}.${secret}`,
    secretDigestInput: secret,
  };
}

export function digestApiKeySecret(secret: string, pepper: Buffer): string {
  return createHmac("sha256", pepper).update(secret, "utf8").digest("hex");
}

function constantTimeEqualHex(left: string, right: string): boolean {
  if (!/^[a-f0-9]{64}$/i.test(left) || !/^[a-f0-9]{64}$/i.test(right)) {
    return false;
  }
  return timingSafeEqual(Buffer.from(left, "hex"), Buffer.from(right, "hex"));
}

function parseApiKey(rawKey: string): {
  credentialId: string;
  secret: string;
} {
  if (!rawKey.startsWith(prefix)) {
    throw new AuthorizationError("invalid_credential", "Invalid API key.");
  }
  const [version, credentialId, secret, ...remainder] = rawKey.split(".");
  if (
    version !== "apc2" ||
    !credentialId?.startsWith("key_") ||
    remainder.length > 0
  ) {
    throw new AuthorizationError("invalid_credential", "Invalid API key.");
  }
  if (!credentialId || !secret || secret.length < 40) {
    throw new AuthorizationError("invalid_credential", "Invalid API key.");
  }
  return { credentialId, secret };
}

export class ApiKeyService {
  constructor(
    private readonly credentials: CredentialRepository,
    private readonly pepper: Buffer,
    private readonly now: () => Date = () => new Date(),
  ) {}

  async authenticate(rawKey: string): Promise<ApiCredential> {
    const parsed = parseApiKey(rawKey);
    const credential = await this.credentials.findById(parsed.credentialId);
    const suppliedDigest = digestApiKeySecret(parsed.secret, this.pepper);
    const storedDigest = credential?.secretDigest ?? "0".repeat(64);

    if (!constantTimeEqualHex(suppliedDigest, storedDigest) || !credential) {
      throw new AuthorizationError("invalid_credential", "Invalid API key.");
    }
    if (credential.revokedAt) {
      throw new AuthorizationError("token_revoked", "API key is revoked.");
    }
    if (credential.expiresAt && new Date(credential.expiresAt) <= this.now()) {
      throw new AuthorizationError("token_expired", "API key is expired.");
    }

    await this.credentials.touchLastUsed(
      credential.credentialId,
      this.now().toISOString(),
    );
    return credential;
  }
}
```

### Subscription validation and Redis caching

```ts
// reference/node-control-plane/src/billing/entitlement-service.ts
import type { RedisClientType } from "redis";
import {
  AuthorizationError,
  type Entitlement,
} from "../domain/types.js";

export interface EntitlementRepository {
  get(sellerId: string): Promise<Entitlement | null>;
}

export const entitlementKey = (sellerId: string) =>
  `agentpay:entitlement:${sellerId}`;
export const entitlementEpochKey = (sellerId: string) =>
  `agentpay:entitlement-epoch:${sellerId}`;

export class EntitlementService {
  constructor(
    private readonly repository: EntitlementRepository,
    private readonly redis: RedisClientType,
    private readonly now: () => Date = () => new Date(),
  ) {}

  async resolve(
    sellerId: string,
    consistency: "cached" | "strong",
  ): Promise<Entitlement> {
    if (consistency === "cached") {
      const cached = await this.redis.get(entitlementKey(sellerId));
      if (cached) return JSON.parse(cached) as Entitlement;
    }

    const entitlement = await this.repository.get(sellerId);
    if (!entitlement) {
      throw new AuthorizationError(
        "subscription_inactive",
        "Seller subscription is unavailable.",
      );
    }

    await this.redis
      .multi()
      .set(entitlementKey(sellerId), JSON.stringify(entitlement), { EX: 20 })
      .set(
        entitlementEpochKey(sellerId),
        String(entitlement.entitlementEpoch),
      )
      .exec();
    return entitlement;
  }

  async assertActive(
    sellerId: string,
    consistency: "cached" | "strong" = "strong",
  ): Promise<Entitlement> {
    const entitlement = await this.resolve(sellerId, consistency);
    const now = this.now();
    const accessEnded =
      entitlement.accessEndsAt !== null &&
      new Date(entitlement.accessEndsAt) <= now;
    const statusAllowsAccess = entitlement.status === "active";

    if (!statusAllowsAccess || accessEnded) {
      throw new AuthorizationError(
        "subscription_inactive",
        "Seller subscription is not active.",
      );
    }
    return entitlement;
  }

  async assertEpoch(sellerId: string, tokenEpoch: number): Promise<void> {
    let current = await this.redis.get(entitlementEpochKey(sellerId));
    if (current === null) {
      const entitlement = await this.resolve(sellerId, "strong");
      current = String(entitlement.entitlementEpoch);
    }
    if (!Number.isSafeInteger(tokenEpoch) || Number(current) !== tokenEpoch) {
      throw new AuthorizationError("token_revoked", "Access was revoked.");
    }
  }
}
```

For token exchange, MCP mutations, publication, intent creation, challenge
issuance, and payment verification, use `strong`. A short cached read is
acceptable only for non-sensitive private views when the epoch is still
checked. Redis errors on transaction-critical operations must become
`dependency_unavailable`, not an allow decision.

### Short-lived signed access tokens

```ts
// reference/node-control-plane/src/auth/capability-token-service.ts
import { randomUUID } from "node:crypto";
import {
  SignJWT,
  jwtVerify,
  type KeyLike,
  type JWTPayload,
} from "jose";
import type {
  ApiCredential,
  CapabilityClaims,
} from "../domain/types.js";

const issuer = "https://api.agentpay.example";
const audience = "urn:agentpay:mcp";
const accessLifetimeSeconds = 300;

export class CapabilityTokenService {
  constructor(
    private readonly signingKey: KeyLike,
    private readonly verificationKey: KeyLike,
    private readonly keyId: string,
  ) {}

  async issue(
    credential: ApiCredential,
    entitlementEpoch: number,
  ): Promise<string> {
    const now = Math.floor(Date.now() / 1000);
    return new SignJWT({
      sellerId: credential.sellerId,
      credentialId: credential.credentialId,
      scope: credential.scopes
        .filter((scope) => scope !== "rotate")
        .join(" "),
      entitlementEpoch,
    })
      .setProtectedHeader({
        alg: "ES256",
        kid: this.keyId,
        typ: "agentpay-access+jwt",
      })
      .setIssuer(issuer)
      .setAudience(audience)
      .setSubject(credential.credentialId)
      .setJti(`cap_${randomUUID().replaceAll("-", "")}`)
      .setIssuedAt(now)
      .setExpirationTime(now + accessLifetimeSeconds)
      .sign(this.signingKey);
  }

  async verify(token: string): Promise<CapabilityClaims & { subject: string }> {
    const result = await jwtVerify(token, this.verificationKey, {
      issuer,
      audience,
      algorithms: ["ES256"],
      clockTolerance: 5,
    });
    return parseClaims(result.payload);
  }
}

function parseClaims(payload: JWTPayload): CapabilityClaims & {
  subject: string;
} {
  if (
    typeof payload.sub !== "string" ||
    typeof payload.jti !== "string" ||
    typeof payload.sellerId !== "string" ||
    typeof payload.credentialId !== "string" ||
    typeof payload.entitlementEpoch !== "number" ||
    typeof payload.scope !== "string"
  ) {
    throw new Error("Invalid capability claims.");
  }
  return {
    subject: payload.sub,
    jti: payload.jti,
    sellerId: payload.sellerId,
    credentialId: payload.credentialId,
    entitlementEpoch: payload.entitlementEpoch,
    scope: payload.scope,
  };
}
```

The sample accepts injected JOSE keys so it is testable. Production token
signing belongs in a cloud-only signing boundary backed by KMS/HSM or a managed
authorization server. Private signing material must never be copied into the
seller connector, seller repository, browser, or ordinary application
configuration.

### Token exchange and authorization middleware

```ts
// reference/node-control-plane/src/auth/authorization-middleware.ts
import type { FastifyReply, FastifyRequest } from "fastify";
import type { IntegrationScope } from "../domain/types.js";
import { ApiKeyService } from "./api-key-service.js";
import { CapabilityTokenService } from "./capability-token-service.js";
import { EntitlementService } from "../billing/entitlement-service.js";

declare module "fastify" {
  interface FastifyRequest {
    agentPayPrincipal?: {
      sellerId: string;
      credentialId: string;
      scopes: IntegrationScope[];
      entitlementEpoch: number;
      jti: string;
    };
  }
}

export function createTokenExchange(dependencies: {
  apiKeys: ApiKeyService;
  entitlements: EntitlementService;
  tokens: CapabilityTokenService;
}) {
  return async function tokenExchange(
    request: FastifyRequest,
    reply: FastifyReply,
  ) {
    const rawKey = request.headers["x-agentpay-api-key"];
    if (typeof rawKey !== "string") {
      return reply.code(401).send({
        error: { code: "unauthorized", message: "API key is required." },
      });
    }

    const credential = await dependencies.apiKeys.authenticate(rawKey);
    const entitlement = await dependencies.entitlements.assertActive(
      credential.sellerId,
      "strong",
    );
    const accessToken = await dependencies.tokens.issue(
      credential,
      entitlement.entitlementEpoch,
    );

    return reply
      .header("Cache-Control", "no-store")
      .send({
        accessToken,
        tokenType: "Bearer",
        expiresIn: 300,
        scope: credential.scopes.join(" "),
      });
  };
}

export function requireCapability(
  dependencies: {
    entitlements: EntitlementService;
    tokens: CapabilityTokenService;
  },
  requiredScope: IntegrationScope,
) {
  return async function authorize(
    request: FastifyRequest,
    reply: FastifyReply,
  ) {
    const authorization = request.headers.authorization;
    if (!authorization?.startsWith("Bearer ")) {
      return reply.code(401).send({
        error: { code: "unauthorized", message: "Access token is required." },
      });
    }

    const claims = await dependencies.tokens.verify(authorization.slice(7));
    if (!claims.scope.split(" ").includes(requiredScope)) {
      return reply.code(403).send({
        error: { code: "permission_denied", message: "Scope is not allowed." },
      });
    }

    await dependencies.entitlements.assertEpoch(
      claims.sellerId,
      claims.entitlementEpoch,
    );
    await dependencies.entitlements.assertActive(claims.sellerId, "strong");

    request.agentPayPrincipal = claims;
  };
}
```

The implementation must translate expected authorization failures into the
documented public error shape and audit both allowed and denied decisions.
Unexpected token or cache errors must not leak details.

### Local MCP connector

```ts
// packages/local-mcp-connector/src/token-client.ts
type TokenResponse = {
  accessToken: string;
  tokenType: "Bearer";
  expiresIn: number;
};

export class AgentPayTokenClient {
  private cached: { token: string; refreshAt: number } | null = null;

  constructor(
    private readonly cloudOrigin: string,
    private readonly apiKey: string,
  ) {}

  async getAccessToken(): Promise<string> {
    if (this.cached && Date.now() < this.cached.refreshAt) {
      return this.cached.token;
    }

    const response = await fetch(
      `${this.cloudOrigin}/v1/integration-access-tokens`,
      {
      method: "POST",
      headers: {
        "X-AgentPay-Project-Key": this.apiKey,
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        audience: "urn:agentpay:mcp",
        scopes: ["read", "configure", "publish", "validate"],
      }),
        signal: AbortSignal.timeout(8_000),
      },
    );
    if (!response.ok) {
      this.cached = null;
      throw new Error(`AgentPay token exchange failed with ${response.status}.`);
    }

    const token = (await response.json()) as TokenResponse;
    const safetyWindowMilliseconds = 30_000;
    this.cached = {
      token: token.accessToken,
      refreshAt:
        Date.now() + token.expiresIn * 1_000 - safetyWindowMilliseconds,
    };
    return token.accessToken;
  }

  clear(): void {
    this.cached = null;
  }
}
```

```ts
// packages/local-mcp-connector/src/mcp-proxy.ts
import { AgentPayTokenClient } from "./token-client.js";

export class CloudMcpProxy {
  constructor(
    private readonly cloudOrigin: string,
    private readonly tokens: AgentPayTokenClient,
  ) {}

  async invoke(jsonRpcRequest: unknown): Promise<unknown> {
    const invoke = async () => {
      const token = await this.tokens.getAccessToken();
      return fetch(`${this.cloudOrigin}/mcp`, {
        method: "POST",
        headers: {
          Authorization: `Bearer ${token}`,
          "Content-Type": "application/json",
        },
        body: JSON.stringify(jsonRpcRequest),
        signal: AbortSignal.timeout(15_000),
      });
    };

    let response = await invoke();
    if (response.status === 401) {
      this.tokens.clear();
      response = await invoke();
    }
    if (!response.ok) {
      throw new Error(`AgentPay MCP request failed with ${response.status}.`);
    }
    return response.json();
  }
}
```

This connector can be forked, but a fork cannot produce a token after the API
key or subscription is revoked. The connector must never cache mutation
results as authorization and must never implement a local payment bypass.

### Cloud MCP request flow

```ts
// reference/node-control-plane/src/mcp/cloud-mcp-handler.ts
import type { FastifyInstance } from "fastify";

export function registerCloudMcp(app: FastifyInstance, services: {
  authorizeRead: ReturnType<typeof import("../auth/authorization-middleware.js").requireCapability>;
  consumeMcpQuota(sellerId: string): Promise<void>;
  dispatch(input: {
    sellerId: string;
    credentialId: string;
    body: unknown;
  }): Promise<unknown>;
}) {
  app.post(
    "/mcp",
    { preHandler: services.authorizeRead },
    async (request, reply) => {
      const principal = request.agentPayPrincipal;
      if (!principal) return reply.code(401).send();

      await services.consumeMcpQuota(principal.sellerId);
      const response = await services.dispatch({
        sellerId: principal.sellerId,
        credentialId: principal.credentialId,
        body: request.body,
      });
      return reply.send(response);
    },
  );
}
```

The dispatcher still enforces each tool's exact scope, cloud-issued one-time
confirmation grant for state-changing tools, idempotency, target ownership,
plan feature, quota, and domain validation. A top-level `read` scope is not
permission to invoke mutation tools. Caller-provided confirmation booleans,
summaries, or timestamps are never production authority.

### x402 verification boundary

```ts
// reference/node-control-plane/src/payments/x402-service.ts
export type PaymentRequirements = {
  scheme: string;
  network: string;
  asset: string;
  amount: string;
  payTo: string;
  resource: string;
};

export type VerifiedPayment = {
  valid: boolean;
  paymentIdentifier: string;
  payerReference?: string;
};

export type SettledPayment = {
  success: boolean;
  networkReference: string;
};

export interface X402Facilitator {
  verify(
    paymentSignature: string,
    requirements: PaymentRequirements,
  ): Promise<VerifiedPayment>;
  settle(
    paymentSignature: string,
    requirements: PaymentRequirements,
  ): Promise<SettledPayment>;
}

export interface PaymentRepository {
  reserveVerifiedPayment(input: {
    transactionId: string;
    paymentIdentifier: string;
    requirements: PaymentRequirements;
  }): Promise<void>; // unique conditional write on paymentIdentifier
  markFinalized(input: {
    transactionId: string;
    networkReference: string;
  }): Promise<void>;
  markFailed(transactionId: string, reason: string): Promise<void>;
}

export class X402PaymentService {
  constructor(
    private readonly facilitator: X402Facilitator,
    private readonly payments: PaymentRepository,
    private readonly assertSellerActive: (sellerId: string) => Promise<void>,
  ) {}

  async verifyAndSettle(input: {
    sellerId: string;
    transactionId: string;
    paymentSignature: string;
    frozenRequirements: PaymentRequirements;
    presentedRequirements: PaymentRequirements;
  }): Promise<SettledPayment> {
    await this.assertSellerActive(input.sellerId);
    assertExactRequirements(
      input.frozenRequirements,
      input.presentedRequirements,
    );

    const verification = await this.facilitator.verify(
      input.paymentSignature,
      input.frozenRequirements,
    );
    if (!verification.valid) {
      await this.payments.markFailed(input.transactionId, "payment_rejected");
      throw new Error("Payment verification failed.");
    }

    await this.payments.reserveVerifiedPayment({
      transactionId: input.transactionId,
      paymentIdentifier: verification.paymentIdentifier,
      requirements: input.frozenRequirements,
    });

    // Recheck immediately before the irreversible settlement boundary.
    await this.assertSellerActive(input.sellerId);
    const settlement = await this.facilitator.settle(
      input.paymentSignature,
      input.frozenRequirements,
    );
    if (!settlement.success) {
      await this.payments.markFailed(input.transactionId, "settlement_failed");
      throw new Error("Payment settlement failed.");
    }

    await this.payments.markFinalized({
      transactionId: input.transactionId,
      networkReference: settlement.networkReference,
    });
    return settlement;
  }
}

function assertExactRequirements(
  frozen: PaymentRequirements,
  presented: PaymentRequirements,
): void {
  const fields: Array<keyof PaymentRequirements> = [
    "scheme",
    "network",
    "asset",
    "amount",
    "payTo",
    "resource",
  ];
  if (fields.some((field) => frozen[field] !== presented[field])) {
    throw new Error("Payment requirements do not match the frozen intent.");
  }
}
```

The concrete `X402Facilitator` adapter must use the pinned official x402 SDK
and its canonical payload/header serialization. AgentPay code must not invent
an alternative proof format. Amounts remain atomic-unit strings; underpayment
and overpayment are rejected.

### Transaction authorization and exactly-once forwarding

```ts
// reference/node-control-plane/src/transactions/transaction-authorizer.ts
import { createHash, randomUUID } from "node:crypto";
import { SignJWT, type KeyLike } from "jose";

export interface TransactionRepository {
  claimFinalizedForForwarding(
    transactionId: string,
  ): Promise<{
    transactionId: string;
    sellerId: string;
    routeId: string;
    method: string;
    path: string;
    requestBody: Uint8Array;
    sellerAudience: string;
  } | null>;
}

export class TransactionAuthorizer {
  constructor(
    private readonly transactions: TransactionRepository,
    private readonly signingKey: KeyLike,
    private readonly keyId: string,
  ) {}

  async claimAndAuthorize(transactionId: string): Promise<{
    transaction: NonNullable<
      Awaited<ReturnType<TransactionRepository["claimFinalizedForForwarding"]>>
    >;
    executionToken: string;
  }> {
    const transaction =
      await this.transactions.claimFinalizedForForwarding(transactionId);
    if (!transaction) {
      throw new Error("Transaction is not eligible for forwarding.");
    }

    const bodyHash = createHash("sha256")
      .update(transaction.requestBody)
      .digest("base64url");
    const now = Math.floor(Date.now() / 1000);
    const executionToken = await new SignJWT({
      kind: "agentpay-fulfillment",
      sellerId: transaction.sellerId,
      routeId: transaction.routeId,
      transactionId: transaction.transactionId,
      method: transaction.method,
      path: transaction.path,
      bodyHash,
    })
      .setProtectedHeader({ alg: "ES256", kid: this.keyId, typ: "JWT" })
      .setIssuer("https://api.agentpay.example")
      .setAudience(transaction.sellerAudience)
      .setJti(randomUUID())
      .setIssuedAt(now)
      .setExpirationTime(now + 60)
      .sign(this.signingKey);

    return { transaction, executionToken };
  }
}
```

The repository claim must be a database conditional write from transaction
status `PAYMENT_VERIFIED` with `paymentFinality=finalized` into the documented
forwarded/claimed state. Only the caller that wins the claim may invoke the
seller. Normal subscription cancellation is not rechecked after a finalized
payment because that transaction is already an obligation; no new product or
payment can be created after suspension. LCH-004 must name the exact persisted
claim representation without inventing a second finality state machine.

### Seller-side execution verification

```ts
// verification/node/src/execution-capability.ts
import { createHash } from "node:crypto";
import { createRemoteJWKSet, jwtVerify } from "jose";

const agentPayKeys = createRemoteJWKSet(
  new URL("https://api.agentpay.example/.well-known/jwks.json"),
);

export async function verifyAgentPayExecution(input: {
  token: string;
  expectedAudience: string;
  method: string;
  path: string;
  rawBody: Uint8Array;
}) {
  const result = await jwtVerify(input.token, agentPayKeys, {
    issuer: "https://api.agentpay.example",
    audience: input.expectedAudience,
    algorithms: ["ES256"],
    clockTolerance: 5,
  });
  const claims = result.payload;
  const bodyHash = createHash("sha256")
    .update(input.rawBody)
    .digest("base64url");

  if (
    claims.kind !== "agentpay-fulfillment" ||
    claims.method !== input.method ||
    claims.path !== input.path ||
    claims.bodyHash !== bodyHash ||
    typeof claims.transactionId !== "string" ||
    typeof claims.jti !== "string"
  ) {
    throw new Error("Invalid AgentPay execution capability.");
  }

  return {
    transactionId: claims.transactionId,
    sellerId: String(claims.sellerId),
    routeId: String(claims.routeId),
    jti: claims.jti,
  };
}
```

Seller middleware should preserve raw request bytes, verify before parsing,
reject redirects or unexpected routes, and use `transactionId` as an
idempotency key. For stronger replay resistance, the middleware can call a
cloud consume endpoint that atomically performs:

```ts
const consumed = await redis.set(
  `agentpay:execution-jti-used:${jti}`,
  "1",
  { NX: true, EX: 90 },
);
if (consumed !== "OK") throw new Error("Execution capability was replayed.");
```

The cloud transaction claim remains authoritative even if a seller removes
this middleware. Removing it only makes the seller's own endpoint less secure;
it does not grant access to AgentPay's payment or transaction systems.

### Revocation and cache invalidation

```ts
// reference/node-control-plane/src/revocation/revocation-service.ts
export interface RevocationRepository {
  revokeCredentialAndIncrementEpoch(input: {
    credentialId: string;
    sellerId: string;
    reason: string;
    occurredAt: string;
  }): Promise<{ newEpoch: number; revision: number }>;

  suspendSellerAndIncrementEpoch(input: {
    sellerId: string;
    reason: string;
    occurredAt: string;
  }): Promise<{ newEpoch: number; revision: number }>;
}

export class RevocationService {
  constructor(
    private readonly repository: RevocationRepository,
    private readonly redis: import("redis").RedisClientType,
  ) {}

  async revokeCredential(input: {
    credentialId: string;
    sellerId: string;
    reason: string;
  }): Promise<void> {
    const occurredAt = new Date().toISOString();
    const result = await this.repository.revokeCredentialAndIncrementEpoch({
      ...input,
      occurredAt,
    });
    await this.invalidate(input.sellerId, result.newEpoch, result.revision);
    await this.redis.set(
      `agentpay:credential-revoked:${input.credentialId}`,
      "1",
      { EX: 600 },
    );
  }

  async suspendSeller(sellerId: string, reason: string): Promise<void> {
    const result = await this.repository.suspendSellerAndIncrementEpoch({
      sellerId,
      reason,
      occurredAt: new Date().toISOString(),
    });
    await this.invalidate(sellerId, result.newEpoch, result.revision);
  }

  private async invalidate(
    sellerId: string,
    newEpoch: number,
    revision: number,
  ): Promise<void> {
    await this.redis
      .multi()
      .del(`agentpay:entitlement:${sellerId}`)
      .set(`agentpay:entitlement-epoch:${sellerId}`, String(newEpoch))
      .publish(
        "agentpay:entitlement.changed",
        JSON.stringify({ sellerId, newEpoch, revision }),
      )
      .exec();
  }
}
```

The repository methods must commit the state change, epoch increment, audit
record, and outbox event atomically. Redis invalidation happens only after that
commit. If Redis publication fails, the outbox retries it; transaction-critical
requests still verify current authoritative state before proceeding.

### Audit service

```ts
// reference/node-control-plane/src/audit/audit-service.ts
export type AuditDecision = "allowed" | "denied" | "completed" | "failed";

export type AuditEvent = {
  eventId: string;
  requestId: string;
  sellerId: string;
  actorType: "seller" | "credential" | "system" | "buyer";
  actorId: string;
  action: string;
  decision: AuditDecision;
  reasonCode: string;
  entitlementEpoch?: number;
  subscriptionRevision?: number;
  transactionId?: string;
  metadata: Record<string, string | number | boolean>;
  occurredAt: string;
};

export interface AuditRepository {
  append(event: AuditEvent): Promise<void>;
}

export class AuditService {
  constructor(private readonly repository: AuditRepository) {}

  async record(event: AuditEvent): Promise<void> {
    const forbidden = /token|secret|signature|authorization|cookie|proof/i;
    for (const key of Object.keys(event.metadata)) {
      if (forbidden.test(key)) {
        throw new Error(`Forbidden audit metadata key: ${key}`);
      }
    }
    await this.repository.append(structuredClone(event));
  }
}
```

Security-denial audit writes should be isolated from the public response. A
temporary audit sink failure must not turn a denied operation into an allowed
operation. Payment and forwarding evidence remain separate append-only domain
records rather than being replaced by general audit events.

### Required integration tests

1. Revoking an API key prevents the next token exchange.
2. Revoking a key invalidates an already-issued access token through the epoch
   check before its normal expiry.
3. Cancelling at period end allows access until `accessEndsAt` and denies it at
   the exact boundary.
4. Immediate suspension denies MCP reads, MCP writes, intent creation,
   publication, challenge issuance, and payment verification.
5. A stale public manifest can be read but cannot create an intent after
   suspension.
6. A seller-forked MCP cannot mint a valid AgentPay access or execution token.
7. A token with a valid signature but old entitlement epoch is denied.
8. A token for the wrong audience, seller, credential, scope, or route is
   denied.
9. Redis invalidation reaches all API instances; a missed Pub/Sub message is
   recovered from the durable outbox/revision.
10. Redis or subscription-storage failure causes transaction authorization to
    fail closed.
11. Cancellation between challenge issuance and settlement causes settlement
    to be denied before funds move.
12. Cancellation after finalized settlement still allows that exact
    transaction to fulfill once.
13. Replayed payment identifiers and execution JTIs are rejected.
14. Concurrent forwarding claims result in exactly one seller invocation.
15. The x402 amount, asset, network, destination, resource, and intent hash must
    all match the frozen seller quote.
16. Audit logs contain the decision and reason but no raw credential, token,
    signature, proof, cookie, or wallet secret.
17. Closed sellers disappear from AgentPay discovery and receive an inactive
    response from authoritative manifests.
18. Reactivation requires an explicit credential rotation and a new entitlement
    epoch before MCP access resumes.

### Required changes to the existing AgentPay contracts

Before implementation, update the authoritative documents and code contracts:

- extend the billing state model beyond only `active` and `suspended` or define
  a separate entitlement projection with `accessEndsAt` and revision;
- add `entitlementEpoch` to the seller/account security state;
- change the MCP contract from accepting a permanent integration credential on
  every operation to accepting short-lived access tokens, with API-key exchange
  or standards-based client authentication;
- define token issuer, audience, scopes, lifetime, JWKS, rotation, and
  introspection/revocation semantics;
- define public discovery availability, expiry, signature, revision, and
  inactive tombstone behavior;
- document subscription checks at intent, challenge, verification, and
  settlement boundaries;
- document the post-settlement fulfillment-obligation exception;
- add transaction execution capability fields and one-time consumption rules;
- add Redis as a cache/replay/invalidation component, never as the sole source
  of truth;
- add durable outbox events for subscription and credential changes;
- add audit event types for every allow/deny decision listed above;
- update OpenAPI, MCP, data model, architecture, security, test plan, runbooks,
  and implementation order together; and
- implement the authoritative services in Go, then align the Node verification
  package and the project-key connector with the same wire contracts, while
  preserving direct OAuth interoperability for standards-compatible MCP hosts.

### Why a fork cannot bypass AgentPay

A fork can remove local checks because the seller owns the machine. It still
cannot create a valid AgentPay transaction because all valuable network
artifacts originate in the cloud:

- the official directory and manifest status;
- a current seller entitlement decision;
- a short-lived scoped access token;
- an immutable purchase intent and quote;
- an x402 challenge tied to a verified payment destination;
- facilitator verification and unique payment claim;
- an AgentPay transaction state transition;
- a one-time execution signature from a cloud-only key;
- an evidence chain and official receipt; and
- dashboard, analytics, webhook, and dispute records.

If a fork invents those values or accepts calls without them, it has created a
separate seller-operated system. AgentPay should protect its signing keys,
verification marks, domains, and branding contractually and technically, but it
should not rely on obfuscation, license checks inside seller code, remote kill
switches, or client-side subscription checks for security.
