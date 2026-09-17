# System architecture

Status: **Locked for hackathon implementation**.

## Purpose

AgentPay sits in front of a seller's API. It publishes machine-readable offers, enforces purchase policies, coordinates approval when required, verifies x402 testnet payment, forwards the request, and records evidence for later dispute handling.

## Runtime components

### Web application

The Next.js application provides seller onboarding, route configuration, the buyer demonstration, live approval, transaction evidence, and dispute views.

Server Components render read-heavy pages. Client Components are limited to buyer interaction, approval decisions, WebSocket status, and small optimistic controls.

### Go API

One deployable Go binary owns all authoritative business rules through isolated packages:

- `catalog`: sellers, routes, manifests, and pricing.
- `intents`: immutable purchase proposals and request hashes.
- `policy`: budget and approval evaluation.
- `approvals`: sessions, invitations, decisions, and approval tokens.
- `payments`: x402 challenge creation and facilitator verification.
- `proxy`: upstream request forwarding and seller request signatures.
- `evidence`: append-only evidence events and chain verification.
- `disputes`: deterministic classification and recommendations.
- `agents`: Bedrock tool orchestration and deterministic fallback.

Packages may call each other through explicit interfaces. They must not write another package's DynamoDB records directly.

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
- CloudWatch receives logs, metrics, alarms, and traces.
- Amplify Hosting deploys the Next.js application.

## Trust boundaries

1. **Browser boundary:** browser input is untrusted and validated by the API.
2. **Model boundary:** Bedrock output is untrusted structured input; Go revalidates all amounts, identifiers, budgets, and policy requirements.
3. **Payment boundary:** only the payments package can access the test wallet or facilitator credentials.
4. **Seller boundary:** forwarded requests are allowlisted by configured method and route, protected against SSRF, and signed for the seller.
5. **Evidence boundary:** evidence writers may append but cannot update or delete objects; verification uses a separate read role.

## Purchase lifecycle

1. The buyer reads the seller manifest.
2. The buyer creates a purchase intent containing route, normalized input hash, quoted amount, currency, and expiration.
3. The policy engine evaluates the immutable intent.
4. If approval is required, the API creates a session and returns HTTP `428` with invitation metadata. No x402 challenge is issued yet.
5. When all required users approve, the backend issues a short-lived approval token bound to the complete intent hash.
6. The buyer requests the paid route with the intent identifier and optional approval token.
7. The gateway returns an x402 challenge when payment is absent.
8. The buyer retries with payment proof.
9. The gateway verifies payment and atomically claims the intent for execution.
10. Evidence is appended for verification, forwarding, response, and final outcome.
11. The proxy forwards exactly once and returns the upstream response.

## Consistency and idempotency

- Client mutations require `Idempotency-Key`.
- Purchase intents are immutable after creation.
- Approval is bound to the intent hash, not only the intent ID.
- Payment identifiers are globally unique in the transaction table.
- A DynamoDB conditional write changes a transaction from `PAYMENT_VERIFIED` to `FORWARDED`; only the winner calls the seller.
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

## Deliberate exclusions

The first implementation does not provide production custody, production settlement, generalized refunds, cross-seller reputation, autonomous negotiation, arbitrary seller code execution, or automatic quality judgments.
