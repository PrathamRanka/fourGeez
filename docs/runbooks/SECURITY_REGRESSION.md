# Consolidated security regression

Status: **Deterministic local gate implemented September 20, 2026; deployed
evidence remains a release gate.**

Run the local gate from the repository root:

```powershell
npm run verify:security-regression
```

The command composes the existing authoritative tests for seller-session
refresh/absolute expiry, credential revocation and rotation, suspended and
cancelled connector denial, stale discovery denial, payment replay,
cancellation races, webhook overlap rotation/retry behavior, and verified
receipt/evidence failure paths. It prints no credentials, cookies, payment
proofs, wallet material, or webhook secrets and explicitly states that it did
not execute live AWS proof.

## Deployed evidence sequence

Use a dedicated seller, receiver, buyer wallet, and approved private email
recipient. Record only UTC timestamps, request IDs, seller/route/credential/
subscription/transaction IDs, publication revisions, HTTP statuses, stable
error codes, retry counts, and audit action names.

1. Run `npm run smoke:auth:deployed` with refresh and absolute-expiry waits
   enabled. Confirm the production cookie rotates at access-token refresh,
   expires at the eight-hour boundary, redirects with the expired-session
   recovery state, and a fresh sign-in restores dashboard access.
2. Rotate the seller integration credential through the authenticated API.
   Keep the connector running, prove the predecessor and its unexpired MCP
   token fail before quota/tool dispatch, then configure only the reveal-once
   successor secret and prove fresh access. Never capture either token.
3. Create a replacement webhook subscription, configure the receiver for
   overlap, verify a signed event from the replacement, disable the predecessor
   through the documented API, drain any already-created predecessor retries,
   and prove a later event creates no predecessor delivery.
4. Use the reviewed launch-entitlement runbook to suspend the seller. Retain a
   previously signed manifest and still-unexpired capability. Prove fresh and
   stale clients cannot mint MCP capabilities, publish, create an intent,
   obtain a challenge, verify or settle payment, or create new official receipt
   or evidence records. Discovery must return a higher-revision inactive
   tombstone.
5. Exercise changed-proof replay, exact same-proof recovery, cancellation just
   before settlement, and cancellation after durable finality. Confirm no
   duplicate charge or seller invocation and exactly one post-finality
   fulfillment obligation.

Do not mark REL-011 through REL-015, issues 61-64, AWS-012, or webhook rotation
live proof complete from the local command. Attach sanitized evidence to the
release record; do not commit it if it contains production identifiers beyond
the allowlist above.
