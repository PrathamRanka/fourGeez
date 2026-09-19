# Merchant SDK Node example

This server preserves exact raw request bytes, verifies AgentPay before parsing,
and loads every identifier or secret from server-side environment variables.
It uses memory replay/idempotency stores only to stay runnable as a local
example. Replace them with atomic shared persistence before deployment.

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
node examples/merchant-sdk-node/server.mjs
```

Do not commit the values. Shopify and WooCommerce factories in this directory
show the environment variables required by those reference adapters.
