import { spawnSync } from "node:child_process";
import { pathToFileURL } from "node:url";

export const localSecurityChecks = Object.freeze([
  {
    name: "seller session expiry, refresh, recovery copy, and sealed-token handling",
    command: "npm",
    args: ["run", "test", "--workspace", "@agentpay/web", "--", "session-refresh.test.ts", "auth-pages.test.tsx", "sealed-session.test.ts"],
  },
  {
    name: "revocation, suspension, stale discovery, replay, cancellation, receipt, and evidence denial",
    command: "node",
    args: ["scripts/go-tool.mjs", "test", "./internal/authorization", "./internal/integrations", "./internal/storefront", "./internal/intents", "./internal/payments", "./internal/transactions", "./internal/evidence", "-count=1"],
  },
  {
    name: "webhook overlap rotation, signing, retry, dead-letter, and redelivery",
    command: "node",
    args: ["scripts/go-tool.mjs", "test", "./internal/notifications", "./internal/persistence/memory", "./internal/persistence/dynamodb", "-count=1"],
  },
  {
    name: "API and deployable harness contracts",
    command: "npm",
    args: ["run", "lint:contracts"],
  },
]);

export const pendingExternalProofs = Object.freeze([
  "canonical-origin eight-hour seller-session expiry and sign-in recovery",
  "operator suspension with stale seller-hosted discovery and publication/commerce denial",
  "webhook replacement delivery, predecessor disablement, retry drain, and DLQ evidence",
  "seller-integration credential rotation against a still-running connector",
  "deployed revocation, replay, cancellation-race, stale-capability, receipt, and evidence denial",
]);

export function runLocalSecurityRegression({ stdout = process.stdout, stderr = process.stderr } = {}) {
  for (const check of localSecurityChecks) {
    stdout.write(`\n[security] ${check.name}\n`);
    const usesNpm = check.command === "npm";
    const command = usesNpm ? process.execPath : check.command;
    const args = usesNpm ? [process.env.npm_execpath, ...check.args] : check.args;
    const result = spawnSync(command, args, {
      cwd: process.cwd(),
      encoding: "utf8",
    });
    if (result.stdout) stdout.write(result.stdout);
    if (result.stderr) stderr.write(result.stderr);
    if (result.status !== 0) return result.status ?? 1;
  }
  stdout.write("\nLocal security regression passed. No live AWS proof was executed.\n");
  return 0;
}

function main() {
  if (process.argv.includes("--list")) {
    process.stdout.write(JSON.stringify({ localSecurityChecks, pendingExternalProofs }, null, 2) + "\n");
    return;
  }
  process.exitCode = runLocalSecurityRegression();
}

if (import.meta.url === pathToFileURL(process.argv[1] ?? "").href) main();
