# Data model and state machines

Status: **Locked for hackathon implementation**.

## Conventions

- IDs use canonical ULIDs with sortable, opaque prefixes: `sel_`, `rte_`, `int_`, `aps_`, `txn_`, `evt_`, `dsp_`.
- Timestamps are RFC 3339 UTC strings.
- Payment amounts are canonical strings in atomic units; floating-point numbers and leading zeros are forbidden at persistence boundaries, except that zero is `"0"`.
- `asset` is a chain-specific contract or asset identifier.
- `network` uses the identifier supplied by the verified x402 SDK.
- Request and response hashes use lowercase SHA-256 hex.
- JSON request bodies are canonicalized with RFC 8785 JCS before hashing; non-JSON bodies are hashed byte-for-byte.
- Intent hashes use the `agentpay.intent.v1` domain separator and include every execution-relevant field.
- Persisted records include `createdAt`; mutable records also include `updatedAt` and integer `version`.

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
| `payTo` | string | Seller testnet address |
| `approvalThresholdAmount` | string/null | Approval required when amount is greater than or equal to threshold |
| `upstreamTimeoutSeconds` | integer | Range 1–30 |
| `enabled` | boolean | Disabled routes cannot issue challenges |
| `createdAt`, `updatedAt` | timestamp | UTC creation and latest configuration change |
| `version` | integer | Starts at 1 and increments on mutation |

Price updates apply only to purchase intents created after the update. Existing intents retain their frozen amount until they expire or execute.

Approval threshold evaluation is inclusive: an amount equal to or greater than the applicable threshold requires approval. A missing threshold means no approval requirement from that policy. The recorded policy version is `approval-threshold-v1`.

### PurchaseIntent

An intent becomes immutable after creation.

| Field | Type | Notes |
|---|---|---|
| `intentId` | string | Primary identifier |
| `sellerId`, `routeId` | string | Resolved offer |
| `buyerId` | string | Demo agent/API-key identity |
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

Each invitation is stored as a child item containing approver label, token hash, expiration, use timestamp, and decision. An invitation can produce one decision only.

### Transaction

| Field | Type | Notes |
|---|---|---|
| `transactionId` | string | Primary identifier |
| `intentId` | string | Unique; an intent executes once |
| `sellerId`, `routeId`, `buyerId` | string | Query dimensions |
| `status` | enum | State machine below |
| `paymentIdentifier` | string/null | Unique replay-protection value supplied by payment adapter |
| `paymentProofHash` | string/null | Never store raw proof |
| `upstreamStatus` | integer/null | HTTP status received from seller |
| `responseHash` | string/null | Hash of captured response bytes |
| `responseSummary` | object/null | Allowlisted metadata only |
| `failureCode` | string/null | Stable internal code |

Transaction states:

```text
PROPOSED -> APPROVAL_PENDING -> APPROVED -> PAYMENT_REQUIRED
PROPOSED -------------------------------> PAYMENT_REQUIRED
PAYMENT_REQUIRED -> PAYMENT_VERIFIED -> FORWARDED -> FULFILLED
                                          |             |
                                          +-> FAILED <---+
FULFILLED|FAILED -> DISPUTED -> REFUND_RECOMMENDED|RESOLVED
REFUND_RECOMMENDED -> RESOLVED
```

Terminal states are `RESOLVED` and an undisputed `FULFILLED`. Invalid transitions return `409 state_conflict`.

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
| `classificationCode` | string | Deterministic rule result |
| `explanation` | string | Human-readable rule explanation |

## DynamoDB layout

Use one table named by environment, with `PK` and `SK` strings and on-demand billing for the hackathon.

Examples:

```text
PK=SELLER#sel_123       SK=PROFILE
PK=SELLER#sel_123       SK=ROUTE#rte_123
PK=INTENT#int_123       SK=PROFILE
PK=APPROVAL#aps_123     SK=PROFILE
PK=APPROVAL#aps_123     SK=INVITE#<tokenHash>
PK=TXN#txn_123          SK=PROFILE
PK=TXN#txn_123          SK=EVENT#000001
PK=DISPUTE#dsp_123      SK=PROFILE
PK=IDEMPOTENCY#<scope>  SK=<key>
```

Required secondary indexes:

- `GSI1PK=SELLER#<id>`, `GSI1SK=TXN#<createdAt>#<id>` for seller transactions.
- `GSI2PK=PAYMENT#<paymentIdentifier>`, `GSI2SK=TXN#<id>` for replay prevention.
- `GSI3PK=SLUG#<slug>`, `GSI3SK=SELLER#<id>` for storefront resolution.

## Retention

- Hackathon operational data may be removed during environment teardown.
- Evidence bucket deletion is blocked by default and must be explicitly approved.
- Production retention and deletion periods are deferred pending legal and customer requirements.
