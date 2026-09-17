# AgentPay — Final Technical Specification

Stack: **Next.js/TypeScript/Tailwind** (frontend) + **Go** (backend/execution) + **AWS Bedrock** (reasoning) + **AWS** (everything else).

---

## 1. Service architecture

```
[Next.js Frontend]  <-- WebSocket -->  [AppSync / API GW WebSockets]  <-- multi-party live sessions
       |
       | REST/HTTPS
       v
[API Gateway] --> [Go Payment Proxy Service (Fargate/Lambda)] --(402 challenge)--> Agent retries w/ signed header
                          |
                          v (verified)
                  [Go Evidence Service] -> S3 (hash-chained) + DynamoDB (index)
                          |
                          v
                  [Go Forward Service] -> Seller's real API -> response relayed
                          |
                          v
                  [Step Functions: Settlement] -> Stripe / stablecoin payout
                          |
                          v
                  [EventBridge] -> [SQS] -> [Go/Python Trust Scoring + RL Training Service]
                          |
                          v
                  [Bedrock Reasoning Service] -- signed decision object only, no payment access --> Go Proxy
```

**Security boundary**: Bedrock (reasoning) never calls payment or merchant APIs directly. It emits a signed decision object that the Go execution layer independently re-validates before acting — this is "Bedrock decides, Go executes," enforced architecturally, not just by convention.

## 2. Go backend services (the execution layer)

All written in Go, deployed on **AWS Fargate** (for long-running services) or **Lambda with the Go runtime** (for the request-scoped proxy handlers — Lambda is fine here since each request is short-lived):

- `payment-proxy`: handles the 402 challenge/verify/forward flow (the hot path).
- `evidence-service`: writes hash-chained evidence records to S3, indexes in DynamoDB.
- `dispute-service`: pulls evidence, runs policy match (single-approver or multi-party), resolves or escalates.
- `settlement-service`: orchestrated via Step Functions, calls Stripe/stablecoin rails.
- `trust-service`: consumes SQS events, updates agent trust scores/tiers.
- `session-service`: manages multi-party live authorization sessions (WebSocket connection handling via API Gateway WebSockets or AppSync, session state in DynamoDB).

## 3. Next.js frontend structure

```
/app
  /dashboard          -- seller dashboard: transactions, disputes, analytics
  /agent               -- buyer-agent chat UI (talks to Bedrock reasoning service via API)
  /sessions/[id]        -- multi-party live authorization session (WebSocket-connected)
  /onboarding           -- seller onboarding flow
/lib
  /api-client.ts        -- typed client for the Go backend REST API
  /ws-client.ts          -- WebSocket client for live sessions
/components
  /TransactionLog.tsx
  /DisputeCard.tsx
  /TrustTierBadge.tsx
  /LiveApprovalPanel.tsx  -- the multi-party approval UI
```

## 4. Data models (DynamoDB)

### `Sellers`
`seller_id` (PK), `name`, `stripe_account_id`, `api_base_url`, `services` (list), `created_at`

### `Agents`
`agent_id` (PK), `wallet_type`, `kms_key_ref`, `trust_tier` (1-4), `trust_score`, `first_seen`, `last_seen`

### `Transactions`
`transaction_id` (PK), `seller_id`, `agent_id`, `service_id`, `amount`, `currency`, `rail`, `status`, `evidence_s3_key`, `approval_session_id` (nullable — set if multi-party), `created_at`, `settled_at`

### `Evidence` (S3, hash-chained JSON objects)
```json
{
  "transaction_id": "...",
  "agent_id": "...",
  "seller_id": "...",
  "requested_resource": "...",
  "amount": "...",
  "payment_header": "<raw x402 payload>",
  "policy_snapshot": { "budget": 3000, "requires_approval_from": ["user_a", "user_b"] },
  "approvals": [ { "user_id": "user_a", "decision": "approved", "timestamp": "..." }, { "user_id": "user_b", "decision": "approved", "timestamp": "..." } ],
  "rl_suggested_option": { "chosen": true, "alternatives_considered": [...] },
  "prev_hash": "...",
  "this_hash": "...",
  "timestamp": "..."
}
```

### `ApprovalSessions`
`session_id` (PK), `transaction_proposal` (map — what's being bought, price, comparison), `required_approvers` (list), `approvals_received` (list), `status` (`pending`/`approved`/`vetoed`/`expired`), `created_at`

### `Disputes`
`dispute_id` (PK), `transaction_id`, `raised_by`, `reason`, `resolution`, `evidence_match_result`

## 5. API contracts

### Seller REST API (Go, via API Gateway)
- `POST /v1/sellers` — onboard
- `POST /v1/sellers/{id}/services` — register a purchasable service
- `GET /v1/sellers/{id}/transactions` — dashboard data
- `POST /v1/disputes` — raise a dispute
- `GET /v1/disputes/{id}` — poll resolution

### Agent-facing (x402 flow)
- `GET /storefront/{seller_id}` → manifest
- `GET /proxy/{seller_id}/{path}` (no payment) → `402` challenge
- Same request + `Payment` header → `200` + `X-AgentPay-Transaction-Id`

### Multi-party session API
- `POST /v1/sessions` — create an approval session
  ```json
  { "transaction_proposal": {...}, "required_approvers": ["user_a", "user_b"] }
  ```
  Returns `{ "session_id": "...", "join_url": "..." }`
- WebSocket events (via AppSync/API Gateway WS): `session.update` (someone joined, someone approved/vetoed), `session.resolved` (all approvals in, or timeout/veto)
- `POST /v1/sessions/{id}/decide` — `{ "user_id": "...", "decision": "approve" | "veto" }`

## 6. RL design (bounded scope)

- **Model**: contextual bandit (e.g., LinUCB or a simple epsilon-greedy bandit) to start — far simpler than full RL, appropriate for the negotiation/merchant-selection decision space, and easier to explain/debug than a deep RL policy.
- **Context features**: user's stated preferences (price sensitivity, quality bar, delivery urgency), merchant historical reliability (from trust-service data), price/quality/delivery attributes of each candidate option.
- **Action**: which option to negotiate toward / present as the recommended purchase.
- **Reward signal**: user accepts without modification (positive), user overrides/vetoes (negative), later dispute on this transaction (strongly negative), positive fulfillment/verification outcome (positive).
- **Training loop**: SQS-fed pipeline (evidence + outcome events) → periodic batch retraining (not real-time for MVP) → updated model artifact deployed to the Bedrock reasoning service's decision logic.
- **Explicit boundary, enforced in code, not just policy**: the RL model's output is a *ranked suggestion* consumed by the reasoning layer when proposing options — it never has write access to the `Agents.trust_tier` field, the policy engine, or the Go execution layer's authorization check. This should be enforced by IAM/service boundaries (the RL/trust-scoring service has no IAM permissions to write to the authorization path), not just by convention.

## 7. Multi-party live session mechanics

1. Agent (via reasoning layer) determines a purchase needs multi-party approval (policy check: amount over threshold, or category flagged as joint-decision).
2. `session-service` creates an `ApprovalSessions` record, generates a join URL per required approver.
3. Each approver opens the Next.js `/sessions/[id]` page, connects via WebSocket.
4. All connected approvers see the same live view: proposed purchase, price comparison, RL-suggested option and why.
5. Each approver sends an approve/veto decision; broadcast to all connected clients in real time.
6. Once all required approvals are in (or any veto is received, or a timeout expires), `session-service` resolves the session and — if approved — releases the transaction to `payment-proxy` to proceed; the full approval record is written into the transaction's evidence bundle.

## 8. AWS service mapping (summary table)

| Layer | Service |
|---|---|
| Frontend hosting | Amplify Hosting or S3+CloudFront for Next.js static/SSR |
| API entry | API Gateway (REST + WebSocket) |
| Backend services | Go on Fargate (long-running) / Lambda-Go (request-scoped) |
| Reasoning | Bedrock |
| Transaction orchestration | Step Functions |
| Data | DynamoDB |
| Evidence store | S3 (hash-chained) |
| Keys | KMS |
| Auth | Cognito |
| Async/events | EventBridge + SQS |
| Live sessions | AppSync (managed GraphQL subscriptions) or API Gateway WebSocket API |
| Observability | CloudWatch |
| Settlement | Stripe (external) + stablecoin path |

## 9. What to simplify for the hackathon

Same guidance as before — build the 402 flow, evidence write, and dispute resolution for real; mock wallet custody and real settlement; for RL, log "here's what we'd learn from this outcome" rather than shipping a trained model; for multi-party, a working 2-person live approval demo is enough — don't build the general N-of-M policy configuration UI under time pressure.

## 10. Demo script

1. Storefront + buyer agent purchase flow (as before).
2. This time: the purchase amount triggers multi-party approval — show a second judge/laptop joining the live session, seeing the same proposal, approving in real time.
3. Show the evidence bundle including both approvals.
4. Trigger a dispute, show auto-resolution using the full evidence (including who approved what).
5. One slide: RL's role (negotiation quality, not safety-critical decisions) and the trust-tier system.
