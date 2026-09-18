# Operator suspension and replay response

Status: **Lean V1 local rehearsal runbook**. Production execution is blocked
until M8 supplies protected operator identity, durable AWS persistence, alarms,
and deployed audit retention.

## Trigger conditions

Use this procedure for suspected credential theft, payment-proof replay,
execution-capability replay, a compromised seller endpoint, fraud quarantine,
or a seller request to terminate network access immediately.

Never copy authorization headers, cookies, project keys, payment signatures,
wallet material, or seller secrets into tickets, chat, or logs. Record only the
incident identifier, seller and credential IDs, request IDs, non-sensitive
payment/provider references, timestamps, and observed machine error codes.

## Immediate containment

1. Stop issuing new capabilities by moving the authoritative entitlement to
   `suspended` or `cancelled` with an incremented `entitlementEpoch` and an exact
   UTC `accessEndsAt`. Fraud quarantine takes precedence over billing events.
2. Revoke every affected project credential. Do not rely on the seller-hosted
   connector stopping; every cloud MCP and commerce boundary rechecks current
   entitlement and credential state.
3. Preserve finalized-payment obligations. A payment finalized before the
   suspension boundary may complete its already-claimed fulfillment exactly
   once; no new challenge, verification, or settlement may begin afterward.
4. Disable or pause affected product routes when the seller endpoint itself may
   be unsafe. Do not delete historical transactions, evidence, disputes, or
   receipts.
5. Confirm the seller-visible audit stream contains the entitlement/credential
   action and actor metadata without secrets. If the audit append fails, treat
   containment as unsuccessful and keep the public edge blocked.

The local launch rehearsal uses the guarded development-only endpoint:

```powershell
Invoke-RestMethod -Method Post `
  http://127.0.0.1:8080/__dev/seed-profile/sellers/sel_01K5D09YJ0C0M7RJM4FWQ0K9H7/cancel
```

This endpoint exists only in the `agentpay_dev` build and cancels the seeded
entitlement while revoking its seeded credential. It must never be exposed by
a production binary. Production operators must use the protected M8 control
plane; direct DynamoDB edits are forbidden.

## Replay investigation

1. Classify the replay boundary: project-key bootstrap, MCP access token,
   confirmation grant, payment identifier, execution-capability JTI,
   idempotency key, or seller fulfillment transaction ID.
2. Query by the safe identifier and request ID. Never persist or reproduce the
   raw credential or payment proof.
3. Verify the expected fail-closed response: `token_replayed`,
   `payment_replayed`, `idempotency_conflict`, or seller verifier `409`.
4. Verify the seller was invoked at most once and the evidence chain remains
   valid. A duplicate upstream side effect is a severity-one incident.
5. If compromise is plausible, keep the seller suspended, rotate project keys
   and signing material, and require endpoint sandbox validation before
   reactivation.

## Recovery

Reactivate only after billing state is current, fraud quarantine is cleared by
an operator, affected credentials are rotated, the seller endpoint passes
signature/replay sandbox checks, and audit/evidence verification succeeds.
Reactivation must increment `entitlementEpoch`; old access capabilities remain
invalid even when their JWT expiry has not elapsed.

Run the cancellation, stale-discovery, browser checkout, agent checkout,
payment-replay, and exactly-once tests before closing the incident. Attach test
names and request IDs, not secrets or raw proofs, to the incident record.
