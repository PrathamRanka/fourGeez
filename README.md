# fourGeez

fourGeez is the repository for **AgentPay** (working name), a seller-side commerce gateway for AI agents. It exposes paid API routes through x402-compatible challenges and adds purchase policy, multi-party approval, evidence capture, and dispute workflows.

The repository is documentation-first. Implementation must follow the numbered tasks and accepted contracts instead of inventing behavior while coding.

## Start here

- [Documentation index](docs/README.md)
- [Task-by-task implementation plan](docs/IMPLEMENTATION.md)
- [System architecture](docs/ARCHITECTURE.md)
- [API contract](docs/api/openapi.yaml)
- [AWS setup runbook](docs/AWS_SETUP.md)

## Current status

- Product and technical planning: complete for the hackathon milestone
- Repository foundations (M1): complete
- API implementation: not started
- Web application: not started
- AWS deployment: not started
- Real-money processing: explicitly out of scope for the first milestone

## Working rules

1. Select the next unblocked task from `docs/IMPLEMENTATION.md`.
2. Implement only behavior defined in the documentation or record a decision first.
3. Add or update tests and documentation in the same change.
4. Commit one logical change at a time using the documented commit convention.
