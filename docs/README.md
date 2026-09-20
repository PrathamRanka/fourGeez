# Documentation index

This directory is the implementation source of truth for AgentPay. The older `*_FINAL.md` files remain useful product inputs, but this documentation resolves their conflicts and separates hackathon scope from post-hackathon scope.

## Current delivery status

Milestones M0 through M7 and the fourteen-task M7.1 Lean V1 launch core are
implemented and verified locally. M7.1 provides seller authentication,
subscription enforcement, cloud-authoritative MCP access, the shared browser
and agent x402 commerce path, exactly-once fulfillment, the essential seller
dashboard, and full-system browser verification. M8 infrastructure work has
deployed the Terraform foundation, protected development state backend,
Mumbai Cognito seller identity, the ARM64 Go Lambda and HTTP API, and the
Vercel production web application. On September 19, 2026, Terraform applied
37 additions with no changes or destroys and a post-apply plan reported no
changes. The deployed API health, readiness, public capability, and payment
capability checks pass; the canonical Vercel deployment is ready and serving
the expected authentication and documentation routes. Thirteen CloudWatch
alarms and the seller operations dashboard are deployed, but AWS-009 remains
in progress because SNS alarm actions and notification delivery are not
configured. Bedrock remains disabled and is not a seller-first V1 blocker.
M9 deployed release verification remains pending. Until those release gates pass,
AgentPay supports local mock and x402 testnet use only and must not be
represented as a production-ready paid service.

The DX-004 seller-package release engineering path is complete: deterministic
connector and merchant SDK artifacts, checksum/provenance verification, clean
offline installation tests, pinned local-host instructions, and a manual
least-privilege GitHub Release workflow are committed. No package release has
been published. Repository owners must still approve customer-use terms and
contributor provenance, protect the release tag and review environment, choose
private or otherwise authorized distribution, and enable immutable releases.

OpenAPI 0.9 adds authoritative pre-payment input-schema validation and the
schema-driven human buyer form. It also adds deterministic paid-but-unfulfilled
recovery with one bounded
same-payment fulfillment retry when non-delivery is provably side-effect-free,
seller review for uncertain delivery, and truthful seller-reported refund
semantics. It retains the OpenAPI 0.8 removal of the seller-callable onboarding
purchase action while
preserving the public buyer storefront and external buyer-agent commerce.
It retains the OpenAPI 0.7 bounded EXT-001 public capability manifest and
deterministic product directory, EXT-004 runtime payment-capability and recovery
contract, and EXT-005 seller-first published product-schema contract. AgentPay
buyer execution, A2A, and negotiation remain deferred to V2. Production
activation still depends on the remaining M8 operational work, external
security/legal review, and M9 deployed verification.

Lean V1 intentionally excludes buyer-side multi-person approval. Historical
M2/M3 approval code and contract history are retained for compatibility, but
the launch runtime exposes no buyer approval REST, cookie, token, WebSocket,
UI, or `428` payment branch. Seller confirmation for MCP commercial mutations
remains required and is a separate authorization boundary.

| Surface                      | Historical M7 behavior                          | Implemented Lean V1 behavior                                                                                              |
| ---------------------------- | ----------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------- |
| Seller project credential    | `apc1` credential used directly by MCP          | `apc2` bootstrap through `/v1/integration-access-tokens`; required local connector and short-lived access token at `/mcp` |
| MCP mutation confirmation    | Caller-supplied boolean, summary, and timestamp | Seller-session-issued one-time grant bound to the exact mutation                                                          |
| Buyer approval               | M2 threshold/two-person runtime                 | Deferred and disabled for Lean V1; wallet authorization plus the buyer maximum is the buyer consent boundary              |
| Browser purchase             | Buyer-agent key only                            | Durable opaque purchase cookie, bounded commerce window, and payer-wallet recovery                                        |
| Seller forwarding            | Per-seller HMAC                                 | Finality-gated ES256 execution capability                                                                                 |
| Seller entitlement           | `active`/`suspended` plan projection            | Full entitlement projection and epoch checks                                                                              |
| Receipts and invoice exports | Schema version 1                                | Version-aware readers with launch writers on schema version 2                                                             |

No production environment may expose a mixed mode in which a target endpoint
accepts a legacy credential or a legacy endpoint bypasses target authorization.
The complete gap analysis and reference security design are recorded in
[`../scripts/flaws.md`](../scripts/flaws.md).

## Authoritative documents

| Document                                                                               | Purpose                                                                                          |
| -------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------ |
| [PRODUCT.md](PRODUCT.md)                                                               | Product scope, seller automation, buyer channels, and revenue boundaries                         |
| [IMPLEMENTATION.md](IMPLEMENTATION.md)                                                 | Ordered task ledger, dependencies, acceptance criteria, and milestone status                     |
| [ARCHITECTURE.md](ARCHITECTURE.md)                                                     | Components, boundaries, request flow, deployment shape, and failure behavior                     |
| [DATA_MODEL.md](DATA_MODEL.md)                                                         | Entities, state machines, identifiers, indexes, and retention                                    |
| [AWS_SETUP.md](AWS_SETUP.md)                                                           | AWS accounts, services, IAM, deployment order, configuration, and teardown                       |
| [SECURITY.md](SECURITY.md)                                                             | Threat model, secrets, evidence integrity, privacy, and production gates                         |
| [REPOSITORY_GOVERNANCE.md](REPOSITORY_GOVERNANCE.md)                                   | Proprietary-source posture, contributor rights, GitHub controls, and owner-only settings         |
| [SUBSCRIPTION_LIFECYCLE.md](SUBSCRIPTION_LIFECYCLE.md)                                 | Stripe Billing adapter, entitlement states, expiry, recovery, revocation, and historical access  |
| [MCP_SECURITY_BOUNDARY.md](MCP_SECURITY_BOUNDARY.md)                                   | Cloud-authoritative MCP, one-time seller confirmation, discovery separation, and fork resistance |
| [TEST_PLAN.md](TEST_PLAN.md)                                                           | Test levels, required scenarios, fixtures, and release gates                                     |
| [runbooks/OPERATOR_SUSPENSION_REPLAY.md](runbooks/OPERATOR_SUSPENSION_REPLAY.md)       | Suspension, cancellation, replay response, evidence preservation, and recovery procedure         |
| [runbooks/X402_TESTNET_RELEASE.md](runbooks/X402_TESTNET_RELEASE.md)                   | Safe evidence procedure for REL-003, REL-004, REL-005, and REL-008                               |
| [runbooks/SELLER_PACKAGE_RELEASE.md](runbooks/SELLER_PACKAGE_RELEASE.md)               | Build, verify, distribute, and install the private seller connector and merchant SDK artifacts   |
| [runbooks/LAUNCH_ENTITLEMENT.md](runbooks/LAUNCH_ENTITLEMENT.md)                       | Dry-run-first operator workflow while Stripe subscription collection is disabled                 |
| [runbooks/CLEAN_SELLER_ONBOARDING_REHEARSAL.md](runbooks/CLEAN_SELLER_ONBOARDING_REHEARSAL.md) | Clean-account seller onboarding, MCP publication, signed discovery, and dashboard rehearsal      |
| [runbooks/WEBHOOK_CANARY.md](runbooks/WEBHOOK_CANARY.md)                               | Exact-body signature, retry, dead-letter, redelivery, and secret-rotation verification           |
| [runbooks/PUBLICATION_INVALIDATION_REHEARSAL.md](runbooks/PUBLICATION_INVALIDATION_REHEARSAL.md) | Durable publication outbox, stale-manifest, schema-refresh, and inactive-route rehearsal          |
| [DECISIONS.md](DECISIONS.md)                                                           | Locked decisions, assumptions, deferred choices, and change procedure                            |
| [SOURCES.md](SOURCES.md)                                                               | External protocol and platform sources that must be verified before implementation               |
| [api/openapi.yaml](api/openapi.yaml)                                                   | REST/HTTP API contract                                                                           |
| [api/asyncapi.yaml](api/asyncapi.yaml)                                                 | Deferred historical approval WebSocket contract; no Lean V1 runtime channel                      |
| [api/mcp.md](api/mcp.md)                                                               | Remote MCP transport, authentication, and resource contract                                      |
| [api/webhooks.md](api/webhooks.md)                                                     | Seller webhook event envelope and signature contract                                             |
| [api/receipts.md](api/receipts.md)                                                     | Versioned buyer and seller purchase receipt contract                                             |
| [SELLER_VERIFICATION.md](SELLER_VERIFICATION.md)                                       | Versioned seller-request verification package contract                                           |
| [MERCHANT_SDK.md](MERCHANT_SDK.md)                                                     | TypeScript merchant SDK, idempotent fulfillment, and reference adapter contract                   |
| [SETUP_BUNDLES.md](SETUP_BUNDLES.md)                                                   | Versioned coding-agent setup resources and prompt contract                                       |
| [SANDBOX_VALIDATION.md](SANDBOX_VALIDATION.md)                                         | Pre-publication seller integration validation contract                                           |
| [STOREFRONT_VALIDATION.md](STOREFRONT_VALIDATION.md)                                   | Deterministic SEO, AEO, discovery, accessibility, and performance checks                         |
| [INTEGRATION_RECIPES.md](INTEGRATION_RECIPES.md)                                       | Maintained stack recipes, middleware order, generated files, and fixture gates                   |
| [../scripts/flaws.md](../scripts/flaws.md)                                             | Consolidated launch gaps, subscription-enforcement design, and implementation reference          |
| [uml/system-context.puml](uml/system-context.puml)                                     | System context diagram                                                                           |
| [uml/containers.puml](uml/containers.puml)                                             | Runtime/container diagram                                                                        |
| [uml/purchase-sequence.puml](uml/purchase-sequence.puml)                               | Lean V1 purchase and x402 sequence                                                               |
| [uml/seller-integration-sequence.puml](uml/seller-integration-sequence.puml)           | Coding-agent seller integration and publication sequence                                         |
| [uml/browser-wallet-purchase-sequence.puml](uml/browser-wallet-purchase-sequence.puml) | Browser-wallet x402 purchase through the shared commerce pipeline                                |
| [uml/dispute-sequence.puml](uml/dispute-sequence.puml)                                 | Dispute sequence                                                                                 |

## Authority and conflict rules

1. OpenAPI and AsyncAPI files control public wire contracts.
2. `DATA_MODEL.md` controls persisted fields and state transitions.
3. `ARCHITECTURE.md` controls component ownership and security boundaries.
4. `PRODUCT.md` controls MVP product scope and user journeys but cannot override wire, persistence, or security contracts.
5. `IMPLEMENTATION.md` controls work order and milestone inclusion.
6. When a conflict remains, record and resolve it in `DECISIONS.md` before coding.

## Documentation status labels

- **Locked**: implementation may rely on this decision.
- **Assumption**: safe default for the hackathon; verify before production.
- **Deferred**: intentionally excluded from the current milestone.
- **Unverified**: must not be represented as working behavior.
