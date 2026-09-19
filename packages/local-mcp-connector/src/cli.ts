#!/usr/bin/env node

import { CloudMcpTransport } from "./cloud-transport.js";
import {
  ConnectorConfigurationError,
  ConnectorHttpError,
  ConnectorProtocolError,
} from "./errors.js";
import { runStdioProxy } from "./stdio-proxy.js";
import { AccessTokenClient } from "./token-client.js";

const DEFAULT_SCOPES = ["read", "configure", "publish", "validate"] as const;
const DEFAULT_TIMEOUT_MS = 10_000;
const DEFAULT_MAX_MESSAGE_BYTES = 1024 * 1024;
const PREFLIGHT_ARGUMENT = "--check";
const PREFLIGHT_REQUEST_ID = "agentpay-preflight";
const PREFLIGHT_INITIALIZE_REQUEST = JSON.stringify({
  jsonrpc: "2.0",
  id: PREFLIGHT_REQUEST_ID,
  method: "initialize",
  params: {
    protocolVersion: "2026-07-28",
    capabilities: {},
    clientInfo: { name: "agentpay-connector-preflight", version: "0.1.0" },
  },
});

async function main(): Promise<void> {
  const preflight = parseArguments(process.argv.slice(2));
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
  if (preflight) {
    const response = await transport.send(
      PREFLIGHT_INITIALIZE_REQUEST,
      shutdown.signal,
    );
    validatePreflightResponse(response);
    process.stdout.write(
      "AgentPay connector preflight passed: project key exchange and MCP authorization succeeded.\n",
    );
    return;
  }
  await runStdioProxy({
    input: process.stdin,
    output: process.stdout,
    diagnostics: process.stderr,
    transport,
    maxMessageBytes,
    signal: shutdown.signal,
  });
}

function validatePreflightResponse(response: string | undefined): void {
  if (response === undefined) {
    throw new ConnectorProtocolError(
      "AgentPay MCP preflight returned no response",
    );
  }
  let payload: unknown;
  try {
    payload = JSON.parse(response);
  } catch {
    throw new ConnectorProtocolError(
      "AgentPay MCP preflight returned invalid JSON",
    );
  }
  if (
    typeof payload !== "object" ||
    payload === null ||
    Array.isArray(payload) ||
    (payload as Record<string, unknown>).id !== PREFLIGHT_REQUEST_ID ||
    !("result" in payload) ||
    "error" in payload
  ) {
    throw new ConnectorProtocolError("AgentPay MCP preflight was rejected");
  }
}

function parseArguments(argumentsList: readonly string[]): boolean {
  if (argumentsList.length === 0) {
    return false;
  }
  if (argumentsList.length === 1 && argumentsList[0] === PREFLIGHT_ARGUMENT) {
    return true;
  }
  throw new ConnectorConfigurationError(
    "supported usage is agentpay-mcp or agentpay-mcp --check",
  );
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
  process.stderr.write(`${safeFailureMessage(error)}\n`);
  process.exitCode = 1;
});

function safeFailureMessage(error: unknown): string {
  if (error instanceof ConnectorConfigurationError) {
    return `AgentPay connector stopped: ${error.message}`;
  }
  if (error instanceof ConnectorHttpError) {
    const fields = [
      `AgentPay connector stopped: HTTP ${error.status}`,
      error.code ? `code=${safeDiagnosticValue(error.code)}` : undefined,
      error.requestId
        ? `requestId=${safeDiagnosticValue(error.requestId)}`
        : undefined,
      error.retryAfterSeconds === undefined
        ? undefined
        : `retryAfter=${error.retryAfterSeconds}s`,
    ].filter((field): field is string => field !== undefined);
    return fields.join(" ");
  }
  const errorName = error instanceof Error ? error.name : "ConnectorError";
  return `AgentPay connector stopped: ${safeDiagnosticValue(errorName)}`;
}

function safeDiagnosticValue(value: string): string {
  return /^[A-Za-z0-9_.:-]{1,128}$/u.test(value) ? value : "redacted";
}
