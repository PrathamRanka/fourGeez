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
};

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

export const setupPrompt = `Connect this project to AgentPay. Identify sellable API routes, propose products and prices, install AgentPay request verification, generate the storefront and stack-native technical SEO/AEO metadata, run the integration tests, and prepare the changes for my approval.`;

// createMCPConfiguration returns the copyable remote MCP settings for one issued credential.
export function createMCPConfiguration(apiOrigin: string, token: string) {
  return JSON.stringify(
    {
      mcpServers: {
        agentpay: {
          url: `${apiOrigin.replace(/\/$/, "")}/mcp`,
          headers: {
            Authorization: `Bearer ${token}`,
          },
        },
      },
    },
    null,
    2,
  );
}
import type { ActionResult } from "@/lib/agentpay-api";
