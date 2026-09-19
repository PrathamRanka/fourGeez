# Seller sandbox validation contract

Status: **Locked for AUT-008 and DX-005**.

AgentPay validates a draft paid route before publication by probing a dedicated,
side-effect-free seller endpoint at `/.well-known/agentpay/sandbox`. The
validator never invokes the configured paid business route.

## Seller endpoint

The coding-agent integration creates a `POST /.well-known/agentpay/sandbox`
endpoint behind the same version-2 AgentPay execution-capability verification
middleware used by fulfillment. After successful verification, the endpoint
returns `200 application/json` without executing product logic. The signed
request is a bounded `agentpay.sandbox.v2` JSON document containing the draft
`routeId`, exact `routeVersion`, and canonical closed `inputSchema` and
`outputSchema`. The response is a closed JSON document containing the same
`schemaVersion`, `routeId`, and `routeVersion` plus `ready: true`;
authorization is carried by the short-lived ES256
`X-AgentPay-Execution-Capability` header and its bound transaction identifier.

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
transaction identifier. It performs these fixed checks in order:

1. `endpoint_reachability`: the stored seller origin returns a bounded response
   from the fixed sandbox path;
2. `signed_exchange`: malformed and body-mismatched capabilities are rejected,
   while the exact signed request is accepted;
3. `schema_contract`: both stored route schemas remain valid closed contracts
   and the successful sandbox response matches the closed v2 response schema;
4. `fulfillment_readiness`: the valid signed no-op request returns `200` and
   explicitly reports `ready: true` without invoking product logic;
5. `payment_gating`: a request without an execution capability returns `401`;
   and
6. `replay_idempotency`: replaying the accepted execution capability returns
   `409`.

Transport, capability-signing, JWKS, DNS, timeout, and upstream failures fail
closed and do not publish the route. The validator uses the existing seller
forwarder's SSRF, redirect, timeout, body-size, and response-size protections.

## Publication rule

`sandbox_validate_route` is a non-commercial MCP verification command requiring
the `validate` scope. It returns all checks. AgentPay persists the latest completed result,
including failures, in the seller workspace and appends a seller-scoped audit
event. The result is bound to the exact route version and becomes stale after a
route change.

`publish_route` requires the `publish` scope, explicit confirmation,
idempotency, deterministic route validation, and a fresh sandbox run in the
same operation. Publication stops when any sandbox check fails. Publication
does not trust the persisted onboarding result; it always performs a fresh run.
