# x402 testnet release evidence

Status: **Required for REL-003, REL-004, REL-005, and REL-008**.

This runbook exercises testnet value only. It must not be used with mainnet,
production assets, card checkout, platform fees, or custody.

## Locked profile

- Facilitator: `https://x402.org/facilitator`
- Network: Base Sepolia, `eip155:84532`
- Asset: USDC, `0x036CbD53842c5426634e7929541eC2318f3dCF7e`
- Payment scheme: `exact`
- Seller payout: the seller-provided, ownership-verified public address stored
  on that seller's payment destination
- Facilitator credential: none

## Current validation fixture

These are public Base Sepolia values for the release exercise only. They are
not application defaults and must not be placed in Terraform state or secret
storage.

- Seller payout: `0xbaF8eb4B7C5Dce1C80E1C3d308a89f5F55Ac366f`
- Buyer wallet: `0xb4F21d0fC70b56B4e586E4629b19d1Cc58fe39F3`
- Exact price: `100000` atomic units (`0.1 USDC` at 6 decimals)

The buyer's funded test wallet is required only to perform the real testnet
release purchase. Keep its key in the browser wallet or an isolated release
runner secret. Never provide it to the API Lambda, seller onboarding, source
control, Terraform state, logs, screenshots, or evidence objects.

## Preconditions

1. Record the deployed commit SHA, UTC start time, frontend origin, HTTP API
   origin, and AWS environment without recording credentials.
2. Confirm the Lambda environment reports `AGENTPAY_PAYMENT_MODE=x402` and the
   exact locked facilitator, network, and asset values above.
3. Confirm mock payment mode, Stripe/card checkout, platform transaction fees,
   mainnet configuration, and Bedrock are disabled.
4. Confirm the seller has an active entitlement, verified service endpoint,
   verified Base Sepolia payment destination, and one published fixed-price
   product using that destination.
5. Confirm the buyer-controlled wallet is on Base Sepolia and has enough
   testnet USDC for two small purchases. Record only its public address and
   balance; never record the private key or seed phrase.
6. Create an evidence directory outside source control. Redact cookies,
   authorization headers, `PAYMENT-SIGNATURE`, raw proofs, and one-time tokens.

## Manual wallet signatures

The seller first connects the wallet whose address exactly matches the payout
address above. AgentPay requests `personal_sign` over the one-time ownership
challenge returned by the API. The seller approves that message signature; it
does not transfer funds.

For the purchase, connect the buyer wallet above and let AgentPay request a
switch to Base Sepolia (`0x14a34`). Review the wallet's EIP-712
`TransferWithAuthorization` prompt and confirm all of these values before
signing:

- `from`: `0xb4F21d0fC70b56B4e586E4629b19d1Cc58fe39F3`
- `to`: `0xbaF8eb4B7C5Dce1C80E1C3d308a89f5F55Ac366f`
- `value`: `100000`
- domain chain ID: `84532`
- verifying contract: `0x036CbD53842c5426634e7929541eC2318f3dCF7e`
- primary type: `TransferWithAuthorization`
- `validAfter`: `0`
- `validBefore`: the current Unix time plus the challenge's
  `maxTimeoutSeconds`
- `nonce`: the fresh random 32-byte value shown in the prompt

Approve the typed-data signature in the wallet. AgentPay packages that
signature into the x402 `PAYMENT-SIGNATURE` request header and submits it to the
API; never copy a private key or seed phrase into AgentPay, a terminal, logs,
or the evidence bundle.

## REL-003 — one real testnet transaction

1. Open the published product from the deployed storefront and confirm the
   visible `Testnet only` label, exact amount, Base Sepolia network, USDC asset,
   and seller payment address.
2. Set a buyer maximum equal to or above the seller quote and create the
   immutable purchase intent.
3. Save the redacted `402` response status and decoded non-secret payment
   requirements. Do not save the raw payment signature.
4. Authorize the exact payment in the buyer wallet and retry the paid request.
5. Record the successful response, transaction ID, safe on-chain payment
   reference, finality, seller invocation ID, receipt hash, and evidence-chain
   verification result.
6. Confirm the seller destination received the exact testnet amount and the
   upstream service was invoked exactly once.

## REL-004 — buyer ceiling and exact authorization

1. Submit the same product with `maximumAmount` one atomic unit below the frozen
   seller quote.
2. Confirm intent creation fails validation and no `402` challenge,
   transaction, payment verification, settlement, or seller invocation occurs.
3. Repeat with a sufficient maximum and confirm the challenge still requests
   the seller's exact quote, not the buyer maximum.
4. Confirm no buyer-approval endpoint, token, WebSocket, or `428` branch appears.

## REL-005 — duplicate and dispute outcomes

1. Replay the completed payment request concurrently and confirm the payment
   identifier remains unique and the seller invocation count remains one.
2. Exercise a controlled seller timeout/non-delivery fixture and confirm a
   `not_delivered` dispute becomes `refund_recommended` from recorded facts.
3. Exercise a `quality_or_output` dispute and confirm it enters seller review
   rather than receiving an automatic quality judgment.
4. Record one seller-completed external full refund against the recommended
   dispute. Confirm exact idempotent replay returns the original record and a
   changed request is rejected. Label it seller-reported; AgentPay does not move
   funds.

## REL-008 — browser and agent parity

1. Complete one browser-wallet purchase and one agent/x402 purchase for the
   same published product using separate intents and payment proofs.
2. Confirm both use the same seller quote, destination, network, asset,
   verification, finality, execution-capability, fulfillment, receipt,
   evidence, and dispute rules.
3. Confirm both transactions appear once in the seller dashboard and the Base
   Sepolia USDC totals equal the two exact quotes without cross-asset summing.

## Evidence bundle

Preserve a redacted manifest containing:

- commit SHA and deployment identifiers;
- UTC timestamps;
- testnet facilitator, network, asset, and seller public payout address;
- transaction IDs and safe payment references;
- HTTP statuses and documented error codes;
- seller invocation counts;
- receipt schema versions and evidence root/head hashes;
- dashboard reconciliation result; and
- pass/fail notes for each release task.

Do not mark REL-003, REL-004, REL-005, or REL-008 complete until the deployed
environment produces this evidence and every observed value matches the
authoritative transaction and evidence records.
