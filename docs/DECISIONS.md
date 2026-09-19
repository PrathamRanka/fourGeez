# Architecture decisions and assumptions

Last reviewed: 2026-09-19.

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
| ADR-007 | Superseded for Lean V1 by ADR-043: the hackathon used multi-party approval before a payable challenge. | Preserved as historical M2 product and implementation context. |
| ADR-008 | Bedrock proposes actions; Go validates and executes them. | Model output cannot bypass authorization or access payment credentials. |
| ADR-009 | Store evidence as append-only events and never store raw payment credentials. | Limits sensitive data and makes verification deterministic. |
| ADR-010 | The platform recommends refunds but does not unilaterally move production funds. | Keeps the initial product outside custody and settlement ownership. |
| ADR-011 | Use deterministic dispute rules; no RL in the first milestone. | There is no reliable training dataset yet. |
| ADR-012 | `AgentPay` is a working name only. | Naming and trademark review are required before launch. |
| ADR-013 | Default development/demo region is `us-east-1`, overridable by deployment configuration. | Provides one documented default while requiring Bedrock model availability verification. |
| ADR-014 | Treat each published paid route as the V1 product record. | Supports API calls and digitally fulfilled products without introducing a speculative catalog abstraction. |
| ADR-015 | Make seller onboarding agent-assisted through a remote MCP server and supported coding-agent instructions. | Sellers can prepare an integration from their existing repository while AgentPay retains validation and authorization. |
| ADR-016 | Require explicit seller confirmation for price changes, publication, credential rotation, and production deployment. | Repository content and model output are untrusted and cannot authorize commercial or security-sensitive changes. |
| ADR-017 | Serve human and agent buyers through the same intent, transaction, evidence, fulfillment, and dispute domains. | Prevents channel-specific rules from producing inconsistent prices or authorization outcomes. |
| ADR-018 | Use maintained request-verification packages rather than generated cryptographic implementations. | Keeps seller integration small while preserving one audited signature protocol. |
| ADR-019 | Fund the initial product through seller subscriptions and metered software usage, separately from buyer-to-seller settlement. | Allows monetization without taking custody of buyer funds. |
| ADR-020 | Use Terraform for infrastructure and remove the existing CDK placeholder during AWS-000. | Prevents dual ownership of AWS resources and matches the repository engineering contract. |
| ADR-021 | Ship the first seller dashboard around direct seller x402 settlement; defer card checkout to H1. | The dashboard can use AgentPay transaction records without placing AgentPay in the buyer-funds flow. |
| ADR-022 | Require a verified seller payment destination for each asset and network before route publication. | A copied or mistyped payout address must not silently receive buyer funds. |
| ADR-023 | Treat AgentPay transaction, reconciliation, evidence, and aggregate records as the dashboard source of truth; the seller coding agent is setup-time tooling, not a runtime sales reporter. | Sales remain visible when the coding agent is disconnected and can be isolated by seller. |
| ADR-024 | Generate technical SEO, AEO, and agent-discovery assets through setup bundles, but never promise rankings or generate deceptive content. | Search placement is controlled by external systems; AgentPay can improve discoverability and correctness only. |
| ADR-025 | Publish an explicit tested-stack matrix. A stack is advertised as supported only after its integration recipe and focused fixture pass. | Language-level verification primitives do not prove framework-level raw-body, middleware-order, routing, or rendering compatibility. |
| ADR-026 | Use the seller storefront as the V1 human buyer experience and reserve `/demo/agent-checkout` for the visible agent-channel demonstration. | Avoids an unexplained buyer account area and keeps browser and agent purchases on one commerce pipeline. |
| ADR-027 | Use `/store/{sellerSlug}` for one seller and `/store/{sellerSlug}/products/{productSlug}` for one public product while retaining immutable internal IDs behind the route. | Gives people and agents readable canonical URLs without using mutable slugs as authorization or persistence identity. |
| ADR-028 | Superseded for Lean V1 by ADR-042: use the proprietary `POST /v1/integration-access-tokens` bootstrap exchange through the required AgentPay connector. | Direct remote MCP OAuth remains a compatibility goal but is not required for the first launch. |
| ADR-029 | Issue ES256 JWT access capabilities for at most five minutes, publish overlapping public keys at `/.well-known/jwks.json`, and require exact issuer, audience, subject, seller, credential, scope, entitlement-epoch, JTI, issued-at, and expiry claims. | Short lifetimes plus authoritative revision checks bound credential leakage and permit immediate cloud-side revocation without distributing signing authority. |
| ADR-030 | Expose the authoritative seller entitlement revision as `entitlementEpoch`, `source`, `sourceRevision`, `accessEndsAt`, and optimistic `version`; transaction-critical authorization compares the token epoch with current cloud state. | Subscription state must be independently versioned, replay-safe, and enforceable at the exact UTC access boundary. Lifecycle transitions and provider mapping remain owned by LCH-005. |
| ADR-031 | Publish a signed, short-lived public discovery document for active storefronts and a signed `410 Gone` tombstone for inactive storefronts. Seller-hosted copies and `llms.txt` are discovery hints only. | Cached discovery can identify a candidate product but can never prove current entitlement or authorize a transaction. |
| ADR-032 | Authorize browser checkout with an opaque server-side purchase grant represented by a Secure HttpOnly cookie, separating ten-minute commerce authority from remediation-window read access and allowing post-payment recovery only through the bound payer wallet. | Public human checkout must survive reloads and support later receipts/disputes without exposing a durable bearer token to JavaScript or restoring payment authority during recovery. |
| ADR-033 | Superseded for Lean V1 by ADR-043: the M2 approval design used fragment invitation exchange, server-side browser grants, CSRF, and exact Origin validation. | Preserved as historical design context if enterprise approval is reintroduced. |
| ADR-034 | Replace seller-request HMAC authority with a 30-60 second ES256 execution capability in `X-AgentPay-Execution-Capability`, bound to the transaction, seller, route, method, literal path, raw-body hash, audience, payment finality, JTI, issue time, and expiry. | A seller can verify AgentPay authorization through JWKS but cannot mint an official execution capability after modifying or forking local software. Webhook HMAC remains separate. |
| ADR-035 | Rotate a project key by atomically creating a successor and revoking the predecessor; retain the committed response secret only as tightly scoped KMS envelope-encrypted idempotency ciphertext for ten minutes, then require rotation of the successor if delivery was lost. | Rotation must immediately terminate the predecessor without making a transient response loss unrecoverable or persisting plaintext key material. |
| ADR-036 | Treat OpenAPI 0.5 and the LCH-004 companion contracts as the M7.1 production target while explicitly preserving the implemented M7 behavior as a development-only migration baseline. | Contract-first work must not falsely claim that the current Go runtime already serves the launch protocol, and production must never partially activate incompatible authorization behavior. |
| ADR-037 | Use Stripe Billing as the initial seller-subscription provider while keeping AgentPay's `SellerEntitlement` projection authoritative and completely separate from buyer x402 settlement. | Stripe provides the seller billing lifecycle without making provider terminology, redirects, or webhook order an authorization boundary or placing AgentPay in the buyer-funds flow. |
| ADR-038 | End paid network participation exactly at `accessEndsAt`; a fixed 72-hour `grace` state permits billing recovery and historical reads only. Scheduled cancellation remains `active` until the paid boundary, while fraud quarantine suspends immediately and cannot be cleared by Stripe alone. | Unpaid sellers cannot continue through MCP, discovery, or commerce merely because a webhook or background transition is delayed. |
| ADR-039 | Treat every seller-hosted MCP connector, repository, prompt, generated integration, discovery copy, and fulfillment component as untrusted. Keep the official MCP and all signing, entitlement, payment, publication, and transaction authority in AgentPay cloud. A production MCP commercial mutation requires a five-minute opaque one-time confirmation grant minted only by the authenticated seller browser/BFF and bound to seller, credential, tool, target, canonical arguments, and resource version. | A seller can modify or fork local code and can self-assert confirmation fields, so local checks cannot prove consent or network authority. Cloud issuance and atomic consumption make confirmation enforceable without relying on connector integrity. |
| ADR-040 | Reduce M7.1 from 51 launch tasks to fourteen Lean V1 tasks. Defer the global directory, direct OAuth MCP, Redis/outbox scaling, full operator console, automated reimbursement, advanced product analytics, multi-owner accounts, and nonessential notification automation. | The smaller scope preserves the secure seller-to-buyer transaction and cancellation boundary while shortening the path to a usable launch candidate. |
| ADR-041 | Keep transaction-critical entitlement and revocation checks on authoritative persistence in Lean V1 rather than adding Redis. | Strong direct checks are simpler and safer at initial scale; Redis and distributed invalidation may be added later without changing authorization semantics. |
| ADR-042 | Require the AgentPay local connector for project-key MCP installations in Lean V1 and defer direct remote OAuth clients. Every MCP request still uses a short-lived capability and rechecks current cloud state. | One supported integration path reduces implementation and support surface without trusting seller-hosted code or accepting permanent credentials at `/mcp`. |
| ADR-043 | Disable buyer-side multi-person approval for Lean V1. The seller-approved fixed quote is authoritative, `maximumAmount` is the buyer ceiling, and the buyer wallet authorization/signature is payment consent. Historical approval code and records remain compatibility-only; approval REST routes, cookies, tokens, WebSockets, UI, `428 approval_required`, and approval transaction states are not active V1 surfaces. Seller confirmation for commercial configuration remains mandatory and separate. | Removes an unnecessary checkout branch while retaining deterministic price, budget, wallet-consent, replay, and seller-control protections. |
| ADR-044 | Let an authenticated seller record one append-only, idempotent external refund for a `refund_recommended` dispute through `POST /v1/sellers/{sellerId}/disputes/{disputeId}/refund-records`. AgentPay validates finalized payment and an exact amount/asset/network match but does not move funds or claim network proof. | Gives Lean V1 an auditable remediation record without making AgentPay a custodian or refund executor. |
| ADR-045 | Use one high-contrast, near-black editorial interface system across the public site and authenticated seller dashboard: neutral sans typography, square controls, thin gray structural rules, generous layout rails, and restrained blue/lilac/pink light fields only for focused emphasis. Dashboard density, hierarchy, tables, charts, status states, and responsive navigation must follow the same system without copying reference-site content or replacing real AgentPay behavior. | A single visual language makes the product feel deliberate from acquisition through daily operations while keeping commerce data legible, accessible, and clearly distinct from decorative marketing content. |
| ADR-046 | Supersede ADR-045 with a component-led dual-theme system derived from the approved Aceternity and 21st.dev references in `design/design.md component`. Rebuild page composition around useful bounded bento sections, shader-led hero emphasis, integration diagrams, an interactive globe treatment, premium authentication and pricing layouts, and finance-grade data visualization. Use a distinctive display face for headings, a separate readable interface face, consistent spacing, and equivalent contrast and visual strength in light and dark modes. | The approved component references now control the product's visual language; rebuilding the composition prevents the previous implementation from looking like restyled legacy UI while preserving AgentPay's real routes, data, accessibility, and commerce behavior. |
| ADR-047 | Keep the authenticated seller dashboard in a dedicated dark graphite theme even when public pages support light and dark modes. Dashboard pages use progressive disclosure: one primary decision or dataset per section, compact summary metrics, bounded bento only for related information, and secondary technical details behind clearly labeled sections. | Sellers need a calm fintech workspace for repeated operational use; reducing simultaneous visual weight improves scan speed without removing access to products, payments, evidence, disputes, billing, or integration controls. |
| ADR-048 | Use a reusable animated radial-gradient field as the public landing hero's signature visual while preserving the existing AgentPay proposition, calls to action, and purchase-flow illustration. The field is decorative, uses the existing `motion` dependency, avoids remote animation assets, and becomes static under reduced-motion preferences. | A focused payment-aurora treatment gives the hero a stronger identity without adding an unnecessary Lottie runtime or external asset dependency, weakening semantic content, or compromising accessibility and performance. |
| ADR-049 | Refine ADR-048 by allowing one small decorative Lottie mascot in the landing hero, loaded through the pinned DotLottie React player. Keep the central reading area neutral, limit the gradient accents to blue, pink, and orange, hide the animation from assistive technology, and disable autoplay when reduced motion is requested. | The approved visual reference uses a compact friendly character and distant color bands; containing that treatment to the marketing hero preserves clarity while giving AgentPay a warmer, more recognizable first impression. |

## Hackathon assumptions

- One AWS region is used for all resources.
- One demo seller exposes one paid `POST /research` route.
- A dedicated test wallet contains only testnet assets.
- Buyer-side multi-person approval is disabled for Lean V1.
- Refund execution remains external to AgentPay; seller-recorded refund metadata and dispute classification are real.
- The deterministic buyer remains available if Bedrock is unavailable.
- The initial sellable products are API-backed or digitally fulfilled; physical commerce is excluded.

## Deferred decisions

- Production payment rails and facilitator commercial terms.
- Production refund/reimbursement mechanism.
- Cross-seller reputation and portable agent identity.
- Data residency and multi-region recovery.
- Enterprise SSO and SCIM.
- Real custodial wallets.
- Direct remote MCP OAuth clients and discovery metadata.
- Redis-backed distributed authorization caching and invalidation.
- A global cross-seller directory and ranking system.
- A full internal operator web console; Lean V1 uses audited administrative APIs and runbooks.
- Multi-owner seller teams and granular team RBAC.
- Enterprise buyer policy and configurable N-of-M approval, including any
  future invitation, WebSocket, and completion-token runtime.
- The production card-checkout provider and merchant-of-record arrangement, deferred to H1.
- MPP, AP2, or UCP adapters.
- Physical-product inventory, shipping, taxation, and returns.
- Negotiation optimization and RL.

## Decision change procedure

Any change to a locked decision requires:

1. A new ADR entry with motivation and consequences.
2. Updates to affected API, data model, diagrams, and tasks.
3. A standalone documentation commit before implementation begins.
