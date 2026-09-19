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

- Product and technical contracts through M7: implemented and covered by the
  existing unit and contract suites.
- Public site, seller workflows, storefronts, buyer demonstration, approvals,
  transactions, evidence, analytics, webhooks, and disputes: implemented as a
  development preview.
- M7.1 launch hardening: not started. Production authentication, subscription
  enforcement, short-lived MCP capabilities, integrated local runtime, complete
  browser checkout, operator tooling, and full-system E2E verification remain
  required.
- AWS deployment and Cognito configuration in M8: not started.
- Release gates in M9: not started.
- Payment scope: mock and x402 testnet only until every production gate passes.

The current development preview is not a production-ready paid service. Use
[`docs/IMPLEMENTATION.md`](docs/IMPLEMENTATION.md) for authoritative task
status and [`scripts/flaws.md`](scripts/flaws.md) for the consolidated launch
gap and security analysis.

## Working rules

1. Select the next unblocked task from `docs/IMPLEMENTATION.md`.
2. Implement only behavior defined in the documentation or record a decision first.
3. Add or update tests and documentation in the same change.
4. Commit one logical change at a time using the documented commit convention.

## Ownership and license

AgentPay is proprietary source, not an open-source project. Copyright (c) 2026
Pratham Ranka and Ayush Garg in their respective AgentPay-authored materials;
all rights are reserved. Third-party components and independently authored
contributions remain subject to their own rights and licenses.

Public visibility permits access and GitHub-hosted forking under GitHub's Terms
of Service, but grants no broader permission to use, modify, redistribute, or
commercialize AgentPay. A public repository cannot be made technically
uncloneable. See [LICENSE](LICENSE), [NOTICE](NOTICE),
[CONTRIBUTING.md](CONTRIBUTING.md), and
[repository governance](docs/REPOSITORY_GOVERNANCE.md).
