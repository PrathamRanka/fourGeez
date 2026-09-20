import { describe, expect, it } from "vitest";
import {
  createMCPConfiguration,
  createPowerShellSetup,
  setupPrompt,
} from "@/features/onboarding/model";

describe("seller MCP host configuration", () => {
  it("assigns repository edits to the seller's coding agent", () => {
    expect(setupPrompt).toContain(
      "You are the seller's coding agent. Edit this repository",
    );
    expect(setupPrompt).toContain(
      "Use AgentPay MCP tools for bounded analysis, configuration, and verification",
    );
    expect(setupPrompt).toContain(
      "AgentPay cloud mutations require explicit seller confirmation",
    );
  });

  it.each([
    ["claude-code", '"type": "stdio"', '"mcpServers"'],
    [
      "codex",
      "[mcp_servers.agentpay]",
      'env_vars = ["AGENTPAY_API_BASE_URL", "AGENTPAY_PROJECT_KEY"]',
    ],
    [
      "generic-mcp",
      '"schemaVersion": "agentpay.mcp-connection.v1"',
      '"transport": "stdio"',
    ],
  ] as const)("uses the local connector for %s", (host, marker, hostShape) => {
    const configuration = createMCPConfiguration(host);
    expect(configuration).toContain(marker);
    expect(configuration).toContain(hostShape);
    expect(configuration).toContain("mcp-connector\\\\0.1.0");
    expect(configuration).toContain("dist\\\\cli.js");
    expect(configuration).toContain("AGENTPAY_PROJECT_KEY");
    expect(configuration).not.toContain("npx");
    expect(configuration).not.toContain("Authorization");
    expect(configuration).not.toContain("/mcp");
  });

  it.each([
    ["claude-code", "claude"],
    ["codex", "codex"],
    ["generic-mcp", "generic MCP host"],
  ] as const)(
    "provides a secret-safe Windows preflight for %s",
    (host, startMarker) => {
      const setup = createPowerShellSetup(
        "https://api.agentpay.example/",
        host,
      );
      expect(setup).toContain(
        'Read-Host "Paste the project key shown once" -MaskInput',
      );
      expect(setup).toContain("$ConnectorEntry");
      expect(setup).toContain("node $ConnectorEntry --check");
      expect(setup).not.toContain("npx");
      expect(setup).toContain(startMarker);
      expect(setup).not.toContain("apc2.");
    },
  );
});
