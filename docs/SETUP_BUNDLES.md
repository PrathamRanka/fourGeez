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

The prompt is executed by the seller's coding agent. That agent reads and edits
the local repository. AgentPay MCP provides bounded analysis, configuration,
and verification tools plus seller-confirmed cloud mutations; neither the cloud
MCP server nor the local connector writes repository files.

Version 2 also includes a Windows PowerShell setup sequence and a connector
preflight command. The sequence sets the API base URL and project key only in
the current process environment, verifies the requested scopes by exchanging
the project key for a short-lived capability, and then starts the selected MCP
host. It never writes the raw project key into `.mcp.json`,
`.codex/config.toml`, `agentpay.mcp.json`, or committed files.

Version 2 additionally contains the complete explicit stack matrix, each
stack's support tier, pinned language verification setup when available,
stack-native integration notes, and the required SEO/AEO validation checklist.
Unsupported stacks remain visible in the matrix but have no verification setup
and cannot be selected by the setup prompt.

Templates launch the checksum-verified `@agentpay/local-mcp-connector` 0.1.0
GitHub Release artifact from its versioned user-local installation directory.
They may name `AGENTPAY_API_BASE_URL` and `AGENTPAY_PROJECT_KEY` as inherited
environment variables. `AGENTPAY_MCP_SCOPES` is optional. Templates never
contain a resolved credential, seller signing secret, wallet material,
approval token, deployment credential, npm registry invocation, mutable Git
URL, or vendored tarball path.

## Host configuration

- Claude Code uses a project-scoped `.mcp.json` stdio entry that runs `node`
  with the absolute path to the installed connector entry point and inherits
  the API base URL and project key from the launching shell.
- Codex uses project-scoped `.codex/config.toml` with the same versioned entry
  point and
  only the two required allowlisted environment-variable names. The optional
  `AGENTPAY_MCP_SCOPES` variable is omitted so Codex does not treat it as a
  required startup input; the connector's fixed default scopes apply.
- Generic hosts receive an AgentPay-neutral JSON descriptor naming the stdio
  command, pinned connector entry point, required environment variables, and
  separately labeled optional environment variables. The host must map those
  values into its supported local-process configuration.

The connector exchanges the project key only at
`POST /v1/integration-access-tokens`, keeps the returned 2-5 minute capability
in process memory, and relays bounded JSON-RPC to `/mcp`. The project key is
never sent to `/mcp`, and direct remote OAuth configuration is not published in
Lean V1.

Running the pinned connector with `--check` performs configuration validation,
the proprietary project-key exchange, and one read-only authenticated MCP
initialization. It does not invoke a seller tool or mutation. Success reports no
token, seller identifier, credential identifier, or secret. Failure output is
limited to an actionable static configuration message or sanitized HTTP status,
stable error code, request ID, and retry delay.

The seller dashboard exposes these setup artifacts only after the
server-authoritative onboarding prerequisites pass. It reports project-key
lifecycle separately from connector authorization: creating a key does not
mean a connector is connected. The first authenticated MCP initialization
records connector verification idempotently. Dashboard diagnostics distinguish
`disconnected`, `connected`, `expired`, and `revoked`, retain no raw key after
the reveal-once response, and pair failures with safe retry instructions.

## Setup prompt

The MCP prompt `prepare_agentpay_integration` requires `host` and `stack`
arguments in version 2. Supported hosts are `claude-code`, `codex`, and
`generic-mcp`. Before selecting the prompt, the coding agent calls
`detect_repository_stacks` with bounded committed package/framework manifests
and allowlisted source markers. It must select one returned stack that owns the
paid route and reject a caller-provided stack absent from that result.

The prompt instructs the coding agent to:

1. inspect repository instructions and existing tests;
2. call `detect_repository_stacks` with only committed allowlisted evidence and select the stack that owns the paid route;
3. read the authenticated seller, route, and setup-bundle resources;
4. analyze only the allowlisted repository manifest and OpenAPI contract;
5. install the maintained AgentPay verification package;
6. add raw-body signature verification before fulfillment and the dedicated
   side-effect-free `POST /.well-known/agentpay/sandbox` endpoint returning the
   closed `agentpay.sandbox.v2` response with the supplied route ID/version and
   `ready: true`;
7. generate storefront discovery and integration code from published routes;
8. generate stack-native title and description metadata, canonical URLs,
   Open Graph and social metadata, robots directives, sitemap entries,
   semantically correct product content, and truthful JSON-LD supported by
   visible page facts;
9. generate and cross-check `llms.txt` and the AgentPay storefront manifest for
   agent and answer-engine discovery;
10. add focused signature, stale-request, replay, payment-gating, sandbox, SEO,
   accessibility, and metadata-consistency tests;
11. measure the generated artifacts, call `validate_storefront_artifacts`, and
    fix every failed deterministic check;
12. run the repository's existing checks and the bundle's focused test; and
13. present the diff, validation result, route proposals, SEO/AEO changes, and
   commands for seller review.

The prompt must state that the coding agent cannot invent prices, publish a
route, select or change a payout address, rotate credentials, or deploy
production changes without explicit seller confirmation. Numeric prices and
payout destinations are seller-provided inputs, not model proposals. It must
also state that generated SEO/AEO work cannot
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

The `detect_repository_stacks` input allowlist is limited to repository-relative
`package.json`, `go.mod`, `.go`, `requirements.txt`, `pyproject.toml`, `.csproj`,
`pom.xml`, `.gradle`, `.gradle.kts`, `Gemfile`, and `composer.json` paths. Nested
manifests are supported for monorepos. Any other path, absolute path, traversal,
more than 100 files, or more than 1 MiB of total evidence is rejected before
detection.

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
