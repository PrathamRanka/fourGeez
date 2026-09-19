# Seller request verification

Status: **Version 2 contract and runtime are implemented by LCH-013; EXT-003
composes the Node verifier into the TypeScript merchant SDK**.

Implementation status: version 1 uses the per-seller HMAC contract and is
retained only for M7 compatibility and sandbox fixtures. Version 2 uses
cloud-only asymmetric execution capabilities and JWKS verification. Webhook
HMAC is a separate receiver-authentication contract and is unaffected.

AgentPay maintains equivalent verification packages for Go, Node.js, Python,
.NET, Java, Ruby, and PHP applications. These packages support the maintained
Go and Python frameworks, JavaScript and TypeScript stacks, ASP.NET Core,
Spring Boot, Rails, and Laravel. Version 1 verifies the exact signing contract
in `SECURITY.md` using HMAC-SHA256 and constant-time comparison.

Each verifier requires a secret of at least 32 bytes and checks:

1. `X-AgentPay-Signature` is valid base64 and matches the canonical request.
2. `X-AgentPay-Timestamp` is RFC 3339 UTC and no more than five minutes old or
   in the future.
3. `X-AgentPay-Transaction-Id` is present and has not already been accepted.
4. The method, literal URL path, and lowercase SHA-256 hash of the exact raw
   request body match the signed values.

Replay storage is an explicit application boundary. The included in-memory
implementation is for tests and local development only. Production sellers
must use a shared atomic store when more than one process can receive requests.

Verification failure returns `401`; replay returns `409`; verifier or replay
store failure returns `503`. Middleware never logs or returns the secret,
signature, authorization headers, body, or internal error details.

The .NET package exposes ASP.NET Core middleware that buffers and rewinds the
request body. The Java package exposes a verifier and Spring filter primitive,
the Ruby package exposes Rack middleware for Rails, and the PHP package exposes
the verifier used by Laravel middleware. Their committed fixture tests cover a
valid signature, modified-body rejection, stale-request rejection, replay
rejection, and minimum secret length. Run all four with
`npm run test:verification:extended`; missing .NET, Ruby, or PHP runtimes use
pinned official Docker images, while Java 17 or newer is required locally.

## Version 2 production contract

Version 2 replaces shared seller HMAC authority for production fulfillment.
The cloud may mint this capability only after an atomic forwarding claim whose
condition includes `status=PAYMENT_VERIFIED`, `paymentFinality=finalized`, the
expected transaction version, and absence of a prior forwarding owner. The
current M7 Go repository checks status only and therefore remains development
compatibility code until LCH-024 updates that condition.
Every AgentPay-to-seller request includes
`X-AgentPay-Execution-Capability: <compact JWT>` and
`X-AgentPay-Transaction-Id: txn_...` along with the exact configured method,
literal path, and raw body.

The JWT protected header requires `alg=ES256`, a known AgentPay JWKS `kid`, and
`typ=agentpay-execution+jwt`. Required claims and checks are:

| Claim | Required verification |
|---|---|
| `iss` | Exact configured AgentPay API origin |
| `aud` | Exact `urn:agentpay:seller:<sellerId>` value |
| `sub` | Equal to `transactionId` |
| `sellerId` | Equal to the receiving seller |
| `routeId` | Equal to the locally configured AgentPay route |
| `transactionId` | Equal to `X-AgentPay-Transaction-Id` |
| `method` | Equal to the uppercase request method |
| `path` | Equal to the literal request path; query is excluded |
| `bodySha256` | Equal to lowercase SHA-256 of exact raw request bytes |
| `paymentFinality` | Exactly `finalized` |
| `jti` | Unique `xec_` identifier consumed once |
| `iat`, `exp` | Numeric dates; lifetime is 30-60 seconds |

Verification packages fetch public keys from `/.well-known/jwks.json`, cache
only according to response headers, pin ES256, reject unknown `kid` values, and
refresh once for an unseen key during overlapping rotation. They never accept
an algorithm from configuration or token input.

Middleware preserves and hashes raw bytes before JSON parsing. After signature
and claim verification, it atomically consumes `jti` in a shared replay store
and uses `transactionId` as the seller application's fulfillment idempotency
key. A repeated JTI returns `409`; an already completed transaction returns its
stored outcome without rerunning business logic.

Verification failure returns `401`, binding or audience mismatch returns `403`,
replay returns `409`, and JWKS/replay-store failure returns `503`. Middleware
never logs or returns the JWT, authorization headers, raw body, payment proof,
or internal verifier errors.

Seller code receives verification authority only. It never receives an
AgentPay private signing key, payment-verification authority, publication
authority, or capability-minting endpoint. Forking or removing middleware can
weaken only the seller's endpoint; it cannot create an AgentPay transaction,
receipt, evidence chain, or valid execution capability.

Version 1 remains available only to local and sandbox fixtures during
migration. Production publication rejects integrations that validate only
version 1. Webhook HMAC is separate and unaffected. All language fixtures must
cover a valid version-2 request, modified body, wrong seller/route/method/path/
audience, unknown key, expiry, replay, JWKS failure, and transaction-level
idempotent retry.
