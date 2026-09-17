# Seller webhook contract

Status: **Locked for schema version 1**.

AgentPay sends canonical JSON with these required fields:

```json
{
  "schemaVersion": "1",
  "eventId": "evt_...",
  "sellerId": "sel_...",
  "eventType": "payment.verified",
  "occurredAt": "2026-09-17T10:00:00Z",
  "payload": {}
}
```

Allowed event types are `payment.verified`, `fulfillment.succeeded`,
`fulfillment.failed`, and `dispute.changed`. Payload fields are event-specific,
allowlisted facts and never contain raw payment proofs, authorization headers,
cookies, wallet material, or seller secrets.

Each request includes `X-AgentPay-Webhook-Id`, `X-AgentPay-Webhook-Timestamp`,
and `X-AgentPay-Webhook-Signature`. The signature is base64 HMAC-SHA256 over:

```text
agentpay.webhook.v1\n<eventId>\n<timestamp>\n<lowercase SHA-256 body hex>
```

Consumers verify the exact received body bytes with the subscription secret,
compare signatures in constant time, reject stale timestamps, and deduplicate by
event ID. Subscription creation returns the signing secret once; AgentPay stores
only an opaque secret reference in the subscription record.
