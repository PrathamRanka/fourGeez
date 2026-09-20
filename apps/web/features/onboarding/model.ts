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

export type IntegrationVerificationCheckName =
  | "endpoint_reachability"
  | "signed_exchange"
  | "schema_contract"
  | "fulfillment_readiness"
  | "payment_gating"
  | "replay_idempotency";

export type IntegrationVerificationResult = {
  schemaVersion: "agentpay.sandbox.v2";
  sellerId: string;
  routeId: string;
  routeVersion: number;
  completedAt: string;
  valid: boolean;
  checks: Array<{
    name: IntegrationVerificationCheckName;
    passed: boolean;
    message: string;
  }>;
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
  | "integration_verification"
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
  integrationVerification?: IntegrationVerificationResult;
  version: number;
  updatedAt?: string | null;
};

export type SellerJourneyStep = {
  id: "service" | "payout" | "products" | "publish" | "sales";
  label: string;
  status: "complete" | "current" | "waiting";
};

export type SellerNextAction = {
  label: string;
  description: string;
  href: string;
};

export type MCPHost = "claude-code" | "codex" | "generic-mcp";

export type CreateStorefrontInput = {
  name: string;
  slug: string;
  upstreamBaseUrl: string;
};

export type ActivateSellerServiceInput = {
  sellerId: string;
  expectedVersion: number;
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
  activateSellerService: (
    input: ActivateSellerServiceInput,
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

export const setupPrompt = `You are the seller's coding agent. Edit this repository to connect it to AgentPay. Use AgentPay MCP tools for bounded analysis, configuration, and verification. Inspect only bounded committed manifests and OpenAPI, detect one maintained stack, propose sellable routes plus truthful SEO/AEO changes, install AgentPay request verification, and generate focused tests. Ask me for every exact price and payout destination. AgentPay cloud mutations require explicit seller confirmation. Never invent or change prices or payout addresses, publish, rotate credentials, or deploy without my explicit confirmation.`;

const journeyGroups: Array<{
  id: SellerJourneyStep["id"];
  label: string;
  steps: OnboardingStepName[];
}> = [
  {
    id: "service",
    label: "Connect service",
    steps: [
      "account_verified",
      "storefront_created",
      "service_connection_verified",
      "subscription_active",
    ],
  },
  {
    id: "payout",
    label: "Confirm payout",
    steps: ["payment_destination_verified"],
  },
  {
    id: "products",
    label: "Review detected products",
    steps: ["project_key_created", "connector_verified", "product_configured"],
  },
  {
    id: "publish",
    label: "Publish",
    steps: ["integration_verification", "storefront_previewed"],
  },
  { id: "sales", label: "Monitor sales", steps: [] },
];

const nextActions: Record<OnboardingStepName, SellerNextAction> = {
  account_verified: {
    label: "Verify your account",
    description: "Confirm the seller email attached to this workspace.",
    href: "/verify",
  },
  storefront_created: {
    label: "Create your store",
    description:
      "Name the store and enter the HTTPS service that delivers products.",
    href: "/dashboard/onboarding#storefront",
  },
  service_connection_verified: {
    label: "Enable secure service",
    description:
      "Allow AgentPay to verify signed delivery requests for this service.",
    href: "/dashboard/onboarding#service-readiness",
  },
  subscription_active: {
    label: "Request launch access",
    description:
      "Ask the AgentPay launch operator to activate this testnet store.",
    href: "/dashboard/onboarding#launch-entitlement",
  },
  payment_destination_verified: {
    label: "Confirm payout wallet",
    description:
      "Prove control of the Base Sepolia address that receives USDC.",
    href: "/dashboard/onboarding#payment-destination",
  },
  project_key_created: {
    label: "Create project connection key",
    description:
      "Create the reveal-once key used to connect your coding agent.",
    href: "/dashboard/onboarding#project-credential",
  },
  connector_verified: {
    label: "Connect your coding agent",
    description: "Run the provided preflight, then refresh this page.",
    href: "/dashboard/onboarding#connector",
  },
  product_configured: {
    label: "Review detected products",
    description: "Confirm product names, prices, inputs, and delivery routes.",
    href: "/dashboard/products",
  },
  integration_verification: {
    label: "Run the integration check",
    description: "Run the no-payment check from your connected coding agent.",
    href: "/dashboard/onboarding#validation",
  },
  storefront_previewed: {
    label: "Review and publish",
    description: "Preview the buyer-facing contract, then publish it.",
    href: "/dashboard/products",
  },
};

export function buildSellerJourney(
  onboarding: SellerOnboardingState,
): SellerJourneyStep[] {
  const completeSteps = new Set(
    onboarding.steps
      .filter((step) => step.status === "complete")
      .map((step) => step.name),
  );
  let currentAssigned = false;

  return journeyGroups.map((group) => {
    const complete =
      group.id === "sales"
        ? onboarding.complete && onboarding.publication.allowed
        : group.steps.every((step) => completeSteps.has(step));
    if (complete)
      return { id: group.id, label: group.label, status: "complete" };
    if (!currentAssigned) {
      currentAssigned = true;
      return { id: group.id, label: group.label, status: "current" };
    }
    return { id: group.id, label: group.label, status: "waiting" };
  });
}

export function getSellerNextAction(
  onboarding: SellerOnboardingState,
): SellerNextAction {
  const current =
    onboarding.steps.find((step) => step.status !== "complete") ??
    onboarding.steps.find((step) => step.name === onboarding.currentStep);
  const failedCheck = onboarding.integrationVerification?.checks.find(
    (check) => !check.passed,
  );

  if (current?.name === "integration_verification" && failedCheck) {
    return {
      label: `Fix ${integrationCheckActionLabels[failedCheck.name]}, then run the check again`,
      description: failedCheck.message,
      href: "/dashboard/onboarding#validation",
    };
  }
  if (current) return nextActions[current.name];
  if (onboarding.publication.allowed) {
    return {
      label: "Publish an approved product",
      description:
        "All server checks pass. Choose the product that should go live.",
      href: "/dashboard/products",
    };
  }
  return {
    label: "Refresh store status",
    description: "AgentPay has not returned a publishable state yet.",
    href: "/dashboard/onboarding",
  };
}

const integrationCheckActionLabels: Record<
  IntegrationVerificationCheckName,
  string
> = {
  endpoint_reachability: "endpoint reachability",
  signed_exchange: "signed request and response",
  schema_contract: "the product contract",
  fulfillment_readiness: "delivery readiness",
  payment_gating: "payment protection",
  replay_idempotency: "replay protection",
};

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
