# Coding-agent setup bundle contract

Status: **Locked for AUT-007; setup bundle v2 is planned in SEO-002**.

AgentPay publishes deterministic setup bundles for Claude Code, Codex, and
generic Model Context Protocol hosts. Bundle schema versions are independent
from the MCP protocol version. Version 2 adds stack identification and the
technical SEO/AEO workflow while preserving the payment and publication safety
boundaries from version 1.

## Published resources

The authenticated MCP server exposes these read-only resources:

| Host | Resource URI |
|---|---|
| Claude Code | `agentpay://integration/setup/v1/claude-code` |
| Codex | `agentpay://integration/setup/v1/codex` |
| Generic MCP | `agentpay://integration/setup/v1/generic-mcp` |

Version 2 resources use the same host names under
`agentpay://integration/setup/v2/<host>`. Version 1 remains readable until all
published clients have migrated.

Each JSON resource contains:

- the bundle schema version and target host;
- the MCP configuration path and a configuration template;
- the environment-variable names for the MCP URL and integration credential;
- pinned verification-package installation instructions for Go, Node.js, and
  Python;
- a framework-specific focused test command;
- the ordered integration workflow; and
- the prompt used to prepare a reviewable seller-repository change.

Version 2 additionally contains the detected stack, support tier, stack-native
integration notes, and the required SEO/AEO validation checklist.

Templates may contain `${AGENTPAY_MCP_URL}` and
`${AGENTPAY_INTEGRATION_TOKEN}` references. They never contain a resolved
credential, seller signing secret, wallet material, approval token, or
deployment credential.

## Host configuration

- Claude Code uses a project-scoped `.mcp.json` entry with `type: "http"` and
  environment-variable expansion in `url` and `headers`.
- Codex uses project-scoped `.codex/config.toml` with a Streamable HTTP `url`
  and `bearer_token_env_var = "AGENTPAY_INTEGRATION_TOKEN"`.
- Generic hosts receive an AgentPay-neutral JSON descriptor that names the
  Streamable HTTP transport, endpoint environment variable, and bearer-token
  environment variable. The host must map those values into its own supported
  configuration format.

## Setup prompt

The MCP prompt `prepare_agentpay_integration` requires `host` and `stack`
arguments in version 2. Supported hosts are `claude-code`, `codex`, and
`generic-mcp`. The coding agent must detect the stack from committed package and
framework manifests and must reject a caller-provided stack that conflicts with
the repository.

The prompt instructs the coding agent to:

1. inspect repository instructions and existing tests;
2. read the authenticated seller, route, and setup-bundle resources;
3. analyze only the allowlisted repository manifest and OpenAPI contract;
4. install the maintained AgentPay verification package;
5. add raw-body signature verification before fulfillment and the dedicated
   side-effect-free `POST /.well-known/agentpay/sandbox` endpoint;
6. generate storefront discovery and integration code from published routes;
7. generate stack-native title and description metadata, canonical URLs,
   Open Graph and social metadata, robots directives, sitemap entries,
   semantically correct product content, and truthful JSON-LD supported by
   visible page facts;
8. generate and cross-check `llms.txt` and the AgentPay storefront manifest for
   agent and answer-engine discovery;
9. add focused signature, stale-request, replay, payment-gating, sandbox, SEO,
   accessibility, and metadata-consistency tests;
10. run the repository's existing checks and the bundle's focused test; and
11. present the diff, validation result, route proposals, SEO/AEO changes, and
   commands for seller review.

The prompt must state that the coding agent cannot invent prices, publish a
route, rotate credentials, or deploy production changes without explicit
seller confirmation. It must also state that generated SEO/AEO work cannot
guarantee ranking and cannot use hidden text, keyword stuffing, fabricated
reviews, unsupported structured data, or claims absent from visible content.

## Version 1 package targets

| Framework | Package | Pinned install command | Focused test command |
|---|---|---|---|
| Go | `github.com/fourgeez/agentpay/verification/go` | `go get github.com/fourgeez/agentpay/verification/go@v0.1.0` | `go test ./...` |
| Node.js | `@agentpay/verify-node` | `npm install --save-exact @agentpay/verify-node@0.1.0` | `npm test` |
| Python | `agentpay-verify` | `python -m pip install agentpay-verify==0.1.0` | `python -m unittest discover -v` |

Package publication is a release operation outside this repository task. The
bundle pins the first package contract so setup behavior cannot drift silently.

## Version 2 supported-stack matrix

| Support tier | Stacks |
|---|---|
| Maintained verification and setup fixtures | Go `net/http`; Node.js/Express; Python ASGI/FastAPI/Starlette |
| Planned adapters using maintained language verification primitives | Next.js, React/Vite with a Node API, Remix, Nuxt, SvelteKit, Astro, Fastify, NestJS, Gin, Echo, Fiber, Flask, Django |
| Not advertised until dedicated verification packages pass | ASP.NET Core, Spring Boot, Rails, Laravel |

The generic MCP host is not a generic framework implementation. A stack moves
to the maintained tier only after raw-body handling, middleware order, replay
storage, sandbox behavior, metadata generation, and focused tests pass in a
committed fixture.
