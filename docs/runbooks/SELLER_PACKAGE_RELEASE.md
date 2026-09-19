# Seller package release and installation

This runbook covers `@agentpay/local-mcp-connector` and
`@agentpay/merchant-sdk` version `0.1.0`. Neither package is published to an npm
registry. The supported Lean V1 distribution is a pair of proprietary npm
tarballs attached to the protected immutable tag
`agentpay-packages-v0.1.0`, with `SHA256SUMS` and `provenance.json` from the same
build.

The repository is proprietary. Do not distribute an artifact until the owners
have approved customer-use terms, contributor provenance, and the included
notices. Possession of a tarball alone does not grant use rights.

## Package audit

The release gate verifies these package properties:

| Package                         | Runtime surface                                                                     | Distribution result                                                                               |
| ------------------------------- | ----------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------- |
| `@agentpay/local-mcp-connector` | One `agentpay-mcp` executable; no library export and no runtime dependency          | Restricted `files` allowlist; installed executable is exercised from the tarball                  |
| `@agentpay/merchant-sdk`        | ESM `main`, conditional `exports`, TypeScript declarations, and no lifecycle script | Exact `@agentpay/verify-node` 0.1.0 runtime is bundled so offline installation does not query npm |

Both packages remain `private: true`, require Node.js 24 or newer, include the
package license and notice, and expose no install-time scripts. The release
test rejects tests, source maps, environment files, caches, common credential
formats, private-key markers, or a project-key assignment in packed content.
The legal customer-use agreement and historical contributor-provenance review
remain release-owner gates; this technical workflow does not resolve them.

## Release-owner procedure

Requirements are Node.js 24 or newer, npm 11 or newer, a clean reviewed Git
commit, and no credentials in the shell command or repository.

```powershell
git status --short
npm ci
npm run test:mcp-connector
npm run test:merchant-sdk
npm run typecheck:mcp-connector
npm run typecheck:merchant-sdk
npm run test:package-distribution

$ReleaseDirectory = Resolve-Path .\artifacts -ErrorAction SilentlyContinue
if (-not $ReleaseDirectory) {
  New-Item -ItemType Directory -Path .\artifacts | Out-Null
}

npm run package:release -- --output .\artifacts\agentpay-packages-v0.1.0
npm run package:verify -- --directory .\artifacts\agentpay-packages-v0.1.0
```

The build refuses a dirty worktree. It runs `npm pack --dry-run --json`, rejects
unexpected or secret-bearing files, builds both tarballs, and writes:

```text
agentpay-local-mcp-connector-0.1.0.tgz
agentpay-merchant-sdk-0.1.0.tgz
provenance.json
SHA256SUMS
```

Before upload, compare `provenance.json` with `git rev-parse HEAD`. Protect and
sign the release tag according to repository governance. Upload all four files
without renaming them. Do not run `npm publish`; both manifests deliberately
retain `private: true`.

## Download and checksum verification on Windows

The release must already exist and its tag must be protected before a seller
runs these commands. If distribution is access-controlled, replace
`$ReleaseBaseUrl` with the exact authorized immutable artifact URL supplied in
the AgentPay dashboard.

```powershell
$Version = "0.1.0"
$ReleaseTag = "agentpay-packages-v$Version"
$ReleaseBaseUrl = "https://github.com/PrathamRanka/fourGeez/releases/download/$ReleaseTag"
$DownloadDirectory = Join-Path $env:USERPROFILE "Downloads\AgentPay-$ReleaseTag"
New-Item -ItemType Directory -Force -Path $DownloadDirectory | Out-Null

$Files = @(
  "agentpay-local-mcp-connector-$Version.tgz",
  "agentpay-merchant-sdk-$Version.tgz",
  "SHA256SUMS",
  "provenance.json"
)
foreach ($File in $Files) {
  Invoke-WebRequest -Uri "$ReleaseBaseUrl/$File" -OutFile (Join-Path $DownloadDirectory $File)
}

$ExpectedHashes = @{}
Get-Content (Join-Path $DownloadDirectory "SHA256SUMS") | ForEach-Object {
  if ($_ -notmatch '^([a-f0-9]{64})  ([A-Za-z0-9@._/-]+\.tgz)$') {
    throw "Invalid AgentPay checksum manifest"
  }
  $ExpectedHashes[$Matches[2]] = $Matches[1]
}

foreach ($PackageFile in $Files.Where({ $_.EndsWith(".tgz") })) {
  $ActualHash = (Get-FileHash -Algorithm SHA256 (Join-Path $DownloadDirectory $PackageFile)).Hash.ToLowerInvariant()
  if ($ActualHash -ne $ExpectedHashes[$PackageFile]) {
    throw "Checksum verification failed for $PackageFile"
  }
}
```

Inspect `provenance.json` and require its full `source.commit` to match the
protected release tag. Stop if the checksum, commit, package version, or file
name differs from the dashboard release record.

## Install the local connector

Install into a versioned user-local directory. This keeps the executable stable
for MCP hosts and does not alter the seller repository.

```powershell
$ConnectorRoot = Join-Path $env:LOCALAPPDATA "AgentPay\mcp-connector\0.1.0"
New-Item -ItemType Directory -Force -Path $ConnectorRoot | Out-Null
Push-Location $ConnectorRoot
npm init --yes | Out-Null
npm install --offline --ignore-scripts --no-audit --no-fund --save-exact (Join-Path $DownloadDirectory "agentpay-local-mcp-connector-0.1.0.tgz")
Pop-Location

$ConnectorEntry = Join-Path $ConnectorRoot "node_modules\@agentpay\local-mcp-connector\dist\cli.js"
$env:AGENTPAY_API_BASE_URL = Read-Host "Paste the AgentPay API base URL shown in the dashboard"
$env:AGENTPAY_PROJECT_KEY = Read-Host "Paste the reveal-once AgentPay project key" -MaskInput
node $ConnectorEntry --check
```

Keep that PowerShell session open when starting the MCP host. The project key
is inherited by the connector process but is not written to the host's project
configuration. Set it again in a new terminal session; do not use `setx`, commit
it to `.env`, or pass it through `--env` command-line arguments.

## Claude Code

From the seller repository, in the same PowerShell session:

```powershell
claude mcp remove agentpay 2>$null
claude mcp add --transport stdio --scope project agentpay -- node $ConnectorEntry
claude mcp get agentpay
claude
```

The generated project configuration contains only the `node` command and the
absolute connector path. Review it before committing and confirm that it does
not contain `AGENTPAY_PROJECT_KEY` or any secret value.

## Codex

From the seller repository, in the same PowerShell session:

```powershell
codex mcp remove agentpay 2>$null
codex mcp add agentpay -- node $ConnectorEntry
codex mcp get agentpay
codex
```

Codex inherits `AGENTPAY_API_BASE_URL` and `AGENTPAY_PROJECT_KEY` from the
launching PowerShell process. Do not use `codex mcp add --env` for the project
key because command history and persistent configuration are not acceptable
secret stores.

## Generic stdio MCP host

Configure the host with an absolute path and no embedded environment values:

```json
{
  "mcpServers": {
    "agentpay": {
      "command": "node",
      "args": [
        "C:\\Users\\SELLER\\AppData\\Local\\AgentPay\\mcp-connector\\0.1.0\\node_modules\\@agentpay\\local-mcp-connector\\dist\\cli.js"
      ]
    }
  }
}
```

Launch the generic host from the PowerShell session that contains the two
AgentPay environment variables. If the host does not inherit environment
variables, use its operating-system secret-store integration; never place the
project key in JSON or TOML.

## Install the merchant SDK

Run this inside the seller's Node.js or TypeScript server project after the
checksum verification above:

```powershell
npm install --offline --ignore-scripts --no-audit --no-fund --save-exact (Join-Path $DownloadDirectory "agentpay-merchant-sdk-0.1.0.tgz")
```

The tarball includes the exact Node verifier runtime from the same source
commit. No AgentPay registry package is fetched during installation. Import the
SDK only in server-side code and keep webhook secrets, seller credentials, and
adapter credentials in the seller's secret manager.

## Rotation and removal

A package upgrade is a new versioned directory and verified tarball; never
overwrite an older artifact in place. After the host points at the new version
and its preflight succeeds, remove the old host entry and directory. Rotating
or revoking the project key remains a separate dashboard action and immediately
affects cloud authorization regardless of the locally installed package.
