# Publication invalidation rehearsal

Status: local deterministic rehearsal implemented; deployed AWS proof pending.

This runbook covers issue-register items 17 through 20 without treating a
seller-hosted manifest as authorization. DynamoDB remains authoritative and
signed discovery versions remain monotonic.

## Local deterministic gate

Run from the repository root:

```powershell
node scripts/go-tool.mjs test ./internal/publicationops ./internal/persistence/dynamodb ./internal/storefront ./internal/integrations/setupbundles
node scripts/go-tool.mjs test -tags agentpay_dev ./internal/devseed
```

The tests prove:

1. route-version and entitlement-version changes write immutable publication
   outbox events in the same DynamoDB transaction as authoritative state;
2. the consumer invalidates before regenerating signed publication and records
   completion only after both operations succeed;
3. failed regeneration remains retryable and completed event replay is
   idempotent;
4. cancellation makes a previously signed active manifest stale, authoritative
   commerce rejects its route, and discovery returns a higher-revision signed
   inactive tombstone;
5. approved input/output schema changes advance the signed storefront and
   product-document revision; all 21 maintained stack prompts require every
   public artifact to be regenerated from that latest signed contract; and
6. the local launch-ready profile includes a deterministic paused route that is
   excluded from active commerce.

## Deployed proof still required

Do not mark AWS-012, REL-001, REL-010, or REL-013 complete from the local gate.
After AWS-011 provides the private cache and a CDN endpoint exists, deploy the
stream-triggered consumer with bounded retries and a dead-letter queue, then
capture sanitized evidence for:

- an entitlement cancellation and route update producing stream records;
- cache/CDN invalidation and signed revision refresh;
- retry after an injected refresh failure and redrive from the dead-letter
  queue;
- stale manifest intent and browser checkout rejection at the canonical origin;
- refresh checks for every supported stack fixture; and
- the inactive demo route being absent from discovery and rejected by checkout.

Record timestamps, event IDs, seller IDs, aggregate versions, publication
revisions, HTTP status/error codes, retry counts, and alarm state. Never record
credentials, authorization headers, payment proofs, wallet material, or signing
keys.
