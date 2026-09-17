# Data model and state machines

Status: **Locked for the implemented backend; planned M6/M7 additions are explicitly labeled**.

## Conventions

- Implemented IDs use canonical ULIDs with sortable, opaque prefixes: `sel_`, `rte_`, `int_`, `aps_`, `txn_`, `evt_`, `dsp_`, `key_`, `dst_`, and `whk_`. Planned M7 entities add `whd_` webhook deliveries, `aud_` audit events, and `mtr_` usage-meter events.
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
| `status` | enum | `draft`, `active`, `suspended` |
| `createdAt`, `updatedAt` | timestamp | UTC creation and latest status/configuration change |
| `version` | integer | Starts at 1 and increments on mutation |

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
| `tokenHash` | string | SHA-256 or stronger one-way token digest |
| `label` | string | Seller-visible installation name |
| `scopes` | string array | Explicit read, configure, publish, validate, or rotate permissions |
| `expiresAt` | timestamp/null | Required for temporary setup credentials |
| `revokedAt` | timestamp/null | Revocation makes the credential unusable immediately |
| `createdAt`, `updatedAt` | timestamp | UTC lifecycle timestamps |
| `version` | integer | Used for guarded rotation and revocation |

Credential scope never implies permission to deploy a seller repository or
access seller infrastructure. Deployment authorization remains local to the
seller's coding-agent environment.

Raw credentials use `apc1.<sellerId>.<credentialId>.<randomSecret>`. The
seller and credential identifiers permit a point read, but they grant no
authority without constant-time verification of the complete token hash.

### PaidRoute

| Field | Type | Notes |
|---|---|---|
| `routeId` | string | Primary identifier |
| `sellerId` | string | Owning seller |
| `method` | enum | `GET`, `POST` for hackathon |
| `pathPattern` | string | Literal path in v1; no arbitrary regex |
| `description` | string | Published in manifest |
| `mimeType` | string | Expected successful response type |
| `amount` | string | Atomic units |
| `asset` | string | Testnet asset identifier |
| `network` | string | Testnet network identifier |
| `payTo` | string | Compatibility snapshot of the verified seller destination used by new intents |
| `paymentDestinationId` | string/null | Planned M7 reference to the verified seller destination |
| `approvalThresholdAmount` | string/null | Approval required when amount is greater than or equal to threshold |
| `upstreamTimeoutSeconds` | integer | Range 1–30 |
| `enabled` | boolean | Disabled routes cannot issue challenges |
| `createdAt`, `updatedAt` | timestamp | UTC creation and latest configuration change |
| `version` | integer | Starts at 1 and increments on mutation |

Price updates apply only to purchase intents created after the update. Existing intents retain their frozen amount until they expire or execute.

`enabled` is the V1 publication flag. Seller API route creation remains an
explicit seller-authorized operation and creates an enabled route for backward
compatibility. MCP route configuration creates `enabled=false` drafts. The MCP
publish operation revalidates seller ownership, active seller status, signing
configuration, route configuration, confirmation metadata, expected version,
idempotency, and the seller sandbox endpoint before changing the draft to
`enabled=true` with a conditional write. Deterministic and sandbox validation
results are computed responses and are not persisted; publication performs a
fresh sandbox run so a stale result cannot authorize a changed route.

Approval threshold evaluation is inclusive: an amount equal to or greater than the applicable threshold requires approval. A missing threshold means no approval requirement from that policy. The recorded policy version is `approval-threshold-v1`.

### PurchaseIntent

An intent becomes immutable after creation.

| Field | Type | Notes |
|---|---|---|
| `intentId` | string | Primary identifier |
| `sellerId`, `routeId` | string | Resolved offer |
| `buyerId` | string | Demo agent/API-key identity |
| `purchaseChannel` | enum | Planned M7 field: `agent` or `browser` |
| `requestMethod`, `requestPath` | string | Canonical target |
| `requestBodyHash` | string | Hash of canonical request bytes |
| `amount`, `asset`, `network` | string | Frozen quote |
| `maximumAmount` | string | Buyer safety limit |
| `requiresApproval` | boolean | Policy output |
| `intentHash` | string | Canonical hash of all execution-relevant fields |
| `expiresAt` | timestamp | Ten minutes after creation by default |
| `status` | enum | `ready`, `approval_pending`, `approved`, `expired`, `executed` |

The price and commercial fields of an intent never change after creation. Seller price updates affect only newly created intents.

### ApprovalSession

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

Each invitation is stored as a child item containing approver label, token hash, expiration, use timestamp, and decision. An invitation can produce one decision only.

Raw invitation tokens are returned only when the session is created. When the second approval resolves a session, a short-lived HMAC-signed approval token is returned once and only its SHA-256 hash is persisted. The token is bound to the session ID, intent ID, complete intent hash, and session expiration.

### Transaction

| Field | Type | Notes |
|---|---|---|
| `transactionId` | string | Primary identifier |
| `intentId` | string | Unique; an intent executes once |
| `sellerId`, `routeId`, `buyerId` | string | Query dimensions |
| `purchaseChannel` | enum | Planned M7 field: `agent` or `browser` |
| `paymentRail` | enum | Planned M7 field: `x402`; future rails require a migration |
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

Transaction states:

```text
PROPOSED -> APPROVAL_PENDING -> APPROVED -> PAYMENT_REQUIRED
PROPOSED -------------------------------> PAYMENT_REQUIRED
PAYMENT_REQUIRED -> PAYMENT_VERIFIED -> FORWARDED -> FULFILLED
                         |                |             |
                         +-> FAILED       +-> FAILED <---+
FULFILLED|FAILED -> DISPUTED -> REFUND_RECOMMENDED|RESOLVED
REFUND_RECOMMENDED -> RESOLVED
```

Terminal states are `RESOLVED` and an undisputed `FULFILLED`. Invalid transitions return `409 state_conflict`.

Payment verification records `paymentFinality=confirmed`. A successful x402
settlement records `paymentFinality=finalized`, the safe facilitator or network
transaction reference, and `reconciledAt` before seller forwarding. A definitive
settlement rejection records `paymentFinality=failed` and moves the transaction
to `FAILED`; timeout or unavailable responses remain `confirmed` because final
network outcome is unknown. Records written before this migration that contain
a payment identifier but no finality are interpreted conservatively as
`confirmed`, never `finalized`.

The API derives one reconciliation bucket without mutating persisted state:
`challenged`, `verified`, `finalized`, `fulfilled`, `failed`, or `disputed`.
Every bucket carries one atomic-unit amount and its exact asset/network pair, so
unlike currencies are never combined.

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
observation and the complete evidence chain verifies. Schema version `1`
contains the public transaction fields, safe payment reference, fulfillment
result, the complete signed evidence chain, event count, root event hash, and
head event hash. It never contains a payment identifier, payment proof hash,
raw payment proof, authorization value, approval token, wallet material, or
seller secret.

The receipt uses the transaction ID as its stable identity. Repeated downloads
for unchanged transaction and evidence state produce the same JSON fields.
Sellers may download receipts for their own transactions; authenticated agent
buyers may download only receipts whose `buyerId` matches their subject.

### SellerPlan

AgentPay billing is independent from buyer-to-seller settlement. The versioned
V1 plan catalog contains `starter`, `growth`, and `scale`. Plan definitions have
operational limits and feature flags only; they never contain buyer payment
amounts, payment destinations, wallet material, or settlement instructions.

| Limit | Starter | Growth | Scale |
|---|---:|---:|---:|
| API requests per UTC month | 10,000 | 100,000 | 1,000,000 |
| MCP operations per UTC month | 1,000 | 10,000 | 100,000 |
| Published routes | 5 | 50 | 500 |
| Webhook subscriptions | 1 | 5 | 25 |
| Webhook deliveries per UTC month | 1,000 | 25,000 | 250,000 |
| Analytics window days | 7 | 90 | 365 |
| Evidence retention days | 30 | 180 | 3,650 |

Starter enables webhooks but not approval workflows or advanced analytics.
Growth enables approvals, webhooks, and advanced analytics. Scale adds priority
support. These flags describe entitlement only; enforcement belongs to
`OPS-001`.

Each seller has one `SellerPlan` record at `PK=SELLER#<sellerId>`,
`SK=BILLING_PLAN`. It stores `sellerId`, `planId`, `planVersion`, `status`, UTC
period start/end, assignment timestamps, and optimistic `version`. Status is
`active` or `suspended`. A missing record is initialized to Starter version 1;
changing plans requires an explicit trusted billing operation and never changes
buyer settlement state.

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

### AuditEvent (planned M7)

Audit events append seller-scoped changes to credentials, payment destinations,
routes, prices, publication state, webhook subscriptions, quotas, and
administrative status. Each event records actor, action, target, outcome,
request ID, timestamp, and allowlisted changed-field names without secrets or
repository contents.

### ApprovalConnection

Approval WebSocket connections are ephemeral registrations used only for event delivery. REST approval snapshots remain authoritative.

| Field | Type | Notes |
|---|---|---|
| `connectionId` | string | API Gateway or local WebSocket connection identifier |
| `sessionId` | string | Approval session authorized by the invitation token at connect time |
| `expiresAt` | timestamp | Must not exceed the approval-session expiration |
| `connectedAt` | timestamp | UTC registration time |

Raw invitation tokens are never persisted with a connection. Expired or delivery-gone connections are deleted, and reconnecting creates a new registration and immediately receives a complete redacted `session.snapshot` event.

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

## DynamoDB layout

Use one table named by environment, with `PK` and `SK` strings and on-demand billing for the hackathon.

Examples:

```text
PK=SELLER#sel_123       SK=PROFILE
PK=SELLER#sel_123       SK=ROUTE#rte_123
PK=SELLER#sel_123       SK=CREDENTIAL#key_123
PK=SELLER#sel_123       SK=DESTINATION#dst_123
PK=SELLER#sel_123       SK=DESTINATION_ACTIVE#<sha256(asset + NUL + network)>
PK=SELLER#sel_123       SK=AGGREGATE#2026-09-17#USDC#eip155:84532#ALL
PK=SELLER#sel_123       SK=WEBHOOK#whk_123
PK=SELLER#sel_123       SK=WEBHOOK_DELIVERY#whd_123
PK=SELLER#sel_123       SK=WEBHOOK_EVENT#whk_123#evt_123
PK=SELLER#sel_123       SK=BILLING_PLAN
PK=SELLER#sel_123       SK=AUDIT#<createdAt>#aud_123
PK=SELLER#sel_123       SK=METER#<createdAt>#mtr_123
PK=SELLER#sel_123       SK=METER_SOURCE#successful_transaction#txn_123
PK=INTENT#int_123       SK=PROFILE
PK=APPROVAL#aps_123     SK=PROFILE
PK=APPROVAL#aps_123     SK=INVITE#<tokenHash>
PK=APPROVAL#aps_123     SK=CONNECTION#<connectionId>
PK=TXN#txn_123          SK=PROFILE
PK=TXN#txn_123          SK=EVENT#000001
PK=DISPUTE#dsp_123      SK=PROFILE
PK=IDEMPOTENCY#<scope>  SK=<key>
PK=PAYMENT#<paymentIdentifier> SK=CLAIM
PK=SLUG#<slug>          SK=CLAIM
```

Integration credentials remain in the seller partition so listing and
authorization use point/query operations. Production authentication must never
scan by token hash.

`PAYMENT#...` and `SLUG#...` claim items are created in the same DynamoDB transaction as their owning record with `attribute_not_exists(PK)` conditions. They enforce uniqueness; the corresponding GSIs remain the query paths for transaction and storefront lookup.

Required secondary indexes:

- `GSI1PK=SELLER#<id>`, `GSI1SK=TXN#<createdAt>#<id>` for seller transactions.
- `GSI2PK=PAYMENT#<paymentIdentifier>`, `GSI2SK=TXN#<id>` for replay prevention.
- `GSI3PK=SLUG#<slug>`, `GSI3SK=SELLER#<id>` for storefront resolution.
- `GSI4PK=ROUTE#<routeId>`, `GSI4SK=SELLER#<sellerId>` for purchase-intent route resolution.

## Retention

- Hackathon operational data may be removed during environment teardown.
- Evidence bucket deletion is blocked by default and must be explicitly approved.
- Production retention and deletion periods are deferred pending legal and customer requirements.
