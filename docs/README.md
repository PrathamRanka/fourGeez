# Documentation index

This directory is the implementation source of truth for AgentPay. The older `*_FINAL.md` files remain useful product inputs, but this documentation resolves their conflicts and separates hackathon scope from post-hackathon scope.

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
| [uml/system-context.puml](uml/system-context.puml) | System context diagram |
| [uml/containers.puml](uml/containers.puml) | Runtime/container diagram |
| [uml/purchase-sequence.puml](uml/purchase-sequence.puml) | Purchase and approval sequence |
| [uml/seller-integration-sequence.puml](uml/seller-integration-sequence.puml) | Coding-agent seller integration and publication sequence |
| [uml/human-purchase-sequence.puml](uml/human-purchase-sequence.puml) | Human checkout through the shared commerce pipeline |
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
