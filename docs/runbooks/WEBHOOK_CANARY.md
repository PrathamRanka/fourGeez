# Seller webhook canary runbook

Status: **Local overlap-rotation verification is implemented. Deployed
delivery, retry/DLQ, and rotation certification remain pending AWS-012 and a
real receiver rehearsal.**

## Purpose

The canary proves that AgentPay signs the exact raw body, preserves event
identity across retries, stops after the bounded retry policy, exposes a
dead-letter delivery, and can move a seller to a newly generated signing
secret without logging either secret.

Use a dedicated HTTPS endpoint and test seller. Never point the canary at a
production fulfillment endpoint or a third-party request inspector.

## Receiver requirements

The receiver must persist only:

- exact raw request bytes in an encrypted temporary file;
- `X-AgentPay-Webhook-Id`;
- `X-AgentPay-Webhook-Timestamp`;
- `X-AgentPay-Webhook-Signature`; and
- the response status selected for the test.

Do not log the subscription secret, authorization headers, cookies, wallet
material, payment proofs, or unrestricted payloads. Delete temporary body and
header files after verification.

## Signature check

Set the reveal-once subscription secret and captured headers only in the
current process environment:

```powershell
$env:AGENTPAY_WEBHOOK_SECRET = Read-Host "Webhook secret"
$env:AGENTPAY_WEBHOOK_ID = "<captured-event-id>"
$env:AGENTPAY_WEBHOOK_TIMESTAMP = "<captured-rfc3339-timestamp>"
$env:AGENTPAY_WEBHOOK_SIGNATURE = "<captured-base64-signature>"
npm run ops:webhook:canary -- verify .\private-canary\raw-body.json
Remove-Item Env:AGENTPAY_WEBHOOK_SECRET,Env:AGENTPAY_WEBHOOK_ID,Env:AGENTPAY_WEBHOOK_TIMESTAMP,Env:AGENTPAY_WEBHOOK_SIGNATURE
```

The verifier hashes the exact bytes, checks the versioned HMAC domain in
constant time, and rejects timestamps older than five minutes.

## Retry and dead-letter check

1. Configure the canary receiver to return `503`.
2. Trigger one allowlisted seller webhook event.
3. Confirm the same event and delivery IDs are used for attempts at roughly
   1 minute, 5 minutes, 30 minutes, and 2 hours.
4. Confirm the fifth failed attempt enters `dead_letter` and no sixth automatic
   attempt occurs.
5. Read the redacted state without printing the bearer token:

```powershell
$env:AGENTPAY_API_ORIGIN = "<terraform-http-api-url>"
$env:AGENTPAY_SELLER_ID = "sel_<seller-ulid>"
$env:AGENTPAY_SELLER_BEARER = Read-Host "Short-lived seller bearer"
npm run ops:webhook:canary -- history
Remove-Item Env:AGENTPAY_SELLER_BEARER
```

6. Change the receiver to `204`, request seller redelivery with a fresh
   idempotency key, and confirm delivery succeeds once with the original event
   identity.

Automatic attempts require the durable webhook worker in AWS-012. Until that
worker is deployed, this procedure cannot certify live retries or DLQ behavior.

## Secret rotation check

Do not overwrite a Secrets Manager value behind the seller's back. The safe
rotation is overlap-based:

1. Create a replacement subscription and capture its new reveal-once secret.
2. Configure the receiver to accept old and new secrets by subscription ID.
3. Trigger and verify an event through the replacement subscription.
4. Disable the old subscription through
   `POST /v1/sellers/{sellerId}/webhook-subscriptions/{subscriptionId}/disable`
   with the predecessor's current `expectedVersion` and a fresh idempotency
   key. Exact replay returns the same redacted response; a stale version fails
   closed.
5. Remove the old secret only after no retryable old deliveries remain.

Never simulate rotation with direct DynamoDB or Secrets Manager edits. Local
tests prove replacement creation, predecessor disablement, audit metadata, and
that disabled subscriptions receive no new deliveries. Live endpoint delivery
and retry-drain evidence is still required.
