# Coding-agent setup bundle contract

Status: **Locked for AUT-007**.

AgentPay publishes deterministic setup bundles for Claude Code, Codex, and
generic Model Context Protocol hosts. Bundle schema version
`agentpay.setup.v1` is independent from the MCP protocol version and changes
only when a consumer-visible bundle field or workflow requirement changes.

## Published resources

The authenticated MCP server exposes these read-only resources:

| Host | Resource URI |
|---|---|
| Claude Code | `agentpay://integration/setup/v1/claude-code` |
| Codex | `agentpay://integration/setup/v1/codex` |
| Generic MCP | `agentpay://integration/setup/v1/generic-mcp` |

Each JSON resource contains:

- the bundle schema version and target host;
- the MCP configuration path and a configuration template;
- the environment-variable names for the MCP URL and integration credential;
- pinned verification-package installation instructions for Go, Node.js, and
  Python;
- a framework-specific focused test command;
- the ordered integration workflow; and
- the prompt used to prepare a reviewable seller-repository change.

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

The MCP prompt `prepare_agentpay_integration` requires `host` and `framework`
arguments. Supported hosts are `claude-code`, `codex`, and `generic-mcp`.
Supported frameworks are `go`, `node`, and `python`.

The prompt instructs the coding agent to:

1. inspect repository instructions and existing tests;
2. read the authenticated seller, route, and setup-bundle resources;
3. analyze only the allowlisted repository manifest and OpenAPI contract;
4. install the maintained AgentPay verification package;
5. add raw-body signature verification before fulfillment;
6. generate storefront discovery and integration code from published routes;
7. add focused signature, stale-request, replay, and payment-gating tests;
8. run the repository's existing checks and the bundle's focused test; and
9. present the diff, validation result, route proposals, and commands for
   seller review.

The prompt must state that the coding agent cannot invent prices, publish a
route, rotate credentials, or deploy production changes without explicit
seller confirmation.

## Version 1 package targets

| Framework | Package | Pinned install command | Focused test command |
|---|---|---|---|
| Go | `github.com/fourgeez/agentpay/verification/go` | `go get github.com/fourgeez/agentpay/verification/go@v0.1.0` | `go test ./...` |
| Node.js | `@agentpay/verify-node` | `npm install --save-exact @agentpay/verify-node@0.1.0` | `npm test` |
| Python | `agentpay-verify` | `python -m pip install agentpay-verify==0.1.0` | `python -m unittest discover -v` |

Package publication is a release operation outside this repository task. The
bundle pins the first package contract so setup behavior cannot drift silently.
