# Architecture decisions and assumptions

Last reviewed: 2026-09-17.

## Locked decisions

| ID | Decision | Reason |
|---|---|---|
| ADR-001 | Build a seller-side gateway for API sellers first. | Produces one economic buyer and a focused onboarding flow. |
| ADR-002 | Deliver a hackathon vertical slice, then harden the same boundaries into an MVP. | Avoids a disposable demo without imposing premature production complexity. |
| ADR-003 | Use a Go modular monolith rather than independent microservices. | Keeps one deployable backend while preserving package boundaries. |
| ADR-004 | Use Next.js, TypeScript, and Tailwind for the web application. | Matches the existing product specification and team plan. |
| ADR-005 | Use AWS CDK in TypeScript for infrastructure. | Keeps infrastructure reviewable and reproducible. |
| ADR-006 | Use x402 testnet only in the hackathon. | Demonstrates a real payment protocol without production funds or custody. |
| ADR-007 | Trigger multi-party approval before issuing a payable challenge. | Prevents funds from moving before required authorization exists. |
| ADR-008 | Bedrock proposes actions; Go validates and executes them. | Model output cannot bypass authorization or access payment credentials. |
| ADR-009 | Store evidence as append-only events and never store raw payment credentials. | Limits sensitive data and makes verification deterministic. |
| ADR-010 | The platform recommends refunds but does not unilaterally move production funds. | Keeps the initial product outside custody and settlement ownership. |
| ADR-011 | Use deterministic dispute rules; no RL in the first milestone. | There is no reliable training dataset yet. |
| ADR-012 | `AgentPay` is a working name only. | Naming and trademark review are required before launch. |
| ADR-013 | Default development/demo region is `us-east-1`, overridable by deployment configuration. | Provides one documented default while requiring Bedrock model availability verification. |

## Hackathon assumptions

- One AWS region is used for all resources.
- One demo seller exposes one paid `POST /research` route.
- A dedicated test wallet contains only testnet assets.
- Two approvers are required when a configured price threshold is crossed.
- Approval links are single-use and expire after ten minutes.
- Refund execution is simulated and clearly labeled; dispute classification is real.
- The deterministic buyer remains available if Bedrock is unavailable.

## Deferred decisions

- Production payment rails and facilitator commercial terms.
- Production refund/reimbursement mechanism.
- Cross-seller reputation and portable agent identity.
- Data residency and multi-region recovery.
- Enterprise SSO and SCIM.
- Real custodial wallets.
- MPP, card, AP2, or UCP adapters.
- Negotiation optimization and RL.

## Decision change procedure

Any change to a locked decision requires:

1. A new ADR entry with motivation and consequences.
2. Updates to affected API, data model, diagrams, and tasks.
3. A standalone documentation commit before implementation begins.
