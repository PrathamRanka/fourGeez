# AgentPay MCP contract

Status: **Locked through AUT-004**.

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

The transaction summary excludes buyer identity, payment identifiers, payment
proof hashes, response bodies, evidence payloads, and seller secrets.

The server exposes no subscriptions, arbitrary files, shell commands, or
unrestricted HTTP requests.

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
| `publish_route` | `publish` | Re-runs validation and conditionally enables one draft route using an expected version. |
| `analyze_repository` | `validate` | Parses an allowlisted repository manifest and OpenAPI contract into deterministic, unpublished route proposals. |

Idempotency is bound to credential, operation, target, and canonical request
bytes. A replay returns the stored redacted result; reuse with different input
returns a conflict. Tools never accept seller IDs, signing secrets, deployment
credentials, arbitrary URLs, shell commands, or raw repository contents.

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
