# Merchant SDK Node example

This server preserves exact raw request bytes, verifies AgentPay before parsing,
and loads every identifier or secret from server-side environment variables.
It defaults to seller-owned DynamoDB state so concurrent instances share replay
and fulfillment claims. Process-local memory state requires the explicit
`AGENTPAY_STATE_MODE=local-memory` setting and is only for local development.

When this example is used through an AgentPay setup bundle, the seller's coding
agent applies the repository edits. AgentPay MCP supplies bounded analysis,
configuration guidance, verification, and seller-confirmed cloud mutations; it
does not edit this repository itself.

Build the packages, configure a seller-owned HTTPS fulfillment endpoint, then
run:

```powershell
npm run build:verification
npm run build:merchant-sdk
$env:AGENTPAY_API_ORIGIN = "https://api.example.com"
$env:AGENTPAY_SELLER_ID = "sel_..."
$env:AGENTPAY_ROUTE_ID = "rte_..."
$env:MERCHANT_FULFILLMENT_URL = "https://merchant.example/internal/fulfill"
$env:AGENTPAY_WEBHOOK_SECRET = "load-from-your-secret-manager"
$env:AWS_REGION = "ap-south-1"
$env:AGENTPAY_DYNAMODB_TABLE = "seller-owned-agentpay-state"
node examples/merchant-sdk-node/server.mjs
```

Do not commit the values. Shopify and WooCommerce factories in this directory
show the environment variables required by those reference adapters.

The table must have string partition key `PK`, string sort key `SK`, and
DynamoDB TTL enabled on numeric attribute `expiresAt`. The seller runtime role
needs only `dynamodb:PutItem`, `dynamodb:GetItem`, `dynamodb:UpdateItem`, and
`dynamodb:DeleteItem` on that table. Replay entries expire; fulfillment entries
do not. Do not point the example at AgentPay's cloud-authoritative table.

For single-process local development only:

```powershell
$env:AGENTPAY_STATE_MODE = "local-memory"
node examples/merchant-sdk-node/server.mjs
```
