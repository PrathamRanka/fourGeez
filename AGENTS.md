# AgentPay engineering contract

Last updated: 2026-09-17.

This file applies to the entire repository. Every human or coding agent must read it before changing code. More specific `AGENTS.md` files may add stricter rules inside a subdirectory, but may not weaken these rules.

## 1. Required context before every task

Before editing anything:

1. Read this file completely.
2. Read `docs/README.md` and the relevant sections of `docs/IMPLEMENTATION.md`.
3. Read the controlling contract: `docs/api/openapi.yaml`, `docs/api/asyncapi.yaml`, `docs/DATA_MODEL.md`, or the relevant runbook.
4. Inspect the existing implementation and tests. Never guess that a file, field, endpoint, table, or component exists.
5. Check `git status --short` and preserve unrelated work.
6. Work on one numbered task from `docs/IMPLEMENTATION.md` at a time.

If documents conflict, stop implementation and resolve the documentation first. OpenAPI/AsyncAPI control wire contracts, `DATA_MODEL.md` controls persisted data and state transitions, and `IMPLEMENTATION.md` controls scope and order.

## 2. Universal implementation rules

- Follow test-driven development: write a failing test, run it, add the minimum implementation, rerun tests, then refactor.
- Every bug fix begins with a regression test that demonstrates the failure.
- Do not mark a task complete until its acceptance tests pass.
- Follow SOLID where it improves an existing boundary. SOLID does not justify interfaces, factories, wrappers, managers, or layers without a concrete need.
- Do not create speculative abstractions. An abstraction requires either two real consumers/implementations or a documented external boundary such as storage, payments, signing, or model invocation.
- Prefer a stable, maintained package over custom infrastructure for a solved problem. Check maintenance, license, security history, framework compatibility, and bundle/runtime cost before adding it.
- Do not add a package for a trivial function that is clearer and safer in local code.
- Pin dependency versions. Update the lockfile in the same commit.
- Do not add fallback behavior that silently changes money, policy, security, identity, or database semantics.
- Do not implement undocumented behavior. Update the controlling document first when the behavior must change.
- Keep one logical change per commit and use Conventional Commits.

## 3. Naming and code clarity

- Use domain names from the API and data model exactly: `purchaseIntent`, `approvalSession`, `paymentIdentifier`, `evidenceEvent`, and `transaction`.
- Do not use vague names such as `data`, `item`, `obj`, `thing`, `temp`, `foo`, `bar`, `manager`, `processor`, `handlerUtil`, or `common` when a domain name exists.
- Single-letter variables are allowed only for conventional short loop indexes or mathematical expressions whose meaning is obvious.
- Do not rename database or API fields inside application layers without an explicit mapping type and reason.
- No magic amounts, durations, statuses, table keys, header names, or policy values. Use named constants close to the owning domain.
- Store and compare money using atomic-unit strings or validated integers. Never use floating point for money.
- Use UTC timestamps and RFC 3339 at system boundaries.
- Comments explain why a decision exists, not what obvious code syntax does.
- Delete dead code instead of commenting it out.

## 4. Frontend rules

The web application is Next.js, React, TypeScript, and Tailwind.

### Component priority

Before writing a UI component, search these sources in order:

1. Aceternity UI.
2. 21st.dev component registry.
3. shadcn/ui.
4. A stable npm package for specialized behavior not covered above, such as complex charts, schema validation, or accessible interaction primitives.
5. Custom implementation only when no suitable maintained component exists.

Use the official installation or registry command and inspect the generated source and dependencies before committing. Components copied from a registry become repository code and must pass our accessibility, security, lint, type, and responsive checks.

### No unnecessary frontend abstractions

- Use selected registry components directly in feature pages/components.
- Do not wrap buttons, cards, dialogs, inputs, tables, or layout primitives merely to rename props or re-export them.
- Create a product-specific wrapper only when it adds real repeated AgentPay behavior, such as transaction-state rendering or evidence verification.
- Do not create generic `BaseComponent`, `ComponentFactory`, `UIManager`, `utils.ts`, or barrel-export layers.
- Do not duplicate a component that already exists in the selected registry or repository.
- Do not introduce global state for data owned by one route or component.
- Derive values during render when possible; do not synchronize derived state through effects.

### Next.js implementation

- Use Server Components by default.
- Add `"use client"` only for browser APIs, stateful interaction, or live subscriptions.
- Keep client-component props small and serializable.
- Start independent server requests together and await them in parallel.
- Dynamically load genuinely heavy optional UI.
- Import modules directly; avoid broad barrel imports.
- Never expose server secrets through `NEXT_PUBLIC_*`, serialized props, logs, or browser bundles.

### Frontend quality gates

- Every interactive component must work with keyboard-only navigation.
- Use visible focus, semantic HTML, labels, error descriptions, and appropriate ARIA announcements.
- Support reduced motion.
- Implement loading, empty, retryable error, terminal error, disabled, and permission-denied states where applicable.
- Verify layouts at 360, 768, 1280, and 1440 pixel widths.
- Use the project design tokens; do not add arbitrary one-off colors, shadows, radii, or spacing values when a token exists.
- Tests must cover user-visible behavior, not private component implementation.

## 5. Backend rules

The backend is a Go modular monolith. Package boundaries are defined in `docs/ARCHITECTURE.md`.

### Test-first backend workflow

For each behavior:

1. Add a focused table-driven unit test.
2. Run it and confirm the expected failure.
3. Implement the smallest domain behavior that passes.
4. Add repository/transport integration tests only after domain behavior passes.
5. Run formatting, vet, race tests, and affected integration tests.

Do not write the HTTP handler first and add domain tests afterward.

### Backend design

- Domain rules belong in domain packages, not HTTP handlers, Lambda adapters, DynamoDB repositories, or Bedrock prompts.
- HTTP handlers validate transport input, invoke one use case, and translate the result.
- Return typed domain errors and map them to documented API error codes at the transport boundary.
- Use interfaces only at real external boundaries: persistence, clocks, ID generation, payment adapters, signing, seller forwarding, and model invocation.
- Keep interfaces small and owned by the package that consumes them.
- Do not create service/repository/controller layers automatically for every entity.
- No package may write another package's DynamoDB records directly.
- Bedrock output is untrusted input. Revalidate IDs, prices, limits, route eligibility, and policy before execution.
- Model code never receives wallet keys, seller secrets, approval tokens, or direct payment capability.

### Payment and transaction safety

- The seller's configured/frozen quote is authoritative. A buyer maximum is a ceiling, not the amount to charge.
- Exact-price payment must reject underpayment and overpayment.
- Every mutation requires idempotency where specified by OpenAPI.
- Payment identifiers are unique and protected by conditional writes.
- Exactly one caller may claim a transaction for seller forwarding.
- Never store raw payment signatures/proofs when a non-sensitive hash and provider reference are sufficient.
- Never call the seller before required payment and evidence preconditions succeed.

## 6. Database rules

- `docs/DATA_MODEL.md` is the source of truth for entities, key formats, indexes, states, and transitions.
- Do not invent a field, status, table, index, or access pattern only inside code.
- Update the data model and migration/compatibility notes before changing persistence.
- Use the documented DynamoDB `PK`, `SK`, and GSI forms exactly.
- Validate state transitions in the domain layer and enforce concurrency with DynamoDB conditions.
- Keep immutable purchase intents immutable.
- Bind approvals to the complete intent hash.
- Store append-only evidence events; never update an existing evidence event.
- Persist schema/version fields where replay or historical interpretation depends on them.
- No production code may scan a DynamoDB table to implement a normal request path.
- Tests must use the same key-building functions as production code and cover conditional-write failure.

## 7. RL and recommendation rules

- The `rl/` workspace ranks already-valid candidate offers only.
- RL never authorizes purchases, modifies limits, chooses whether approval is required, changes trust tiers, executes payments, or resolves disputes.
- Implement strict versioned contracts and a deterministic baseline before any bandit/model.
- Train and evaluate offline first with reproducible seeds.
- Compare every model against the deterministic baseline.
- Label synthetic datasets and results clearly; never present synthetic improvement as real customer performance.
- Production transaction data, prompts, wallet details, personal data, and payment proofs must not be committed.
- The Go backend revalidates every recommendation before using it.

## 8. Infrastructure rules

- Terraform is the chosen infrastructure-as-code direction.
- Do not add new CDK resources. The existing CDK placeholder remains only until the documented Terraform migration task removes it.
- Do not manage the same AWS resource with both Terraform and CDK.
- All AWS resources must be reproducible from committed Terraform; avoid undocumented console configuration.
- Run `terraform fmt`, `terraform validate`, and a reviewed `terraform plan` before apply.
- Use remote encrypted/versioned state and state locking per environment.
- Authenticate CI to AWS with short-lived OIDC credentials; never commit or store long-lived AWS access keys.
- Do not put secret values into Terraform variables or outputs when they would be persisted in state. Create secret containers with Terraform and inject values separately.
- Use separate `dev`, `demo`, and future `prod` state/configuration.
- Apply least-privilege IAM and explicit resource scoping.
- Evidence storage must retain versioning, Object Lock, encryption, and deletion protection requirements.

## 9. Security and data handling

- Follow `docs/SECURITY.md` for every task.
- Never log authorization headers, cookies, payment signatures, invitation tokens, approval tokens, wallet material, or seller secrets.
- Reject unknown JSON fields on control-plane APIs.
- Enforce body-size, timeout, response-size, and candidate-count limits.
- Protect the seller proxy against private, loopback, link-local, metadata, redirect, IPv6, and DNS-rebinding SSRF paths.
- Store only allowlisted evidence metadata and hashes; do not store unrestricted seller responses.
- Use constant-time comparisons for tokens and HMACs.
- Never weaken validation to make a test or demo pass.

## 10. Required checks and commits

Before committing, run all checks affected by the task. The full baseline is:

```powershell
npm run lint
npm run typecheck
npm test
npm run build
```

Run Terraform checks after the Terraform migration exists. Run Python/RL checks from `rl/` when that workspace changes.

Commit rules:

- One completed task or one tightly related fix per commit.
- Use Conventional Commits, for example `feat(api): add immutable purchase intents`.
- Include tests, implementation, contract updates, and task-status updates in the same logical commit.
- Do not commit generated caches, build output, local state, secrets, or `.env` files.
- Push only when requested or when the active workflow explicitly requires pushing.

## 11. Definition of done

A task is complete only when:

- Its documented acceptance criteria are satisfied.
- Tests were written first and now pass.
- Existing relevant tests pass.
- Public contracts and data documentation match implementation.
- Security and failure paths are tested.
- No unnecessary abstraction or dependency was introduced.
- The implementation task is marked complete only after verification.
- The work is committed with a descriptive message.
