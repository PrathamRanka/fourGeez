# fourGeez

fourGeez is the repository for **AgentPay**, an automated commerce gateway for sellers of APIs and digitally fulfilled products. It prepares seller integrations through MCP-enabled coding agents and serves the same products to human storefront buyers and autonomous agents.

The repository is documentation-first. Implementation must follow the numbered tasks and accepted contracts instead of inventing behavior while coding.

## Start here

- [Documentation index](docs/README.md)
- [Product contract](docs/PRODUCT.md)
- [Task-by-task implementation plan](docs/IMPLEMENTATION.md)
- [System architecture](docs/ARCHITECTURE.md)
- [API contract](docs/api/openapi.yaml)
- [AWS setup runbook](docs/AWS_SETUP.md)
- [Recommendation/RL teammate workspace](rl/README.md)

## Current status

- Product and technical planning: updated for the dual-channel MVP
- Repository foundations through the buyer backend (M1-M5): complete
- Seller automation, web application, human checkout, and deployment: planned
- Web application: not started
- AWS deployment: not started
- Real-money processing: explicitly out of scope for the first milestone

## Working rules

1. Select the next unblocked task from `docs/IMPLEMENTATION.md`.
2. Implement only behavior defined in the documentation or record a decision first.
3. Add or update tests and documentation in the same change.
4. Commit one logical change at a time using the documented commit convention.
