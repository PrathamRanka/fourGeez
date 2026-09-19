# Security requirements and threat model

Status: **Required for every implementation task**.

Implementation status: the existing controls cover the M0–M7 development
preview. The reduced M7.1 Lean V1 adds production subscription enforcement,
short-lived MCP capabilities, authoritative entitlement and credential checks,
signed discovery status, execution capabilities, authenticated seller sessions,
and their required failure tests. Redis/outbox scaling and a full operator UI
are deferred; transaction-critical authorization reads authoritative persistence
and fails closed. Until M7.1, M8, and M9 pass, cancellation-safe production
access is not implemented.

## Security goals

- A model cannot authorize or execute a purchase by itself.
- One payment proof cannot cause more than one seller invocation.
- The buyer maximum and wallet authorization apply only to the exact immutable intent.
- Evidence alteration is detectable.
- Seller configuration cannot turn the proxy into an SSRF service.
- Secrets and payment proofs never appear in browser bundles, API responses, evidence payloads, or logs.
- A dispute result is reproducible from recorded facts and rule version.
- A coding agent cannot publish products, rotate credentials, or deploy production changes without explicit seller authorization.
- Browser and agent purchase channels cannot bypass the same pricing, payment, and fulfillment rules.
- A seller payment destination cannot become active without bounded ownership verification.
- Generated SEO/AEO content cannot invent claims, reviews, prices, availability, or hidden search content.

## Protected assets

- Test-wallet private key and future payment credentials.
- Seller HMAC signing secrets.
- Seller identity data.
- Browser purchase grants and execution capabilities.
- Payment proof and facilitator response.
- Seller request/response content.
- Evidence chain, signatures, and dispute decisions.
- Seller project credentials and MCP authorization grants.
- MCP confirmation-grant secrets, keyed digests, and exact mutation bindings.
- Browser purchase-session capabilities and entitlement revision state.
- Seller repository contents, deployment credentials, and generated configuration.

The per-seller HMAC request-signing contract is the implemented M7 mechanism,
not the final fork-resistant execution authorization. Because a seller holding
the shared HMAC secret can generate the same MAC locally, LCH-004, LCH-006, and
LCH-013 must replace fulfillment authorization with a cloud-only
asymmetric execution capability before production launch. Webhook HMAC remains
a separate receiver-authentication mechanism and does not grant cloud
transaction authority.

## Primary threats and controls

| Threat | Required controls |
|---|---|
| Payment replay | Unique payment identifier index, conditional write, intent expiration, exactly-once forwarding claim |
| Intent or quote modification after wallet consent | Canonical intent and request hashes; exact amount, asset, network, destination, and resource validation; reject every mismatch |
| Model prompt injection | Fixed tool schemas, no direct HTTP tool, server-side route lookup, amount/budget revalidation, bounded calls and timeouts |
| SSRF through seller URL | HTTPS allowlist, DNS/IP validation, block loopback/link-local/private/metadata ranges, no redirects, re-resolve on connection |
| Malicious seller response | Response byte/time limits, content-type allowlist, no active HTML rendering, hash before storage |
| Evidence tampering | Append-only object keys, Object Lock, versioning, canonical hashes, previous hash, KMS signature, separate verifier role |
| Secret leakage | Secrets Manager, log redaction, no secret env values where avoidable, browser bundle scan, rotation procedure |
| Tenant data access | Cognito subject-to-seller authorization on every seller route; no caller-supplied tenant trust |
| Seller session replay | Validate exact Cognito issuer, client ID, access-token use, RS256 signature, timestamps, JTI, and token-family identifier; keep the versioned production session payload bounded, compressed before AES-256-GCM sealing, and safely below browser cookie limits; check a hashed server-side revocation record on every request |
| Duplicate mutation | Required idempotency key bound to caller, operation, and request hash |
| Cancellation/payment race | Conditionally transition one intent from `ready` to exactly one of `cancelled`, `expired`, or `executed`; checkout must win the `executed` claim before issuing a challenge |
| Forged refund record | Seller bearer authentication, concealed cross-tenant lookup, finalized-payment and `refund_recommended` checks, exact amount/asset/network binding, append-only one-record-per-dispute persistence, and idempotency |
| Denial of service | API throttles, body limits, route limits, Lambda concurrency, upstream timeout, Bedrock call budget |
| Repository prompt injection | Treat repository text as untrusted, expose only allowlisted MCP tools, and require confirmation for commercial or deployment mutations |
| Self-asserted MCP confirmation | Ignore caller approval booleans, summaries, and timestamps as authority; require a cloud-issued one-time grant bound to the authenticated seller, credential, exact tool, target, canonical arguments hash, expected resource version, and expiry |
| Over-scoped integration credential | Bind each credential to one seller, use explicit scopes and expiration, hash it at rest, and support immediate revocation |
| Forked or modified local MCP | Keep token issuance, entitlement, publication, payment, and transaction authority in AgentPay cloud; the local connector can only request short-lived capabilities and proxy bounded messages |
| Stale access capability | Verify ES256, exact issuer/audience/type, expiry, current credential state, and current `entitlementEpoch` on every protected operation |
| Stale or forged discovery | Verify the AgentPay discovery signature and expiry, then perform fresh cloud authorization before intent creation, challenge, verification, or settlement |
| Stale or forged product schema | Canonicalize bounded closed schemas, bind validation and seller confirmation to the exact route version and contract hash, sign the published product contract, and reauthorize commerce from current cloud state |
| Forged browser checkout | Use a short-lived purchase capability bound to seller, route, product slug, request hash, maximum amount, and browser channel; never expose seller credentials to the browser |
| Generated secret exposure | Write secrets only to ignored server-side configuration, scan generated changes, and never serialize secrets into browser code or model prompts |
| Unauthorized publication or deployment | Produce a reviewable plan and diff, then require seller confirmation before publish, credential rotation, or production deployment |
| Human checkout forgery or replay | Authenticate provider callbacks, bind them to immutable intents, process them idempotently, and reuse transaction replay protection |
| Capability downgrade or false fallback | Publish only runtime-enabled payment capabilities, bind the selected rail/network/asset to the immutable intent and challenge, and never silently substitute another rail |
| Unknown settlement causes double authorization | Persist the verified payment identifier and proof hash before settlement; require the identical proof and intent on recovery and reject a changed proof |
| Payout-address substitution | Verify wallet ownership, bind destinations to seller and asset/network, require explicit confirmed rotation, and freeze the destination in each purchase intent |
| Dashboard revenue inflation | Derive aggregates idempotently from authoritative payment and transaction events and keep assets/networks separate |
| Forged seller webhook | Sign canonical payloads, include event IDs and timestamps, use constant-time verification, and make redelivery idempotent |
| Webhook SSRF | Apply the seller-proxy public-address, DNS-rebinding, redirect, timeout, and response-size controls to webhook destinations |
| SEO/AEO abuse | Require visible-content consistency, prohibit fabricated claims and keyword stuffing, validate structured data, and never promise ranking |
| Tenant resource exhaustion | Apply seller-scoped quotas and rate limits to API, MCP, route, analytics, and webhook operations |
| Unpaid seller retains network access | Require `status=active` and `now < accessEndsAt` at every privileged boundary; grace is recovery/read-only and never paid-network authority |
| Unauthorized free launch access | Never auto-provision cloud entitlements or expose a seller self-grant API/UI; require an environment-bound assumed least-privilege operator role, explicit expiry/effective time, optimistic entitlement version, reviewed plan digest, exact confirmation phrase, and an atomic idempotency, reconciliation, projection, and audit write |
| Forged or replayed Stripe webhook | Verify `Stripe-Signature` over exact raw bytes, bind the endpoint secret to environment/account, deduplicate provider event IDs, and reconcile current provider objects instead of trusting arrival order |
| Stripe outage or missed webhook | Enforce the local `accessEndsAt` independently, fail closed for new commerce, and recover through the durable inbox plus scheduled reconciliation |
| Fraudulent reactivation | Give fraud quarantine precedence over provider state and require operator clearance plus project-key rotation before access resumes |

## Canonical hashing

Payment-destination ownership challenges use EIP-191 `personal_sign` for the
initial `eip155` network support. The signed text includes a version, seller ID,
destination ID, asset, network, public address, 256-bit nonce, and RFC 3339 UTC
expiry. Challenges expire after ten minutes. AgentPay stores only the SHA-256
challenge hash and compares it in constant time before signature recovery. Raw
challenges are returned with `Cache-Control: no-store`; raw signatures are never
persisted or logged. Contract-wallet ownership proofs require a separately
documented verifier and are not accepted by the initial EOA verifier.

Webhook signing uses a per-subscription 256-bit secret returned once at
creation. Domain records contain only an opaque secret reference. Versioned
canonical JSON is signed with HMAC-SHA256 over the event ID, delivery timestamp,
and body digest; verification uses constant-time comparison. Subscription URLs
must be public HTTPS endpoints, and delivery-time DNS checks remain mandatory.

Canonicalization must be versioned. Version 1 uses:

1. UTF-8 JSON.
2. Object keys sorted lexicographically.
3. No insignificant whitespace.
4. Integers and amount strings preserved exactly.
5. Arrays retain order.
6. Hash input includes canonicalization version and domain separator.

Use different domain separators for intent, evidence, and MCP mutation hashes,
such as `agentpay.intent.v1`, `agentpay.evidence.v1`, and
`agentpay.mcp-mutation.v1`. The MCP mutation hash excludes the one-time grant
and idempotency key but includes the exact tool, target, expected resource
version, and canonical arguments. Never hash an ambiguous string concatenation.

Signed discovery uses `agentpay.discovery.v1`, followed by one NUL byte and the
RFC 8785 canonical JSON bytes of the unsigned manifest, product document, or
tombstone. The signature is raw 64-byte ES256 `(r || s)` encoded with base64url
without padding. Discovery expires after five minutes and never substitutes
for fresh commerce authorization.

The dedicated product document uses `agentpay.product-contract.v1`, one NUL
byte, and RFC 8785 canonical JSON. Its signature binds the route version,
closed input/output schemas, exact price and payment terms, availability,
fulfillment limits, and expiry. It is discovery evidence only.

## Capability classes and key publication

AgentPay uses separate capabilities for separate trust boundaries:

| Capability | Holder | Maximum lifetime | Audience | Authority |
|---|---|---:|---|---|
| Project key | Seller-side connector | Until expiry/rotation/revocation | Bootstrap endpoint only | Request an MCP access token |
| MCP access token | Connector process | 300 seconds | `urn:agentpay:mcp` | Exact seller MCP scopes only |
| MCP confirmation grant | Selected MCP interaction | 5 minutes, one use | One exact cloud MCP mutation | Prove authenticated seller confirmation of exact bound arguments |
| Browser purchase grant | Secure HttpOnly cookie backed by server-side state | Commerce: 10 minutes; read/remediation: 30 days after terminal outcome or dispute resolution | One browser grant containing bounded purchase sessions | Create/pay once during commerce window; read/recover/dispute afterward |
| Execution capability | AgentPay proxy and seller endpoint | 60 seconds | `urn:agentpay:seller:<sellerId>` | One exact finalized transaction request |

JWT capabilities use ES256 only. Protected headers pin a capability-specific
`typ` and a known `kid`. Public verification keys are served from
`/.well-known/jwks.json` with overlapping publication during rotation. Private
signing material remains behind the cloud KMS/HSM boundary and is never placed
in application configuration, seller infrastructure, browser code, logs, or
repositories.

The project-key exchange is proprietary AgentPay bootstrap, not OAuth. It uses
`POST /v1/integration-access-tokens` and returns `Cache-Control: no-store`.
Lean V1 project-key installations must use the AgentPay connector; project keys
are never accepted by `/mcp` or commerce routes. Direct remote OAuth MCP clients
are deferred. Connector access tokens carry exact issuer, audience, subject,
seller, credential, scopes, `entitlementEpoch`, JTI, issued-at, and expiry
claims. Middleware additionally loads current credential and entitlement state;
a valid signature with a stale epoch or revoked credential is rejected.

An MCP access token is permission to request a scoped tool, not confirmation of
a commercial mutation. The seller dashboard's authenticated browser/BFF
boundary displays the canonical change and creates an opaque 256-bit
confirmation grant. Issuance requires seller ownership, current active
entitlement, exact allowed Origin, double-submit CSRF, and any required recent
reauthentication. The grant is returned with `Cache-Control: no-store`; only a
keyed digest and binding metadata are persisted. Project keys, MCP bearers,
buyer credentials, models, repositories, and connectors cannot mint grants.

The MCP mutation atomically consumes the grant with its idempotency decision.
Wrong seller, credential, tool, target, canonical arguments, expected resource
version, expiry, revocation, or prior consumption fails before mutation.
Caller-supplied `approved`, `summary`, and `confirmedAt` values are display
metadata only during migration and are forbidden as production authority.

## Seller package supply chain

Lean V1 seller packages are distributed as immutable release artifacts, not
from a mutable branch, workspace subdirectory, or unpinned URL. A release must:

- originate from a clean protected commit and record its full Git SHA;
- produce deterministic npm tarballs whose second build has identical SHA-256
  digests;
- include only declared runtime files, the package README, proprietary license,
  and reviewed notice;
- install into an empty project without registry access or lifecycle scripts;
- contain no source maps, tests, caches, environment files, credentials,
  private keys, authorization material, or seller data;
- publish a checksum manifest and machine-readable provenance document beside
  the tarballs; and
- remain blocked from npm publication while `private: true` is set and until
  customer-use terms, contributor provenance, third-party notices, protected
  release tags, and registry credentials are explicitly approved.

Seller instructions verify the checksum before installation and keep the
reveal-once project key outside repository files and host configuration. An
installed connector remains untrusted and receives no cloud signing or payment
authority.

Buyer-side approval invitation, cookie, token, and WebSocket mechanisms are
disabled for Lean V1. Historical M2 code and records are not transaction
authority and must not be exposed by the launch runtime. Seller sessions,
project keys, access tokens, browser purchase cookies, payment proofs, and
execution capabilities must never appear in URLs.

## Seller request authorization

The production contract authenticates each forwarded seller request with
`X-AgentPay-Execution-Capability` and `X-AgentPay-Transaction-Id`. The ES256 JWT
has `typ=agentpay-execution+jwt` and binds issuer, exact seller audience,
transaction, seller, route, method, literal path, lowercase SHA-256 of the exact
raw body, `paymentFinality=finalized`, JTI, issued-at, and expiry. Seller
middleware verifies a pinned algorithm, resolves `kid` through AgentPay JWKS,
compares every binding, consumes the JTI once, and uses `transactionId` as the
fulfillment idempotency key. Removing seller middleware cannot mint a valid
AgentPay capability or alter the cloud forwarding claim.

Version 1 uses `X-AgentPay-Signature`, `X-AgentPay-Timestamp`, and
`X-AgentPay-Transaction-Id` for existing development fixtures and signs the
following newline-delimited UTF-8 fields with HMAC-SHA256 and base64-encodes the
result:

```text
agentpay.seller-request.v1
<RFC3339 UTC timestamp>
<HTTP method>
<literal route path>
<lowercase SHA-256 body hash>
<transactionId>
```

Version 1 is not permitted for production transaction authorization. Its
secret is resolved server-side from the seller's `signingSecretRef`, must
contain at least 32 bytes, and is never added to request models, logs, evidence,
or responses. Seller verification uses constant-time signature comparison.

## Evidence payload policy

Allowed:

- IDs, timestamps, state transitions, amount/asset/network.
- Hashes of request, payment proof, and response.
- Historical M2 approval labels and decisions when reading retained evidence.
- Upstream HTTP status, duration, content length, and allowlisted headers.
- Dispute reason, rule version, classification, and explanation.

Forbidden:

- Raw `PAYMENT-SIGNATURE` or wallet private material.
- Authorization headers, cookies, historical invitation/approval tokens, or seller secrets.
- Unredacted prompts or arbitrary seller response bodies.
- Full personal addresses, financial account details, or unnecessary user content.

## API requirements

- Maximum JSON body: 1 MiB; paid-route limit is configurable downward.
- Strict content types and JSON decoding with unknown-field rejection for control APIs.
- Stable machine error codes; internal stack traces never leave the service.
- Error responses use the documented status/code pairs: `401` for
  `invalid_credential`, `token_expired`, or `token_revoked`; `403` for
  `subscription_inactive`, `insufficient_scope`, or `permission_denied`; `404`
  for `not_found`; `409` for `state_conflict`, `idempotency_conflict`,
  `payment_replayed`, or `token_replayed`; `410` for `seller_inactive`,
  `payment_expired`, or an expired one-time resource; `422` for
  `validation_failed` or `payment_capability_unsupported`; `429` for
  `rate_limited`; `402` for `payment_required`
  or `payment_rejected`; and `503` for `dependency_unavailable`,
  `payment_unavailable`, or `payment_outcome_unknown`.
- Payment recovery actions are limited to `connect_wallet`, `switch_network`,
  `sign_fresh_authorization`, `retry_same_request`, `retry_same_payment`, and
  `start_new_checkout`. Details never contain a raw proof, wallet signature,
  cookie, authorization header, or private wallet material.
- Authenticate and authorize the seller before consuming seller-scoped API
  quota so an attacker cannot exhaust another tenant's allowance.
- Manual refund recording never moves funds. It derives `sellerId` and
  `recordedBy` from the authenticated seller principal, conceals cross-seller
  disputes as `404`, requires a finalized transaction and current
  `refund_recommended` dispute, validates an exact full amount/asset/network
  match, stores only a bounded external reference, and appends at most one
  record per dispute. Exact idempotent replay returns the original response;
  changed replay fails closed.
- Intent cancellation requires the owning buyer authority, an
  `Idempotency-Key`, and a conditional `ready` transition. At or after
  `expiresAt`, the server records/returns `expired` for a still-`ready` intent;
  an intent claimed earlier remains `executed` and cannot be cancelled.
  Cancellation responses use `Cache-Control: no-store`.
- Lifecycle and recovery projections are derived from authoritative intent,
  transaction, dispute, and refund-record facts. They do not authorize state
  changes, fabricate payment finality, or convert `seller_reported` remediation
  into network-verified proof.
- Network authorization requires `status=active` and current UTC time strictly
  before `accessEndsAt`. `grace`, `suspended`, `cancelled`, and `closed` return
  `subscription_inactive`; grace never authorizes MCP, discovery, publication,
  intent creation, challenge issuance, verification, settlement, or new
  execution.
- Return `permission_denied` for static entitlements, and
  `rate_limited` for exhausted monthly counters; neither response may reveal
  another seller's plan or usage.
- CORS limited to configured web origins.
- `Cache-Control: no-store` on transaction, dispute, refund-record, and 402 responses.
- Security headers on the web app, including CSP and `frame-ancestors 'none'` for sensitive checkout pages unless embedding is intentionally added.
- Constant-time comparison for token and HMAC verification.
- V1 capability metadata must explicitly report AgentPay buyer runtime, A2A
  execution, negotiation, mainnet, multi-currency, and ranking as unavailable.

## MCP and coding-agent requirements

- The AgentPay MCP server exposes bounded commerce operations, documentation resources, and setup prompts; it does not expose an arbitrary shell or unrestricted HTTP proxy.
- Read-only tools are separated from mutating tools by scope.
- Product creation, price updates, publication, credential rotation, and production deployment require explicit confirmation.
- Production MCP product creation, storefront changes, price changes, and
  publication require the cloud-issued one-time confirmation grant; caller-
  asserted confirmation fields never satisfy this requirement.
- Every mutation requires idempotency and records the seller, credential, operation, target, and outcome without recording secrets or repository contents.
- Every authenticated MCP POST consumes one seller-scoped monthly operation
  unit; invalid credentials do not consume quota.
- Project credentials are seller-scoped, stored only as HMAC-SHA-256 digests using a cloud-held pepper, compared in constant time, revocable, and never committed to the seller repository.
- Project credentials are accepted only by the proprietary bootstrap exchange;
  ordinary MCP requests require a short-lived access capability.
- Generated integrations use maintained verification packages. Coding agents must not invent alternate signing or payment validation.
- Automated seller integration verification accepts only the authenticated
  seller and stored draft route identifiers. It uses the protected seller
  forwarder, a fixed `/.well-known/agentpay/sandbox` path, fixed-size protocol
  bodies, no redirects, public-address DNS checks, bounded time and response
  sizes, and redacted fixed messages. It never accepts a URL or secret from the
  caller, creates commerce records, settles funds, or invokes a paid route.
- The TypeScript merchant SDK and adapters are server-only. They preserve exact
  raw request bytes through verification, require seller-provided atomic replay
  and fulfillment stores for multi-instance use, and never accept wallet keys,
  x402 proofs, AgentPay private signing material, or project keys.
- Shopify and WooCommerce credentials remain seller-owned deployment secrets.
  They must not appear in source, fixtures, browser bundles, generated diffs,
  logs, or AgentPay configuration.
- Repository analysis must not upload unrelated source files, `.env` contents, credentials, wallet material, customer information, or proprietary data to AgentPay.
- SEO/AEO generation may inspect public page structure and allowlisted product
  metadata only. It must not upload private source, generate hidden content, or
  state that ranking improvement is guaranteed.
- Storefront validation accepts bounded artifact facts only, performs no remote
  fetches, rejects generated reviews or ratings, and never converts its checks
  into a search-ranking prediction.
- A modified or forked connector receives no private signing key, facilitator
  credential, publication authority, entitlement authority, or transaction
  authority. If it bypasses AgentPay cloud, its traffic is not an AgentPay
  transaction and cannot receive official receipts, evidence, signatures, or
  discovery status.

## Human checkout requirements

- Card checkout is deferred to milestone H1 and is not required for the
  agent-first x402 release.
- Browser x402 checkout uses an opaque server-side purchase grant distinct from
  seller and buyer-agent credentials. A Secure HttpOnly cookie survives reloads
  and is bound to one product/request/maximum amount. Commerce authority expires
  within ten minutes, while read and dispute authority lasts for 30 days after
  terminal fulfillment/failure or a timely dispute's resolution. Finalized payment binds the payer wallet so a
  one-time signed recovery challenge can restore read/remediation access without
  restoring commerce authority. State-changing cookie-authenticated requests
  require double-submit CSRF and exact Origin validation.
- A future card checkout provider must be selected and its official integration guidance recorded before adding a dependency.
- A provider success redirect is not proof of payment; only an authenticated server callback may advance payment state.
- Provider events require replay protection and idempotent processing.
- Human checkout uses the same frozen seller quote, buyer-maximum check, wallet authorization, fulfillment claim, evidence chain, and dispute rules as agent x402 checkout.
- Merchant-of-record, platform-fee, refund, tax, chargeback, and seller-payout responsibility must be documented before production activation.

## Production blockers

The system must not process real funds until all are complete:

- External application and infrastructure security review.
- Legal review of payment role, refund handling, sanctions, privacy, and retention.
- Production wallet architecture avoiding raw application-held private keys.
- Incident response and key-compromise runbooks.
- Backup/restore and disaster recovery test.
- Tenant isolation penetration test.
- Dependency, container, and IaC scanning in CI.
- Documented facilitator SLA and failure semantics.

## Security verification checklist

- [ ] No tracked file contains secret-like values.
- [ ] Tests prove replay and duplicate forwarding are blocked.
- [ ] Tests prove modified intents, quotes, destinations, or request bodies invalidate payment authorization.
- [ ] Tests cover private, loopback, link-local, IPv6, redirect, and DNS-rebinding SSRF cases.
- [ ] Evidence verification fails after any payload, order, hash, or signature change.
- [ ] Browser build contains no server-only configuration.
- [ ] Logs are tested for credential redaction.
- [ ] IAM policies pass least-privilege review before demo deployment.
