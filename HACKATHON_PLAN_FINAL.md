# AgentPay — Final Hackathon Build Plan (4-person team, Go + Next.js + AWS)

## Team split

- **Person A — Go payment proxy**: `payment-proxy` service, the 402 challenge/verify/forward flow. Critical path — everything depends on this.
- **Person B — Evidence, disputes, trust**: `evidence-service`, `dispute-service`, `trust-service`. This is the moat — protect this time above all else.
- **Person C — Frontend (Next.js) + buyer agent**: seller dashboard, buyer-agent chat UI, Bedrock reasoning integration.
- **Person D — Multi-party sessions + infra glue**: `session-service`, WebSocket wiring, DynamoDB tables, deployment (SAM/CDK), RL stub.

## Timeline (36-hour window, adjust to your actual length)

**Hours 0–2: Setup**
- Repo, AWS account, IaC tool choice (CDK recommended given Go + Next.js mix), shared API contracts agreed from TECHNICAL_SPEC_FINAL.md so nobody blocks anybody.
- DynamoDB tables stood up, one demo seller record created manually.

**Hours 2–10: Core paths in parallel**
- A: Go `payment-proxy` — 402 challenge working, signature verification against a hardcoded test key.
- B: `evidence-service` writing real hash-chained records to S3/DynamoDB.
- C: Next.js shell + buyer-agent chat hitting a stubbed Bedrock call; seller dashboard skeleton.
- D: DynamoDB schema finalized, `session-service` skeleton with a basic WebSocket echo (prove the connection works before adding logic).

**Hours 10–18: Wire the core commerce loop**
- End-to-end: buyer agent → storefront → 402 → signed retry → verified → evidence written → forwarded to a mocked seller endpoint (build a simple one yourselves, don't integrate a real external API under time pressure) → response relayed.

**Hours 18–26: Multi-party session + dispute flow**
- D: real multi-party approval flow — session creation, two clients joining via WebSocket, live approve/veto broadcast, session resolution releasing the transaction.
- B: dispute endpoint pulling evidence (including approval records), one real policy check, auto-resolve/escalate.
- C: `LiveApprovalPanel` component wired to the WebSocket, dashboard showing dispute resolution live.

**Hours 26–30: RL stub + settlement**
- A: mocked settlement step, clearly logged as "would settle here" if real Stripe/stablecoin integration doesn't fit.
- Whoever has slack: a simple contextual-bandit stub — even just logging "if we had outcome data, here's the feature vector we'd train on" is enough to demo the concept honestly; do not attempt to ship a real trained model in a hackathon.

**Hours 30–34: Polish + rehearse**
- Cut anything not on the demo path. Run the full script (below) three times.

**Hours 34–36: Buffer**

## Cut list, in order, if behind schedule
1. RL — reduce to a clearly-labeled stub/mock, never fake a "trained model" claim.
2. MPP/fiat rail — x402 only.
3. Real KMS wallets — fixed test wallet, labeled honestly.
4. General N-of-M multi-party config — hardcode a 2-approver demo case.
5. Multi-seller support — one seller is enough.

## What must not be cut
- The 402 flow actually working end to end.
- The evidence write on every transaction.
- The live multi-party approval demo — two people, two screens, one shared decision in real time. This is your best differentiator on stage.
- The live dispute auto-resolution.

## Demo script (final)
1. Buyer agent proposes a purchase.
2. It requires multi-party approval — a second person joins the live session on their own device, sees the same proposal, approves.
3. Transaction executes, evidence bundle shown (including both approvals).
4. Live dispute raised, auto-resolved in seconds using the evidence.
5. Closing line: "Every rail that lets agents pay skipped the trust layer — we built it, and we built it for decisions that need more than one person's yes."
