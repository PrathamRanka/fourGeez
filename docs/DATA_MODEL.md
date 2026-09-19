# Data model and state machines

Status: **Implemented M7 fields remain the development compatibility baseline; sections explicitly introduced by LCH-003/LCH-004 are the locked M7.1 production target and require the named implementation/migration tasks before activation**.

The current schema does not yet contain the complete Lean V1 model for
subscription lifecycle, execution-token replay, or buyer remediation. LCH-003
and LCH-004 lock product identity, capability, discovery, browser
purchase-session, and execution-authorization contracts. The reduced launch
tasks LCH-008 through LCH-014 own all remaining persistence changes. Lean V1
uses authoritative database reads for transaction-critical entitlement and
credential checks; Redis and distributed invalidation records are deferred.

## Conventions

- Implemented IDs use canonical ULIDs with sortable, opaque prefixes: `sel_`, `rte_`, `int_`, `aps_`, `txn_`, `evt_`, `dsp_`, `key_`, `dst_`, `whk_`, `whd_`, `aud_`, and `mtr_`. The historical `aps_` records are not created or consumed by Lean V1.
- Timestamps are RFC 3339 UTC strings.
- Payment amounts are canonical strings in atomic units; floating-point numbers and leading zeros are forbidden at persistence boundaries, except that zero is `"0"`.
- `asset` is a chain-specific contract or asset identifier.
- `network` uses the identifier supplied by the verified x402 SDK.
- Request and response hashes use lowercase SHA-256 hex.
- JSON request bodies are canonicalized with RFC 8785 JCS before hashing; non-JSON bodies are hashed byte-for-byte.
- Intent hashes use the `agentpay.intent.v1` domain separator and include every execution-relevant field.
- Persisted records include `createdAt`; mutable records also include `updatedAt` and integer `version`.

## Commerce terminology and compatibility

A published `PaidRoute` is the V1 product offered in both the browser storefront and machine-readable catalog. A separate product entity is not introduced until a real fulfillment type cannot be represented by an HTTPS route.

The browser-wallet milestone adds `purchaseChannel` to new purchase intents and
transactions. Records created before that migration omit the field and are
interpreted as `purchaseChannel=agent`. The initial `paymentRail` remains
`x402`; a future card rail requires its own compatibility migration. Writers
must not emit new fields until the corresponding migration and repository tests
are complete.

## Entities

### Seller

| Field | Type | Notes |
|---|---|---|
| `sellerId` | string | Primary identifier |
| `ownerSubject` | string | Cognito subject |
| `slug` | string | Unique public storefront slug |
| `name` | string | Display name |
| `upstreamBaseUrl` | string | HTTPS only outside local development |
| `signingSecretRef` | string | Secrets Manager ARN/reference, never secret material |
| `verifiedUpstreamBaseUrl` | string/null | Exact service origin last verified by the AgentPay sandbox; a configured-origin change invalidates it |
| `verifiedSigningSecretRefHash` | string/null | Domain-separated SHA-256 of the signing-secret reference used by the successful sandbox probe |
| `serviceEndpointVerifiedAt` | timestamp/null | Cloud-observed successful signed, replay-safe endpoint verification |
| `status` | enum | `draft`, `active`, `suspended` |
| `createdAt`, `updatedAt` | timestamp | UTC creation and latest status/configuration change |
| `version` | integer | Starts at 1 and increments on mutation |

### SellerWorkspace

`SellerWorkspace` stores only resumable seller-operating preferences and
system-attested onboarding progress. Payment, entitlement, credential,
product, transaction, evidence, and webhook facts remain authoritative in
their owning records and are re-read when the dashboard or publication gate is
evaluated.

| Field | Type | Notes |
|---|---|---|
| `sellerId` | string | Owning seller; `PK=SELLER#<sellerId>`, `SK=WORKSPACE` |
| `ownerSubjectHash` | string | SHA-256 of the authenticated owner subject; raw identity claims are not duplicated |
| `connectorVerifiedAt` | timestamp/null | Written only after a cloud-observed connector verification |
| `sandboxPurchaseTransactionId` | string/null | Must name an authoritative fulfilled transaction owned by the seller |
| `storefrontPreviewedAt` | timestamp/null | Server-recorded completion of the preview step |
| `settings.supportEmail` | string | Optional seller support address |
| `settings.securityNotificationEmail` | string | Optional security-notification address |
| `settings.webhookFailureNotifications` | boolean | Seller preference for delivery-failure notices |
| `createdAt`, `updatedAt` | timestamp | UTC lifecycle timestamps |
| `version` | integer | Optimistic concurrency version shared by onboarding progress and settings |

Browser state never marks account verification, subscription, payment
destination, credential, product, sandbox transaction, or publication
readiness as complete. Those checks are derived from the identity principal and
the owning authoritative records. Publication fails closed when any required
record is missing or unavailable.

### StorefrontPublication

The signed discovery revision is persisted separately from seller and route
records at `PK=SELLER#<sellerId>`, `SK=STOREFRONT_PUBLICATION`. It stores the
latest domain-separated state fingerprint, monotonically increasing
`publicationRevision`, `updatedAt`, and optimistic `version`. A fresh discovery
read strongly reloads seller, entitlement, routes, payment destinations, and
publication prerequisites; when their public-authority fingerprint changes it
conditionally advances the revision. Concurrent readers may retry but may
never decrease or reuse a revision for different authority state.

Lean V1 permits exactly one seller for each normalized identity-provider
subject. Creation atomically reserves
`PK=SELLER_OWNER#<sha256("agentpay.seller-owner.v1" + NUL + ownerSubject)>`,
`SK=SELLER` with the seller ID; current-seller lookup reads this claim and then
strongly reads the seller profile. Raw subjects never appear in DynamoDB keys.

### SellerSessionRevocation

Seller access tokens are validated against the exact Cognito issuer, app-client
ID, `token_use=access`, signature, issue time, expiry, subject, token JTI, and
token-family `origin_jti`. Immediate sign-out stores only a domain-separated
SHA-256 digest of `origin_jti` (falling back to `jti` for a provider-equivalent
local session) until the token expiry. A matching unexpired record denies every
seller API request; a persistence failure fails authentication closed.

### PaymentDestination

A payment destination belongs to one seller and one exact asset/network pair.
It contains public addressing and verification metadata only. Private keys,
seed phrases, raw signed challenges, and wallet-provider credentials are never
accepted or persisted.

| Field | Type | Notes |
|---|---|---|
| `destinationId` | string | `dst_` prefixed ULID |
| `sellerId` | string | Owning seller |
| `asset`, `network` | string | Exact payment pair supported by the x402 adapter |
| `address` | string | Public destination validated for the selected network |
| `status` | enum | `pending_verification`, `active`, `disabled`, `rotated` |
| `challengeHash` | string/null | Hash of the current one-time ownership challenge |
| `challengeExpiresAt` | timestamp/null | Short-lived verification expiry |
| `verifiedAt` | timestamp/null | First successful ownership proof |
| `createdAt`, `updatedAt` | timestamp | UTC lifecycle timestamps |
| `version` | integer | Used for guarded activation, disabling, and rotation |

Only one destination may be active for a seller, asset, and network. A route
references an active destination. Rotation affects only purchase intents
created after the rotation and requires explicit seller confirmation.

For `eip155` networks, ownership verification uses a ten-minute EIP-191
`personal_sign` challenge that binds the seller, destination, asset, network,
address, nonce, and expiry. The raw challenge is returned to the seller once;
only its lowercase SHA-256 hash and expiry are persisted. Raw signatures are
validated and discarded. The first verified destination claims its exact
asset/network pair. Replacing that claim atomically activates the new
destination and marks the prior destination `rotated` only when the seller sent
`confirmRotation=true`.

### IntegrationCredential

An integration credential authorizes one coding-agent or MCP connection to a
single seller. Raw credential values are displayed only at creation and are
never persisted.

| Field | Type | Notes |
|---|---|---|
| `credentialId` | string | `key_` prefixed ULID |
| `sellerId` | string | The only seller this credential may access |
| `tokenHash` | string | HMAC-SHA-256 digest of the complete token using a cloud-held pepper; the pepper is never persisted with the record |
| `label` | string | Seller-visible installation name |
| `scopes` | string array | Explicit read, configure, publish, validate, or rotate permissions |
| `entitlementEpoch` | integer | Entitlement epoch at issuance; bootstrap exchange requires an exact match with the current authoritative entitlement |
| `expiresAt` | timestamp/null | Required for temporary setup credentials |
| `revokedAt` | timestamp/null | Revocation makes the credential unusable immediately |
| `lastUsedAt` | timestamp/null | Last successful bootstrap exchange; ordinary MCP use does not update it |
| `replacedByCredentialId` | string/null | Successor created by an atomic rotation |
| `createdAt`, `updatedAt` | timestamp | UTC lifecycle timestamps |
| `version` | integer | Used for guarded rotation and revocation |

Credential scope never implies permission to deploy a seller repository or
access seller infrastructure. Deployment authorization remains local to the
seller's coding-agent environment.

New production credentials use `apc2.<credentialId>.<randomSecret>`.
`credentialId` is a globally unique lookup hint and grants no authority without
constant-time verification of the complete keyed digest. Existing
`apc1.<sellerId>.<credentialId>.<randomSecret>` credentials are migration-only
bootstrap credentials and must be replaced by the next explicit rotation.
Neither format is accepted as an ordinary MCP, purchase, control-plane, or
seller-execution bearer token.

Rotation is atomic: create a successor with a new ID and secret, mark the
predecessor revoked, preserve the approved scopes unless explicitly narrowed,
and return one successor secret value. Failure leaves the predecessor valid and
creates no successor. Success denies future bootstrap exchanges for the
predecessor and invalidates access tokens that name its `credentialId`.

Rotation is idempotent without storing plaintext credentials. The idempotency
record stores the committed response secret only as KMS envelope-encrypted
ciphertext for at most ten minutes, scoped to the authenticated seller,
operation, predecessor, and request hash. An exact retry within that window
returns the same successor and secret. After the encrypted response expires,
the predecessor remains revoked and the API returns `409 state_conflict` with
the successor credential ID; the seller must rotate that successor. A timeout
before the atomic database commit leaves the predecessor valid, while a lost
response after commit follows this recovery rule.

### IntegrationAccessCapability

The proprietary project-key bootstrap exchange returns a non-persisted ES256
JWT. It is not OAuth and does not use an OAuth grant type. Required claims are:

| Claim | Rule |
|---|---|
| `iss` | Exact environment API origin |
| `aud` | Exact AgentPay audience, initially `urn:agentpay:mcp` |
| `sub` | The authenticated `credentialId` |
| `sellerId`, `credentialId` | Match the current credential record |
| `scope` | Space-delimited subset of current credential scopes and MCP-supported scopes (`read`, `configure`, `publish`, `validate`); `rotate` is never issued to the MCP audience |
| `entitlementEpoch` | Current seller entitlement epoch |
| `jti` | Unique `cap_` identifier |
| `iat`, `exp` | Numeric dates with a 120-300 second lifetime |

The protected header is `typ=agentpay-access+jwt`, `alg=ES256`, and a known
AgentPay JWKS `kid`. Authorization checks current credential revocation and
entitlement epoch; signature validity alone never authorizes a request.

This proprietary capability is used only by the mandatory AgentPay connector
for project-key installations. Direct remote MCP OAuth clients and OAuth
protected-resource metadata are deferred in Lean V1. Project keys are never
accepted by the remote `/mcp` endpoint.

### MCPConfirmationGrant

An `MCPConfirmationGrant` is an opaque, one-time authorization created only by
the authenticated seller browser/BFF after the seller reviews one exact MCP
commercial mutation. MCP access scope alone cannot create or replace it.

| Field | Type | Notes |
|---|---|---|
| `confirmationGrantId` | string | `mcg_` prefixed ULID and lookup hint |
| `sellerId` | string | Derived from the authenticated seller session |
| `credentialId` | string | Exact integration credential permitted to relay the mutation |
| `tool` | enum | `configure_storefront`, `configure_route`, `change_route_price`, or `publish_route` |
| `targetType` | enum | `seller` or `paid_route` |
| `targetId` | string | Seller ID for seller configuration/new-route creation; route ID for route mutation |
| `argumentsSha256` | string | Lowercase SHA-256 of RFC 8785 canonical arguments with `agentpay.mcp-mutation.v1`; excludes grant and idempotency key |
| `expectedResourceVersion` | integer | Exact seller or route version reviewed by the seller |
| `summary` | string | Seller-visible redacted description; never authority by itself |
| `tokenDigest` | string | Keyed digest of the `mcg1.<confirmationGrantId>.<randomSecret>` opaque 256-bit secret; raw grant is never persisted |
| `issuedBySellerPrincipal` | string | Authenticated seller-session subject used for audit only |
| `issuedAt`, `expiresAt` | timestamp | UTC timestamps; expiry is exclusive and at most five minutes after issue |
| `consumedAt` | timestamp/null | Set once in the same transaction as the mutation/idempotency decision |
| `consumedByIdempotencyKeyHash` | string/null | Non-sensitive binding used to distinguish exact retry from replay |
| `revokedAt` | timestamp/null | Optional pre-consumption revocation by seller or security controls |
| `version` | integer | Conditional-write version |

The creation response returns the raw grant once with `Cache-Control: no-store`.
The record stores only its keyed digest and metadata. Reissuing the same binding
atomically revokes any earlier unconsumed grant, so a browser retry leaves at
most one usable grant. Consumption requires the same seller, credential, tool,
target, arguments hash, and expected resource version, plus an unexpired,
unrevoked, unconsumed record. The consume operation and MCP mutation
idempotency decision are atomic. An exact idempotent replay returns the stored
redacted mutation result; different input or a second use fails closed.
Caller-provided confirmation booleans, summaries, and timestamps are not
authorization fields.

### PaidRoute

| Field | Type | Notes |
|---|---|---|
| `routeId` | string | Primary identifier |
| `sellerId` | string | Owning seller |
| `displayName` | string | Seller-approved human product name; normalized whitespace; 1-120 visible characters |
| `productSlug` | string | Seller-scoped immutable public slug; normalized to 3-80 lowercase ASCII letters, digits, and single hyphens |
| `method` | enum | `GET`, `POST` for hackathon |
| `pathPattern` | string | Literal path in v1; no arbitrary regex |
| `description` | string | Published in manifest |
| `mimeType` | string | Expected successful response type |
| `amount` | string | Atomic units |
| `asset` | string | Testnet asset identifier |
| `network` | string | Testnet network identifier |
| `payTo` | string | Compatibility snapshot of the verified seller destination used by new intents |
| `paymentDestinationId` | string/null | Planned M7 reference to the verified seller destination |
| `approvalThresholdAmount` | string/null | Historical M2 compatibility field; ignored by Lean V1 and omitted by new public contracts |
| `upstreamTimeoutSeconds` | integer | Range 1–30 |
| `lifecycleStatus` | enum | `draft`, `published`, `paused`, `archived`, or `emergency_disabled` |
| `enabled` | boolean | Compatibility publication flag; true only while `lifecycleStatus=published` |
| `createdAt`, `updatedAt` | timestamp | UTC creation and latest configuration change |
| `version` | integer | Starts at 1 and increments on mutation |

Price updates apply only to purchase intents created after the update. Existing intents retain their frozen amount until they expire or execute.

`routeId`, `sellerId`, `productSlug`, `method`, and `pathPattern` are immutable.
Product slugs are unique within a seller across every lifecycle state, including
archived routes, so a historical public URL is never reassigned to a different
product. The create boundary trims and collapses display-name whitespace and
normalizes slug whitespace, underscores, repeated hyphens, and ASCII case to
one lowercase hyphenated value before the uniqueness claim is written.

Records written before LCH-003 may omit `displayName` and `productSlug`. Reads
derive the display name from the existing seller-authored description and
derive a collision-safe slug from that name plus the final nine characters of
the immutable `routeId`. The next successful route mutation persists both
derived values and atomically reserves the seller-scoped slug claim. This
compatibility path never changes `routeId`, `method`, or `pathPattern`.

Storefront manifests publish the normalized display name and product slug. The
canonical browser URL is `/store/{sellerSlug}/products/{productSlug}`;
`displayName` supplies the visible product and structured-data name while
`description` remains the summary/meta description. Receipt schema version 1
continues to identify historical purchases by immutable `routeId` and does not
retroactively add or resolve catalog text, so existing receipt verification
behavior remains stable.

`lifecycleStatus` is the authoritative route lifecycle. Existing records that
do not contain it are read as `published` when `enabled=true` and `draft` when
`enabled=false`; the next successful mutation writes both fields. Seller API
route creation remains backward compatible and publishes immediately unless
the caller explicitly sends `publishImmediately=false`. MCP route configuration
always creates a `draft`.

Allowed lifecycle transitions are:

- `draft` to `published` or `archived`;
- `published` to `paused` or `emergency_disabled`;
- `paused` to `published` or `archived`;
- `emergency_disabled` to `published` or `archived`; and
- `archived` is terminal.

Publication revalidates seller ownership, active seller status, signing
configuration, route configuration, expected version, quota, idempotency, and
the seller sandbox endpoint where the integration flow requires it. Pause is a
reversible seller action. Emergency disable is a separate high-urgency action
so operations and audit history can distinguish it from a planned pause.
Archive requires a non-published route and is irreversible. Every transition
uses a conditional version write and synchronizes `enabled` to the lifecycle.
Deterministic and sandbox validation results are computed responses and are not
persisted; publication performs fresh validation so a stale result cannot
authorize a changed route.

Historical M2 records may contain `approvalThresholdAmount`. The old
`approval-threshold-v1` evaluator treated the boundary as inclusive. Lean V1
does not evaluate this field, create approval sessions, or block a challenge on
buyer-side approval. New writes must omit it. The seller-approved fixed quote,
the buyer's `maximumAmount`, and wallet authorization are the active V1 consent
and spending controls.

### PurchaseIntent

An intent becomes immutable after creation.

| Field | Type | Notes |
|---|---|---|
| `intentId` | string | Primary identifier |
| `sellerId`, `routeId` | string | Resolved offer |
| `buyerId` | string | Buyer-agent subject, or `browser:<purchaseSessionId>` for a browser purchase |
| `purchaseSessionId` | string/null | Browser session owner; null for agent purchases |
| `purchaseChannel` | enum | `agent` or `browser` |
| `productDisplayName`, `productSlug` | string | Immutable public identity snapshot |
| `paymentDestinationId`, `payTo` | string | Immutable verified destination snapshot |
| `requestMethod`, `requestPath` | string | Canonical target |
| `requestBodyHash` | string | Hash of canonical request bytes |
| `amount`, `asset`, `network` | string | Frozen quote |
| `maximumAmount` | string | Buyer safety limit |
| `intentHash` | string | Canonical hash of all execution-relevant fields |
| `expiresAt` | timestamp | Ten minutes after creation by default |
| `status` | enum | `ready`, `cancelled`, `expired`, `executed` |
| `cancelledAt` | timestamp/null | Server time of an explicit buyer cancellation |
| `cancellationReason` | enum/null | `buyer_requested`; absent unless cancelled |
| `version` | integer | Starts at 1 and increments for the one lifecycle transition |

The price and commercial fields of an intent never change after creation.
Only lifecycle metadata may change. The legal Lean V1 transition is exactly one
of `ready -> cancelled`, `ready -> expired`, or `ready -> executed`.
Cancellation requires the owning buyer authority and is accepted only before
checkout claims the intent. Checkout conditionally claims `ready -> executed`
before issuing the first payment challenge; retries continue through the
deterministic transaction identity. At `now >= expiresAt`, expiration wins over
a new cancellation or execution claim while the intent is `ready`. An intent
claimed before that boundary remains `executed` so the existing transaction can
complete or recover without authorizing a second payment. Terminal intent
states never transition again.
Seller price updates affect only newly created intents. Historical M2 intents
may contain `requiresApproval` and the `approval_pending` or `approved` states;
they are retained for migration/read compatibility only and cannot enter the
Lean V1 payment path.

Intent and transaction API responses derive, but do not persist, an
`ExactPriceBreakdown` containing `calculation=fixed_single_product`,
`quantity=1`, `unitAmount`, `subtotal`, `adjustments=0`, `total`, `asset`, and
`network`. All amount fields are atomic-unit strings. For V1, `unitAmount`,
`subtotal`, and `total` equal the frozen quote; the projection never implies
shipping, tax, discounts, or platform fees.

### BrowserPurchaseSession

A public product page creates a bounded browser purchase session before it may
create an intent or call a paid route. It is distinct from seller sessions,
seller project credentials, and buyer-agent credentials.

| Field | Type | Notes |
|---|---|---|
| `purchaseSessionId` | string | `bps_` prefixed ULID |
| `sellerId`, `routeId` | string | Resolved from seller and product slugs |
| `productSlug` | string | Immutable public product identity |
| `requestBodyHash` | string | Exact request authorization binding |
| `maximumAmount` | string | Browser safety ceiling in atomic units |
| `browserGrantHash` | string | Hash of the opaque cookie grant; the raw cookie is never persisted or logged |
| `csrfTokenHash` | string | Hash of the double-submit CSRF token |
| `walletBindingHash` | string/null | Verified payer network/address hash recorded no later than finalized payment |
| `transactionId` | string/null | The one transaction created from this session |
| `status` | enum | `active`, `completed`, `expired`, or `revoked` |
| `commerceExpiresAt` | timestamp | Exclusive boundary, at most ten minutes after creation, for intent creation, challenge issuance, verification, and settlement |
| `accessExpiresAt` | timestamp | Exclusive browser-access boundary: initially commerce expiry, extended after finalized payment to 30 days after terminal fulfillment/failure, or 30 days after a timely dispute resolves |
| `createdAt`, `updatedAt` | timestamp | UTC lifecycle timestamps |

Creation returns no JavaScript-readable bearer token. It sets an opaque Secure,
HttpOnly, SameSite=Strict `__Host-agentpay_purchase` cookie and a separate
Secure, SameSite=Strict, non-HttpOnly `__Host-agentpay_purchase_csrf` cookie.
State-changing requests authenticated by the purchase cookie require the CSRF
value in `X-AgentPay-CSRF`, exact allowlisted `Origin`, and compatible
`Sec-Fetch-Site`; buyer-agent-key requests do not use this CSRF mechanism. The
server may associate multiple purchase sessions with one browser grant.

`active` may create exactly one intent and proceed through payment until
`commerceExpiresAt`. Finalized payment binds the verified payer wallet and
moves the session to `completed`; completed sessions cannot create or pay again
but retain read and dispute authority for 30 days after terminal fulfillment or
failure. A dispute opened during that period extends access until 30 days after
resolution. A reload retains
the HttpOnly cookie. If the cookie is lost after finalized payment, a short,
single-use server nonce signed by the bound payer wallet may restore only
transaction, receipt, evidence, and dispute access. Losing the cookie before
payment requires creating a new purchase session.

Each recovery challenge is a child record containing `challengeId`,
`purchaseSessionId`, the expected wallet-binding hash, a random nonce, canonical
message hash, `expiresAt`, and nullable `usedAt`. The raw signature is validated
and discarded. Recovery consumes the challenge conditionally, compares the
recovered signer to the finalized payer binding, rotates the browser grant and
CSRF values, and never extends `accessExpiresAt`.

### ApprovalSession (historical M2; deferred and disabled)

These records document the completed M2 implementation and remain available
for migration and compatibility tests. Lean V1 creates no approval sessions,
serves no approval REST or WebSocket runtime, accepts no approval token, and
never requires approval before issuing an x402 challenge. Re-enabling this
domain requires a future contract and migration task.

| Field | Type | Notes |
|---|---|---|
| `sessionId` | string | Primary identifier |
| `intentId`, `intentHash` | string | Approval binding |
| `requiredApprovals` | integer | Fixed to 2 for hackathon |
| `status` | enum | `pending`, `approved`, `vetoed`, `expired` |
| `expiresAt` | timestamp | Same or earlier than intent expiration |
| `approvalTokenHash` | string/null | Only the token hash is persisted |
| `createdAt`, `updatedAt` | timestamp | UTC creation and latest state change |
| `version` | integer | Starts at 1 and increments on every decision or expiration |

Each invitation is stored as a child item containing approver label, token hash,
expiration, exchange timestamp, browser-grant binding, decision timestamp, and
decision. An invitation can be exchanged once and can produce one decision
only.

Raw invitation tokens are returned only in invitation URL fragments and are
exchanged once into a server-side grant set represented by the opaque Secure,
HttpOnly, SameSite=Strict `__Host-agentpay_approvals` cookie. A browser grant
may contain multiple independently bounded approval invitations, so concurrent
sessions do not overwrite one another. The companion
`__Host-agentpay_approval_csrf` cookie is non-HttpOnly; approval decisions
require its value in `X-AgentPay-CSRF`, exact approval-page `Origin`, strict
JSON content type, and the matching session/approver grant. Query-parameter
invitation tokens are not accepted.

Approvers never receive purchase authority. After the session resolves, only
the buyer agent or browser purchase session that owns the intent may call the
completion-token endpoint. That endpoint issues a short-lived approval token
bound to the session ID, intent ID, complete intent hash, purchase owner, and
session expiration; only its SHA-256 hash is stored on the approval session.
The idempotency record may retain the response token as KMS envelope-encrypted
ciphertext no longer than the token expiry so an exact retry returns the same
response without persisting plaintext. A new idempotency key may issue a
replacement only after atomically invalidating any prior unconsumed token,
preventing a lost response from blocking the purchase owner.

### ApprovalBrowserGrant (historical M2; deferred and disabled)

The approval cookie identifies one browser grant by an opaque random value; only
its keyed hash is stored. Each exchanged invitation creates a separate child
binding containing `sessionId`, approver identity, invitation hash,
`csrfTokenHash`, `expiresAt`, and nullable decision timestamp. Adding one
binding never replaces another session's binding. Revoking, expiring, or using
one invitation affects only that child binding. WebSocket authorization selects
the exact child binding named by the channel `sessionId` and also verifies the
configured approval Origin.

No Lean V1 endpoint creates, exchanges, reads, or consumes this grant.

### Transaction

| Field | Type | Notes |
|---|---|---|
| `transactionId` | string | Primary identifier |
| `intentId` | string | Unique; an intent executes once |
| `sellerId`, `routeId`, `buyerId` | string | Query dimensions; browser buyer ID is `browser:<purchaseSessionId>` |
| `purchaseSessionId` | string/null | Browser session owner; null for buyer-agent purchases |
| `productDisplayName`, `productSlug` | string | Immutable purchase-time product snapshot |
| `paymentDestinationId` | string | Immutable verified destination identifier snapshot |
| `purchaseChannel` | enum | `agent` or `browser` |
| `paymentRail` | enum | `x402`; future rails require a migration |
| `status` | enum | State machine below |
| `paymentIdentifier` | string/null | Unique replay-protection value supplied by payment adapter |
| `paymentProofHash` | string/null | Never store raw proof |
| `paymentReference` | string/null | Safe facilitator or network reference, never a raw proof |
| `paymentFinality` | enum/null | `confirmed`, `finalized`, or `failed`; null before a payment observation |
| `reconciledAt` | timestamp/null | Last successful reconciliation time |
| `upstreamStatus` | integer/null | HTTP status received from seller |
| `responseHash` | string/null | Hash of captured response bytes |
| `responseSummary` | object/null | Allowlisted metadata only |
| `failureCode` | string/null | Stable internal code |

For the first implementation, a transaction ID reuses its purchase intent's
ULID payload with the `txn_` prefix. This provides a deterministic point lookup
and enforces one transaction per intent without a table scan.

Lean V1 transaction states:

```text
PROPOSED -> PAYMENT_REQUIRED
PAYMENT_REQUIRED -> PAYMENT_VERIFIED -> FORWARDED -> FULFILLED
                         |                |             |
                         +-> FAILED       +-> FAILED <---+
FULFILLED|FAILED -> DISPUTED -> REFUND_RECOMMENDED|RESOLVED
REFUND_RECOMMENDED -> RESOLVED
```

Historical M2 transactions may contain `APPROVAL_PENDING` or `APPROVED` and
remain readable. Lean V1 writers never emit those states and the payment path
does not accept them as authority.

Terminal states are `RESOLVED` and an undisputed `FULFILLED`. Invalid transitions return `409 state_conflict`.

Payment verification records `paymentFinality=confirmed`. A successful x402
settlement records `paymentFinality=finalized`, the safe facilitator or network
transaction reference, and `reconciledAt` before seller forwarding. A definitive
settlement rejection records `paymentFinality=failed` and moves the transaction
to `FAILED`; timeout or unavailable responses remain `confirmed` because final
network outcome is unknown. Records written before this migration that contain
a payment identifier but no finality are interpreted conservatively as
`confirmed`, never `finalized`.

The forwarding claim is one conditional mutation requiring all of
`status=PAYMENT_VERIFIED`, `paymentFinality=finalized`, the expected transaction
version, and no prior forwarding owner. Only that winner changes the status to
`FORWARDED` and may request an execution capability. A merely confirmed
transaction can never be claimed, even though it shares the
`PAYMENT_VERIFIED` business status while reconciliation is pending.

The API derives one reconciliation bucket without mutating persisted state:
`challenged`, `verified`, `finalized`, `fulfilled`, `failed`, or `disputed`.
Every bucket carries one atomic-unit amount and its exact asset/network pair, so
unlike currencies are never combined.

The API also derives a `CommerceLifecycleProjection` from the authoritative
transaction without persisting a second order record. It exposes
`externalReference=transactionId`, an order-compatible `commerceState`,
separate `paymentState`, `fulfillmentState`, and `refundState`, plus a bounded
`recoveryAction`. Recovery actions are advisory and deterministic:
`retry_same_request` before a payment is observed, `await_reconciliation` while
finality is unknown, `none` after successful fulfillment, `open_dispute` after
a finalized delivery failure, `await_resolution` for an open dispute, and
`record_external_refund` only for `REFUND_RECOMMENDED`. A definitive failed
payment requires a new intent and never reuses the failed authorization.

### SellerSalesAggregate (derived M7 read model)

Sales aggregates are deterministic read models computed from a bounded,
seller-scoped transaction query. They are not persisted as an independent
source of truth in M7, so retries cannot double count a transaction. Duplicate
transaction snapshots are reduced to the highest version before aggregation;
equal-version snapshots with different reconciliation facts fail closed. The
read model never combines different assets or networks.

| Field | Type | Notes |
|---|---|---|
| `sellerId` | string | Aggregate owner |
| `bucketDate` | string | UTC date in `YYYY-MM-DD` form |
| `asset`, `network` | string | Required grouping dimensions |
| `routeId` | string/null | Null for seller-wide bucket; set for route bucket |
| `stage` | enum | `challenged`, `verified`, `finalized`, `fulfilled`, `failed`, or `disputed` |
| `transactionCount` | integer | Number of unique transactions in this exact bucket |
| `amount` | string | Atomic-unit total for this exact asset/network/stage bucket |
| `lastTransactionAt` | timestamp | Newest transaction update included in this bucket |

### WebhookSubscription and WebhookDelivery

`WebhookSubscription` stores a seller-scoped HTTPS destination, allowlisted
event types, secret reference, enabled status, and version. `WebhookDelivery`
uses ID prefix `whd_` and stores one stable event for one subscription. The
unique identity is `(subscriptionId, eventId)` so duplicate enqueue requests
return the existing record instead of creating another delivery.

| Field | Type | Rule |
|---|---|---|
| `deliveryId` | ID | Stable `whd_` identifier |
| `sellerId` | ID | Owning seller |
| `subscriptionId` | ID | Target webhook subscription |
| `event` | object | Allowlisted versioned webhook event used for retry |
| `payloadHash` | string | Lowercase SHA-256 of the canonical event body |
| `status` | enum | `pending`, `retry_scheduled`, `delivered`, or `dead_letter` |
| `attemptCount` | integer | Completed HTTP attempts; automatic delivery stops at five and explicit redelivery preserves and may increase this count |
| `nextAttemptAt` | timestamp or null | Due time for pending or retry work |
| `lastAttemptAt` | timestamp or null | Most recent completed HTTP attempt |
| `deliveredAt` | timestamp or null | First successful `2xx` response |
| `responseStatusCode` | integer or null | Most recent bounded HTTP status metadata |
| `responseBodyHash` | string or null | SHA-256 of the bounded response body; the body is not retained |
| `errorCode` | string or null | Stable allowlisted delivery failure code |
| `createdAt` | timestamp | Creation time |
| `updatedAt` | timestamp | Latest transition time |
| `version` | integer | Optimistic concurrency version |

The first attempt is due immediately. Retryable transport failures, timeouts,
`408`, `425`, `429`, and `5xx` responses use delays of 1 minute, 5 minutes,
30 minutes, and 2 hours. The fifth failed attempt enters `dead_letter` and has
no automatic retry. Other `4xx` responses enter `dead_letter` immediately.
Seller-triggered redelivery is allowed only from `dead_letter`, preserves the
same delivery ID and event ID, clears prior response metadata, and schedules an
immediate attempt. It does not reset `attemptCount`, so delivery history remains
truthful; each redelivery may perform one additional bounded attempt.

Webhook responses are limited to 64 KiB for hashing and are never stored. Every
attempt repeats public-address DNS validation, pins the validated address at
connection time, rejects redirects, uses TLS 1.2 or newer, and times out after
10 seconds.

Webhook subscription IDs use `whk_`. Creation returns a 256-bit signing secret
once and stores it through the configured secret-store boundary; the
subscription record stores only its opaque reference. Event bodies use schema
version `1` and one of `payment.verified`, `fulfillment.succeeded`,
`fulfillment.failed`, or `dispute.changed`. Canonical JSON is signed with
HMAC-SHA256 over the event ID, delivery timestamp, and body hash.

### PurchaseReceipt read model

`PurchaseReceipt` is generated on demand and is not a second persisted payment
record. It is available only after the transaction has a `finalized` payment
observation and the complete evidence chain verifies. Schema version `2` adds
the immutable product display-name, product-slug, purchase-channel,
payment-rail, and frozen payment-destination identifier snapshots. Readers must
continue to accept schema version `1`, which identifies a historical product by
`routeId`. Both versions contain the safe payment reference, fulfillment
result, complete signed evidence chain, event count, root event hash, and head
event hash. Neither contains a payment identifier, payment proof hash, raw
payment proof, authorization value, approval token, wallet material, execution
capability, or seller secret.

The receipt uses the transaction ID as its stable identity. Repeated downloads
for unchanged transaction and evidence state produce the same JSON fields.
Sellers may download receipts for their own transactions; authenticated agent
buyers may download only receipts whose `buyerId` matches their subject;
browser grants may download only receipts whose `purchaseSessionId` is one of
their active or recovered read-authority bindings.

### SellerEntitlement

AgentPay billing is independent from buyer-to-seller settlement. The versioned
V1 plan catalog contains `starter`, `growth`, and `scale`. Plan definitions have
operational limits and feature flags only; they never contain buyer payment
amounts, payment destinations, wallet material, or settlement instructions.
Stripe Billing is the initial seller-subscription provider. The exact lifecycle
and provider mapping are locked in `SUBSCRIPTION_LIFECYCLE.md`.

| Limit | Starter | Growth | Scale |
|---|---:|---:|---:|
| API requests per UTC month | 10,000 | 100,000 | 1,000,000 |
| MCP operations per UTC month | 1,000 | 10,000 | 100,000 |
| Published routes | 5 | 50 | 500 |
| Webhook subscriptions | 1 | 5 | 25 |
| Webhook deliveries per UTC month | 1,000 | 25,000 | 250,000 |
| Analytics window days | 7 | 90 | 365 |
| Evidence retention days | 30 | 180 | 3,650 |

Starter enables webhooks but not advanced analytics. Growth enables webhooks
and advanced analytics. Scale adds priority support. Buyer-side approval is not
a Lean V1 plan feature. These flags describe entitlement only; enforcement
belongs to `OPS-001`.

Each seller has one authoritative entitlement projection at
`PK=SELLER#<sellerId>`, `SK=BILLING_PLAN`. The wire record is named
`SellerEntitlement` and stores `sellerId`, `planId`, `planVersion`, `status`,
UTC period start/end, exact `accessEndsAt`, nullable `graceEndsAt`,
`cancelAtPeriodEnd`, monotonically increasing `entitlementEpoch`, provider
`source`, opaque ordered `sourceRevision`, nullable `statusReason`,
`providerCustomerId`, `providerSubscriptionId`, `providerPriceId`, nullable
`lastProviderEventId`, `lastReconciledAt`, `credentialRotationRequired`,
assignment timestamps, and optimistic `version`. Status vocabulary is
`active`, `grace`, `suspended`, `cancelled`, or `closed`. A missing production
record is not auto-created and transaction-critical authorization fails closed.
Buyer settlement state remains independent.

Only `status=active` with `now < accessEndsAt` grants network authority.
`grace` begins exactly at an unpaid `accessEndsAt`, ends exactly 72 hours later,
and permits billing recovery and historical reads only. Scheduled cancellation
keeps `status=active` and `cancelAtPeriodEnd=true` until `accessEndsAt`, then
becomes `cancelled` without grace. Authorization compares the current UTC time
to `accessEndsAt` directly, so delayed events or jobs cannot extend service.

`entitlementEpoch` increments whenever all existing seller capabilities must be
invalidated, including entry into grace, suspension, cancellation, closure,
administrative/fraud quarantine, and reactivation. Reactivation from a
non-active state sets `credentialRotationRequired=true`; existing project keys
cannot exchange tokens until replaced through the seller session.
`sourceRevision` is a zero-padded local reconciliation sequence. Stripe event
timestamps are audit facts, not ordering authority. Redis may cache this
projection, but the database is authoritative.

### SubscriptionProviderEvent

Stripe webhooks enter a durable inbox before processing. Each record contains
provider `eventId`, event type, payload hash, livemode/environment, API version,
provider creation time, received time, customer/subscription/invoice IDs,
processing state, attempt count, and applied entitlement version. The event ID
is unique. An exact duplicate succeeds without reapplying effects; the same ID
with a different hash is quarantined as a security incident. Workers fetch the
current Stripe objects and conditionally commit a complete entitlement snapshot
instead of applying events in arrival order.

### StorefrontDiscoveryDocument

AgentPay-hosted discovery is a signed read model, not transaction authority. An
active document contains schema version, immutable seller ID, seller name and
slug, `availability=active`, monotonically increasing `publicationRevision`,
`issuedAt`, short `expiresAt`, canonical origin, and explicit public product
projections. Public products expose route ID, display name, product slug,
description, output MIME type, exact amount/asset/network, canonical product
URL, and purchase-session endpoint. They never expose upstream URLs, payout
addresses, project credentials, or policy secrets.

An inactive seller resolves to a signed tombstone with
`availability=inactive`, seller identity, publication revision, issue/expiry
times, canonical origin, and an allowlisted reason of `suspended`, `cancelled`,
or `closed`; it contains no products. Unknown slugs return `404`, while known
inactive slugs return `410`. The cloud ES256 signature covers the
`agentpay.discovery.v1` domain separator and RFC 8785 canonical JSON. Cached or
seller-hosted discovery cannot authorize intent creation or payment.

### ExecutionCapability

An execution capability is a non-persisted cloud-signed JWT issued only after
payment finality and the authoritative exactly-once forwarding claim. Required
claims are exact issuer, `aud=urn:agentpay:seller:<sellerId>`,
`sub=transactionId`, `sellerId`, `routeId`, `transactionId`, HTTP `method`,
literal `path`, lowercase raw-body `bodySha256`,
`paymentFinality=finalized`, unique `jti`, `iat`, and `exp`. Lifetime is 30-60
seconds. Its protected header is `typ=agentpay-execution+jwt`, `alg=ES256`, and
a current or overlapping-rotation `kid`.

AgentPay sends it in `X-AgentPay-Execution-Capability`; the separate
`X-AgentPay-Transaction-Id` header must equal the claim. Seller middleware
verifies JWKS, exact audience and bindings, consumes the JTI once, and uses
`transactionId` as its fulfillment idempotency key. Execution capabilities are
never returned to buyers or stored in receipts, evidence, webhooks, or logs.

### QuotaCounter

Operational monthly quotas use one atomic counter per seller, UTC month, and
quota name at `PK=SELLER#<sellerId>`,
`SK=QUOTA#<YYYY-MM>#<quotaName>`. The quota names are `api_request`,
`mcp_operation`, and `webhook_delivery`. Each record stores the current `count`,
exclusive `limit`, inclusive `periodStart`, exclusive `periodEnd`, and
`updatedAt`. Counter increments use a conditional write that cannot advance the
count above the frozen limit supplied by the assigned plan.

Webhook retries must not consume multiple units for the same logical delivery.
The first enqueue for a `(subscriptionId, eventId)` source atomically creates
`SK=QUOTA_CLAIM#<YYYY-MM>#webhook_delivery#<sha256(source)>` and increments the
monthly counter. A repeated source finds the claim and succeeds without another
increment. Claims contain no webhook payload or secret material.

### UsageMeterEvent

Usage meter events use ID prefix `mtr_` and are immutable and idempotently
derived from successful AgentPay operations. V1 defines meter
`successful_transaction`; it records quantity `1` only after a transaction is
both payment-finalized and fulfilled. The uniqueness key is
`(meterName, sourceTransactionId)`, so retries cannot double count usage.

Each event stores `meterEventId`, `sellerId`, `meterName`, `quantity`,
`sourceTransactionId`, `planId`, `planVersion`, and `occurredAt`. The invoice
export is a generated usage statement grouped by meter, plan ID, and plan
version for an explicit UTC window of at most 31 days and at most 1,000 events.
It contains no buyer funds, price, tax, currency, payment destination, or
collection instruction. A future billing adapter may price and collect the
export without changing x402 settlement.

### AuditEvent

Audit events are immutable seller-scoped records of committed control-plane
changes. Each event stores `auditEventId`, `sellerId`, `actorType`, `actorId`,
`action`, `targetType`, `targetId`, `outcome`, `requestId`, `changedFields`, and
`occurredAt`. `changedFields` contains sorted, unique field names only. Audit
events never contain old or new values, credential material, wallet challenges
or proofs, webhook secrets, payment proofs, request bodies, or repository
contents.

Actor vocabulary is `seller_user`, `integration_credential`, `administrator`,
and `system`. Target vocabulary is `integration_credential`,
`mcp_confirmation_grant`, `payment_destination`, `paid_route`,
`webhook_subscription`, and `seller`.
Outcome vocabulary is `succeeded`, `failed`, and `denied`.

Action vocabulary is fixed to:

- `credential.created`, `credential.revoked`, `credential.rotated`,
  `credential.exchange_succeeded`, and `credential.exchange_denied`;
- `entitlement.changed`;
- `service_endpoint.verified`;
- `mcp_confirmation.issued`, `mcp_confirmation.consumed`, and
  `mcp_confirmation.denied`;
- `payment_destination.created`, `payment_destination.verified`,
  `payment_destination.disabled`, and `payment_destination.rotated`;
- `route.draft_created`, `route.price_changed`, `route.published`,
  `route.paused`, `route.archived`, and `route.emergency_disabled`;
- `webhook_subscription.created`, `webhook_subscription.updated`, and
  `webhook_subscription.disabled`; and
- `seller.suspended`.

Each action has an allowlist of changed fields owned by that domain. Unknown
actions, mismatched target types, duplicate field names, and fields outside the
action allowlist are rejected before persistence. The M7 runtime appends audit
events for successful committed mutations. The M7.1 confirmation boundary also
persists grant issuance, successful consumption, and denied validation without
recording the raw grant or canonical arguments. Other failed and denied
attempts remain in security and operational logs until their durable attempt-
audit transaction is introduced.

Seller history is queried newest-first from `PK=SELLER#<sellerId>` and
`SK=AUDIT#<occurredAt>#<auditEventId>`. Pages default to 50 events and are
bounded to 100. Cursors are opaque, seller-bound, and identify the last event
from the previous page. Audit records are append-only and have no update or
delete operation.

### ApprovalConnection (historical M3; deferred and disabled)

These registrations document the completed M3 approval WebSocket. Lean V1 does
not create them or start the approval WebSocket runtime.

| Field | Type | Notes |
|---|---|---|
| `connectionId` | string | API Gateway or local WebSocket connection identifier |
| `sessionId` | string | Approval session authorized by the exchanged invitation cookie at connect time |
| `expiresAt` | timestamp | Must not exceed the approval-session expiration |
| `connectedAt` | timestamp | UTC registration time |

Raw invitation tokens and cookie values are never persisted with a connection.
The WebSocket handshake uses the same-origin approval grant cookie; expired,
mismatched, decided, or revoked child grants are rejected before upgrade.
Expired or delivery-gone connections are deleted, and reconnecting creates a
new registration and immediately receives a complete redacted
`session.snapshot` event.

### EvidenceEvent

Evidence is append-only and ordered per transaction.

| Field | Type | Notes |
|---|---|---|
| `eventId` | string | Unique event identifier |
| `transactionId` | string | Chain owner |
| `sequence` | integer | Starts at 1 and increments by one |
| `eventType` | string | Stable event vocabulary |
| `actorType`, `actorId` | string | System, seller, buyer, or approver |
| `payload` | object | Allowlisted facts for this event |
| `previousEventHash` | string/null | Null for sequence 1 |
| `eventHash` | string | Hash of canonical event without signature |
| `kmsKeyId` | string | Signing key version/reference |
| `kmsSignature` | string | Base64 signature |
| `createdAt` | timestamp | Server timestamp |

Event vocabulary: `intent.created`, `approval.requested`, `approval.decided`, `approval.resolved`, `payment.challenged`, `payment.verified`, `proxy.forwarded`, `delivery.succeeded`, `delivery.failed`, `dispute.opened`, `dispute.classified`, `dispute.resolved`.

### Dispute

| Field | Type | Notes |
|---|---|---|
| `disputeId` | string | Primary identifier |
| `transactionId` | string | Disputed transaction |
| `reason` | enum | `unauthorized`, `duplicate`, `wrong_amount`, `not_delivered`, `quality_or_output` |
| `statement` | string | User-provided explanation, maximum 2,000 characters |
| `status` | enum | `open`, `refund_recommended`, `seller_review`, `denied`, `resolved` |
| `ruleVersion` | string | Deterministic classifier version used for replay |
| `classificationCode` | string | Deterministic rule result |
| `explanation` | string | Human-readable rule explanation |

Dispute classification uses rule version `dispute-rules-v1` and only recorded transaction facts. Each reason evaluates its corresponding fact:

| Reason | Recorded fact | Classification when claim is confirmed | Classification when claim is disproved |
|---|---|---|---|
| `unauthorized` | payment authorization was valid | `refund_recommended` / `authorization_not_valid` | `denied` / `authorization_valid` |
| `duplicate` | the payment identifier was charged more than once | `refund_recommended` / `duplicate_payment_confirmed` | `denied` / `duplicate_payment_not_found` |
| `wrong_amount` | paid amount differs from the frozen intent amount | `refund_recommended` / `wrong_amount_confirmed` | `denied` / `amount_matches_intent` |
| `not_delivered` | successful seller delivery was recorded | `refund_recommended` / `delivery_not_confirmed` | `denied` / `delivery_confirmed` |
| `quality_or_output` | no automatic quality judgment is permitted | `seller_review` / `quality_review_required` | Not applicable |

For `unauthorized`, `duplicate`, `wrong_amount`, and `not_delivered`, a missing fact produces `seller_review` / `insufficient_evidence`. Classification never executes a refund; it records a reproducible recommendation only.

### ManualRefundRecord

A `ManualRefundRecord` is an append-only seller assertion that the seller
completed a refund outside AgentPay. AgentPay does not custody funds, submit the
refund, or treat the record alone as network proof. Lean V1 permits at most one
record per dispute.

| Field | Type | Notes |
|---|---|---|
| `disputeId` | string | Primary identity; the dispute must belong to the authenticated seller |
| `transactionId` | string | Finalized transaction referenced by the dispute |
| `sellerId` | string | Derived from the authenticated seller and transaction; never accepted from the request body |
| `amount` | string | Positive atomic-unit amount; must exactly equal the finalized transaction amount in Lean V1 |
| `asset`, `network` | string | Must exactly match the finalized transaction |
| `reference` | string | Seller-supplied external refund/provider/network reference, trimmed, 1-512 characters, with control characters rejected |
| `recordedBy` | string | Authenticated seller principal/subject; never supplied by the request body |
| `recordedAt` | timestamp | Server-assigned UTC timestamp |

`POST /v1/sellers/{sellerId}/disputes/{disputeId}/refund-records` requires
`sellerBearer` and `Idempotency-Key`. The dispute must be
`refund_recommended`, its transaction must have `paymentFinality=finalized`,
and amount, asset, and network must match exactly. A cross-seller or unknown
dispute is concealed as `404 not_found`; malformed values or an unfinalized
transaction return `422 validation_failed`; a dispute outside the
`refund_recommended` state returns `409 state_conflict`. An exact idempotency
replay returns the original `201` response, while reuse of the key or the
dispute's one-record slot with changed input returns
`409 idempotency_conflict` or `409 state_conflict`, respectively.

`GET /v1/sellers/{sellerId}/disputes/{disputeId}/refund-records/current`
returns the one record to the owning seller. Authorized buyer transaction and
dispute reads may expose only the same bounded record as a remediation
projection. Its verification state is always `seller_reported`; a future
provider or network verifier requires a separate contract and may not rewrite
the append-only seller assertion.

## DynamoDB layout

Use one table named by environment, with `PK` and `SK` strings and on-demand billing for the hackathon.

Examples:

```text
PK=SELLER#sel_123       SK=PROFILE
PK=SELLER_OWNER#<subjectDigest> SK=SELLER
PK=SELLER_SESSION#<sessionDigest> SK=REVOCATION
PK=SELLER#sel_123       SK=ROUTE#rte_123
PK=SELLER#sel_123       SK=PRODUCT_SLUG#<productSlug>
PK=SELLER#sel_123       SK=CREDENTIAL#key_123
PK=CREDENTIAL#key_123   SK=LOOKUP
PK=CREDENTIAL#key_123   SK=RATE_LIMIT#PROJECT_KEY_EXCHANGE#<windowStart>
PK=MCP_CONFIRMATION#mcg_123 SK=PROFILE
PK=SELLER#sel_123       SK=MCP_CONFIRMATION_BINDING#<bindingHash>
PK=SELLER#sel_123       SK=DESTINATION#dst_123
PK=SELLER#sel_123       SK=DESTINATION_ACTIVE#<sha256(asset + NUL + network)>
PK=SELLER#sel_123       SK=AGGREGATE#2026-09-17#USDC#eip155:84532#ALL
PK=SELLER#sel_123       SK=WEBHOOK#whk_123
PK=SELLER#sel_123       SK=WEBHOOK_DELIVERY#whd_123
PK=SELLER#sel_123       SK=WEBHOOK_EVENT#whk_123#evt_123
PK=SELLER#sel_123       SK=BILLING_PLAN
PK=STRIPE_EVENT#evt_123 SK=INBOX
PK=SELLER#sel_123       SK=SUBSCRIPTION_RECONCILIATION#<sourceRevision>
PK=SELLER#sel_123       SK=QUOTA#2026-09#api_request
PK=SELLER#sel_123       SK=QUOTA_CLAIM#2026-09#webhook_delivery#<sha256(source)>
PK=SELLER#sel_123       SK=AUDIT#<createdAt>#aud_123
PK=SELLER#sel_123       SK=METER#<createdAt>#mtr_123
PK=SELLER#sel_123       SK=METER_SOURCE#successful_transaction#txn_123
PK=INTENT#int_123       SK=PROFILE
PK=PURCHASE_SESSION#bps_123 SK=PROFILE
PK=PURCHASE_SESSION#bps_123 SK=RECOVERY#bpr_123
PK=BROWSER_GRANT#<grantHash> SK=PURCHASE#bps_123
PK=APPROVAL#aps_123     SK=PROFILE
PK=APPROVAL#aps_123     SK=INVITE#<tokenHash>
PK=APPROVAL_GRANT#<grantHash> SK=SESSION#aps_123#<approverId>
PK=APPROVAL#aps_123     SK=CONNECTION#<connectionId>
PK=TXN#txn_123          SK=PROFILE
PK=TXN#txn_123          SK=EVENT#000001
PK=DISPUTE#dsp_123      SK=PROFILE
PK=DISPUTE#dsp_123      SK=REFUND_RECORD
PK=IDEMPOTENCY#<scope>  SK=<key>
PK=PAYMENT#<paymentIdentifier> SK=CLAIM
PK=SLUG#<slug>          SK=CLAIM
```

The `APPROVAL#...` and `APPROVAL_GRANT#...` forms above are historical M2/M3
compatibility records only. Lean V1 does not write or query them on an active
commerce path.

Integration credentials remain in the seller partition so listing and
authorization use point/query operations. Production authentication must never
scan by token hash.

`PAYMENT#...` and `SLUG#...` claim items are created in the same DynamoDB transaction as their owning record with `attribute_not_exists(PK)` conditions. `PRODUCT_SLUG#...` claims are created in the seller partition with `attribute_not_exists(PK) AND attribute_not_exists(SK)`. They enforce global payment/storefront uniqueness and seller-scoped product-slug uniqueness respectively; the corresponding GSIs remain the query paths for transaction and storefront lookup.

Required secondary indexes:

- `GSI1PK=SELLER#<id>`, `GSI1SK=TXN#<createdAt>#<id>` for seller transactions.
- `GSI2PK=PAYMENT#<paymentIdentifier>`, `GSI2SK=TXN#<id>` for replay prevention.
- `GSI3PK=SLUG#<slug>`, `GSI3SK=SELLER#<id>` for storefront resolution.
- `GSI4PK=ROUTE#<routeId>`, `GSI4SK=SELLER#<sellerId>` for purchase-intent route resolution.

## Retention

- Hackathon operational data may be removed during environment teardown.
- Evidence bucket deletion is blocked by default and must be explicitly approved.
- Production retention and deletion periods are deferred pending legal and customer requirements.
