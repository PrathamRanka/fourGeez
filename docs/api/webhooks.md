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

## Delivery policy

AgentPay creates at most one delivery for each subscription and event ID. The
event ID and delivery ID remain stable across automatic retries and explicit
seller redelivery. A successful `2xx` response completes delivery. Transport
failures, timeouts, `408`, `425`, `429`, and `5xx` responses are retried after
1 minute, 5 minutes, 30 minutes, and 2 hours; the fifth failed attempt enters
the terminal `dead_letter` state. Other `4xx` responses dead-letter immediately.

Delivery-time validation rejects private, loopback, link-local, multicast,
unspecified, and mixed public/private DNS results. Connections re-resolve and
pin a public address, redirects are rejected, TLS 1.2 is required, requests time
out after 10 seconds, and at most 64 KiB of response bytes are hashed. Response
bodies are not stored.

Sellers list delivery history with
`GET /v1/sellers/{sellerId}/webhook-deliveries` and may request replay-safe
redelivery with
`POST /v1/sellers/{sellerId}/webhook-deliveries/{deliveryId}/redeliver`.
Redelivery is valid only for dead-letter deliveries and preserves the original
event identity.
