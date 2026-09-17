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

## Protected assets

- Test-wallet private key and future payment credentials.
- Seller HMAC signing secrets.
- Seller and approver identity data.
- Approval invitation and execution tokens.
- Payment proof and facilitator response.
- Seller request/response content.
- Evidence chain, signatures, and dispute decisions.

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

## Canonical hashing

Canonicalization must be versioned. Version 1 uses:

1. UTF-8 JSON.
2. Object keys sorted lexicographically.
3. No insignificant whitespace.
4. Integers and amount strings preserved exactly.
5. Arrays retain order.
6. Hash input includes canonicalization version and domain separator.

Use different domain separators for intent and evidence hashes, such as `agentpay.intent.v1` and `agentpay.evidence.v1`. Never hash an ambiguous string concatenation.

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

