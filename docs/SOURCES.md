# Verified sources and implementation unknowns

Last reviewed: 2026-09-17.

Only official documentation and repositories should determine protocol wire behavior, SDK imports, AWS resource behavior, and security-sensitive configuration. Blog posts may provide context but cannot override these sources.

## x402

| Topic | Source | Status |
|---|---|---|
| Seller quickstart and current HTTP flow | https://docs.x402.org/getting-started/quickstart-for-sellers | Verified 2026-09-17 |
| Protocol repository | https://github.com/coinbase/x402 | Verified as official repository entry point 2026-09-17 |
| v2 HTTP headers | Official x402 documentation/repository: `PAYMENT-REQUIRED`, `PAYMENT-SIGNATURE`, `PAYMENT-RESPONSE` | Verified 2026-09-17 |
| Go SDK module path and pinned version | Must be confirmed from the official repository immediately before PAY-001 | **Unverified; do not add a guessed dependency** |
| Testnet facilitator URL, network identifier, asset identifier, and funding procedure | Must be copied from the selected official quickstart during PAY-001 | **Unverified configuration** |

Protocol rule: `docs/api/openapi.yaml` defines AgentPay's surrounding API, but the official x402 SDK defines payment payload serialization and verification. If they conflict, update the AgentPay contract before implementing.

## AWS

| Topic | Official source | Key implementation consequence |
|---|---|---|
| CDK v2 bootstrapping | https://docs.aws.amazon.com/cdk/v2/guide/bootstrapping.html | Bootstrap each target account/region before deployment. |
| Bedrock model access | https://docs.aws.amazon.com/bedrock/latest/userguide/model-access.html | Model/provider access and regional availability must be checked before demo deployment. |
| Bedrock regional model support | https://docs.aws.amazon.com/bedrock/latest/userguide/models-region-compatibility.html | Model ID remains deployment configuration, not a hardcoded architecture decision. |
| S3 Object Lock | https://docs.aws.amazon.com/AmazonS3/latest/userguide/object-lock.html | Object Lock prevents overwrite/deletion according to retention mode. |
| Object Lock configuration | https://docs.aws.amazon.com/AmazonS3/latest/userguide/object-lock-configure.html | Enabling Object Lock has irreversible bucket/versioning consequences; create and retain deliberately. |
| API Gateway WebSocket APIs | https://docs.aws.amazon.com/apigateway/latest/developerguide/apigateway-websocket-api.html | Use `$connect`, `$disconnect`, and `$default`; REST remains authoritative state. |
| WebSocket route selection | https://docs.aws.amazon.com/apigateway/latest/developerguide/websocket-api-develop-routes.html | Use `$request.body.action` only if client-to-server message actions are needed. |

## Product reference

| Topic | Source | Usage |
|---|---|---|
| ZeroClick public architecture and capabilities | https://docs.zeroclick.ai/llms.txt | Behavioral reference only; do not copy source code, branding, text, private APIs, or undocumented behavior. |

## Open questions that block production, not the hackathon

- Which production payment providers and reimbursement mechanisms will be supported?
- What contractual evidence do card networks and stablecoin providers accept?
- What data retention and deletion commitments will design partners require?
- Which party bears loss for authorization, duplication, non-delivery, and quality disputes?
- Which agent/principal identity standard will be supported after the seller-only MVP?

