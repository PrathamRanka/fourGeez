# Hackathon demo runbook

## Pre-demo preparation

1. Confirm the intended AWS account and region.
2. Run all release checks from `TEST_PLAN.md`.
3. Confirm test wallet balance without displaying the private key.
4. Seed the demo seller and both routes.
5. Open seller dashboard, buyer view, and two approval links in separate browser profiles/devices.
6. Confirm deterministic fallback is enabled.
7. Clear only demo operational records through the seed/reset command; never delete the evidence bucket manually.

## Three-minute script

1. Show the machine-readable storefront and two paid API routes.
2. Ask the buyer agent for the higher-value research report.
3. Show the immutable intent and explain that payment has not happened.
4. Open the live approval room on two devices and approve from each.
5. Show the x402 challenge and real testnet payment verification.
6. Show the signed seller response and chain-of-custody evidence rail.
7. Raise a non-delivery dispute against a prepared failed transaction.
8. Show deterministic `refund_recommended` classification and the simulated-refund label.

## Failure recovery

| Failure | Recovery |
|---|---|
| Bedrock unavailable | Switch the buyer view to deterministic mode and continue with the same APIs. |
| WebSocket unavailable | Refresh approval state through REST polling. |
| Facilitator unavailable | Use the previously prepared successful transaction for evidence/dispute demonstration; state that live testnet verification is unavailable. |
| Seller timeout | Treat as the planned non-delivery scenario and continue to dispute flow. |
| Browser state corrupted | Open a clean browser profile and use the current session snapshot endpoint. |

Do not silently replace real behavior with mocked behavior. The interface must label deterministic buyer mode and simulated refund behavior.

