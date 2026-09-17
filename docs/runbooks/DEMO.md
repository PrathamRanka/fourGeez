# Hackathon demo runbook

## Pre-demo preparation

1. Confirm the intended AWS account and region.
2. Run all release checks from `TEST_PLAN.md`.
3. Confirm test wallet balance without displaying the private key.
4. Seed the demo seller and both routes.
5. Connect a supported coding agent to the AgentPay MCP sandbox using a seller-scoped temporary credential.
6. Open seller dashboard, hosted storefront, agent buyer view, and two approval links in separate browser profiles/devices.
7. Confirm deterministic fallback is enabled.
8. Clear only demo operational records through the seed/reset command; never delete the evidence bucket manually.

## Three-minute script

1. Ask the seller's coding agent to inspect the demo API and prepare the AgentPay integration.
2. Show the proposed repository diff, route prices, and explicit publication confirmation.
3. Show the generated human storefront, manifest, and `llms.txt` for the same products.
4. Complete the lower-value product through human checkout.
5. Ask the buyer agent for the higher-value research report.
6. Show the immutable intent, complete two-person approval, and verify the x402 testnet payment.
7. Show both purchases in the unified seller dashboard with signed fulfillment evidence.
8. Raise a prepared non-delivery dispute and show the deterministic recommendation with the simulated-refund label.

## Failure recovery

| Failure | Recovery |
|---|---|
| Bedrock unavailable | Switch the buyer view to deterministic mode and continue with the same APIs. |
| MCP host unavailable | Use the generated integration already committed for the demo and show the same seller controls in the dashboard. |
| Human checkout provider unavailable | Use the prepared successful human transaction and continue with the agent/x402 live path. |
| WebSocket unavailable | Refresh approval state through REST polling. |
| Facilitator unavailable | Use the previously prepared successful transaction for evidence/dispute demonstration; state that live testnet verification is unavailable. |
| Seller timeout | Treat as the planned non-delivery scenario and continue to dispute flow. |
| Browser state corrupted | Open a clean browser profile and use the current session snapshot endpoint. |

Do not silently replace real behavior with mocked behavior. The interface must label deterministic buyer mode and simulated refund behavior.
