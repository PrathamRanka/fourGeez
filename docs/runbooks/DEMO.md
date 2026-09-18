# Hackathon demo runbook

## Pre-demo preparation

1. Confirm the intended AWS account and region.
2. Run all release checks from `TEST_PLAN.md`.
3. Confirm test wallet balance without displaying the private key.
4. Seed the demo seller and fixed-price routes.
5. Connect a supported coding agent to the AgentPay MCP sandbox using a seller-scoped temporary credential.
6. Open the seller dashboard, hosted storefront, browser checkout, and agent buyer view.
7. Confirm deterministic fallback is enabled.
8. Clear only demo operational records through the seed/reset command; never delete the evidence bucket manually.

## Three-minute script

1. Ask the seller's coding agent to inspect the demo API and prepare the AgentPay integration.
2. Show the proposed repository diff, route prices, and explicit publication confirmation.
3. Show the generated browser storefront, sitemap, manifest, and `llms.txt` for the same products.
4. Complete one product through the browser wallet/x402 flow.
5. Ask the buyer agent for the research report with a maximum spend.
6. Show the immutable intent, exact seller quote, wallet authorization, and verified x402 testnet payment without a buyer-approval step.
7. Show both purchases once in the seller dashboard with asset-separated totals and signed fulfillment evidence.
8. Raise a prepared non-delivery dispute and show the deterministic recommendation with the simulated-refund label.

## Failure recovery

| Failure | Recovery |
|---|---|
| Bedrock unavailable | Switch the buyer view to deterministic mode and continue with the same APIs. |
| MCP host unavailable | Use the generated integration already committed for the demo and show the same seller controls in the dashboard. |
| Browser wallet unavailable | Use the prepared successful browser transaction and continue with the agent/x402 live path. |
| Facilitator unavailable | Use the previously prepared successful transaction for evidence/dispute demonstration; state that live testnet verification is unavailable. |
| Seller timeout | Treat as the planned non-delivery scenario and continue to dispute flow. |
| Browser state lost after payment | Recover read and dispute access with the payer-wallet recovery challenge; never restore commerce authority. |

Do not silently replace real behavior with mocked behavior. The interface must label deterministic buyer mode and simulated refund behavior.
