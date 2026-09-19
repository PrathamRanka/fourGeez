# `@agentpay/merchant-sdk`

Server-only TypeScript helpers for receiving AgentPay fulfillment requests and
seller webhooks. Version `0.1.0` is distributed as a checksummed private release
tarball; it is not published to a package registry.

Install the verified artifact with lifecycle scripts disabled:

```powershell
npm install --ignore-scripts --no-audit --no-fund --save-exact .\agentpay-merchant-sdk-0.1.0.tgz
```

The release artifact bundles its exact `@agentpay/verify-node` runtime, so npm
does not need registry access to resolve an unpublished AgentPay dependency.
Do not install a workspace subdirectory directly from Git.

## What it includes

- ES256 execution-capability verification over exact raw request bytes;
- an explicit legacy HMAC verifier for local/sandbox migration only;
- webhook HMAC verification and strict envelope parsing;
- seller-side idempotent fulfillment coordination;
- generic HTTPS, Shopify Admin GraphQL, and WooCommerce REST reference adapters.

It does not verify x402 payments, hold keys, mint capabilities, or replace the
AgentPay transaction service.

## Build and test

```powershell
npm install
npm run test:merchant-sdk
npm run typecheck:merchant-sdk
```

## Production fulfillment

Preserve the exact raw body, verify before parsing, and use shared atomic stores
for execution JTI replay and fulfillment results:

```ts
import {
  createRemoteJwksResolver,
  processAgentPayFulfillment,
} from "@agentpay/merchant-sdk";

const result = await processAgentPayFulfillment({
  verification: {
    issuer: process.env.AGENTPAY_API_ORIGIN!,
    sellerId: process.env.AGENTPAY_SELLER_ID!,
    routeId: process.env.AGENTPAY_ROUTE_ID!,
    method: request.method,
    path: new URL(request.url).pathname,
    rawBody,
    headers,
    keyResolver: createRemoteJwksResolver({
      jwksUrl: `${process.env.AGENTPAY_API_ORIGIN}/.well-known/jwks.json`,
    }),
    replayStore,
  },
  fulfillmentStore,
  parseInput: (body) => JSON.parse(Buffer.from(body).toString("utf8")),
  adapter,
});
```

`MemoryExecutionReplayStore`, `MemoryWebhookReplayStore`, and
`MemoryFulfillmentStore` are for tests and single-process local development
only. They are not safe for horizontally scaled production deployments.

## Webhooks

Load the reveal-once webhook secret from the seller's secret manager, not from
browser code or a committed file:

```ts
const event = await verifyAgentPayWebhook({
  secret: process.env.AGENTPAY_WEBHOOK_SECRET!,
  rawBody,
  headers,
  replayStore,
  expectedSellerId: process.env.AGENTPAY_SELLER_ID!,
});
```

The verifier authenticates the exact received bytes before parsing, binds the
signed header event ID to the envelope, requires the envelope seller ID to
match configuration, and returns that verified envelope. Do not parse a
framework-reconstructed JSON body for signature verification.

## Reference adapters

The generic HTTPS adapter forwards a verified, idempotent merchant request to a
seller-owned HTTPS service.

The Shopify adapter accepts a seller-authored Admin GraphQL operation. Required
runtime configuration is the seller's `*.myshopify.com` domain, a pinned API
version, and a server-side app access token.

The WooCommerce adapter creates an order through `/wp-json/wc/v3/orders` and
adds `_agentpay_transaction_id` metadata. It requires an HTTPS store origin and
server-side consumer key and secret.

These are tested reference transports, not managed or certified integrations.
They do not provide inventory reservation, tax, shipping, refunds, payment
capture, or physical fulfillment.
