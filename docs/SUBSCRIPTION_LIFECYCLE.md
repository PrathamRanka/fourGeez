# Seller subscription and entitlement lifecycle

Status: **Locked for the M7.1 production target by LCH-005**.

Last verified against official Stripe documentation: **2026-09-18**.

## Provider and boundary

Stripe Billing is the initial provider for the seller's AgentPay software
subscription. It does not verify, custody, route, settle, refund, or reconcile
buyer-to-seller x402 payments. Stripe identifiers and invoice states are inputs
to the billing adapter; the AgentPay `SellerEntitlement` projection is the only
authorization source used by MCP, discovery, publishing, and commerce.

Checkout success redirects never activate access. AgentPay verifies signed
Stripe webhooks from the exact raw body, durably records each event, and then
reconciles the current Stripe subscription and invoice state using a pinned
Stripe API version. Webhook arrival order and delivery count are not trusted.

## Authorization rule

Paid network participation is allowed only when all of these are true:

```text
seller account is active
AND entitlement.status == active
AND now < accessEndsAt
AND credentialRotationRequired == false for credential-backed access
AND no administrative or fraud quarantine applies
```

At `now == accessEndsAt`, access is denied. A delayed webhook, scheduler, cache,
or status transition never extends this boundary. `grace` is billing-recovery
and historical-read-only access; it never authorizes MCP, active discovery,
publication, new purchase intents, x402 challenges, payment verification,
settlement, or new transaction execution.

## Internal states

| State | Meaning | Network authority | Seller access |
|---|---|---|---|
| `active` | A paid Stripe period is currently valid and `now < accessEndsAt`. | Allowed subject to scopes, quota, seller status, trust, route and transaction checks. | Full dashboard and supported control-plane operations. |
| `grace` | The paid period ended without confirmed renewal; a fixed 72-hour recovery window is open. | Blocked. Storefront is inactive and no new commerce is allowed. | Billing portal, payment-method recovery, plan/status, historical transactions, receipts, evidence, disputes, exports, support and credential revocation only. |
| `suspended` | Recovery grace expired, Stripe reports an access-blocking condition, or an operator suspended the seller. | Blocked. | Historical/billing recovery access according to reason; no MCP or commercial mutation. |
| `cancelled` | Voluntary cancellation became effective at `accessEndsAt`, or immediate cancellation was requested. | Blocked. No grace is added for voluntary cancellation. | Historical records, export, billing reactivation and support according to retention policy. |
| `closed` | AgentPay account closure completed. | Blocked permanently for this seller identity. | No normal dashboard or API access; retained records are available only through authorized legal/support procedures. |

`cancelAtPeriodEnd=true` does not itself change `status` to `cancelled`. A
currently paid seller remains `active` only until the exclusive
`accessEndsAt`. At that instant AgentPay locally denies access and transitions
to `cancelled` even if Stripe's deletion event has not arrived.

## State transitions

```text
no entitlement --paid activation--> active

active --renewal paid--> active with later accessEndsAt
active --renewal unpaid at accessEndsAt--> grace
active --scheduled cancellation reaches accessEndsAt--> cancelled
active --immediate cancellation--> cancelled
active --administrative suspension/Stripe paused--> suspended
active --fraud quarantine--> suspended(fraud_quarantine)

grace --payment succeeds before graceEndsAt--> active + credential rotation required
grace --graceEndsAt reached--> suspended(payment_failed)
grace --voluntary cancellation--> cancelled
grace --fraud quarantine--> suspended(fraud_quarantine)

suspended --verified payment and allowed recovery--> active + credential rotation required
cancelled --new paid subscription--> active + credential rotation required
suspended(fraud_quarantine) --operator clears and billing is paid--> active + credential rotation required

active|grace|suspended|cancelled --account closure--> closed
closed --any provider event--> closed
```

The grace period is exactly 72 hours beginning at the previous
`accessEndsAt`. It never moves `accessEndsAt` and never restores network
authority. A successful payment during grace establishes a new paid period;
AgentPay does not simply extend the old deadline by 72 hours.

## Stripe event mapping

Every accepted Stripe event triggers reconciliation. The mapping below states
the event's purpose, not permission to apply its embedded object blindly.

| Stripe event | AgentPay action |
|---|---|
| `checkout.session.completed` | Persist the verified Stripe customer/subscription association and request reconciliation. Never activate access from the redirect or this event alone. |
| `invoice.paid` | Fetch the current subscription and invoice. When the invoice covers the configured subscription and the subscription is eligible, set `active` and set `accessEndsAt` to the exclusive end of the paid service period. |
| `invoice.payment_failed` | Record billing recovery required and notify the seller. Do not extend `accessEndsAt`. Remain active only while the previously paid boundary is still in the future; enter `grace` at that boundary. |
| `invoice.payment_action_required` | Record required seller action and notify. Do not extend access without `invoice.paid`. |
| `invoice.finalization_failed` | Record provider failure for operations/support. Do not activate or extend access. |
| `customer.subscription.created` | Associate and reconcile. Initial access still requires confirmed paid entitlement. |
| `customer.subscription.updated` | Reconcile plan, quantity, status, period boundary and `cancel_at_period_end`; never trust delivery order. |
| `customer.subscription.deleted` | Reconcile and enter `cancelled` for voluntary/end-of-period cancellation, or `suspended` when provider evidence identifies non-payment or administrative termination. |
| `customer.subscription.paused` | Enter `suspended` at the effective pause boundary and revoke network authority. |
| `customer.subscription.resumed` | Reconcile; restore `active` only when a paid service period is confirmed, then require credential rotation. |
| `customer.subscription.trial_will_end` | Notification only. V1 plans do not grant network authority from `trialing`; enabling trials requires a new documented policy. |

Stripe subscription statuses are adapter inputs:

| Stripe status | Candidate interpretation after authoritative fetch |
|---|---|
| `active` | Eligible for `active` only with a paid period covering `now`. |
| `past_due` | No extension. Active only before the prior `accessEndsAt`; then `grace`, then `suspended`. |
| `unpaid` | `suspended` once the prior paid period ends; no network grace. |
| `canceled` | `cancelled` for voluntary cancellation; non-payment/administrative reasons map to `suspended`. |
| `paused` | `suspended`. |
| `incomplete`, `incomplete_expired` | Never active; remain or become `suspended`/unprovisioned. |
| `trialing` | Not entitled in V1. Treat as configuration error and fail closed until a trial policy is separately approved. |

## Webhook authenticity, idempotency and order

The Stripe endpoint:

1. reads the bounded raw request body;
2. verifies `Stripe-Signature` with Stripe's maintained SDK and the
   environment-specific endpoint secret;
3. rejects test/live-mode mismatch and unrecognized account/environment;
4. atomically inserts an inbox record keyed by Stripe `event.id` with event
   type, payload hash, object IDs, API version, `created`, and receive time;
5. treats an exact duplicate as success without repeating effects;
6. treats the same event ID with a different payload hash as a security
   incident; and
7. returns success only after durable inbox storage, then processes
   reconciliation asynchronously.

Stripe event `created` timestamps are audit facts, not ordering guarantees.
Workers fetch the current provider objects, derive a complete candidate
projection, and conditionally commit it using the current AgentPay version. A
conflict causes a fresh fetch and recomputation. `sourceRevision` is the
zero-padded, monotonically increasing AgentPay reconciliation sequence, not a
Stripe event timestamp. The applied snapshot hash and triggering event IDs are
retained for audit and replay.

A scheduled reconciliation runs at least every 15 minutes for active,
cancel-at-period-end, grace and suspended subscriptions, and at startup after a
worker outage. Authorization still enforces `accessEndsAt` directly, so this
schedule is recovery, not the expiry mechanism.

## Epoch, cache and credential effects

Entering `grace`, `suspended`, `cancelled`, `closed`, or fraud quarantine
atomically increments `entitlementEpoch`, writes an outbox event, and removes
active discovery. Token exchange is denied and existing MCP tokens fail their
epoch check. Redis is invalidated after commit and is never authoritative.

Reactivation from any non-active state requires all of:

1. a reconciled paid Stripe period with `now < accessEndsAt`;
2. no unresolved administrative or fraud quarantine;
3. an incremented `entitlementEpoch`;
4. `credentialRotationRequired=true`; and
5. explicit seller-session rotation of each project key intended for reuse.

Existing project keys remain stored only so the seller can identify and revoke
them; they cannot exchange tokens after reactivation. Successful rotation
creates an eligible successor and clears the requirement for that successor.
Removing `cancelAtPeriodEnd` before the paid boundary does not require rotation
because authority was never interrupted.

## Historical access and commerce races

Grace, payment-related suspension and cancellation preserve seller-session
access to billing recovery, invoices, plan/status, historical transactions,
receipts, evidence, disputes, exports, support and credential revocation. They
do not permit product changes, publication, key creation, MCP, discovery, new
intents, x402 challenges, verification or settlement. Administrative and fraud
suspensions may narrow historical access further, but never broaden network
authority. `closed` has no normal authenticated access.

Buyer settlement remains independent:

- cancellation or loss of entitlement before settlement blocks verification
  and settlement;
- a payment finalized before normal cancellation remains a bounded fulfillment
  obligation and may execute exactly once;
- fraud quarantine may hold an already-finalized transaction only through the
  explicit incident/refund path; and
- Stripe subscription refunds, invoices and payment methods never modify buyer
  x402 payment records.

## Implementation gate

LCH-005 changes contracts only. The current Go `SellerPlan` implementation
still supports only `active` and `suspended`, auto-creates a default plan, and
does not enforce `accessEndsAt`. Production activation remains blocked until
LCH-010, LCH-016, LCH-021 through LCH-023, LCH-027, LCH-040, LCH-041 and
AWS-013 implement and verify this lifecycle.
