# Test and release plan

Status: **Locked for the current backend and planned MVP release**.

## Test layers

### Unit tests

Run without AWS or network access.

- Money parsing/comparison with no floating-point conversion.
- Canonical JSON and request hashing fixtures.
- Purchase-intent validation and expiration.
- Policy threshold boundary: below, equal, and above.
- Approval approve/approve, approve/veto, duplicate decision, wrong invitation, expiration, and changed intent.
- Every legal and illegal transaction state transition.
- Evidence chain construction and tamper detection.
- Every dispute rule and fallback to seller review.
- Header and log redaction.
- URL and IP SSRF rejection.

### API contract tests

- Validate `openapi.yaml` with Redocly.
- Validate `asyncapi.yaml` with the AsyncAPI CLI.
- For every OpenAPI operation, test one success and every documented error class.
- Reject undocumented fields on control-plane JSON requests.
- Verify required authentication and idempotency headers.
- Snapshot machine error codes, not prose-only messages.

### Integration tests

Run against local in-memory repositories first, then DynamoDB/S3/KMS in a disposable AWS development environment.

- Idempotency records return the original result for the same request hash and reject key reuse with a different hash.
- DynamoDB conditional writes permit one forwarding claimant.
- S3 event objects are append-only and verify against KMS signatures.
- WebSocket reconnect receives a complete session snapshot.
- Facilitator timeout and rejection never call the seller.
- Seller timeout records delivery failure.
- MCP credentials cannot cross seller boundaries or exceed their scopes.
- MCP mutation retries return the original result and do not duplicate products.
- Generated seller middleware accepts valid AgentPay signatures and rejects modified bodies, stale timestamps, and replayed transaction identifiers.
- Authenticated human-checkout callbacks and x402 verification create the same normalized transaction outcome.

### End-to-end tests

1. Discover storefront, create below-threshold intent, receive 402, pay on testnet, and receive seller response.
2. Create above-threshold intent, approve from two browser contexts, then pay and receive seller response.
3. Veto from one browser and prove no payment challenge is issued.
4. Expire a session and prove old links and tokens fail.
5. Replay the paid request and prove the seller was invoked once.
6. Raise `not_delivered` dispute after a seller timeout and receive `refund_recommended`.
7. Raise `quality_or_output` dispute and receive `seller_review`.
8. Disable Bedrock and complete the purchase through deterministic fallback.
9. Connect a supported coding agent, generate a seller integration, review the diff, explicitly approve publication, and pass the sandbox validator.
10. Buy the same published product through the human storefront and agent/x402 flow and verify both sales appear in the seller dashboard.

## Web quality checks

- Keyboard-only operation for onboarding, buyer, approval, and dispute flows.
- Visible focus and correctly associated labels/errors.
- Approval status announced through an ARIA live region.
- Reduced-motion mode removes nonessential transitions.
- Layout checks at 360, 768, 1280, and 1440 pixel widths.
- No horizontal page overflow.
- Human storefront product and checkout pages remain usable without agent tooling.
- Seller automation screens clearly distinguish proposed, validated, published, and failed integration states.
- Lighthouse targets on the deployed demo: accessibility at least 95; best practices at least 90.

## Performance and resilience targets

These are hackathon engineering targets, not customer SLAs:

- Health and storefront p95 under 300 ms excluding cold starts.
- Control API p95 under 700 ms excluding Bedrock.
- Payment verification timeout at 8 seconds.
- Seller upstream timeout configurable from 1–30 seconds; demo default 20 seconds.
- Browser receives approval update within 2 seconds under normal demo conditions.
- Load test proves 20 concurrent purchase attempts without duplicate seller invocation.

## Required fixtures

- Seller with below-threshold and approval-required routes.
- Valid and invalid agent API keys.
- Two approval invitations.
- Mock facilitator outcomes: success, rejection, timeout, duplicate proof.
- Mock seller outcomes: 200, 400, 500, timeout, oversized response.
- Fixed canonical-hash and evidence-signature golden fixtures.

## CI gates

Every pull request and implementation commit must pass relevant checks:

```text
Go formatting, vet, unit tests, race tests
Web lint, typecheck, unit tests, production build
OpenAPI and AsyncAPI validation
CDK synth until AWS-000 removes the placeholder
Terraform formatting, validation, and reviewed plan after AWS-000
Secret scanning
Dependency audit
```

End-to-end testnet payment is a demo-release gate, not a per-commit gate.

## Demo release checklist

- [ ] Clean environment deployment succeeds from documented commands.
- [ ] No undocumented console changes are required.
- [ ] All unit, contract, integration, accessibility, and E2E tests pass.
- [ ] One real testnet transaction has a valid evidence chain.
- [ ] Two-device approval works after reconnecting either client.
- [ ] Duplicate forwarding test shows one seller invocation.
- [ ] Simulated refund is visibly labeled as simulated.
- [ ] Bedrock fallback is tested immediately before presentation.
- [ ] Coding-agent setup produces a reviewable diff and cannot publish without confirmation.
- [ ] Human and agent purchases appear in one seller transaction history.
- [ ] Three consecutive three-minute rehearsals succeed without data repair.
