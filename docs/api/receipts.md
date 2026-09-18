# Purchase receipt contract

Status: **Schema version 1 is the implemented M7 compatibility contract; version 2 is the locked M7.1 launch target**.

`GET /v1/transactions/{transactionId}/receipt` returns
`application/vnd.agentpay.receipt+json` only after payment is finalized and the
complete evidence chain verifies. Authorization is seller session, the buyer
agent that owns the transaction, or the durable browser purchase grant bound to
the transaction's purchase session. The browser grant remains read-only after
its commerce window closes and lasts until 30 days after terminal fulfillment,
failure, or a timely dispute's resolution. If its cookie is lost after payment, the buyer may restore
read/remediation access through a one-time challenge signed by the payer wallet
bound during finalized x402 verification.

Schema version `1` remains valid for historical records and for the current M7
runtime; it identifies the product by immutable `routeId`. Version `2`
additionally contains immutable
purchase-time snapshots:

- `productDisplayName` and `productSlug`;
- `purchaseChannel` and `paymentRail`;
- `paymentDestinationId` but never the private key or unrestricted payout data;
- exact `amount`, `asset`, and `network`;
- safe payment reference and finality;
- fulfillment outcome; and
- verified evidence root/head hashes and signed evidence events.

Receipts never include payment identifiers, raw proofs, proof hashes, project
keys, access tokens, browser purchase cookies, approval invitations/cookies/
tokens, execution capabilities, authorization headers, wallet private material,
seller secrets, or unrestricted request/response bodies.

Repeated downloads for unchanged transaction and evidence state produce the
same JSON fields. `404 not_found` hides unauthorized cross-tenant resources,
`409 state_conflict` reports a transaction that is not finalized or whose
evidence does not verify, `401` covers invalid/expired/revoked buyer authority,
`403 permission_denied` covers a valid caller that does not own the receipt,
`429 rate_limited` includes `Retry-After`, and `503 dependency_unavailable`
means the authoritative transaction or evidence state cannot be established.
