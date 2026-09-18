# AgentPay MCP contract

Status: **M7.1 production target contract locked by LCH-004; not yet served by the M7 runtime**.

Implementation status: M0–M7 still uses direct integration credentials at
runtime. LCH-012 through LCH-018 must implement the short-lived contract below
before production use. The contract is authoritative while runtime work remains
incomplete.

AgentPay exposes the official Model Context Protocol `2026-07-28` over
stateless Streamable HTTP at `POST /mcp`. Requests and responses use the
official MCP JSON-RPC schemas. The server does not create HTTP sessions and
limits each request body to 1 MiB.

## Authentication

Project-key installations must use the AgentPay local connector. The seller
project key is accepted only by the proprietary
`POST /v1/integration-access-tokens` bootstrap exchange. This endpoint is not
OAuth and clients must not send OAuth grant parameters. It authenticates
`X-AgentPay-Project-Key`, checks current credential and entitlement state,
narrows requested scopes to `read`, `configure`, `publish`, or `validate`, and returns an ES256 access token for
`aud=urn:agentpay:mcp` with a 120–300 second lifetime and
`Cache-Control: no-store`. The connector keeps that token in process memory and
proxies local MCP traffic; a project key is never configured directly as the
remote `/mcp` bearer token.

Standards-compatible clients that connect directly to the remote `/mcp`
resource use OAuth authorization and discover the protected resource metadata
at `/.well-known/oauth-protected-resource/mcp`. That document identifies the
exact MCP resource URL, authorization server metadata, supported bearer method,
and scopes. OAuth authorization creates or references a revocable AgentPay
integration-credential identity, so direct-client access tokens retain the same
seller, credential, scope, entitlement-epoch, and revocation checks as connector
tokens. Direct clients never receive the seller project key.

Every `POST /mcp` request requires
`Authorization: Bearer <mcpAccessToken>`. The JWT header requires
`typ=agentpay-access+jwt`, `alg=ES256`, and a known `kid` from
`/.well-known/jwks.json`. Claims are exact `iss`, exact `aud`,
`sub=credentialId`, `sellerId`, `credentialId`, space-delimited `scope`,
`entitlementEpoch`, unique `jti`, `iat`, and `exp`.

A missing or invalid bearer token returns `401` with
`WWW-Authenticate: Bearer resource_metadata="<absolute metadata URL>"` and the
stable JSON error envelope. The resource metadata URL must use HTTPS in
production, match the configured MCP origin, and is never derived from an
untrusted forwarding header.

Before dispatch, AgentPay verifies the signature and claims, current credential
revocation, current entitlement epoch and access boundary, target ownership,
exact operation scope, and quota. A top-level `read` scope never authorizes a
mutation. A modified local connector cannot bypass these cloud checks or obtain
signing material.

Every authorized request consumes one `mcp_operation` unit before JSON-RPC
dispatch. Authentication and authorization failures do not consume quota.

| HTTP | Code | Meaning |
|---:|---|---|
| 401 | `invalid_credential` | Missing, malformed, unknown, or invalid signature |
| 401 | `token_expired` | Capability is outside its accepted time window |
| 401 | `token_revoked` | Credential is revoked or entitlement epoch is stale |
| 403 | `subscription_inactive` | Seller access is not currently entitled |
| 403 | `insufficient_scope` | Token lacks the exact resource/tool scope |
| 403 | `permission_denied` | Ownership, feature, confirmation, or static entitlement denied |
| 409 | `idempotency_conflict` | Mutation key was reused with different canonical input |
| 429 | `rate_limited` | Operation quota exceeded; includes `Retry-After` |
| 503 | `dependency_unavailable` | Current authorization state cannot be established |

JSON-RPC method and parameter errors are returned only after HTTP authorization
succeeds. Internal repository details are never returned.

Resource identity is derived only from the authenticated capability. Resource
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
| `configure_route` | `configure` | Creates a validated `enabled=false` paid-route draft with a seller-approved `displayName` and `productSlug`; the slug is normalized and reserved uniquely within the seller. |
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

The coding agent may propose a product display name and slug for seller review,
but `configure_route` must carry both values explicitly inside the confirmed
commercial change. AgentPay normalizes both fields and rejects a seller-scoped
slug collision. `routeId` and `pathPattern` remain separate immutable technical
identifiers and are never inferred from the public slug.
