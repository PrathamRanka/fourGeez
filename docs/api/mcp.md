# AgentPay MCP contract

Status: **M7.1 contract implemented locally by LCH-010; deployed verification is pending**.

Implementation status: the historical M0–M7 runtime used direct integration
credentials. The current Go service implements the short-lived connector
contract below and rejects project keys at `/mcp`. Public seller use remains blocked on
AWS-005/AWS-006, approved connector and verification-package distribution, and
the M9 deployed release checks. The local implementation must not be described
as a production service until those dependencies pass.

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

Lean V1 supports the AgentPay local connector as the only production MCP client
path. Direct remote OAuth clients and protected-resource metadata are deferred.
This does not weaken the boundary: the project key is accepted only by the
bootstrap exchange, and `/mcp` accepts only short-lived access capabilities.

Every `POST /mcp` request requires
`Authorization: Bearer <mcpAccessToken>`. The JWT header requires
`typ=agentpay-access+jwt`, `alg=ES256`, and a known `kid` from
`/.well-known/jwks.json`. Claims are exact `iss`, exact `aud`,
`sub=credentialId`, `sellerId`, `credentialId`, space-delimited `scope`,
`entitlementEpoch`, unique `jti`, `iat`, and `exp`.

A missing or invalid bearer token returns `401` with a Bearer challenge and the
stable JSON error envelope.

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

The read-scoped `prepare_agentpay_integration` prompt requires `host` and
`stack`. Before selecting it, the coding agent calls the validate-scoped
`detect_repository_stacks` tool with bounded committed manifest and source
evidence. The selected stack must appear in that deterministic result. The
prompt then selects setup bundle v2, includes the stack's support tier,
stack-native routing and metadata conventions, and pins the maintained language
verification package. Unknown, unsupported, and unevidenced stacks fail closed.
Version-one resources remain readable for existing clients, but new prompt
invocations use version two.

The prompt requires technical SEO, AEO, semantic visible content, truthful
JSON-LD, canonical URLs, robots and sitemap output, `llms.txt`, AgentPay
manifest consistency, accessibility checks, and performance budgets. It also
states that these changes cannot guarantee ranking and prohibits hidden or
fabricated search content.

## Mutation tools

Production state-changing tool inputs reject unknown fields and include:

- `idempotencyKey`: 8-128 visible ASCII characters;
- `confirmationGrant`: an opaque cloud-issued one-time grant returned by
  `POST /v1/sellers/{sellerId}/mcp-confirmation-grants`; and
- the exact expected seller or route version reviewed when that grant was
  issued.

`configure_route` adds `expectedSellerVersion`; the other state-changing tools
use their existing seller or route version field. The grant request and the MCP
tool arguments must name the same value.

The authenticated seller dashboard displays the canonical change and asks its
trusted browser/BFF boundary to mint the grant. The grant expires within five
minutes and is bound to the authenticated seller, selected credential, exact
tool, target, RFC 8785 canonical arguments hash using
`agentpay.mcp-mutation.v1`, and expected resource version. The local connector
only relays it. An MCP bearer, project key, model, repository, prompt, or forked
connector cannot mint or approve it.

The M7 Go compatibility inputs `confirmation.approved`,
`confirmation.summary`, and `confirmation.confirmedAt` are not production
authority. Production must withhold state-changing MCP tools until the cloud
grant path is implemented; it must never silently accept those caller-asserted
fields as a fallback.

| Tool | Scope | Behavior |
|---|---|---|
| `configure_storefront` | `configure` | Consumes a matching confirmation grant and updates the existing seller display name and upstream base URL using the bound seller version. Initial seller creation stays in the seller API/dashboard because credentials are seller-scoped. |
| `configure_route` | `configure` | Consumes a matching confirmation grant bound to the seller catalog version and creates a validated `enabled=false` paid-route draft with a seller-approved `displayName` and `productSlug`; the slug is normalized and reserved uniquely within the seller. |
| `change_route_price` | `configure` | Consumes a matching confirmation grant and updates the authoritative price for future intents using the bound route version. |
| `validate_route` | `validate` | Returns deterministic publication checks without persisting state; no confirmation grant is required. |
| `sandbox_validate_route` | `validate` | Probes the dedicated seller sandbox endpoint for discovery, signature, payment-gating, and replay behavior without persisting success. |
| `publish_route` | `publish` | Consumes a matching confirmation grant, re-runs deterministic and sandbox validation, then conditionally enables one draft route using the bound route version. |
| `detect_repository_stacks` | `validate` | Reads only bounded committed dependency manifests and allowlisted source markers, then returns every evidenced maintained stack in the fixed 21-stack matrix order. It does not read files itself or accept environment values, credentials, customer data, or deployment configuration. |
| `analyze_repository` | `validate` | Parses an allowlisted repository manifest and OpenAPI contract into deterministic, unpublished route proposals. |
| `validate_storefront_artifacts` | `validate` | Validates bounded generated metadata, canonical URLs, robots, sitemap, JSON-LD, semantic content, `llms.txt`, manifest consistency, accessibility facts, and performance budgets without producing a ranking score. |

Idempotency is bound to credential, operation, target, and canonical request
bytes excluding the confirmation secret. Grant consumption and the mutation
idempotency decision are atomic. An exact replay returns the stored redacted
result; reuse with different input returns `idempotency_conflict`, while reuse
of a consumed grant for another request returns `token_replayed`. A missing,
expired, revoked, or mismatched grant returns `permission_denied` before any
domain mutation. Tools never accept seller IDs, signing secrets, deployment
credentials, arbitrary URLs, shell commands, or raw repository contents.

`detect_repository_stacks` accepts a `files` map containing at most 100
repository-relative allowlisted paths and at most 1 MiB total content. The
allowlist is `package.json`, `go.mod`, `.go`, `requirements.txt`,
`pyproject.toml`, `.csproj`, `pom.xml`, `.gradle`, `.gradle.kts`, `Gemfile`, and
`composer.json`; nested manifests are supported. Environment files, lockfiles,
absolute paths, traversal, arbitrary source files, and unknown top-level fields
are rejected.

Publishing checks the seller's current enabled-route count before changing a
draft. Static route or webhook-subscription exhaustion returns
`permission_denied`; it is not represented as a transient monthly rate limit.

### Repository analysis input

`analyze_repository` accepts at most 512 KiB of OpenAPI JSON or YAML and this
allowlisted manifest only: `schemaVersion`, `serviceName`, `framework`, and
`openapiPath`. Framework is one of `go`, `node`, `python`, `dotnet`, `java`,
`ruby`, or `php`, and
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
