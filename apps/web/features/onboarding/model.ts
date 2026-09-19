import type { ActionResult } from "@/lib/agentpay-api";

export type SellerStatus = "draft" | "active" | "suspended";
export type PaymentDestinationStatus =
  "pending_verification" | "active" | "disabled" | "rotated";
export type IntegrationScope =
  "read" | "configure" | "publish" | "validate" | "rotate";

export type Seller = {
  sellerId: string;
  name: string;
  slug: string;
  upstreamBaseUrl: string;
  status: SellerStatus;
  version: number;
};

export type PaymentDestination = {
  destinationId: string;
  sellerId: string;
  asset: string;
  network: string;
  address: string;
  status: PaymentDestinationStatus;
  version: number;
};

export type IntegrationCredential = {
  credentialId: string;
  sellerId: string;
  label: string;
  scopes: IntegrationScope[];
  version: number;
  revokedAt?: string | null;
  expiresAt?: string | null;
  lastUsedAt?: string | null;
};

export type CredentialCreated = IntegrationCredential & {
  token: string;
};

export type PreparedPaymentDestination = {
  destination: PaymentDestination;
  challenge: string;
};

export type OnboardingSnapshot = {
  seller: Seller | null;
  paymentDestinations: PaymentDestination[];
  credentials: IntegrationCredential[];
  onboarding: SellerOnboardingState;
};

export type OnboardingStepName =
  | "account_verified"
  | "storefront_created"
  | "service_connection_verified"
  | "subscription_active"
  | "payment_destination_verified"
  | "project_key_created"
  | "connector_verified"
  | "product_configured"
  | "sandbox_purchase"
  | "storefront_previewed";

export type SellerOnboardingStep = {
  name: OnboardingStepName;
  status: "complete" | "incomplete" | "blocked";
  blocking: boolean;
  message: string;
};

export type SellerOnboardingState = {
  sellerId: string | null;
  complete: boolean;
  currentStep?: OnboardingStepName;
  steps: SellerOnboardingStep[];
  publication: {
    allowed: boolean;
    blockers: OnboardingStepName[];
  };
  version: number;
  updatedAt?: string | null;
};

export type MCPHost = "claude-code" | "codex" | "generic-mcp";

export type CreateStorefrontInput = {
  name: string;
  slug: string;
  upstreamBaseUrl: string;
};

export type PreparePaymentDestinationInput = {
  sellerId: string;
  asset: string;
  network: string;
  address: string;
};

export type VerifyPaymentDestinationInput = {
  sellerId: string;
  destinationId: string;
  challenge: string;
  signature: string;
};

export type CreateIntegrationCredentialInput = {
  sellerId: string;
};

export type { ActionResult } from "@/lib/agentpay-api";

export type OnboardingActions = {
  createStorefront: (
    input: CreateStorefrontInput,
  ) => Promise<ActionResult<Seller>>;
  preparePaymentDestination: (
    input: PreparePaymentDestinationInput,
  ) => Promise<ActionResult<PreparedPaymentDestination>>;
  verifyPaymentDestination: (
    input: VerifyPaymentDestinationInput,
  ) => Promise<ActionResult<PaymentDestination>>;
  createIntegrationCredential: (
    input: CreateIntegrationCredentialInput,
  ) => Promise<ActionResult<CredentialCreated>>;
};

export const setupPrompt = `Connect this project to AgentPay. Inspect only bounded committed manifests and OpenAPI, detect one maintained stack, propose sellable routes plus truthful SEO/AEO changes, install AgentPay request verification, and generate focused tests. Ask me for every exact price and payout destination. Never invent or change prices or payout addresses, publish, rotate credentials, or deploy without my explicit confirmation.`;

export function createMCPConfiguration(host: MCPHost): string {
  if (host === "codex") {
    return `[mcp_servers.agentpay]\ncommand = "npx"\nargs = ["--yes", "@agentpay/local-mcp-connector@0.1.0"]\nenv_vars = ["AGENTPAY_API_BASE_URL", "AGENTPAY_PROJECT_KEY"]\nrequired = true`;
  }
  if (host === "generic-mcp") {
    return JSON.stringify(
      {
        schemaVersion: "agentpay.mcp-connection.v1",
        name: "agentpay",
        transport: "stdio",
        command: "npx",
        args: ["--yes", "@agentpay/local-mcp-connector@0.1.0"],
        requiredEnvironmentVariables: [
          "AGENTPAY_API_BASE_URL",
          "AGENTPAY_PROJECT_KEY",
        ],
        optionalEnvironmentVariables: [
          "AGENTPAY_MCP_SCOPES",
          "AGENTPAY_REQUEST_TIMEOUT_MS",
          "AGENTPAY_MAX_MESSAGE_BYTES",
        ],
      },
      null,
      2,
    );
  }
  return JSON.stringify(
    {
      mcpServers: {
        agentpay: {
          type: "stdio",
          command: "npx",
          args: ["--yes", "@agentpay/local-mcp-connector@0.1.0"],
          env: {
            AGENTPAY_API_BASE_URL: "${AGENTPAY_API_BASE_URL}",
            AGENTPAY_PROJECT_KEY: "${AGENTPAY_PROJECT_KEY}",
          },
        },
      },
    },
    null,
    2,
  );
}

export function createPowerShellSetup(
  apiOrigin: string,
  host: MCPHost,
): string {
  const startCommand =
    host === "claude-code"
      ? "claude"
      : host === "codex"
        ? "codex"
        : "# Start your generic MCP host after importing agentpay.mcp.json";
  return `$env:AGENTPAY_API_BASE_URL = "${apiOrigin.replace(/\/$/, "")}"\n$env:AGENTPAY_PROJECT_KEY = Read-Host "Paste the project key shown once" -MaskInput\nnpx --yes @agentpay/local-mcp-connector@0.1.0 --check\n${startCommand}`;
}
