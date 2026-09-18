# AgentPay MCP contract

Status: **Locked through SEO-002**.

Implementation status: this document describes the currently implemented
M0–M7 direct integration-credential transport. It is not the production
cancellation-enforcement design. LCH-012 through LCH-018 will replace ordinary
MCP use of permanent credentials with short-lived, seller-scoped access tokens
and current entitlement checks while preserving the official MCP transport.

AgentPay exposes the official Model Context Protocol `2026-07-28` over
stateless Streamable HTTP at `POST /mcp`. Requests and responses use the
official MCP JSON-RPC schemas. The server does not create HTTP sessions and
limits each request body to 1 MiB.

## Authentication

Every request requires `Authorization: Bearer <integrationCredential>`. The
credential is authenticated through the same seller-scoped credential service
used by the control API and must include the `read` scope. Missing, malformed,
expired, revoked, or unknown credentials return `401`. Valid credentials
without `read` return `403`. Repository failures return a generic `500` without
exposing internal details.

Every authenticated `POST /mcp` request consumes one `mcp_operation` unit from
the seller's UTC-month plan quota before JSON-RPC dispatch. A suspended plan
returns HTTP `403` with `permission_denied`; an exhausted monthly quota returns
HTTP `429` with `rate_limited` and `Retry-After`. Authentication failures do not
consume quota. Retried webhook delivery quota is separate and does not affect
MCP usage.

Resource identity is derived only from the authenticated credential. Resource
URIs never accept a caller-supplied seller identifier.

## Read-only resources

| URI | MIME type | Contents |
|---|---|---|
| `agentpay://seller` | `application/json` | Seller profile without owner identity or signing-secret reference |
| `agentpay://storefront` | `application/json` | Public storefront manifest containing enabled routes |
| `agentpay://routes` | `application/json` | All configured routes for the authenticated seller, including disabled routes |
| `agentpay://transactions/summary` | `application/json` | Status counts and redacted metadata for the 100 newest transactions; `hasMore` reports truncation |
| `agentpay://integration/documentation` | `text/markdown` | Versioned setup and safety guidance for coding agents |
| `agentpay://integration/setup/v1/claude-code` | `application/json` | Claude Code project configuration and AgentPay integration workflow |
| `agentpay://integration/setup/v1/codex` | `application/json` | Codex project configuration and AgentPay integration workflow |
| `agentpay://integration/setup/v1/generic-mcp` | `application/json` | Host-neutral Streamable HTTP configuration and AgentPay integration workflow |
| `agentpay://integration/setup/v2/claude-code` | `application/json` | Claude Code configuration, stack matrix, verification setup, and SEO/AEO generation requirements |
| `agentpay://integration/setup/v2/codex` | `application/json` | Codex configuration, stack matrix, verification setup, and SEO/AEO generation requirements |
| `agentpay://integration/setup/v2/generic-mcp` | `application/json` | Host-neutral configuration, stack matrix, verification setup, and SEO/AEO generation requirements |

The transaction summary excludes buyer identity, payment identifiers, payment
proof hashes, response bodies, evidence payloads, and seller secrets.

The server exposes no subscriptions, arbitrary files, shell commands, or
unrestricted HTTP requests.

## Setup prompt

The read-scoped `prepare_agentpay_integration` prompt now requires `host` and
`stack`. It selects setup bundle v2, includes the stack's support tier,
stack-native routing and metadata conventions, and pins the maintained language
verification package. Unknown and unsupported stacks fail closed. Version-one
resources remain readable for existing clients, but new prompt invocations use
version two.

The prompt requires technical SEO, AEO, semantic visible content, truthful
JSON-LD, canonical URLs, robots and sitemap output, `llms.txt`, AgentPay
manifest consistency, accessibility checks, and performance budgets. It also
states that these changes cannot guarantee ranking and prohibits hidden or
fabricated search content.

## Mutation tools

All mutation inputs reject unknown fields and include:

- `idempotencyKey`: 8-128 visible ASCII characters;
- `confirmation.approved`: must be `true`;
- `confirmation.summary`: 10-500 characters describing the exact commercial
  change shown to the seller; and
- `confirmation.confirmedAt`: RFC 3339 UTC timestamp no more than ten minutes
  old and not in the future.

The MCP host must populate confirmation metadata only after an explicit seller
action. Agent or repository text is not authorization.

| Tool | Scope | Behavior |
|---|---|---|
| `configure_storefront` | `configure` | Updates the existing seller display name and upstream base URL using an expected version. Initial seller creation stays in the seller API/dashboard because credentials are seller-scoped. |
| `configure_route` | `configure` | Creates a validated `enabled=false` paid-route draft. |
| `change_route_price` | `configure` | Updates the authoritative price for future intents using an expected version. |
| `validate_route` | `validate` | Returns deterministic publication checks without persisting state. |
| `sandbox_validate_route` | `validate` | Probes the dedicated seller sandbox endpoint for discovery, signature, payment-gating, and replay behavior without persisting success. |
| `publish_route` | `publish` | Re-runs deterministic and sandbox validation, then conditionally enables one draft route using an expected version. |
| `analyze_repository` | `validate` | Parses an allowlisted repository manifest and OpenAPI contract into deterministic, unpublished route proposals. |
| `validate_storefront_artifacts` | `validate` | Validates bounded generated metadata, canonical URLs, robots, sitemap, JSON-LD, semantic content, `llms.txt`, manifest consistency, accessibility facts, and performance budgets without producing a ranking score. |

Idempotency is bound to credential, operation, target, and canonical request
bytes. A replay returns the stored redacted result; reuse with different input
returns a conflict. Tools never accept seller IDs, signing secrets, deployment
credentials, arbitrary URLs, shell commands, or raw repository contents.

Publishing checks the seller's current enabled-route count before changing a
draft. Static route or webhook-subscription exhaustion returns
`permission_denied`; it is not represented as a transient monthly rate limit.

### Repository analysis input

`analyze_repository` accepts at most 512 KiB of OpenAPI JSON or YAML and this
allowlisted manifest only: `schemaVersion`, `serviceName`, `framework`, and
`openapiPath`. Framework is one of `go`, `node`, or `python`, and
`schemaVersion` is `agentpay.repository.v1`. Source files, environment values,
credentials, customer records, prompts, and deployment configuration are not
accepted.

The analyzer proposes at most 50 literal `GET` or `POST` operations, sorted by
path and method. Templated paths and unsupported methods are reported as
rejections. Proposals include method, path, description, and response MIME type
only. They never invent price, asset, network, payout address, approval policy,
publication state, or deployment changes.
