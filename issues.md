# AgentPay V1 issue register

Last reviewed: 2026-09-20

This register records issues found during the real seller onboarding, MCP configuration, publication, browser checkout, Base Sepolia x402 settlement, fulfillment, receipt, and AWS verification flow.

## Locked product decisions

- Sellers must never see or operate buyer checkout, including during onboarding or testing.
- Sellers configure and publish products, receive funds, and monitor transactions.
- Buyer checkout belongs only to human buyers and external buyer agents.
- Seller integration tests must run automatically and report pass/fail without requiring the seller to connect or authorize a buyer wallet.
- AgentPay's own autonomous buyer runtime remains V2 scope. External x402 buyer agents are the autonomous buyer path supported by V1.

## 1. Seller account and onboarding flow

### P0

1. **Resolved (2026-09-20):** Seller onboarding and dashboard routes no longer expose buyer checkout, wallet authorization, or the seller-owned test-purchase action. Public buyer storefront and external buyer-agent checkout remain unchanged.
2. **Resolved (2026-09-20):** `sandbox_validate_route` now records a bounded,
   route-version-bound six-check result and onboarding consumes that
   authoritative result without creating commerce or invoking a paid route.
3. Manual launch entitlement provisioning is still required while Stripe billing is disabled.

### P1

4. The seller journey has not been repeated from a completely clean account after all production fixes.
   - A secret-safe production-shaped rehearsal harness and runbook are prepared
     for execution after issues 1-3 merge. The issue remains open until that
     deployed clean-account run passes and its sanitized evidence is reviewed.
5. **Resolved locally (2026-09-20):** Seller onboarding now states that the workspace configures the store and that buyers purchase elsewhere; no buyer checkout control was added.
6. **Resolved locally (2026-09-20):** The onboarding and dashboard entry surfaces now show publication readiness and integration health from the server-authored onboarding projection.
6a. **Resolved locally (2026-09-20):** The entry experience collapses the underlying security checkpoints into one five-stage founder launch rail while retaining detailed checks under the setup surface.
6b. **Resolved locally (2026-09-20):** The visible journey is connect service -> confirm payout -> review detected products -> publish -> monitor sales.
6c. **Resolved locally (2026-09-20):** The UI maps authoritative onboarding state and the bounded integration result to pass/fail health plus one precise next action, including the first failed verification check. Deployed verification has not been claimed or performed for this change.

## 2. MCP, SDK, and seller repository integration

### P0

7. **Resolved in repository (2026-09-20; publication pending owner approval):** `@agentpay/local-mcp-connector` and `@agentpay/merchant-sdk` now have a manual, environment-reviewed GitHub Release workflow for protected `agentpay-packages-v*` tags. It separates read-only build from release write permission and refuses to replace an existing release. This task did not publish externally; the first release remains blocked on the documented owner controls and legal approvals.
8. **Resolved (2026-09-20):** seller-facing onboarding and documentation no longer invoke an unpublished npm package or rely on a repository-vendored tarball. They launch the checksum-verified connector from a versioned user-local installation created from the downloaded release asset.
9. **Resolved in repository (2026-09-20; legal approval remains a release gate):** release generation produces deterministic tarballs, `SHA256SUMS`, and source-commit provenance; verification binds the bundle to an expected tag and full commit and fails on dirty or tampered artifacts; tests inspect packed contents and perform clean offline installs. Package licenses and notices no longer claim exclusive ownership by two named people and explicitly disclose the unresolved historical assignment review. GitHub immutable-release enablement, tag protection, repository visibility, customer-use terms, and contributor provenance remain release-owner prerequisites.

### P1

10. **Resolved (2026-09-20):** Public site, onboarding, documentation, setup
    bundles, metadata, package guidance, and examples now state that the
    seller's coding agent edits the repository using AgentPay MCP tools and
    guidance. AgentPay MCP is limited to bounded analysis, configuration,
    verification, and seller-confirmed cloud mutations; it does not write
    repository files.
11. **Resolved (2026-09-20):** The seller prototype now defaults to a
    seller-owned DynamoDB table for conditional execution/webhook replay claims
    and durable transaction-keyed fulfillment results. Replay records use TTL;
    fulfillment records deliberately do not. Process-local memory stores require
    the explicit `local-memory` development mode, and no AWS resources were
    created or changed.
12. **Resolved (2026-09-20):** A focused cloud-boundary regression now issues
    an MCP access token, proves it is usable, revokes the underlying project
    key, and proves the same still-unexpired token receives `401 token_revoked`
    before quota consumption or JSON-RPC dispatch. MCP authorization uses fresh
    authoritative credential state and fails closed when that state is
    unavailable.
13. **Resolved locally (2026-09-20):** Server-authoritative lifecycle tests now
    keep the official connector path and a deliberately token-reusing modified
    client active across both `suspended` and `cancelled` states. The final
    capability-signing boundary reloads current credential and entitlement
    state, fresh exchange returns `subscription_inactive`, and the next MCP
    authorization rejects the still-unexpired token. The official connector
    treats that `403` as terminal and does not expose the response body.

## 3. Product configuration and publication flow

### P0

14. **Resolved locally (2026-09-20):** Seller-hosted `llms.txt` is generated from the same persisted catalog revision used to create the current signed AgentPay manifest and product contracts.
15. **Resolved locally (2026-09-20):** Dashboard and MCP publication, price, pause, archive, and emergency-disable mutations refresh the durable publication snapshot; public responses require revalidation rather than retaining a stale framework cache.
16. **Resolved locally (2026-09-20):** AgentPay directory, signed manifest/product documents, and seller-hosted `llms.txt` consume one server-authoritative publication snapshot. Integration price staging auto-pauses a published route and removes it from discovery, while dashboard contract edits require the seller to pause first; either path requires validation plus explicit republication before the changed version is exposed.
16a. **Resolved locally (2026-09-20):** The dashboard controls every seller-editable product contract field and availability action; immutable product slugs and verified payment rail/destination fields remain read-only.
16b. **Resolved locally (2026-09-20):** Offline draft saves increment the authoritative route version, invalidate prior validation, and reuse the existing version/hash-bound publication path that refreshes discovery projections and signed public documents.
16c. **Resolved locally (2026-09-20):** The product workspace now enforces draft -> validate -> preview -> explicit approve-and-publish, and the API rejects edits while the buyer contract is live.

### P1

17. **Resolved locally (2026-09-20; deployment pending):** paid-route and entitlement versions now atomically write immutable DynamoDB publication outbox events. A tested consumer invalidates before signed refresh, retries incomplete work, and conditionally records completion. The stream trigger, Redis/CDN targets, credential-event coverage, live retry/DLQ redrive, and observability proof remain undeployed under AWS-012.
18. **Resolved locally (2026-09-20; deployed proof pending):** a regression retains a signed active manifest, cancels the seller, proves its route can no longer authorize commerce, and verifies discovery returns a higher-revision signed cancellation tombstone. Canonical deployed intent and browser-checkout evidence remains REL-013.
19. **Resolved locally (2026-09-20; deployed fixture proof pending):** signed storefront and product documents advance together after input/output schema changes, and all 21 maintained stack prompts require public product pages, structured data, sitemap, `llms.txt`, and manifest regeneration from the latest signed contract. Deployed generated-artifact verification remains REL-010.
20. **Resolved locally (2026-09-20; deployed demo proof pending):** the deterministic launch-ready profile now includes a fixed-price paused route with a stable identifier and regression coverage. The canonical deployed demo remains REL-001.

## 4. Buyer discovery and autonomous-agent flow

### P0

21. A real external buyer agent has not completed the production x402 purchase flow; only browser-wallet purchasing is proven.

### P1

22. Agent/x402 and browser-wallet purchases have not both been demonstrated for the same storefront and reconciled in the seller dashboard.
23. Supported buyer-agent compatibility is documented but not dynamically negotiated.
24. AgentPay-operated buyer execution, negotiation, A2A execution, and marketplace ranking remain deferred V2 capabilities and must not be advertised as active V1 behavior.

## 5. Buyer checkout UI and request validation

### P0

25. Checkout validates JSON syntax but does not validate the request against the published `inputSchema` before payment.
26. Required fields such as `productName` and `audience` can be omitted and discovered only after settlement.
27. Checkout incorrectly labels the request body as optional when the product schema requires input.
28. Raw JSON is unsuitable for normal human buyers; the UI should render schema-driven form fields.
29. Unknown properties, types, required fields, string limits, and other schema rules are not enforced before payment.

### P1

30. Buyer and seller wallet addresses are not clearly displayed before authorization.
31. MetaMask account labels can make different addresses appear identical, creating avoidable confusion.
32. The checkout challenge temporarily reports the transaction ID as `pending`, reducing support traceability.
33. Checkout contains corrupted typography such as `Â·` in user-facing copy.
34. The retry action can reuse stale checkout state after expiration or a backend payment-contract change instead of forcing a fresh intent.

## 6. Payment authorization and settlement flow

### Confirmed fixed

35. Wallet authorization previously inherited the seller HTTP timeout and expired after 30 seconds; it is now independently set to five minutes.
36. AgentPay omitted Base Sepolia USDC EIP-712 `name` and `version`; these fields are now included in the challenge and facilitator request.
37. The production x402 path now successfully verifies and settles exact Base Sepolia USDC directly to the seller.

### Remaining P0/P1

38. Payment rejection messages are too generic and do not distinguish expiry, invalid signature, wallet mismatch, unsupported capability, or facilitator rejection.
39. A paid-but-invalid request proved that settlement can succeed before fulfillment input validity is established.
40. Live buyer-maximum rejection and exact authorization boundaries have not been preserved as release evidence.
41. Live payment replay and duplicate-request behavior has not been demonstrated end to end.
42. Cancellation immediately before and after settlement has not been demonstrated against the documented buyer-obligation rules.

## 7. Fulfillment, failure recovery, and refunds

### P0

43. There is no automatic remediation when payment settles but seller fulfillment fails.
44. Buyers cannot safely correct an invalid request and retry fulfillment without risking a new checkout and another payment.
45. The UI does not clearly distinguish “payment succeeded, fulfillment failed” from “payment failed.”
46. The earlier paid-but-failed transaction requires manual refund or explicit seller resolution.
47. AgentPay records seller-reported refunds but does not automatically transfer USDC back to the buyer.

### P1

48. There is no locked compensation policy choosing among automatic fulfillment retry, seller review, refund, or dispute creation.
49. Payment-failure and checkout-failure recovery are implemented partially but have not been fully verified live.
50. Duplicate, non-delivery, and quality-dispute outcomes have not been exercised end to end.
51. Transaction and order reconciliation after uncertain settlement outcomes has not been demonstrated live.

## 8. Transaction, dashboard, and lifecycle UX

### P0 dashboard redesign

- The overall dashboard UI/UX is not launch quality and currently feels AI-generated rather than intentionally designed for a SaaS founder workflow.
- Information architecture is organized around internal technical systems instead of the seller's jobs: launch a product, receive money, monitor sales, and resolve failures.
- Labels and navigation use terms such as intents, routes, destinations, credentials, evidence, and reconciliation without translating them into direct founder language.
- Dashboard naming must be short, precise, consistent, and action-oriented. Prefer names such as Products, Payout wallet, API connection, Sales, Delivery status, and Activity.
- Typography has inconsistent scale: some secondary information is too small while unrelated headings are randomly oversized.
- Font size, weight, line height, and hierarchy need one deliberate type scale across every dashboard page.
- Margins and page gutters are inconsistent, making content feel cramped in some places and disconnected in others.
- Cards and boxes are overused, nested without clear hierarchy, and create a generic generated-dashboard appearance.
- Grid alignment, card heights, internal padding, section spacing, and responsive wrapping are inconsistent.
- The dashboard lacks a stable page shell with predictable navigation, title, status summary, primary action, content area, and contextual help.
- Important actions and statuses do not have enough visual priority, while low-value technical details occupy prominent space.
- Empty, loading, permission-denied, failed, partially configured, and successful states need deliberate layouts and precise next actions.
- The dashboard should progressively disclose technical details rather than showing implementation concepts to every founder by default.
- Product, payout, publication, integration, sales, fulfillment, and support controls should be manageable from one coherent dashboard without terminal or AWS-console work for normal operations.
- The complete dashboard requires another product-design iteration using real founder tasks, not only visual cleanup.

### P1

52. Abandoned checkout attempts accumulate as `PAYMENT_REQUIRED` transactions.
53. Expired and abandoned attempts need clear dashboard labels, grouping, and filtering.
54. Dashboard summaries can become noisy because test attempts and real transactions are not clearly separated.
55. Failed fulfillment, rejected payment, abandoned payment, and successful fulfillment require distinct buyer and seller statuses.
56. Only one clean fulfilled production transaction has been verified; three consecutive clean runs remain pending.

## 9. Authentication, authorization, and data safety

### Confirmed fixed

57. Wallet verification previously generated an empty DynamoDB expression and failed with `ExpressionAttributeValues must not be empty`.
58. Lambda previously lacked `dynamodb:ConditionCheckItem`.
59. The audit vocabulary previously omitted `service_endpoint.verified`.
60. Publication previously had a circular prerequisite requiring a purchase before publication.

### Remaining verification gaps

61. Seller-session expiry and recovery have not been verified across the complete deployed journey.
62. Seller suspension and stale discovery behavior have not been verified end to end.
63. Secret rotation for webhooks and seller integrations has not been proven live.
64. A full production security regression covering revocation, replay, cancellation races, and stale capabilities is incomplete.

## 10. Infrastructure and deployment

### P0/P1

65. Direct IAM and Lambda patches created Terraform drift that must be reconciled with committed infrastructure.
66. The public demo is served from `dev` resources such as `agentpay-dev-api` and `agentpay-dev-main` rather than a separately managed demo or production environment.
67. The Terraform teardown path has not been verified while retaining protected evidence resources.
68. **Partially resolved locally (2026-09-20):** durable route/entitlement publication outbox writes, consumer ordering/retry/idempotency, and completion persistence are implemented and tested. Subscription/credential stream deployment, Redis/CDN integration, retry/DLQ redrive, and live observability remain open under AWS-012.
69. The planned private managed Redis-compatible cache is not provisioned; DynamoDB remains authoritative without the complete distributed cache and revocation layer.
70. Stripe webhook infrastructure, retries, dead-letter handling, and rotation remain incomplete while Stripe is disabled.
71. Lambda regional concurrency is currently limited to 10, acceptable for zero traffic but not validated for bursts.
72. At least one Lambda invocation hit the 15-second runtime timeout and still needs attribution.
73. Seller-facing responses and links can expose the raw API Gateway deployment origin (`execute-api.ap-south-1.amazonaws.com`) instead of a stable branded backend domain. Route public API traffic through an API Gateway custom domain such as `api.agentpay.prathamranka.in`, or keep browser traffic behind the Vercel BFF. Nginx is not required for the current Lambda and API Gateway architecture; it would add cost and operational complexity unless the backend later moves to ECS or EC2.

## 11. Observability and operations

### P0/P1

74. Operational alarms are incomplete for payment rejection spikes, settlement failures, seller forwarding failures, fulfillment failures, Lambda timeouts, and DynamoDB conditional-write errors.
75. Failed fulfillment does not trigger a complete automated recovery or compensation workflow.
76. Production runbooks do not yet cover every observed failure from signup through settlement and fulfillment.
77. Evidence and video capture for the final production demonstration are incomplete.

## 12. UI/UX and product communication

### P0/P1

78. Seller-facing pages must not contain buyer checkout buttons, wallet controls, or seller-funded test-purchase instructions.
79. Seller testing should show automated checks such as endpoint reachability, schema validation, payment gating, signature verification, replay prevention, and publication readiness.
80. Buyer errors need actionable recovery guidance without exposing sensitive verifier details.
81. Product copy must accurately distinguish AgentPay's V1 seller infrastructure from the deferred AgentPay-operated buyer agent.
82. Seller, buyer, testnet, and production contexts need unmistakable labels throughout the interface.
82a. The full dashboard needs a cohesive visual system for spacing, typography, surfaces, data density, responsive behavior, focus states, and feedback instead of page-by-page styling decisions.
82b. Decorative boxes and oversized marketing treatments must not replace clear operational hierarchy in authenticated product screens.
82c. Each dashboard page should have one obvious primary task and should hide advanced technical information until the seller requests it.

## 13. Release verification gaps

1. After owner approval of customer-use terms and contributor provenance, enable the documented repository controls and run the manual workflow to publish the first immutable MCP connector and Merchant SDK release. No release was published by the issue 7-9 engineering task.
2. Complete one external buyer-agent x402 production purchase.
3. Run the complete production-shaped flow three consecutive times without manual repair.
4. Demonstrate buyer-maximum rejection and exact wallet authorization.
5. Demonstrate payment replay and duplicate fulfillment prevention.
6. Demonstrate cancellation before settlement and after finalized settlement.
7. Demonstrate refund and dispute outcomes.
8. Repeat seller suspension against the deployed environment and complete
   stale-manifest, publication, commerce, receipt/evidence, and connector-key
   revocation verification. Local official/modified connector capability and
   MCP denial is covered by issue 13.
9. Verify webhook delivery, retries, dead-letter handling, and secret rotation.
10. Verify automatic signed discovery and seller-hosted metadata refresh.
11. Reconcile direct AWS changes into Terraform and verify a clean reproducible deployment.
12. Finish the sanitized evidence and video bundle.

## Confirmed successful production path

The following path has been proven in the deployed environment:

`seller onboarding -> MCP configuration -> route validation -> publication -> signed discovery -> browser purchase session -> immutable intent -> x402 challenge -> wallet authorization -> Base Sepolia USDC settlement -> seller fulfillment -> evidence -> receipt`

Successful transaction: `txn_01M2XYBTN5YVMAD3XQ2AC2ZHAK`.
