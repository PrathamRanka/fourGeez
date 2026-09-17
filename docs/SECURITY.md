# Security requirements and threat model

Status: **Required for every implementation task**.

## Security goals

- A model cannot authorize or execute a purchase by itself.
- One payment proof cannot cause more than one seller invocation.
- Approval applies only to the exact intent shown to approvers.
- Evidence alteration is detectable.
- Seller configuration cannot turn the proxy into an SSRF service.
- Secrets and payment proofs never appear in browser bundles, API responses, evidence payloads, or logs.
- A dispute result is reproducible from recorded facts and rule version.
- A coding agent cannot publish products, rotate credentials, or deploy production changes without explicit seller authorization.
- Browser and agent purchase channels cannot bypass the same pricing, approval, and fulfillment rules.
- A seller payment destination cannot become active without bounded ownership verification.
- Generated SEO/AEO content cannot invent claims, reviews, prices, availability, or hidden search content.

## Protected assets

- Test-wallet private key and future payment credentials.
- Seller HMAC signing secrets.
- Seller and approver identity data.
- Approval invitation and execution tokens.
- Payment proof and facilitator response.
- Seller request/response content.
- Evidence chain, signatures, and dispute decisions.
- Seller project credentials and MCP authorization grants.
- Seller repository contents, deployment credentials, and generated configuration.

## Primary threats and controls

| Threat | Required controls |
|---|---|
| Payment replay | Unique payment identifier index, conditional write, intent expiration, exactly-once forwarding claim |
| Intent modification after approval | Canonical intent hash included in session and approval token; reject every mismatch |
| Invitation theft | 256-bit random token, hash at rest, ten-minute expiry, single use, scoped session/approver, `Referrer-Policy: no-referrer` |
| Model prompt injection | Fixed tool schemas, no direct HTTP tool, server-side route lookup, amount/budget revalidation, bounded calls and timeouts |
| SSRF through seller URL | HTTPS allowlist, DNS/IP validation, block loopback/link-local/private/metadata ranges, no redirects, re-resolve on connection |
| Malicious seller response | Response byte/time limits, content-type allowlist, no active HTML rendering, hash before storage |
| Evidence tampering | Append-only object keys, Object Lock, versioning, canonical hashes, previous hash, KMS signature, separate verifier role |
| Secret leakage | Secrets Manager, log redaction, no secret env values where avoidable, browser bundle scan, rotation procedure |
| Tenant data access | Cognito subject-to-seller authorization on every seller route; no caller-supplied tenant trust |
| Duplicate mutation | Required idempotency key bound to caller, operation, and request hash |
| WebSocket impersonation | Validate invitation token on connect, bind connection to session, authorize every callback, expire connections |
| Denial of service | API throttles, body limits, route limits, Lambda concurrency, upstream timeout, Bedrock call budget |
| Repository prompt injection | Treat repository text as untrusted, expose only allowlisted MCP tools, and require confirmation for commercial or deployment mutations |
| Over-scoped integration credential | Bind each credential to one seller, use explicit scopes and expiration, hash it at rest, and support immediate revocation |
| Generated secret exposure | Write secrets only to ignored server-side configuration, scan generated changes, and never serialize secrets into browser code or model prompts |
| Unauthorized publication or deployment | Produce a reviewable plan and diff, then require seller confirmation before publish, credential rotation, or production deployment |
| Human checkout forgery or replay | Authenticate provider callbacks, bind them to immutable intents, process them idempotently, and reuse transaction replay protection |
| Payout-address substitution | Verify wallet ownership, bind destinations to seller and asset/network, require explicit confirmed rotation, and freeze the destination in each purchase intent |
| Dashboard revenue inflation | Derive aggregates idempotently from authoritative payment and transaction events and keep assets/networks separate |
| Forged seller webhook | Sign canonical payloads, include event IDs and timestamps, use constant-time verification, and make redelivery idempotent |
| Webhook SSRF | Apply the seller-proxy public-address, DNS-rebinding, redirect, timeout, and response-size controls to webhook destinations |
| SEO/AEO abuse | Require visible-content consistency, prohibit fabricated claims and keyword stuffing, validate structured data, and never promise ranking |
| Tenant resource exhaustion | Apply seller-scoped quotas and rate limits to API, MCP, route, analytics, and webhook operations |

## Canonical hashing

Payment-destination ownership challenges use EIP-191 `personal_sign` for the
initial `eip155` network support. The signed text includes a version, seller ID,
destination ID, asset, network, public address, 256-bit nonce, and RFC 3339 UTC
expiry. Challenges expire after ten minutes. AgentPay stores only the SHA-256
challenge hash and compares it in constant time before signature recovery. Raw
challenges are returned with `Cache-Control: no-store`; raw signatures are never
persisted or logged. Contract-wallet ownership proofs require a separately
documented verifier and are not accepted by the initial EOA verifier.

Canonicalization must be versioned. Version 1 uses:

1. UTF-8 JSON.
2. Object keys sorted lexicographically.
3. No insignificant whitespace.
4. Integers and amount strings preserved exactly.
5. Arrays retain order.
6. Hash input includes canonicalization version and domain separator.

Use different domain separators for intent and evidence hashes, such as `agentpay.intent.v1` and `agentpay.evidence.v1`. Never hash an ambiguous string concatenation.

## Seller request signatures

AgentPay authenticates each forwarded seller request with `X-AgentPay-Signature`,
`X-AgentPay-Timestamp`, and `X-AgentPay-Transaction-Id`. Version 1 signs the
following newline-delimited UTF-8 fields with HMAC-SHA256 and base64-encodes the
result:

```text
agentpay.seller-request.v1
<RFC3339 UTC timestamp>
<HTTP method>
<literal route path>
<lowercase SHA-256 body hash>
<transactionId>
```

The secret is resolved server-side from the seller's `signingSecretRef`, must
contain at least 32 bytes, and is never added to request models, logs, evidence,
or responses. Seller verification uses constant-time signature comparison.

## Evidence payload policy

Allowed:

- IDs, timestamps, state transitions, amount/asset/network.
- Hashes of request, payment proof, and response.
- Approval labels and decisions shown in the demo.
- Upstream HTTP status, duration, content length, and allowlisted headers.
- Dispute reason, rule version, classification, and explanation.

Forbidden:

- Raw `PAYMENT-SIGNATURE` or wallet private material.
- Authorization headers, cookies, invitation tokens, approval tokens, or seller secrets.
- Unredacted prompts or arbitrary seller response bodies.
- Full personal addresses, financial account details, or unnecessary user content.

## API requirements

- Maximum JSON body: 1 MiB; paid-route limit is configurable downward.
- Strict content types and JSON decoding with unknown-field rejection for control APIs.
- Stable machine error codes; internal stack traces never leave the service.
- CORS limited to configured web origins.
- `Cache-Control: no-store` on approval, transaction, dispute, and 402 responses.
- Security headers on the web app, including CSP and `frame-ancestors 'none'` for approval pages unless embedding is intentionally added.
- Constant-time comparison for token and HMAC verification.

## MCP and coding-agent requirements

- The AgentPay MCP server exposes bounded commerce operations, documentation resources, and setup prompts; it does not expose an arbitrary shell or unrestricted HTTP proxy.
- Read-only tools are separated from mutating tools by scope.
- Product creation, price updates, publication, credential rotation, and production deployment require explicit confirmation.
- Every mutation requires idempotency and records the seller, credential, operation, target, and outcome without recording secrets or repository contents.
- Project credentials are seller-scoped, hashed at rest, revocable, and never committed to the seller repository.
- Generated integrations use maintained verification packages. Coding agents must not invent alternate signing or payment validation.
- Repository analysis must not upload unrelated source files, `.env` contents, credentials, wallet material, customer information, or proprietary data to AgentPay.
- SEO/AEO generation may inspect public page structure and allowlisted product
  metadata only. It must not upload private source, generate hidden content, or
  state that ranking improvement is guaranteed.

## Human checkout requirements

- Card checkout is deferred to milestone H1 and is not required for the
  agent-first x402 release.
- The human checkout provider must be selected and its official integration guidance recorded before adding a dependency.
- A provider success redirect is not proof of payment; only an authenticated server callback may advance payment state.
- Provider events require replay protection and idempotent processing.
- Human checkout uses the same frozen seller quote, approval policy, fulfillment claim, evidence chain, and dispute rules as x402.
- Merchant-of-record, platform-fee, refund, tax, chargeback, and seller-payout responsibility must be documented before production activation.

## Production blockers

The system must not process real funds until all are complete:

- External application and infrastructure security review.
- Legal review of payment role, refund handling, sanctions, privacy, and retention.
- Production wallet architecture avoiding raw application-held private keys.
- Incident response and key-compromise runbooks.
- Backup/restore and disaster recovery test.
- Tenant isolation penetration test.
- Dependency, container, and IaC scanning in CI.
- Documented facilitator SLA and failure semantics.

## Security verification checklist

- [ ] No tracked file contains secret-like values.
- [ ] Tests prove replay and duplicate forwarding are blocked.
- [ ] Tests prove modified intents invalidate approvals.
- [ ] Tests cover private, loopback, link-local, IPv6, redirect, and DNS-rebinding SSRF cases.
- [ ] Evidence verification fails after any payload, order, hash, or signature change.
- [ ] Browser build contains no server-only configuration.
- [ ] Logs are tested for credential redaction.
- [ ] IAM policies pass least-privilege review before demo deployment.
