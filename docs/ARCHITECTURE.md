# System architecture

Status: **Locked for the implemented M0–M7 backend and approved MVP direction**.

Implementation status: M7.1 is required before production launch. The current
local API uses in-memory persistence and static development credentials, and
the current web authentication entry points are previews. Short-lived MCP
capabilities, subscription-expiry enforcement, Redis invalidation, integrated
local services, complete browser checkout, operator tooling, and production
identity are planned in M7.1 and must not be represented as already deployed.

## Purpose

AgentPay turns a seller's existing API or digital service into one commerce surface for people and software agents. It automates repository integration through MCP-enabled coding agents, publishes human and machine-readable storefronts, enforces purchase policies, verifies payment, forwards fulfillment requests, and records evidence for later dispute handling.

## Runtime components

### Web application

The Next.js application provides the public AgentPay marketing and product site,
sign-up and sign-in entry points, seller onboarding, verified
payment-destination setup, product and route configuration, an asset-separated
sales dashboard, seller-branded storefronts, browser-wallet purchase guidance,
the agent buyer demonstration, live approval, transaction evidence, receipts,
webhook status, and dispute views.

Server Components render read-heavy pages. Client Components are limited to buyer interaction, approval decisions, WebSocket status, and small optimistic controls.

The public site and authenticated product share one token, typography,
navigation, and responsive-layout system. Marketing-only visual effects remain
isolated from dashboard interaction code, load progressively, and disable under
`prefers-reduced-motion`. Public pages render meaningful HTML before client
JavaScript so human visitors, crawlers, and software agents receive the same
truthful product explanation.

### Seller automation

The seller automation surface consists of a cloud-hosted remote MCP server,
the AgentPay local connector for project-key installations, standards-compatible
direct OAuth access for capable remote MCP hosts, coding-agent setup
instructions, maintained verification middleware, and sandbox validation
commands. The connector holds the project key, exchanges it through the
proprietary AgentPay bootstrap endpoint, keeps a short-lived MCP access token
in process memory, and proxies bounded JSON-RPC. It contains no payment,
publication, entitlement, or signing authority. A host that connects directly
to `/mcp` never receives or submits a project key and follows the protected
resource metadata advertised at `/.well-known/oauth-protected-resource/mcp`.

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

Setup bundle v2 also detects supported application stacks and proposes
stack-native technical SEO, answer-engine optimization, and agent-discovery
changes. Those changes include visible metadata, canonical URLs, structured
data, sitemap and robots output, semantic page content, `llms.txt`, and manifest
consistency. The automation cannot guarantee ranking and must not generate
deceptive or invisible search content.

### Go API

One deployable Go binary owns all authoritative business rules through isolated packages:

- `catalog`: sellers, routes, manifests, and pricing.
- `intents`: immutable purchase proposals and request hashes.
- `policy`: budget and approval evaluation.
- `approvals`: sessions, invitations, decisions, and approval tokens.
- `payments`: x402 challenge creation and facilitator verification.
- `settlement`: seller payment destinations, ownership verification, rotation,
  and reconciliation status.
- `proxy`: upstream request forwarding and seller request signatures.
- `evidence`: append-only evidence events and chain verification.
- `disputes`: deterministic classification and recommendations.
- `agents`: Bedrock tool orchestration and deterministic fallback.
- `analytics`: deterministic seller-scoped, asset-separated sales read models over bounded transaction queries.
- `notifications`: signed seller webhook subscriptions and delivery attempts.
- `billing`: seller plans, entitlements, usage meters, and invoice exports; it never
  controls buyer funds or seller settlement.
- `operations`: UTC-month API, MCP, and webhook-delivery counters plus static
  route and webhook-subscription entitlement enforcement.
- `audit`: immutable seller-visible control-plane change history and bounded
  tenant-scoped reads.

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

### Feature package layout

Backend feature packages separate responsibilities by file without adding wrapper layers:

- `models.go` owns domain types, states, immutable values, and read accessors.
- `service.go` owns validation, construction, deterministic rules, and internal calculations.
- `controller.go` owns public feature operations and guarded state-changing commands.
- `persistence.go` owns explicit serialization snapshots when private domain state must be stored.

Shared primitives and storage adapters remain organized by their concrete responsibility rather than being forced into controller/service files.

### AWS managed services

- API Gateway HTTP API routes browser, agent, and proxy traffic.
- API Gateway WebSocket API distributes approval updates.
- Lambda runs the Go modular monolith for the hackathon.
- DynamoDB stores mutable operational state and idempotency records.
- S3 stores immutable evidence event objects.
- KMS signs evidence event hashes.
- Cognito authenticates sellers; hackathon approval links use scoped invitation tokens.
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
8. A sandbox purchase verifies discovery, payment gating, signed forwarding, and exactly-once fulfillment.
9. The seller explicitly publishes the storefront and deploys the prepared application.

The coding agent prepares and validates changes. It never receives production payout secrets and never publishes or deploys without seller confirmation.

Payment-destination activation is a settlement-domain operation. HTTP accepts a
bounded proof, the service verifies challenge state and wallet ownership, and
the repository atomically updates the active asset/network claim plus the new
and rotated destination records. Transport code never performs signature
recovery, and persistence never stores raw challenge or signature material.

## Buyer channels

Both buyer channels consume the same published paid routes:

- **Agent channel:** manifest or `llms.txt` discovery, immutable intent, optional approval, x402 payment, and signed fulfillment.
- **Browser channel:** seller-branded product page, immutable intent, optional
  approval, x402-compatible wallet payment, and the same signed fulfillment.

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
  ownership from claims.
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
- Approval invitation fragments are exchanged once into a browser grant set
  represented by a Secure, HttpOnly, SameSite=Strict cookie. State-changing
  approval and browser-purchase requests additionally require a double-submit
  CSRF token and exact allowed Origin. One browser grant can hold multiple
  independently scoped approval sessions.
- Seller fulfillment uses a 30-60 second ES256 execution capability in
  `X-AgentPay-Execution-Capability`; it is minted only after finalized payment
  and the exactly-once forwarding claim.

AgentPay publishes overlapping ES256 public keys at
`/.well-known/jwks.json`. MCP access, discovery, and execution signatures pin
their own type, audience/domain, and claim sets. Browser purchase and approval
authority are opaque server-side grants rather than JWTs. A valid signature or
cookie never replaces current entitlement, credential, ownership, state, CSRF,
or replay checks.

## Purchase lifecycle

1. The buyer reads a short-lived signed AgentPay manifest or product document.
2. A buyer agent authenticates with its buyer credential; a browser creates a
   bounded server-side purchase session for the selected product. Either channel creates a
   purchase intent containing the immutable route, product snapshot, request
   hash, destination, quote, channel, and expiration.
3. The policy engine evaluates the immutable intent.
4. If approval is required, the API creates a session and returns HTTP `428` with invitation metadata. No x402 challenge is issued yet.
5. When all required users approve, the purchase owner—not either approver—claims a short-lived approval token bound to the complete intent hash.
6. The buyer requests the paid route with the intent identifier and optional approval token.
7. The gateway returns an x402 challenge when payment is absent.
8. The buyer retries with payment proof.
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
- Approval is bound to the intent hash, not only the intent ID.
- Payment identifiers are globally unique in the transaction table.
- A DynamoDB conditional write changes a transaction from `PAYMENT_VERIFIED` to `FORWARDED` only when `paymentFinality=finalized`, the expected version matches, and no forwarding owner exists; only the winner calls the seller.
- Retried requests return the stored response metadata when replay is safe.

## Failure behavior

| Failure | Required behavior |
|---|---|
| Bedrock timeout | Offer deterministic buyer fallback; never infer approval. |
| Facilitator unavailable | Return retriable `503`; do not call the seller. |
| Evidence append fails before forwarding | Stop and return `503`; do not call the seller. |
| Seller timeout | Record delivery failure and classify `not_delivered` disputes as refund-recommended. |
| WebSocket disconnect | Approval remains queryable over REST; reconnect receives the current snapshot. |
| Duplicate paid retry | Return the prior transaction outcome; never forward twice. |
| Expired approval | Require a new approval session before issuing another challenge. |
| Reconciliation delayed | Keep payment and finalized amounts separate; never fabricate settlement completion. |
| Seller webhook unavailable | Retain the authoritative event, retry within policy, and expose the failed delivery in the dashboard. |
| Seller entitlement inactive or expired | Return `403 subscription_inactive` for authenticated seller/MCP operations, `410 seller_inactive` for public discovery, and reject new intent, challenge, verification, and settlement authorization. |
| MCP confirmation missing, expired, replayed, or mismatched | Fail before domain mutation; never infer approval from caller fields. Exact idempotent replay returns the stored redacted result only after the original grant was validly consumed. |
| Stripe renewal payment fails | Do not extend `accessEndsAt`; notify the seller, block network participation exactly at the existing boundary, permit only 72 hours of billing recovery/historical reads, then suspend. |
| Stripe events arrive late, duplicated, or out of order | Verify and durably deduplicate the event, fetch current provider state, and conditionally replace the complete entitlement projection using a local monotonic reconciliation revision. |
| Seller reactivates after access stopped | Require confirmed paid state, increment the entitlement epoch, invalidate caches/discovery, and require project-key rotation before credential-backed access resumes. |
| Fraud quarantine | Suspend immediately regardless of Stripe state; Stripe events cannot clear it, and finalized unfulfilled buyer payments enter the explicit incident/refund path. |
| Project key revoked or rotated | Deny bootstrap exchange; reject access tokens naming the revoked predecessor credential. |
| Stale capability epoch | Return `401 token_revoked`; do not consume quota or perform the operation. |
| Redis unavailable on a transaction-critical check | Fail closed with `503 dependency_unavailable`; public discovery may return only an unexpired signed document or inactive result. |
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
shipping, inventory, tax calculation, production custody, generalized refunds,
cross-seller reputation, autonomous negotiation, arbitrary remote code
execution, guaranteed search ranking, or automatic quality judgments.
