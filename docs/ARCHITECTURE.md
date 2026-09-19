# System architecture

Status: **Locked for the implemented M0–M7 backend and approved MVP direction**.

Implementation status: the reduced M7.1 Lean V1 is implemented and verified in
the production-shaped local runtime. The local API still uses in-memory
persistence and development credentials; AWS deployment, production Cognito,
managed signing keys, durable evidence storage, and real testnet release proof
remain M8/M9 work. Redis, direct remote MCP OAuth, cross-seller ranking, and a
full operator console are post-launch scaling features rather than V1 blockers.

## Purpose

AgentPay turns a seller's existing API or digital service into one commerce surface for people and software agents. It automates repository integration through MCP-enabled coding agents, publishes human and machine-readable storefronts, enforces purchase policies, verifies payment, forwards fulfillment requests, and records evidence for later dispute handling.

## Runtime components

### Web application

The Next.js application provides the public AgentPay marketing and product site,
sign-up and sign-in entry points, seller onboarding, verified
payment-destination setup, product and route configuration, an asset-separated
sales dashboard, seller-branded storefronts, browser-wallet purchase guidance,
the agent buyer demonstration, transaction evidence, receipts,
webhook status, and dispute views.

Server Components render read-heavy pages. Client Components are limited to
buyer interaction, wallet interaction, and small optimistic controls.

The public site and authenticated product share one token, typography,
navigation, and responsive-layout system. Marketing-only visual effects remain
isolated from dashboard interaction code, load progressively, and disable under
`prefers-reduced-motion`. Public pages render meaningful HTML before client
JavaScript so human visitors, crawlers, and software agents receive the same
truthful product explanation.

### Seller automation

The Lean V1 seller automation surface consists of a cloud-hosted remote MCP
server, the required AgentPay local connector, coding-agent setup instructions,
maintained verification middleware, and sandbox validation commands. The
connector holds the project key, exchanges it through the proprietary AgentPay
bootstrap endpoint, keeps a short-lived MCP access token in process memory, and
proxies bounded JSON-RPC. It contains no payment, publication, entitlement, or
signing authority. Direct remote OAuth MCP clients are deferred.

The MCP server exposes bounded AgentPay operations; it is not a general remote
shell. Read operations may run without confirmation. Creating or changing
products and publishing a storefront require an appropriately scoped MCP
capability plus a cloud-issued one-time confirmation grant created through the
authenticated seller browser/BFF boundary. Caller-supplied approval booleans,
summaries, timestamps, model output, and repository text are not authority.
Project-key rotation remains a seller-session operation in the dashboard/API
and is never delegated to the MCP access-token audience; deployment
authorization remains local to the seller environment.

Generated integration code must use maintained AgentPay request-verification packages when available. Coding agents must not generate independent cryptographic protocols or place project credentials in browser code.

The TypeScript merchant SDK composes the Node verifier, webhook verification,
typed merchant-side contracts, and an idempotent fulfillment coordinator. Its
adapter interface is a seller-application boundary, not a new AgentPay cloud
service. Shopify and WooCommerce integrations are tested reference adapters
that use seller-owned server credentials; they do not receive payment,
publication, signing, or transaction authority and do not imply inventory,
shipping, tax, refund, or physical-fulfillment support.

Setup bundle v2 also detects supported application stacks and proposes
stack-native technical SEO, answer-engine optimization, and agent-discovery
changes. Those changes include visible metadata, canonical URLs, structured
data, sitemap and robots output, semantic page content, `llms.txt`, and manifest
consistency. The automation cannot guarantee ranking and must not generate
deceptive or invisible search content.

### Go API

One deployable Go binary owns all authoritative business rules through isolated packages:

- `catalog`: sellers, routes, manifests, and pricing.
- `storefront`: authoritative publication readiness, signed AgentPay-hosted
  discovery, persisted publication revisions, and fresh commerce eligibility.
- `intents`: immutable purchase proposals and request hashes.
- `policy`: buyer maximum and budget evaluation. Historical threshold policy
  code is disabled for Lean V1.
- `approvals`: preserved M2 implementation for compatibility and future
  enterprise work; it has no active Lean V1 route, WebSocket, or payment gate.
- `payments`: runtime payment-capability publication, deterministic enabled-rail
  selection, x402 challenge creation, facilitator verification, and recovery
  classification.
- `settlement`: seller payment destinations, ownership verification, rotation,
  and reconciliation status.
- `proxy`: upstream request forwarding and seller request signatures.
- `evidence`: append-only evidence events and chain verification.
- `disputes`: deterministic classification, recommendations, and append-only
  seller-recorded external refund metadata; it never moves funds.
- `agents`: Bedrock tool orchestration and deterministic fallback.
- `analytics`: deterministic seller-scoped, asset-separated sales read models over bounded transaction queries.
- `notifications`: signed seller webhook subscriptions and delivery attempts.
- `billing`: seller plans, entitlements, usage meters, and invoice exports; it never
  controls buyer funds or seller settlement.
- `operations`: UTC-month API, MCP, and webhook-delivery counters plus static
  route and webhook-subscription entitlement enforcement.
- `audit`: immutable seller-visible control-plane change history and bounded
  tenant-scoped reads.
- `sellerworkspace`: current-seller onboarding progress, settings, dashboard
  read models, Stripe plan/portal projection, and the fail-closed publication
  prerequisite gate. It reads owning domains but does not write their records.

The billing package owns a versioned static plan catalog plus one seller plan
assignment record. Stripe Billing is the initial seller-subscription provider,
but provider events enter a durable inbox and are reconciled into AgentPay's
authoritative entitlement projection before they affect access. Other packages
may read entitlements through a narrow billing interface, but billing cannot
mutate purchase intents, transactions, payment destinations, facilitator state,
or seller payout configuration.

Only an `active` entitlement before its exclusive `accessEndsAt` grants MCP,
active discovery, publication or new-commerce authority. A failed renewal does
not extend that deadline. At the boundary the seller enters a fixed 72-hour
read-only `grace` for billing recovery, then `suspended`; voluntary
cancel-at-period-end remains active until the boundary and then becomes
`cancelled` without grace. Authorization enforces the timestamp directly even
if Stripe delivery or the transition worker is late.

The operations package reads the resolved seller plan and owns atomic monthly
quota counters. Consumer packages own narrow interfaces for the quota operation
they need. Invalid authentication and cross-seller authorization failures do
not consume quota. Webhook delivery claims deduplicate retries by subscription
and event identity.

Packages may call each other through explicit interfaces. They must not write another package's DynamoDB records directly.

M6 and M7 add package boundaries after their contracts are finalized:

- `integrations`: project credentials, MCP operations, integration validation, and framework setup metadata.
- `authorization`: project-key bootstrap exchange, capability verification,
  current credential and entitlement checks, browser purchase sessions, and
  JWKS publication.

The `integrations/stacks` package owns deterministic, bounded repository
evidence parsing and the explicit support matrix. It may identify multiple
application layers but never upgrades a support tier based only on detection;
maintained status requires the committed recipe and fixture gates.

The `integrations/discovery` package validates generated storefront artifacts
without network access or model judgment. It returns fixed named checks for
SEO, AEO, manifest agreement, accessibility facts, and performance budgets and
never emits a ranking score or guarantee.

The `integrations/recipes` package owns versioned stack instructions consumed
by setup bundle v2. Recipes pin verification packages, preserve raw request
bytes, define middleware order, require the no-op sandbox endpoint, and name
the framework-native storefront and discovery files verified by fixtures.

V1 product contracts are owned by `catalog` and projected by `storefront`.
Catalog canonicalizes closed input/output schemas and computes a deterministic
contract hash over the route version and execution-relevant terms. The MCP
validation and publication tools bind seller confirmation to that hash.
Storefront discovery signs the resulting public product contract and includes
it in the publication fingerprint, so an approved seller change advances the
discovery revision. Discovery never replaces fresh commerce authorization.

### Feature package layout

Backend feature packages separate responsibilities by file without adding wrapper layers:

- `models.go` owns domain types, states, immutable values, and read accessors.
- `service.go` owns validation, construction, deterministic rules, and internal calculations.
- `controller.go` owns public feature operations and guarded state-changing commands.
- `persistence.go` owns explicit serialization snapshots when private domain state must be stored.

Shared primitives and storage adapters remain organized by their concrete responsibility rather than being forced into controller/service files.

### AWS managed services

- API Gateway HTTP API routes browser, agent, and proxy traffic.
- The historical API Gateway approval WebSocket design is not deployed for
  Lean V1.
- Lambda runs the Go modular monolith for the hackathon.
- DynamoDB stores mutable operational state and idempotency records.
- S3 stores immutable evidence event objects.
- KMS signs evidence event hashes.
- Cognito authenticates sellers.
- Secrets Manager stores seller HMAC secrets and the isolated test-wallet secret.
- Bedrock produces structured purchase proposals and explanations.
- A remote MCP endpoint exposes seller-scoped integration tools and documentation.
- CloudWatch receives logs, metrics, alarms, and traces.
- Amplify Hosting deploys the Next.js application.

## Trust boundaries

1. **Browser boundary:** browser input is untrusted and validated by the API.
2. **Model boundary:** Bedrock output is untrusted structured input; Go revalidates all amounts, identifiers, budgets, and policy requirements.
3. **Payment boundary:** only the payments package can access the test wallet or facilitator credentials.
4. **Seller boundary:** forwarded requests are allowlisted by configured method and route, protected against SSRF, and signed for the seller.
5. **Evidence boundary:** evidence writers may append but cannot update or delete objects; verification uses a separate read role.
6. **Coding-agent boundary:** repository files, prompts, generated code, local
   MCP connectors, and MCP arguments are untrusted; project keys only bootstrap
   short-lived cloud capabilities, and all operations are reauthorized in the
   AgentPay control plane.
7. **Browser-wallet boundary:** browser input and wallet responses are untrusted;
   only server-side x402 verification may advance payment state.
8. **Webhook boundary:** subscriptions use validated public HTTPS destinations;
   deliveries are signed, bounded, retried, and protected against SSRF.
9. **Discovery boundary:** signed manifests and seller-hosted discovery identify
   candidate products but never authorize an intent, payment, or fulfillment.
10. **Seller-execution boundary:** only AgentPay holds the ES256 private key that
    can mint a transaction execution capability. Seller middleware receives
    public JWKS verification material and cannot mint AgentPay authority.
11. **MCP-confirmation boundary:** a scoped MCP bearer can propose a mutation
    but cannot approve it. Only the authenticated seller browser/BFF boundary
    can mint an opaque, five-minute, one-time grant bound to the exact seller,
    credential, tool, target, canonical arguments, and resource version.

## Seller launch lifecycle

1. The seller creates a storefront and receives a project-scoped bootstrap
   credential, displayed once.
2. The seller configures an asset-and-network-specific payment destination and
   proves control without disclosing a private key.
3. The seller connects the AgentPay MCP server to a supported coding agent. A
   local connector exchanges the project key for a 120-300 second MCP access
   token; ordinary MCP requests never carry the project key.
4. The coding agent inspects the local API contract and proposes products backed by concrete HTTPS routes.
5. The agent adds maintained signature-verification middleware, server-only
   configuration, storefront code, technical SEO/AEO, agent discovery, and tests.
6. The seller reviews prices, payment destinations, route publication, generated
   content, and deployment changes.
7. The dashboard creates a cloud confirmation grant for the exact reviewed
   mutation; the MCP server atomically consumes it with the idempotent
   control-plane operation.
8. AgentPay runs the dedicated non-payment sandbox verification and records a
   route-version-bound result covering reachability, signed exchange
   compatibility, closed input/output contracts, no-op fulfillment readiness,
   payment gating, and replay-safe idempotency. The verifier never creates a
   purchase intent or transaction and never invokes the paid business route.
9. The seller explicitly publishes the storefront and deploys the prepared application.

Seller onboarding never exposes buyer checkout or wallet authorization, and
sellers cannot initiate or complete buyer commerce from authenticated seller
routes.

The coding agent prepares and validates changes. It never receives production payout secrets and never publishes or deploys without seller confirmation.

Payment-destination activation is a settlement-domain operation. HTTP accepts a
bounded proof, the service verifies challenge state and wallet ownership, and
the repository atomically updates the active asset/network claim plus the new
and rotated destination records. Transport code never performs signature
recovery, and persistence never stores raw challenge or signature material.

## Buyer channels

Both buyer channels consume the same published paid routes:

- **Agent channel:** manifest or `llms.txt` discovery, immutable intent, buyer
  maximum validation, x402 payment, and signed fulfillment.
- **Browser channel:** seller-branded product page, immutable intent,
  x402-compatible wallet payment, and the same signed fulfillment.

The channel is presentation and payment-rail metadata. It does not create separate pricing, authorization, evidence, transaction, or dispute semantics.

Card checkout is deferred to H1. Selecting a provider later must not change the
authoritative intent, fulfillment, evidence, receipt, or dispute domains.

### Web route ownership

- `/` and the public information routes explain AgentPay.
- `/dashboard/*` is an authenticated seller workspace. Seller identity comes
  from the authenticated principal, never from a query parameter.
- `/store/{sellerSlug}` is the public catalog for one seller.
- `/store/{sellerSlug}/products/{productSlug}` is one public product and the
  human checkout entry point.
- `/demo/agent-checkout` is an educational and testable agent-channel flow. It
  is not a buyer account, a separate marketplace, or an authorization boundary.

The existing route-ID product URL is retained only as a migration-compatible
redirect until the product-slug API contract is implemented. Public navigation
uses seller and product slugs; internal services continue to use immutable IDs
for ownership and persistence.

### Authorization surfaces

- Seller browser sessions use Cognito-backed secure cookies and derive seller
  ownership from verified access-token claims. The BFF sends the token only in
  the `Authorization` header; it is never placed in URLs, browser storage, or
  logs. The Go identity boundary pins the Cognito issuer, app-client ID,
  `token_use=access`, RS256 algorithm, JWKS key ID, timestamps, subject, token
  JTI, and token-family identifier. Local development uses an opaque adapter
  that produces the same normalized claims.
- `GET /v1/me/seller` resolves the one seller mapped to the authenticated
  subject. Seller IDs in resource paths are locators, never authority; every
  seller path is re-authorized against the verified subject and cross-tenant
  records are concealed as `404`.
- `DELETE /v1/me/session` records a hashed token-family revocation until expiry.
  Revocation persistence is checked on every seller authentication and fails
  closed when unavailable.
- Seller project keys are accepted only by
  `POST /v1/integration-access-tokens`, which is a proprietary bootstrap
  exchange and not an OAuth token endpoint.
- MCP uses short-lived ES256 bearer capabilities with
  `aud=urn:agentpay:mcp`, exact scopes, current credential state, and current
  entitlement epoch.
- MCP commercial mutations additionally require an opaque one-time
  confirmation grant issued through the authenticated seller browser/BFF
  session. The grant is not an MCP access token and cannot be minted with a
  project key or MCP bearer.
- Buyer agents use buyer credentials that are separate from seller project
  keys.
- Human product pages create a server-side browser purchase session bound to
  seller, product, request hash, maximum amount, and browser channel. An opaque
  HttpOnly cookie survives reloads; ten-minute commerce authority is separated
  from longer-lived receipt/dispute access and can be recovered after payment
  through proof from the bound payer wallet.
- Buyer-side approval invitation, cookie, token, REST, and WebSocket surfaces
  are disabled for Lean V1. Historical M2 approval records remain readable only
  for migration/testing and never gate a Lean V1 challenge or transaction.
- Seller fulfillment uses a 30-60 second ES256 execution capability in
  `X-AgentPay-Execution-Capability`; it is minted only after finalized payment
  and the exactly-once forwarding claim.

AgentPay publishes overlapping ES256 public keys at
`/.well-known/jwks.json`. MCP access, discovery, and execution signatures pin
their own type, audience/domain, and claim sets. Browser purchase authority is
an opaque server-side grant rather than a JWT. A valid signature or
cookie never replaces current entitlement, credential, ownership, state, CSRF,
or replay checks.

Discovery signatures cover `agentpay.discovery.v1`, one NUL byte, and the RFC
8785 canonical JSON bytes of the unsigned document. They use the current ES256
key published by the shared AgentPay JWKS. Active discovery expires five
minutes after issue. A public read strongly re-evaluates the seller,
entitlement, endpoint verification, route lifecycle, exact active payment
destination, and workspace publication prerequisites before signing.

## Purchase lifecycle

1. The buyer reads a short-lived signed AgentPay manifest or product document
   and may read the runtime payment-capability catalog.
2. The buyer selects the first compatible enabled capability. The current
   testnet runtime exposes only exact x402, Base Sepolia, and USDC.
3. A buyer agent authenticates with its buyer credential; a browser creates a
   bounded server-side purchase session for the selected product. Either channel creates a
   purchase intent containing the immutable route, product snapshot, request
   hash, destination, quote, channel, and expiration.
4. The policy engine verifies that the fixed seller quote does not exceed the
   buyer-provided `maximumAmount`. No buyer-side approval session is created.
5. Before checkout starts, the buyer may cancel the still-`ready` intent. At
   `expiresAt`, expiration wins if the intent is still `ready`. The first
   paid-route request conditionally claims `ready -> executed`; a claim won
   before expiration may continue only through the same deterministic
   transaction identity.
6. The buyer requests the paid route with the intent identifier.
7. The gateway returns an x402 challenge when payment is absent.
8. The buyer wallet authorizes the exact asset, network, amount, destination,
   and resource, then retries with payment proof.
9. The gateway rechecks current entitlement, verifies and settles payment, and
   records finality.
10. One conditional-write winner claims the finalized transaction for
    forwarding and receives a one-time execution capability.
11. Evidence is appended for verification, forwarding, response, and final outcome.
12. The proxy forwards exactly once with the execution capability and returns
    the upstream response.

## Consistency and idempotency

- Client mutations require `Idempotency-Key`.
- Purchase intents are immutable after creation.
- Intent commercial fields remain immutable while one optimistic lifecycle
  transition selects `cancelled`, `expired`, or `executed` from `ready`.
- The buyer maximum, request hash, quote, and payment destination are bound to
  the immutable intent.
- Payment identifiers are globally unique in the transaction table.
- A DynamoDB conditional write changes a transaction from `PAYMENT_VERIFIED` to `FORWARDED` only when `paymentFinality=finalized`, the expected version matches, and no forwarding owner exists; only the winner calls the seller.
- Retried requests return the stored response metadata when replay is safe.

## Failure behavior

| Failure | Required behavior |
|---|---|
| Wallet missing or incompatible | Return or render `payment_capability_unsupported` with the missing capability and a non-payment recovery action; never substitute another rail. |
| Wallet authorization rejected | State that no payment was submitted and allow the unexpired challenge to be authorized again. |
| Intent expired | Return `410 payment_expired` with `recoveryAction=start_new_checkout`; never accept a late proof. |
| Proof rejected before verification | Return `402 payment_rejected` with a fresh exact challenge and `recoveryAction=sign_fresh_authorization`. |
| Bedrock timeout | Offer deterministic buyer fallback; never bypass the buyer maximum or wallet authorization. |
| Facilitator unavailable before verification | Return `503 payment_unavailable` with `recoveryAction=retry_same_request`; do not call the seller. |
| Settlement unavailable after verification | Keep finality `confirmed`, return `503 payment_outcome_unknown` with `recoveryAction=retry_same_payment`, and require the identical proof and intent. |
| Definitive settlement rejection | Record finality `failed`, return `402 payment_rejected` with `recoveryAction=start_new_checkout`, and never forward. |
| Evidence append fails before forwarding | Stop and return `503`; do not call the seller. |
| Seller timeout | Record delivery failure and classify `not_delivered` disputes as refund-recommended. |
| Invalid manual refund record | Reject before persistence; require the authenticated owning seller, a finalized payment, `refund_recommended` status, and an exact amount/asset/network match. |
| Duplicate manual refund request | Exact `Idempotency-Key` replay returns the original `201`; changed input or a conflicting second record returns `409` and never overwrites the first record. |
| Duplicate paid retry | Return the prior transaction outcome; never forward twice. |
| Reconciliation delayed | Keep payment and finalized amounts separate; never fabricate settlement completion. |
| Intent cancellation races checkout | One conditional intent transition wins; a cancelled or expired intent never receives a challenge, while an executed intent cannot be cancelled. |
| Payment outcome is unknown | Keep the transaction at confirmed finality, expose `await_reconciliation`, and never create a replacement charge or call the seller. |
| Finalized payment has delivery failure | Preserve the same transaction identity and expose the dispute/remediation path; never blindly repeat an upstream side effect after an ambiguous timeout. |
| Seller webhook unavailable | Retain the authoritative event, retry within policy, and expose the failed delivery in the dashboard. |
| Seller entitlement inactive or expired | Return `403 subscription_inactive` for authenticated seller/MCP operations, `410 seller_inactive` for public discovery, and reject new intent, challenge, verification, and settlement authorization. |
| MCP confirmation missing, expired, replayed, or mismatched | Fail before domain mutation; never infer approval from caller fields. Exact idempotent replay returns the stored redacted result only after the original grant was validly consumed. |
| Stripe renewal payment fails | Do not extend `accessEndsAt`; notify the seller, block network participation exactly at the existing boundary, permit only 72 hours of billing recovery/historical reads, then suspend. |
| Stripe events arrive late, duplicated, or out of order | Verify and durably deduplicate the event, fetch current provider state, and conditionally replace the complete entitlement projection using a local monotonic reconciliation revision. |
| Seller reactivates after access stopped | Require confirmed paid state, increment the entitlement epoch, invalidate caches/discovery, and require project-key rotation before credential-backed access resumes. |
| Fraud quarantine | Suspend immediately regardless of Stripe state; Stripe events cannot clear it, and finalized unfulfilled buyer payments enter the explicit incident/refund path. |
| Project key revoked or rotated | Deny bootstrap exchange; reject access tokens naming the revoked predecessor credential. |
| Stale capability epoch | Return `401 token_revoked`; do not consume quota or perform the operation. |
| Authoritative entitlement or credential persistence unavailable | Fail closed with `503 dependency_unavailable`; public discovery may return only an unexpired signed document or inactive result. Lean V1 does not use Redis for transaction-critical authorization. |
| Monthly seller quota exhausted | Return `429 rate_limited`; do not execute the requested operation. |
| Static route or webhook limit exhausted | Return `403 permission_denied`; do not create or publish the resource. |
| Search metadata validation fails | Keep the storefront publishable only after the seller fixes or explicitly removes the invalid generated metadata. |

The checkout path records proof verification as `confirmed`, then records the
x402 settle response and its safe network reference as `finalized` before the
seller request may be claimed. Provider timeouts do not promote finality. A
definitive settlement rejection is recorded as failed, while delivery failure
remains a separate transaction outcome after finalization.

## Deliberate exclusions

The first implementation does not provide card checkout, physical goods,
shipping, inventory, tax calculation, production custody, automated refund
execution,
cross-seller reputation, an AgentPay-operated buyer agent, A2A execution,
autonomous negotiation, arbitrary remote code
execution, guaranteed search ranking, or automatic quality judgments.
