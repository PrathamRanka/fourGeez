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
- `README.md` says that the web application is not started while
  `docs/IMPLEMENTATION.md` marks the M7 web tasks complete. Documentation and
  actual launch status therefore disagree.
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

- Tokens must never appear in URLs, logs, client-readable persistent storage,
  repository files, or browser bundles.
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
