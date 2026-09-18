# Documentation index

This directory is the implementation source of truth for AgentPay. The older `*_FINAL.md` files remain useful product inputs, but this documentation resolves their conflicts and separates hackathon scope from post-hackathon scope.

## Current delivery status

Milestones M0 through M7 are implemented as a development preview. Milestone
M7.1 is the mandatory launch-hardening program for production authentication,
subscription enforcement, cloud-authoritative MCP access, complete frontend and
backend integration, buyer checkout, operational readiness, and E2E coverage.
M8 infrastructure and M9 release verification have not started. Until those
milestones pass, AgentPay supports local mock and x402 testnet use only and must
not be represented as a production-ready paid service.

OpenAPI 0.4 and the LCH-004 companion contracts are the locked M7.1 production
target, not a claim about the current binary. They are marked target-state and
must be enabled only after their dependent implementation and migration tests
pass. The current Go runtime remains a development-only M7 compatibility
baseline:

| Surface | Current M7 development behavior | Required M7.1 production target |
|---|---|---|
| Seller project credential | `apc1` credential used directly by MCP | `apc2` bootstrap through `/v1/integration-access-tokens`; connector or OAuth access token at `/mcp` |
| Approval invitation | Query token | Fragment exchange into multi-session HttpOnly grant plus CSRF |
| Browser purchase | Buyer-agent key only | Durable opaque purchase cookie, bounded commerce window, and payer-wallet recovery |
| Seller forwarding | Per-seller HMAC | Finality-gated ES256 execution capability |
| Seller entitlement | `active`/`suspended` plan projection | Full entitlement projection and epoch checks |
| Receipts and invoice exports | Schema version 1 | Version-aware readers with launch writers on schema version 2 |

No production environment may expose a mixed mode in which a target endpoint
accepts a legacy credential or a legacy endpoint bypasses target authorization.
The complete gap analysis and reference security design are recorded in
[`../scripts/flaws.md`](../scripts/flaws.md).

## Authoritative documents

| Document | Purpose |
|---|---|
| [PRODUCT.md](PRODUCT.md) | Product scope, seller automation, buyer channels, and revenue boundaries |
| [IMPLEMENTATION.md](IMPLEMENTATION.md) | Ordered task ledger, dependencies, acceptance criteria, and milestone status |
| [ARCHITECTURE.md](ARCHITECTURE.md) | Components, boundaries, request flow, deployment shape, and failure behavior |
| [DATA_MODEL.md](DATA_MODEL.md) | Entities, state machines, identifiers, indexes, and retention |
| [AWS_SETUP.md](AWS_SETUP.md) | AWS accounts, services, IAM, deployment order, configuration, and teardown |
| [SECURITY.md](SECURITY.md) | Threat model, secrets, evidence integrity, privacy, and production gates |
| [TEST_PLAN.md](TEST_PLAN.md) | Test levels, required scenarios, fixtures, and release gates |
| [DECISIONS.md](DECISIONS.md) | Locked decisions, assumptions, deferred choices, and change procedure |
| [SOURCES.md](SOURCES.md) | External protocol and platform sources that must be verified before implementation |
| [api/openapi.yaml](api/openapi.yaml) | REST/HTTP API contract |
| [api/asyncapi.yaml](api/asyncapi.yaml) | WebSocket event contract |
| [api/mcp.md](api/mcp.md) | Remote MCP transport, authentication, and resource contract |
| [api/webhooks.md](api/webhooks.md) | Seller webhook event envelope and signature contract |
| [api/receipts.md](api/receipts.md) | Versioned buyer and seller purchase receipt contract |
| [SELLER_VERIFICATION.md](SELLER_VERIFICATION.md) | Versioned seller-request verification package contract |
| [SETUP_BUNDLES.md](SETUP_BUNDLES.md) | Versioned coding-agent setup resources and prompt contract |
| [SANDBOX_VALIDATION.md](SANDBOX_VALIDATION.md) | Pre-publication seller integration validation contract |
| [STOREFRONT_VALIDATION.md](STOREFRONT_VALIDATION.md) | Deterministic SEO, AEO, discovery, accessibility, and performance checks |
| [INTEGRATION_RECIPES.md](INTEGRATION_RECIPES.md) | Maintained stack recipes, middleware order, generated files, and fixture gates |
| [../scripts/flaws.md](../scripts/flaws.md) | Consolidated launch gaps, subscription-enforcement design, and implementation reference |
| [uml/system-context.puml](uml/system-context.puml) | System context diagram |
| [uml/containers.puml](uml/containers.puml) | Runtime/container diagram |
| [uml/purchase-sequence.puml](uml/purchase-sequence.puml) | Purchase and approval sequence |
| [uml/seller-integration-sequence.puml](uml/seller-integration-sequence.puml) | Coding-agent seller integration and publication sequence |
| [uml/browser-wallet-purchase-sequence.puml](uml/browser-wallet-purchase-sequence.puml) | Browser-wallet x402 purchase through the shared commerce pipeline |
| [uml/dispute-sequence.puml](uml/dispute-sequence.puml) | Dispute sequence |

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
