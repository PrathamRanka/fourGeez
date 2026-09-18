# Verified sources and implementation unknowns

Last reviewed: 2026-09-18.

Only official documentation and repositories should determine protocol wire behavior, SDK imports, AWS resource behavior, and security-sensitive configuration. Blog posts may provide context but cannot override these sources.

## x402

| Topic | Source | Status |
|---|---|---|
| Seller quickstart and current HTTP flow | https://docs.x402.org/getting-started/quickstart-for-sellers | Verified 2026-09-17 |
| Protocol repository | https://github.com/coinbase/x402 | Verified as official repository entry point 2026-09-17 |
| v2 HTTP headers | Official x402 documentation/repository: `PAYMENT-REQUIRED`, `PAYMENT-SIGNATURE`, `PAYMENT-RESPONSE` | Verified 2026-09-17 |
| Go SDK module path and pinned version | Official `x402-foundation/x402` Go module and `go/v2.9.0` release tag | Verified 2026-09-17: module `github.com/x402-foundation/x402/go`; pin `v0.0.0-20260413171033-1059e866484f` because the module omits a `/v2` suffix |
| Standard-library HTTP adapter | Official `go/http/nethttp` package added in Go SDK `v2.8.0` | Verified 2026-09-17; compatible with AgentPay's `net/http` transport |
| Testnet facilitator URL | Official seller quickstart | Verified 2026-09-17: `https://x402.org/facilitator` |
| Testnet network identifier | Official seller quickstart and Go SDK network constants | Verified 2026-09-17: Base Sepolia `eip155:84532` |
| Testnet asset identifier | Official Go SDK Base Sepolia asset configuration | Verified 2026-09-17: USDC `0x036CbD53842c5426634e7929541eC2318f3dCF7e` |
| Protocol headers | Official Go HTTP package and tests | Verified 2026-09-17: `PAYMENT-REQUIRED`, `PAYMENT-SIGNATURE`, and `PAYMENT-RESPONSE` |
| License and compatibility | Official repository `LICENSE` and Go module | Verified 2026-09-17: Apache-2.0; SDK requires Go 1.24 and AgentPay uses Go 1.26 |
| Security review notes | Official Go changelog and module dependencies | Reviewed 2026-09-17: `v2.6.0` closed fail-open verification paths; `v2.8.0` pins the indirect QUIC security fix. AgentPay will use only the core and HTTP client types required at its payment boundary. |
| EVM ownership proof envelope | https://eips.ethereum.org/EIPS/eip-191 | Verified 2026-09-17: initial `eip155` destination ownership uses the version `0x45` personal-sign envelope before EOA recovery. |

Protocol rule: `docs/api/openapi.yaml` defines AgentPay's surrounding API, but the official x402 SDK defines payment payload serialization and verification. If they conflict, update the AgentPay contract before implementing.

## AWS

| Topic | Official source | Key implementation consequence |
|---|---|---|
| Terraform language and workflow | https://developer.hashicorp.com/terraform/language | Pin providers and modules, use remote state, and review plans before apply. |
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

## Seller subscription billing

| Topic | Official source | Key implementation consequence |
|---|---|---|
| Stripe subscription webhooks | https://docs.stripe.com/billing/subscriptions/webhooks | Verified 2026-09-18: subscription and invoice changes are asynchronous; `invoice.paid` confirms a paid period, while failures require recovery handling. |
| Stripe subscription status | https://docs.stripe.com/api/subscriptions/object | Verified 2026-09-18: adapter inputs include `incomplete`, `incomplete_expired`, `trialing`, `active`, `past_due`, `canceled`, `unpaid`, and `paused`; AgentPay maps these into its own entitlement states. |
| Stripe cancellation | https://docs.stripe.com/billing/subscriptions/cancel | Verified 2026-09-18: `cancel_at_period_end` preserves the subscription through the current paid period; AgentPay independently enforces its exact `accessEndsAt`. |
| Stripe webhook signatures | https://docs.stripe.com/webhooks/signature | Verified 2026-09-18: verify `Stripe-Signature` against the exact raw request body with the endpoint secret. |
| Stripe webhook delivery behavior | https://docs.stripe.com/webhooks | Verified 2026-09-18: duplicate delivery can occur and event ordering is not guaranteed; persist event IDs and reconcile current provider state. |

## Seller automation

| Topic | Official source | Key implementation consequence |
|---|---|---|
| MCP protocol | https://modelcontextprotocol.io/specification/2026-07-28 | Pin the protocol version and expose only declared resources, prompts, and tools. |
| MCP authorization | https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization | Remote seller integrations use standards-based authorization, seller scope, and least privilege. |
| OAuth protected resource metadata | https://www.rfc-editor.org/rfc/rfc9728.html | Publish the canonical `/mcp` resource and authorization-server locations; reverified 2026-09-18. |
| MCP Streamable HTTP transport | https://modelcontextprotocol.io/specification/2026-07-28/basic/transports/streamable-http | Generic setup describes one authenticated Streamable HTTP endpoint without inventing a client-specific configuration schema. Verified 2026-09-17. |
| Codex MCP configuration | https://developers.openai.com/codex/mcp | Use project-scoped `.codex/config.toml`, `url`, `bearer_token_env_var`, `required`, and write-tool approval mode; do not write bearer values into the file. Reverified 2026-09-18. |
| Claude Code MCP configuration | https://code.claude.com/docs/en/mcp | Use project-scoped `.mcp.json`, explicit `type: "http"`, and environment expansion for URL and authorization header values. Reverified 2026-09-18. |

Host-specific configuration must be rechecked against these official sources
before changing a published setup-bundle version.

## Implementation libraries

| Purpose | Source | Pinned version |
|---|---|---|
| RFC 8785 JSON canonicalization for request and intent hashing | https://github.com/gowebpki/jcs | `v1.0.1` |
| Sortable ULID generation | https://github.com/oklog/ulid | `v2.1.2` |
| x402 v2 Go SDK | https://github.com/x402-foundation/x402/tree/go/v2.9.0/go | `v0.0.0-20260413171033-1059e866484f` (`go/v2.9.0`) |
| Official MCP Go SDK | https://github.com/modelcontextprotocol/go-sdk | `v1.8.0`; supports the pinned `2026-07-28` protocol and stateless Streamable HTTP |

## Open questions that block production, not the hackathon

- Which production payment providers and reimbursement mechanisms will be supported?
- What contractual evidence do card networks and stablecoin providers accept?
- What data retention and deletion commitments will design partners require?
- Which party bears loss for authorization, duplication, non-delivery, and quality disputes?
- Which agent/principal identity standard will be supported after the seller-only MVP?
