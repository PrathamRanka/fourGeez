# TypeScript merchant SDK and adapters

Status: **EXT-003 implemented and verified locally**.

## Purpose and release boundary

`@agentpay/merchant-sdk` is a small, server-only TypeScript package for sellers
that receive AgentPay fulfillment requests or webhooks. It composes the existing
Node verification primitives into one integration surface and adds typed
contracts, durable idempotent-fulfillment coordination, and optional merchant
adapters.

The SDK does not create purchase intents, verify or settle x402 payments, hold
wallet keys, mint AgentPay capabilities, publish products, discover merchants,
or persist AgentPay transactions. AgentPay cloud remains authoritative for
payment, transaction, evidence, publication, and execution state.

The initial package is committed and version-pinned in this monorepo. Registry
publication is a separate release task; until then, examples consume the exact
workspace version.

## Package contract

The package exposes these server-side boundaries:

- production execution-request verification backed by the existing ES256
  verifier and a seller-provided replay store;
- explicitly named legacy HMAC request verification for local and sandbox
  migration only;
- webhook signature verification over the exact received bytes, timestamp,
  event identifier, and seller-owned subscription secret;
- strict parsing of the documented webhook envelope and event allowlist;
- an idempotent fulfillment coordinator that requires a durable seller-owned
  store before executing merchant business logic; and
- a small `MerchantAdapter` interface used by tested reference adapters.

All request bodies are `Uint8Array` values until verification completes. The
SDK never reconstructs signed JSON before checking its digest. Errors expose a
bounded machine code and safe HTTP status, never tokens, secrets, signatures,
raw request bodies, or internal cryptographic errors.

## Execution verification

Production fulfillment uses `X-AgentPay-Execution-Capability` and
`X-AgentPay-Transaction-Id`. The verifier pins ES256 and
`typ=agentpay-execution+jwt`, resolves a known `kid`, validates issuer and exact
seller audience, checks the seller, route, transaction, method, literal path,
raw-body SHA-256, finalized-payment claim, 30-60 second lifetime, and atomically
consumes the capability JTI.

The seller supplies a shared atomic replay store in multi-instance production.
The included memory stores are test/local helpers only.

## Webhook verification

`verifyAgentPayWebhook` requires the exact raw request bytes and these headers:

- `X-AgentPay-Webhook-Id`;
- `X-AgentPay-Webhook-Timestamp`; and
- `X-AgentPay-Webhook-Signature`.

It verifies the versioned HMAC-SHA256 contract in `api/webhooks.md`, requires a
secret of at least 32 bytes, rejects malformed or stale timestamps, compares the
signature in constant time, parses only after authentication, binds the signed
header event ID and configured seller ID to the parsed envelope, and atomically
consumes the event ID through a seller-provided replay store. It returns that
verified envelope. The default maximum age is five minutes.

## Idempotent fulfillment

`fulfillOnce` coordinates one seller-side business execution per AgentPay
`transactionId`. Its store contract has explicit `begin`, `complete`, and
`release` operations:

- `begin` atomically returns `started`, `in_progress`, or an existing completed
  result;
- `complete` durably records the serializable result before it is returned;
- `release` makes a failed attempt retryable without recording a false success.

The SDK does not provide a production database implementation. Sellers must
implement the contract with an atomic shared store such as their existing SQL,
Redis, or platform persistence. The included memory implementation is for
tests and single-process local development only.

## Merchant adapter interface

`MerchantAdapter<TInput, TResult>` contains one operation:

```ts
fulfill(context: MerchantFulfillmentContext<TInput>): Promise<TResult>
```

The context contains the verified execution claims, the AgentPay transaction
ID, and the already-parsed seller input. An adapter does not receive an x402
proof, wallet key, AgentPay signing key, project key, or capability-minting
authority.

The generic HTTPS adapter calls a seller-owned public HTTPS endpoint with a
bounded timeout and caller-supplied safe headers. AgentPay's cloud proxy remains
responsible for its own SSRF protections; sellers remain responsible for
allowlisting any secondary endpoint used inside their application.

## Shopify and WooCommerce references

The repository provides tested reference transports used only after AgentPay
execution verification and seller-side idempotency succeed:

- Shopify credentials are supplied at runtime by the seller's server and are
  never embedded in source, browser code, fixtures, or AgentPay configuration;
  the seller supplies the Admin GraphQL operation and variables.
- WooCommerce credentials are likewise seller-owned server secrets.
- Adapter tests use fake transports and assert request shape, authentication
  placement, idempotency metadata, bounded response handling, and safe errors.
- The adapters do not claim inventory reservation, tax, shipping, refunds,
  physical fulfillment, payment capture, or production certification.

Marketing or supported-stack claims may name Shopify or WooCommerce only after
the corresponding focused adapter tests pass. The truthful label is
"tested reference adapter", not "managed integration" or "production
certified".

## Seller configuration required later

AgentPay never asks sellers to send these values to a coding agent or commit
them to the repository. At deployment time, the seller configures them in the
server's secret manager:

- AgentPay API origin, seller ID, route ID, and JWKS access;
- a durable execution-JTI replay store and fulfillment idempotency store;
- each webhook subscription signing secret;
- for Shopify, the store domain, pinned Admin API version, and app access token;
- for WooCommerce, the HTTPS store origin, consumer key, and consumer secret.

## Acceptance criteria

EXT-003 is complete only when:

1. package exports and dependency versions are pinned and documented;
2. execution and webhook verification tests cover valid, modified, stale,
   replayed, malformed, unavailable-key/store, and binding-mismatch cases;
3. fulfillment tests prove one business execution, completed-result replay,
   in-progress rejection, and retry after failure;
4. generic HTTPS, Shopify, and WooCommerce reference adapters have focused
   transport tests and contain no credentials;
5. runnable examples show raw-body preservation and server-only secret loading;
6. affected lint, typecheck, build, and tests pass; and
7. public claims remain limited to behavior proven by those tests.
