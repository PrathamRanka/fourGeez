# Test and release plan

Status: **Locked; the M7.1 local launch gate passes and M9 remains the deployed release gate**.

The M7.1 unit, contract, integration, browser E2E, responsive, and Lighthouse
checks prove the local launch core. They do not prove AWS production readiness.
Redis/outbox-specific testing is deferred because Lean V1 performs
transaction-critical authorization against authoritative persistence. M9
remains the final deployed release gate.

## Test layers

### Unit tests

Run without AWS or network access.

- Money parsing/comparison with no floating-point conversion.
- Canonical JSON and request hashing fixtures.
- Purchase-intent validation and expiration.
- Buyer maximum boundary: quote below, equal to, and above `maximumAmount`.
- Browser purchase commerce/access expiry boundaries, reload persistence,
  single-intent consumption, payer-wallet binding, one-time recovery challenge,
  and recovery that cannot restore payment authority.
- Every legal and illegal transaction state transition.
- Evidence chain construction and tamper detection.
- Every dispute rule and fallback to seller review.
- Manual refund recording: seller ownership concealment, finalized-payment and
  `refund_recommended` preconditions, exact amount/asset/network matching,
  bounded reference validation, one record per dispute, exact idempotent replay,
  and changed replay conflict.
- Header and log redaction.
- URL and IP SSRF rejection.
- Payment-destination challenge expiry, invalid signatures, replay, activation,
  disabling, and rotation.
- Asset/network-separated aggregate updates and duplicate-event rejection.
- Webhook signature, retry, dead-letter, redelivery, and SSRF behavior.
- Plan quota boundaries and immutable usage-meter events.
- Seller entitlement transitions at one nanosecond before, exactly at, and
  after `accessEndsAt`; grace is always recovery/read-only and expires after
  exactly 72 hours.
- Stripe webhook raw-body signature verification, event-ID deduplication,
  conflicting duplicate quarantine, out-of-order delivery reconciliation,
  cancellation-at-period-end, failed renewal, and missed-event sweeps.
- Reactivation increments the entitlement epoch and requires project-key
  rotation; fraud quarantine cannot be cleared by a Stripe payment event.
- Atomic UTC-month API and MCP counters, retry-safe webhook-delivery claims,
  static route/subscription limits, suspended plans, and stable 403/429 codes.
- MCP confirmation grants bind seller, credential, tool, target, canonical
  arguments hash, expected resource version, and exclusive five-minute expiry;
  wrong bindings, expiry, revocation, prior consumption, and caller-asserted
  confirmation fields fail closed.
- Audit action vocabulary, changed-field allowlists, append-only persistence,
  tenant-bound cursors, and seller authorization.
- SEO/AEO validation for canonical URLs, structured data, visible-content
  consistency, sitemap, robots directives, `llms.txt`, and manifest output.
- Storefront validation fails each named check independently and enforces the
  documented HTML, JavaScript, CSS, blocking-script, and LCP budgets.

### API contract tests

- Validate `openapi.yaml` with Redocly.
- Validate `asyncapi.yaml` with the AsyncAPI CLI.
- For every OpenAPI operation, test one success and every documented error class.
- Reject undocumented fields on control-plane JSON requests.
- Verify required authentication and idempotency headers.
- Snapshot machine error codes, not prose-only messages.
- Validate receipt schemas 1 and 2 independently and reject a version-1
  document containing required version-2 semantics.
- Assert every authenticated commerce operation documents its applicable 401,
  403, 404, 409, 410, 422, 429, and 503 outcomes.
- Validate the manual refund-record route requires `sellerBearer` and
  `Idempotency-Key`, rejects unknown request fields, returns the same `201`
  body for an exact replay, and returns the documented 404/409/422 taxonomy.
- Validate MCP protected-resource metadata and the `WWW-Authenticate`
  resource-metadata challenge independently from the proprietary project-key
  bootstrap endpoint.
- Validate the seller-session-only confirmation-grant endpoint, its strict
  request schema and `Cache-Control: no-store`, and prove project-key and MCP
  bearer authentication cannot call it.

### Integration tests

Run against local in-memory repositories first, then DynamoDB/S3/KMS in a disposable AWS development environment.

- Idempotency records return the original result for the same request hash and reject key reuse with a different hash.
- Credential rotation retries recover the same KMS envelope-encrypted response
  during the replay window; after expiry they identify the committed successor
  without reviving the predecessor.
- DynamoDB conditional writes permit one forwarding claimant only when status
  is `PAYMENT_VERIFIED` and `paymentFinality=finalized`.
- S3 event objects are append-only and verify against KMS signatures.
- Facilitator timeout and rejection never call the seller.
- Seller timeout records delivery failure.
- MCP credentials cannot cross seller boundaries or exceed their scopes.
- MCP mutation retries return the original result and do not duplicate products.
- MCP confirmation consumption is atomic with mutation idempotency: an exact
  retry returns the stored redacted result, while changed arguments, target,
  credential, seller, tool, resource version, expiry, or second use is denied.
- A deliberately modified connector that self-asserts approval or removes local
  checks cannot mint confirmation grants, publish, verify payments, create
  official transactions, or obtain execution capabilities.
- Generated seller middleware accepts valid AgentPay signatures and rejects modified bodies, stale timestamps, and replayed transaction identifiers.
- Payment reconciliation never reports unconfirmed value as finalized and remains idempotent across repeated provider observations.
- Dashboard aggregate projection retries cannot double count a transaction.
- Seller webhooks preserve event identity across retries and never block the authoritative transaction write.
- Every advertised stack fixture installs verification, preserves raw request bytes, exposes the sandbox route, and produces valid discovery metadata.
- Stack detection fixtures cover exact dependency evidence, metaframework
  precedence, malformed manifests, bounded input, and explicit unsupported
  ecosystem labels.
- JavaScript and TypeScript recipe fixtures cover Next.js, React/Vite, Remix,
  Nuxt, SvelteKit, Astro, Express, Fastify, and NestJS, including raw-body
  strategy, verification adapter, middleware order, sandbox route, discovery
  files, and focused test commands.
- Go and Python recipe fixtures cover `net/http`, Gin, Echo, Fiber, FastAPI,
  Starlette, Flask, and Django. Python verification tests cover both ASGI and
  synchronous WSGI replay-store paths.
- Extended recipe fixtures cover ASP.NET Core, Spring Boot, Rails, and Laravel.
  Their package tests run under .NET 8, Java 17 or newer, Ruby 3.3, and PHP 8.3
  and cover valid signatures, modified bodies, stale timestamps, replay, and
  weak signing secrets. `npm run test:verification:extended` is the shared
  local and CI entry point.

### End-to-end tests

1. Discover a storefront, create an intent whose fixed quote is within the
   buyer maximum, receive 402, authorize the exact wallet payment, and receive
   the seller response without any buyer-approval step.
2. Submit a quote above `maximumAmount` and prove no payment challenge is issued.
3. Replay the paid request and prove the seller was invoked once.
4. Raise `not_delivered` after a seller timeout and receive `refund_recommended`.
5. As the owning seller, record the exact finalized refund reference, replay it
   idempotently, and prove changed input, cross-seller access, unfinalized
   payment, and a non-recommended dispute fail with the documented taxonomy.
6. Raise `quality_or_output` and receive `seller_review`.
7. Disable Bedrock and complete the purchase through deterministic fallback.
8. Connect a supported coding agent, generate a seller integration, review the diff, explicitly approve publication, and pass the sandbox validator.
9. Buy the same published product through a browser wallet and an agent/x402
    flow and verify both sales appear once in the seller dashboard.
10. Rotate the seller payment destination and prove existing intents retain the
    old frozen destination while new intents use the verified replacement.
11. Disable seller webhooks, complete a purchase, and prove the dashboard and
    evidence remain authoritative.
12. Generate a supported-stack storefront and validate canonical metadata,
    structured data, sitemap, robots, manifest, and `llms.txt` consistency.
13. Reload during browser checkout, complete payment, later recover receipt and
    dispute access with the finalized payer wallet, and prove recovery cannot
    create a second intent or payment.
14. Leave the official connector and a modified fork running, revoke or expire
    the seller entitlement, and prove both lose MCP, publication, discovery,
    intent, x402, and transaction authority while historical access follows the
    subscription contract.

Historical M2 approval tests remain regression coverage for dormant code only.
They are not Lean V1 acceptance tests, no approval server is started, and no
checkout may depend on them.

## Web quality checks

- Public landing content explains the seller setup and buyer payment paths
  without requiring knowledge of MCP or x402 terminology.
- Landing-page sign-up, sign-in, documentation, seller, and buyer navigation
  targets are keyboard reachable and resolve to real routes or explicit
  unavailable states.
- Marketing motion is progressive, uses transform or opacity where possible,
  and is removed when `prefers-reduced-motion` is enabled.
- Keyboard-only operation for onboarding, buyer checkout, and dispute flows.
- Visible focus and correctly associated labels/errors.
- Payment and fulfillment status announced through an ARIA live region.
- Reduced-motion mode removes nonessential transitions.
- Layout checks at 360, 768, 1280, and 1440 pixel widths.
- No horizontal page overflow.
- Public storefront product and browser-wallet purchase pages remain usable without agent tooling.
- Seller automation screens clearly distinguish proposed, validated, published, and failed integration states.
- Dashboard totals label asset and network and never display one combined total for unlike currencies.
- Lighthouse targets on the deployed demo: accessibility at least 95; best practices at least 90.

## Performance and resilience targets

These are hackathon engineering targets, not customer SLAs:

- Health and storefront p95 under 300 ms excluding cold starts.
- Control API p95 under 700 ms excluding Bedrock.
- Payment verification timeout at 8 seconds.
- Seller upstream timeout configurable from 1–30 seconds; demo default 20 seconds.
- Load test proves 20 concurrent purchase attempts without duplicate seller invocation.

## Required fixtures

- Seller with active and inactive fixed-price routes.
- Valid and invalid agent API keys.
- Mock facilitator outcomes: success, rejection, timeout, duplicate proof.
- Mock seller outcomes: 200, 400, 500, timeout, oversized response.
- Fixed canonical-hash and evidence-signature golden fixtures.

## CI gates

Every pull request and implementation commit must pass relevant checks:

```text
Go formatting, vet, unit tests, race tests
Web lint, typecheck, unit tests, production build
OpenAPI and AsyncAPI validation
Terraform layout tests, formatting, and validation
Reviewed Terraform plan for environment deployment tasks
Secret scanning
Dependency audit
```

End-to-end testnet payment is a demo-release gate, not a per-commit gate.

## Demo release checklist

- [ ] Clean environment deployment succeeds from documented commands.
- [ ] No undocumented console changes are required.
- [ ] All unit, contract, integration, accessibility, and E2E tests pass.
- [ ] One real testnet transaction has a valid evidence chain.
- [ ] Duplicate forwarding test shows one seller invocation.
- [ ] Manual refund records are visibly labeled as seller-reported and never as AgentPay-executed refunds.
- [ ] Bedrock fallback is tested immediately before presentation.
- [ ] Coding-agent setup produces a reviewable diff and cannot publish without confirmation.
- [ ] Human and agent purchases appear in one seller transaction history.
- [ ] Three consecutive three-minute rehearsals succeed without data repair.
