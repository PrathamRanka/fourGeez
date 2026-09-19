import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type {
  CredentialCreated,
  OnboardingActions,
  OnboardingSnapshot,
  PaymentDestination,
  Seller,
  SellerOnboardingState,
} from "@/features/onboarding/model";
import { SellerOnboarding } from "@/features/onboarding/view/seller-onboarding";

const seller: Seller = {
  sellerId: "sel_01ARZ3NDEKTSV4RRFFQ69G5FAV",
  name: "Northstar Research",
  slug: "northstar-research",
  upstreamBaseUrl: "https://api.northstar.example",
  status: "active",
  version: 2,
};

const activeDestination: PaymentDestination = {
  destinationId: "dst_01ARZ3NDEKTSV4RRFFQ69G5FAW",
  sellerId: seller.sellerId,
  asset: "USDC",
  network: "eip155:84532",
  address: "0x1111111111111111111111111111111111111111",
  status: "active",
  version: 2,
};

const createdCredential: CredentialCreated = {
  credentialId: "key_01ARZ3NDEKTSV4RRFFQ69G5FAX",
  sellerId: seller.sellerId,
  label: "Primary coding agent",
  scopes: ["read", "configure", "validate", "publish"],
  version: 1,
  token: "apc2.key_01ARZ3NDEKTSV4RRFFQ69G5FAX.once-only-secret",
};

function onboardingState(
  overrides: Partial<
    Record<string, "complete" | "incomplete" | "blocked">
  > = {},
): SellerOnboardingState {
  const names = [
    "account_verified",
    "storefront_created",
    "service_connection_verified",
    "subscription_active",
    "payment_destination_verified",
    "project_key_created",
    "connector_verified",
    "product_configured",
    "storefront_previewed",
  ] as const;
  const defaultComplete = new Set([
    "account_verified",
    "storefront_created",
    "service_connection_verified",
    "subscription_active",
    "payment_destination_verified",
  ]);
  return {
    sellerId: seller.sellerId,
    complete: false,
    currentStep: "project_key_created",
    steps: names.map((name) => {
      const status =
        overrides[name] ??
        (defaultComplete.has(name) ? "complete" : "incomplete");
      return {
        name,
        status,
        blocking: status !== "complete",
        message: `Complete ${name}`,
      };
    }),
    publication: { allowed: false, blockers: ["project_key_created"] },
    version: 1,
  };
}

function snapshot(input: Partial<OnboardingSnapshot> = {}): OnboardingSnapshot {
  return {
    seller,
    paymentDestinations: [activeDestination],
    credentials: [],
    onboarding: onboardingState(),
    ...input,
  };
}

function createActions(): OnboardingActions {
  return {
    createStorefront: vi.fn().mockResolvedValue({ ok: true, value: seller }),
    activateSellerService: vi.fn().mockResolvedValue({
      ok: true,
      value: seller,
    }),
    preparePaymentDestination: vi.fn().mockResolvedValue({
      ok: true,
      value: {
        destination: {
          ...activeDestination,
          status: "pending_verification",
          version: 1,
        },
        challenge: "AgentPay ownership challenge",
      },
    }),
    verifyPaymentDestination: vi
      .fn()
      .mockResolvedValue({ ok: true, value: activeDestination }),
    createIntegrationCredential: vi.fn().mockResolvedValue({
      ok: true,
      value: createdCredential,
    }),
  };
}

describe("seller onboarding MCP gate", () => {
  it("never exposes buyer checkout after connector and product prerequisites", () => {
    const ready = onboardingState({
      project_key_created: "complete",
      connector_verified: "complete",
      product_configured: "complete",
    });
    render(
      <SellerOnboarding
        initialSnapshot={snapshot({
          credentials: [createdCredential],
          onboarding: ready,
        })}
        actions={createActions()}
      />,
    );
    expect(
      screen.queryByRole("region", { name: "Run test purchase" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: /open buyer checkout/i }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByText(/authorize a buyer wallet/i),
    ).not.toBeInTheDocument();
    expect(screen.queryByText(/commerce rehearsal/i)).not.toBeInTheDocument();
  });

  it("shows authoritative prerequisite blockers and no MCP invitation while ineligible", () => {
    const onboarding = onboardingState({
      service_connection_verified: "blocked",
      subscription_active: "blocked",
      payment_destination_verified: "incomplete",
    });
    render(
      <SellerOnboarding
        initialSnapshot={snapshot({ paymentDestinations: [], onboarding })}
        actions={createActions()}
      />,
    );

    const checklist = screen.getByRole("list", { name: "MCP prerequisites" });
    expect(
      within(checklist).getByText(/active HTTPS service endpoint/i),
    ).toBeVisible();
    expect(
      within(checklist).getByText(/testnet launch entitlement/i),
    ).toBeVisible();
    expect(
      within(checklist).getByRole("link", { name: /complete service/i }),
    ).toHaveAttribute("href", "/dashboard/onboarding#service-readiness");
    expect(
      screen.queryByRole("button", {
        name: "Create project connection key",
      }),
    ).not.toBeInTheDocument();
    expect(screen.queryByText("Host configuration")).not.toBeInTheDocument();
    expect(
      screen.queryByText(/Connect this project to AgentPay/),
    ).not.toBeInTheDocument();
  });

  it("activates ES256 service readiness without revealing a shared secret", async () => {
    const actions = createActions();
    const draftSeller = { ...seller, status: "draft" as const, version: 1 };
    render(
      <SellerOnboarding
        initialSnapshot={snapshot({
          seller: draftSeller,
          onboarding: onboardingState({
            service_connection_verified: "blocked",
          }),
        })}
        actions={actions}
      />,
    );

    fireEvent.click(
      screen.getByRole("button", { name: "Enable secure service" }),
    );

    await waitFor(() =>
      expect(actions.activateSellerService).toHaveBeenCalledWith({
        sellerId: draftSeller.sellerId,
        expectedVersion: 1,
      }),
    );
  });

  it("requires the seller-entered payout address to match the connected wallet", async () => {
    const actions = createActions();
    const address = activeDestination.address;
    window.ethereum = {
      request: vi.fn().mockImplementation(({ method }) => {
        if (method === "eth_requestAccounts") return Promise.resolve([address]);
        if (method === "personal_sign") return Promise.resolve("0xsigned");
        return Promise.reject(new Error("unexpected method"));
      }),
    };
    const onboarding = onboardingState({
      payment_destination_verified: "incomplete",
    });
    render(
      <SellerOnboarding
        initialSnapshot={snapshot({ paymentDestinations: [], onboarding })}
        actions={actions}
      />,
    );

    expect(
      screen.getByText(/testnet only.*no real-money production/i),
    ).toBeVisible();

    fireEvent.change(screen.getByLabelText("Payout address"), {
      target: { value: address },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Verify payout address" }),
    );

    await waitFor(() =>
      expect(actions.preparePaymentDestination).toHaveBeenCalledWith({
        sellerId: seller.sellerId,
        asset: "USDC",
        network: "eip155:84532",
        address,
      }),
    );
    expect(actions.verifyPaymentDestination).toHaveBeenCalledWith(
      expect.objectContaining({ signature: "0xsigned" }),
    );
  });

  it("reveals a new key once and publishes connector-only host configuration", async () => {
    const actions = createActions();
    render(
      <SellerOnboarding
        initialSnapshot={snapshot()}
        actions={actions}
        apiOrigin="https://api.agentpay.example"
      />,
    );

    fireEvent.click(
      screen.getByRole("button", {
        name: "Create project connection key",
      }),
    );

    expect(await screen.findByText(createdCredential.token)).toBeVisible();
    expect(screen.getByText(/shown only once/i)).toBeVisible();
    const configuration = screen.getByLabelText("Host configuration");
    expect(configuration).toHaveTextContent(
      "@agentpay/local-mcp-connector@0.1.0",
    );
    expect(configuration).toHaveTextContent("AGENTPAY_PROJECT_KEY");
    expect(configuration).not.toHaveTextContent(createdCredential.token);
    expect(configuration).not.toHaveTextContent("Authorization");
    expect(screen.getByLabelText("Windows PowerShell setup")).toHaveTextContent(
      "Read-Host",
    );
    expect(screen.getByLabelText("Windows PowerShell setup")).toHaveTextContent(
      "--check",
    );
    expect(screen.getAllByText("Connector disconnected")[0]).toBeVisible();
  });

  it("reports real connector authorization separately from credential creation", () => {
    const onboarding = onboardingState({
      project_key_created: "complete",
      connector_verified: "complete",
    });
    render(
      <SellerOnboarding
        initialSnapshot={snapshot({
          credentials: [createdCredential],
          onboarding,
        })}
        actions={createActions()}
      />,
    );

    expect(
      screen.getByText("Project key active — secret hidden"),
    ).toBeVisible();
    expect(screen.getAllByText("Connector connected")[0]).toBeVisible();
    expect(
      screen.getAllByText(/authenticated MCP initialization passed/i)[0],
    ).toBeVisible();
  });

  it("reports revoked credentials with safe retry guidance", () => {
    const onboarding = onboardingState({
      project_key_created: "incomplete",
      connector_verified: "incomplete",
    });
    render(
      <SellerOnboarding
        initialSnapshot={snapshot({
          credentials: [
            { ...createdCredential, revokedAt: "2026-09-19T10:00:00Z" },
          ],
          onboarding,
        })}
        actions={createActions()}
      />,
    );

    expect(screen.getAllByText("Connector revoked")[0]).toBeVisible();
    expect(screen.getAllByText(/create a new project key/i)[0]).toBeVisible();
  });
});
