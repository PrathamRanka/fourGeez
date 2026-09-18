# Architecture decisions and assumptions

Last reviewed: 2026-09-18.

## Delivery status

The locked decisions below describe the product direction and implemented
M0–M7 boundaries. They do not imply production readiness. M7.1 must resolve and
record the remaining decisions for subscription lifecycle, short-lived MCP
capabilities, signed discovery, transaction execution authorization, seller
authentication, billing-provider integration, buyer remediation, operator
access, and data lifecycle before dependent code is implemented.

## Locked decisions

| ID | Decision | Reason |
|---|---|---|
| ADR-001 | Build a seller-side gateway for API sellers first. | Produces one economic buyer and a focused onboarding flow. |
| ADR-002 | Deliver a hackathon vertical slice, then harden the same boundaries into an MVP. | Avoids a disposable demo without imposing premature production complexity. |
| ADR-003 | Use a Go modular monolith rather than independent microservices. | Keeps one deployable backend while preserving package boundaries. |
| ADR-004 | Use Next.js, TypeScript, and Tailwind for the web application. | Matches the existing product specification and team plan. |
| ADR-005 | Superseded by ADR-020: the initial plan selected AWS CDK in TypeScript. | Preserved as decision history; no new CDK resources may be added. |
| ADR-006 | Use x402 testnet only in the hackathon. | Demonstrates a real payment protocol without production funds or custody. |
| ADR-007 | Trigger multi-party approval before issuing a payable challenge. | Prevents funds from moving before required authorization exists. |
| ADR-008 | Bedrock proposes actions; Go validates and executes them. | Model output cannot bypass authorization or access payment credentials. |
| ADR-009 | Store evidence as append-only events and never store raw payment credentials. | Limits sensitive data and makes verification deterministic. |
| ADR-010 | The platform recommends refunds but does not unilaterally move production funds. | Keeps the initial product outside custody and settlement ownership. |
| ADR-011 | Use deterministic dispute rules; no RL in the first milestone. | There is no reliable training dataset yet. |
| ADR-012 | `AgentPay` is a working name only. | Naming and trademark review are required before launch. |
| ADR-013 | Default development/demo region is `us-east-1`, overridable by deployment configuration. | Provides one documented default while requiring Bedrock model availability verification. |
| ADR-014 | Treat each published paid route as the V1 product record. | Supports API calls and digitally fulfilled products without introducing a speculative catalog abstraction. |
| ADR-015 | Make seller onboarding agent-assisted through a remote MCP server and supported coding-agent instructions. | Sellers can prepare an integration from their existing repository while AgentPay retains validation and authorization. |
| ADR-016 | Require explicit seller confirmation for price changes, publication, credential rotation, and production deployment. | Repository content and model output are untrusted and cannot authorize commercial or security-sensitive changes. |
| ADR-017 | Serve human and agent buyers through the same intent, approval, transaction, evidence, fulfillment, and dispute domains. | Prevents channel-specific rules from producing inconsistent prices or authorization outcomes. |
| ADR-018 | Use maintained request-verification packages rather than generated cryptographic implementations. | Keeps seller integration small while preserving one audited signature protocol. |
| ADR-019 | Fund the initial product through seller subscriptions and metered software usage, separately from buyer-to-seller settlement. | Allows monetization without taking custody of buyer funds. |
| ADR-020 | Use Terraform for infrastructure and remove the existing CDK placeholder during AWS-000. | Prevents dual ownership of AWS resources and matches the repository engineering contract. |
| ADR-021 | Ship the first seller dashboard around direct seller x402 settlement; defer card checkout to H1. | The dashboard can use AgentPay transaction records without placing AgentPay in the buyer-funds flow. |
| ADR-022 | Require a verified seller payment destination for each asset and network before route publication. | A copied or mistyped payout address must not silently receive buyer funds. |
| ADR-023 | Treat AgentPay transaction, reconciliation, evidence, and aggregate records as the dashboard source of truth; the seller coding agent is setup-time tooling, not a runtime sales reporter. | Sales remain visible when the coding agent is disconnected and can be isolated by seller. |
| ADR-024 | Generate technical SEO, AEO, and agent-discovery assets through setup bundles, but never promise rankings or generate deceptive content. | Search placement is controlled by external systems; AgentPay can improve discoverability and correctness only. |
| ADR-025 | Publish an explicit tested-stack matrix. A stack is advertised as supported only after its integration recipe and focused fixture pass. | Language-level verification primitives do not prove framework-level raw-body, middleware-order, routing, or rendering compatibility. |

## Hackathon assumptions

- One AWS region is used for all resources.
- One demo seller exposes one paid `POST /research` route.
- A dedicated test wallet contains only testnet assets.
- Two approvers are required when a configured price threshold is crossed.
- Approval links are single-use and expire after ten minutes.
- Refund execution is simulated and clearly labeled; dispute classification is real.
- The deterministic buyer remains available if Bedrock is unavailable.
- The initial sellable products are API-backed or digitally fulfilled; physical commerce is excluded.

## Deferred decisions

- Production payment rails and facilitator commercial terms.
- Production refund/reimbursement mechanism.
- Cross-seller reputation and portable agent identity.
- Data residency and multi-region recovery.
- Enterprise SSO and SCIM.
- Real custodial wallets.
- The production card-checkout provider and merchant-of-record arrangement, deferred to H1.
- MPP, AP2, or UCP adapters.
- Physical-product inventory, shipping, taxation, and returns.
- Negotiation optimization and RL.

## Decision change procedure

Any change to a locked decision requires:

1. A new ADR entry with motivation and consequences.
2. Updates to affected API, data model, diagrams, and tasks.
3. A standalone documentation commit before implementation begins.
