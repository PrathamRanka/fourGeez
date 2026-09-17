# AgentPay MCP contract

Status: **Locked for AUT-003**.

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

AUT-003 exposes no tools, prompts, subscriptions, arbitrary files, shell
commands, or unrestricted HTTP requests. Mutations are introduced only by
AUT-004 and require stronger scopes, idempotency, and explicit confirmation.
