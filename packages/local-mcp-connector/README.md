# `@agentpay/local-mcp-connector`

The required local stdio bridge between a seller-owned MCP host and AgentPay's
cloud-authorized MCP endpoint. It exchanges a reveal-once project key for a
short-lived access capability, keeps that capability in memory, and proxies
bounded JSON-RPC messages.

The connector does not verify or settle payments, sign AgentPay capabilities,
publish without cloud confirmation, or store the project key on disk.

## Installation

This package is not published to npm. Authorized Lean V1 sellers install the
exact downloaded release tarball after verifying `SHA256SUMS`:

```powershell
npm install --ignore-scripts --no-audit --no-fund --save-exact .\agentpay-local-mcp-connector-0.1.0.tgz
```

Do not use an unpinned branch URL or attempt to install this monorepo's
workspace subdirectory directly from Git. See
`docs/runbooks/SELLER_PACKAGE_RELEASE.md` in the matching source release for
the complete Windows, Claude Code, Codex, and generic-host procedure.

## Runtime

The MCP host must inherit these values from its launch environment:

- `AGENTPAY_API_BASE_URL` — the exact AgentPay API origin;
- `AGENTPAY_PROJECT_KEY` — the reveal-once project credential; and
- optional `AGENTPAY_MCP_SCOPES` — a subset of `read configure publish validate`.

Run the secret-safe connectivity check before configuring a host:

```powershell
agentpay-mcp --check
```

Never place the project key in a repository, MCP JSON/TOML file, command-line
argument, log, issue, or support message.
