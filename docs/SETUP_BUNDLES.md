# Coding-agent setup bundle contract

Status: **Locked through LCH-010**.

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
| Claude Code v2 | `agentpay://integration/setup/v2/claude-code` |
| Codex v2 | `agentpay://integration/setup/v2/codex` |
| Generic MCP v2 | `agentpay://integration/setup/v2/generic-mcp` |

Version 2 resources use the same host names under
`agentpay://integration/setup/v2/<host>`. Version 1 remains readable until all
published clients have migrated.

Each JSON resource contains:

- the bundle schema version and target host;
- the MCP configuration path and a configuration template;
- the environment-variable names for the AgentPay API base URL and project key;
- pinned verification-package installation instructions for Go, Node.js, and
  Python;
- a framework-specific focused test command;
- the ordered integration workflow; and
- the prompt used to prepare a reviewable seller-repository change.

Version 2 additionally contains the complete explicit stack matrix, each
stack's support tier, pinned language verification setup when available,
stack-native integration notes, and the required SEO/AEO validation checklist.
Unsupported stacks remain visible in the matrix but have no verification setup
and cannot be selected by the setup prompt.

Templates launch the pinned `@agentpay/local-mcp-connector@0.1.0` package over
stdio and may contain `${AGENTPAY_API_BASE_URL}` and `${AGENTPAY_PROJECT_KEY}`
references. `AGENTPAY_MCP_SCOPES` is optional. Templates never contain a
resolved credential, seller signing secret, wallet material, approval token,
or deployment credential.

## Host configuration

- Claude Code uses a project-scoped `.mcp.json` stdio entry that runs the local
  connector with `npx` and passes the API base URL and project key as process
  environment variables.
- Codex uses project-scoped `.codex/config.toml` with the same stdio command and
  allowlisted environment-variable names.
- Generic hosts receive an AgentPay-neutral JSON descriptor naming the stdio
  command, pinned connector package, and required environment variables. The
  host must map those values into its supported local-process configuration.

The connector exchanges the project key only at
`POST /v1/integration-access-tokens`, keeps the returned 2-5 minute capability
in process memory, and relays bounded JSON-RPC to `/mcp`. The project key is
never sent to `/mcp`, and direct remote OAuth configuration is not published in
Lean V1.

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
10. measure the generated artifacts, call `validate_storefront_artifacts`, and
    fix every failed deterministic check;
11. run the repository's existing checks and the bundle's focused test; and
12. present the diff, validation result, route proposals, SEO/AEO changes, and
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
| .NET | `AgentPay.Verify` | `dotnet add package AgentPay.Verify --version 0.1.0` | `dotnet test` |
| Java | `com.agentpay:agentpay-verify-spring` | `./mvnw dependency:get -Dartifact=com.agentpay:agentpay-verify-spring:0.1.0` | `./mvnw test` |
| Ruby | `agentpay-verify` | `bundle add agentpay-verify --version 0.1.0 --strict` | `bundle exec rails test` |
| PHP | `agentpay/verify` | `composer require agentpay/verify:0.1.0` | `php artisan test` |

Package publication is a release operation outside this repository task. The
bundle pins the first package contract so setup behavior cannot drift silently.

## Version 2 supported-stack matrix

| Support tier | Stacks |
|---|---|
| Maintained verification and setup fixtures | Next.js, React/Vite with a Node API, Remix, Nuxt, SvelteKit, Astro, Express, Fastify, NestJS, Go `net/http`, Gin, Echo, Fiber, FastAPI, Starlette, Flask, Django, ASP.NET Core, Spring Boot, Rails, Laravel |
| Planned adapters using maintained language verification primitives | None |
| Not advertised until dedicated verification packages pass | None |

The generic MCP host is not a generic framework implementation. A stack moves
to the maintained tier only after raw-body handling, middleware order, replay
storage, sandbox behavior, metadata generation, and focused tests pass in a
committed fixture.

### Deterministic stack evidence

Stack detection reads only committed dependency manifests and bounded source
markers supplied by the coding agent. It does not read `.env` files, lockfile
credentials, deployment secrets, or arbitrary repository contents. Package
dependencies are stronger evidence than source markers and are matched by exact
package name, never substring guessing.

| Stack | Required evidence | Initial tier |
|---|---|---|
| Next.js | `package.json` dependency `next` | maintained |
| React/Vite | dependencies `react` and `vite`, without a stronger metaframework match | maintained |
| Remix | dependency `@remix-run/react` | maintained |
| Nuxt | dependency `nuxt` | maintained |
| SvelteKit | dependency `@sveltejs/kit` | maintained |
| Astro | dependency `astro` | maintained |
| Express | dependency `express` | maintained |
| Fastify | dependency `fastify` | maintained |
| NestJS | dependency `@nestjs/core` | maintained |
| Go `net/http` | `go.mod` plus a `.go` source marker importing `net/http`, without a stronger Go framework match | maintained |
| Gin | `go.mod` module `github.com/gin-gonic/gin` | maintained |
| Echo | `go.mod` module `github.com/labstack/echo` | maintained |
| Fiber | `go.mod` module `github.com/gofiber/fiber` | maintained |
| FastAPI | Python dependency `fastapi` | maintained |
| Starlette | Python dependency `starlette`, without FastAPI | maintained |
| Flask | Python dependency `flask` | maintained |
| Django | Python dependency `django` | maintained |
| ASP.NET Core | `.csproj` with `Microsoft.AspNetCore` evidence | maintained |
| Spring Boot | Maven or Gradle dependency containing `spring-boot` | maintained |
| Rails | `Gemfile` dependency `rails` | maintained |
| Laravel | `composer.json` dependency `laravel/framework` | maintained |

When multiple application layers are present, detection returns every evidenced
stack in deterministic matrix order. Setup must select the stack that owns the
paid route and reject a caller-provided stack absent from the detected set.
