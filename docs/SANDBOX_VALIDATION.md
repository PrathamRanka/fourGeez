# Seller sandbox validation contract

Status: **Locked for AUT-008**.

AgentPay validates a draft paid route before publication by probing a dedicated,
side-effect-free seller endpoint at `/.well-known/agentpay/sandbox`. The
validator never invokes the configured paid business route.

## Seller endpoint

The coding-agent integration creates a `POST /.well-known/agentpay/sandbox`
endpoint behind the same AgentPay verification middleware used by fulfillment.
After successful verification, the endpoint returns `200 application/json`
without executing product logic. The body is a signed JSON document containing
`schemaVersion: agentpay.sandbox.v1` and the draft `routeId`.

The endpoint must return:

- `401` when AgentPay signature headers are missing;
- `401` when the signature is invalid;
- `200 application/json` for the first valid signed transaction; and
- `409` when the same signed transaction identifier is replayed.

The dedicated endpoint and signed body keep validation free of product side
effects. Seller code must not weaken the normal verification middleware for
sandbox requests.

## Validator checks

The validator derives seller and route identity from the authenticated
integration credential. It accepts no caller-supplied URL, signing secret, or
transaction identifier. It performs these checks in order:

1. `discovery`: the stored draft can be represented in the versioned AgentPay
   storefront manifest without changing its price or route fields;
2. `payment_gating`: an unsigned request cannot reach the no-op fulfillment
   endpoint and returns `401`;
3. `signature_handling`: an invalid signature returns `401` and a valid
   AgentPay signature returns `200`; and
4. `exactly_once_fulfillment`: replaying the accepted transaction returns
   `409`.

Transport, signing-secret, DNS, timeout, and upstream failures fail closed and
do not publish the route. The validator uses the existing seller forwarder's
SSRF, redirect, timeout, body-size, and response-size protections.

## Publication rule

`sandbox_validate_route` is a read-only MCP tool requiring the `validate`
scope. It returns all checks and does not persist success.

`publish_route` requires the `publish` scope, explicit confirmation,
idempotency, deterministic route validation, and a fresh sandbox run in the
same operation. Publication stops when any sandbox check fails. Because the
result is not persisted, a route or seller configuration change cannot reuse a
stale sandbox success.
