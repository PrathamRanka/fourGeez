# AgentPay — Final PRD (Consolidated)

*Supersedes all earlier drafts. Stack: Next.js/TypeScript/Tailwind (frontend) + Go (backend/execution) + AWS (everything else). This is the version to build.*

---

## 1. One-line description

The trust and commerce layer for AI agents: a seller-side proxy that lets any business sell to AI agents safely (evidence, disputes, authorization-proof) — with a buyer-side reference agent that discovers, negotiates, and pays, optimized over time by RL, and with live multi-party authorization for purchases that need more than one human's sign-off.

## 2. Problem

AI agents are starting to pay for things autonomously across x402, ACP, AP2, and soon India's UAP — but every rail has shipped payment before dispute/recourse (confirmed by x402's own docs, an unresolved Reg E question, NPCI's own flagged concern, and a live seller-agent-commerce product, ZeroClick.ai, with no visible dispute handling). Separately, purchase decisions in real life are often **not single-person decisions** — families, co-founders, and procurement teams need joint sign-off — and no agent-payment system supports that today either.

## 3. Product surfaces

### 3.1 Seller-side proxy (core wedge — ZeroClick-shaped, AWS-native)
A proxy any business drops in front of its API to become agent-discoverable and agent-payable, via the x402 protocol (HTTP 402 challenge/response). See §6 for full mechanism.

### 3.2 Trust & evidence layer (the differentiator)
Every transaction gets a tamper-evident evidence record before money moves. Disputes are resolved automatically by matching evidence against policy. See §7.

### 3.3 Buyer-side reference agent (demo + expansion surface)
A Bedrock-powered agent that fulfills "buy the best X under budget Y" end to end: discover → negotiate → check policy → pay → verify → receipt. Built on top of AgentPay's own API — proof the platform works, and a real product surface in its own right if it gains traction.

### 3.4 RL-optimized negotiation (bounded, non-safety-critical)
The buyer agent's negotiation and merchant-selection logic improves over time via RL (contextual bandit to start), learning better terms, more reliable merchants, and user preference weighting. **RL never touches authorization, spending limits, or trust-tier decisions** — those stay deterministic for dispute/regulatory defensibility. See §8.

### 3.5 Multi-party live authorization ("multiplayer" feature, not a separate product)
For purchases that need more than one person's sign-off — a family purchase, a co-founder expense, a procurement approval above a threshold — AgentPay supports a **live, joinable authorization session**: the relevant humans see the agent's proposed purchase in real time and jointly approve or veto it before the agent executes. This is the same evidence/policy engine from §7, extended so the "policy" can require N-of-M human approvals instead of one static rule. See §9.

## 4. Target users

| Segment | Who | What they get |
|---|---|---|
| **Sellers** (B2B) | Businesses with an API/product wanting agent traffic | Drop-in proxy, evidence capture, dispute API, analytics |
| **Agent developers/platforms** (B2B) | Teams building agents that transact | Full commerce SDK, trust-tier reputation, RL-optimized negotiation |
| **Consumers/households** (B2C) | Individuals or families using agents to shop | The reference agent, plus multi-party approval for family/shared purchases |
| **Teams** (B2B) | Co-founders, procurement teams | Multi-party approval flow for agent-initiated spend |

## 5. Trust tiers (agent reputation, permanent identity)

One permanent identity per agent, never deleted or reset. Supervision tier moves with behavior, fed by dispute outcomes across **all** sellers the agent transacts with (network-effect moat):

| Tier | Behavior |
|---|---|
| 1 — Full autonomy | Auto-executes up to budget, no human check |
| 2 — Soft supervision | Auto-executes under a lower threshold; approval above it |
| 3 — Hard supervision | Every transaction needs sign-off |
| 4 — Suspended | No new transactions until manually reviewed |

Reinstatement from Tier 4 → Tier 3 only, never straight to Tier 1.

## 6. How agents transact (x402 mechanism)

1. Agent requests a resource on a seller's storefront.
2. Server (our proxy) returns `402 Payment Required` with price/rail.
3. Agent's wallet signs a payment, retries with a `Payment` header.
4. Proxy verifies, writes evidence (§7), forwards to seller's real API, relays response, settles funds.

## 7. Evidence & dispute layer

- Every transaction: tamper-evident (hash-chained) record — agent ID, resource, amount, policy snapshot, signature, timestamp — written to S3 before forwarding.
- Dispute API: pulls the evidence bundle, checks against policy (single-approver or multi-party, §9), auto-resolves (declined with evidence attached, or refunded) or escalates.
- Precedent: Chargehound (acquired by PayPal), Verifi (acquired by Visa) — same company shape, new transaction type.

## 8. RL scope (explicit boundary)

**In scope for RL**: negotiation terms, merchant reliability scoring, price/quality/speed weighting based on revealed user preference. Contextual bandit to start; graduate to full RL as transaction volume grows.

**Never in scope for RL**: authorization decisions, spending-limit enforcement, trust-tier movement, multi-party approval logic. These stay rules-based and explainable — a dispute process or regulator needs to see "policy said X," not "the model's weights said so."

## 9. Multi-party authorization (feature detail)

- A purchase can be configured to require N-of-M human approval (e.g., "any purchase over ₹5,000 needs both parents to approve," or "procurement spend over $500 needs manager sign-off").
- When triggered, a **live session** opens (shared, joinable — the multiplayer mechanism): all required approvers see the agent's proposed purchase, its reasoning, price comparison, and policy check, in real time.
- Approval/veto from each required party is itself captured as part of the evidence bundle — so a later dispute has proof not just that the transaction was authorized, but *who* authorized it and what they saw.
- This is the one place RL and multi-party logic intersect safely: RL can *suggest* which purchase option to present for approval (better terms, better fit to preferences), but the approval decision itself is always human, always logged, never automated.

## 10. Architecture (stack-specific)

- **Frontend**: Next.js/TypeScript/Tailwind — seller dashboard, buyer-agent chat UI, multi-party live-session UI (WebSocket-connected).
- **Backend/execution**: Go services — the payment proxy (402 challenge/verify/forward), evidence writer, dispute resolver, settlement orchestration. Go is the *only* layer with money-movement and merchant-API access (security boundary from reasoning).
- **Reasoning**: AWS Bedrock — buyer-agent discovery/negotiation/comparison logic, RL-driven optimization layer. Outputs a signed decision object; never calls payment APIs directly.
- **AWS infra**: API Gateway (entry), Go services on Fargate/Lambda (Go runtime), Step Functions (transaction lifecycle orchestration), DynamoDB (agents, sellers, transactions), S3 (evidence store, hash-chained), KMS (wallet/signing keys), Cognito (auth), EventBridge + SQS (async trust scoring, RL training data pipeline), AppSync or API Gateway WebSockets (multi-party live sessions), CloudWatch (observability, dispute forensics).

Full detail in TECHNICAL_SPEC_FINAL.md.

## 11. Phasing

- **Hackathon**: single demo seller, x402 only, real evidence + dispute flow, one multi-party approval demo (2 people), RL layer stubbed/simplified (rule-based "would learn this" logging instead of a trained model), simplified/mocked wallet and settlement.
- **Phase 1 (post-hackathon)**: real design-partner sellers, real custodial wallets, dispute API hardened, contextual bandit RL live on real negotiation data.
- **Phase 2**: MPP/fiat rail, trust scoring live, multi-party authorization generalized beyond the demo case (procurement, family accounts).
- **Phase 3**: cross-seller network effects, India/UAP support, PSP partnerships.

## 12. Success metrics

Design partners live, evidence-capture coverage %, auto-dispute-resolution rate (>70% target), dispute resolution time (<60s target), multi-party session completion rate (approvals reached without abandonment), RL-driven negotiation improvement (cost/quality delta vs. non-RL baseline once live).

## 13. Key risks

- Protocol-level solutions (AP2 mandates, UAP audit trail) could shrink the dispute gap — validate with real sellers first.
- Feature creep: three features (proxy, RL, multi-party) in one pitch risks diluting the story — lead with the proxy + evidence/dispute wedge; present RL and multi-party as the expansion path, not the headline, in the YC application.
- Multi-party authorization is a narrower use case than the core proxy business — treat it as a differentiator feature, not the primary revenue driver.

## 14. Pitch, one paragraph

AI agents are getting real spending authority, and every payment rail built for them ships the ability to pay without the ability to prove, audit, or recover — the same gap that got two companies (Chargehound, Verifi) acquired by PayPal and Visa for human transactions. We're building that trust layer for agent commerce: a drop-in seller proxy with evidence and automated dispute resolution, a reference buyer-agent whose negotiation gets better over time via RL, and — because not every purchase decision belongs to one person — live multi-party authorization so families, co-founders, and procurement teams can jointly approve an agent's spend in real time, with every decision provably authorized.
