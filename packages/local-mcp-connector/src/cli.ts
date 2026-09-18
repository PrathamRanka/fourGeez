#!/usr/bin/env node

import { CloudMcpTransport } from "./cloud-transport.js";
import { ConnectorConfigurationError } from "./errors.js";
import { runStdioProxy } from "./stdio-proxy.js";
import { AccessTokenClient } from "./token-client.js";

const DEFAULT_SCOPES = ["read", "configure", "publish", "validate"] as const;
const DEFAULT_TIMEOUT_MS = 10_000;
const DEFAULT_MAX_MESSAGE_BYTES = 1024 * 1024;

async function main(): Promise<void> {
  const baseUrl = requiredEnvironmentValue("AGENTPAY_API_BASE_URL");
  const projectKey = requiredEnvironmentValue("AGENTPAY_PROJECT_KEY");
  const scopes = parseScopes(process.env.AGENTPAY_MCP_SCOPES);
  const timeoutMs = parsePositiveInteger(
    process.env.AGENTPAY_REQUEST_TIMEOUT_MS,
    DEFAULT_TIMEOUT_MS,
    "AGENTPAY_REQUEST_TIMEOUT_MS",
  );
  const maxMessageBytes = parsePositiveInteger(
    process.env.AGENTPAY_MAX_MESSAGE_BYTES,
    DEFAULT_MAX_MESSAGE_BYTES,
    "AGENTPAY_MAX_MESSAGE_BYTES",
  );
  const shutdown = new AbortController();
  process.once("SIGINT", () => shutdown.abort());
  process.once("SIGTERM", () => shutdown.abort());

  const tokenClient = new AccessTokenClient({
    baseUrl,
    projectKey,
    scopes,
    timeoutMs,
  });
  const transport = new CloudMcpTransport({
    baseUrl,
    tokenClient,
    timeoutMs,
    maxMessageBytes,
  });
  await runStdioProxy({
    input: process.stdin,
    output: process.stdout,
    diagnostics: process.stderr,
    transport,
    maxMessageBytes,
    signal: shutdown.signal,
  });
}

function requiredEnvironmentValue(name: string): string {
  const value = process.env[name];
  if (!value) {
    throw new ConnectorConfigurationError(`${name} is required`);
  }
  return value;
}

function parseScopes(value: string | undefined): readonly string[] {
  if (!value) {
    return DEFAULT_SCOPES;
  }
  const scopes = value.split(/[ ,]+/u).filter(Boolean);
  if (scopes.length === 0) {
    throw new ConnectorConfigurationError("AGENTPAY_MCP_SCOPES is invalid");
  }
  return scopes;
}

function parsePositiveInteger(
  value: string | undefined,
  fallback: number,
  name: string,
): number {
  if (value === undefined) {
    return fallback;
  }
  const parsed = Number(value);
  if (!Number.isSafeInteger(parsed) || parsed <= 0) {
    throw new ConnectorConfigurationError(`${name} must be a positive integer`);
  }
  return parsed;
}

main().catch((error: unknown) => {
  const errorName = error instanceof Error ? error.name : "ConnectorError";
  process.stderr.write(`AgentPay connector stopped: ${errorName}\n`);
  process.exitCode = 1;
});
