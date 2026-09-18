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
} from "@/features/onboarding/model";
import { SellerOnboarding } from "@/features/onboarding/view/seller-onboarding";

const seller: Seller = {
  sellerId: "sel_01ARZ3NDEKTSV4RRFFQ69G5FAV",
  name: "Northstar Research",
  slug: "northstar-research",
  upstreamBaseUrl: "https://api.northstar.example",
  status: "draft",
  version: 1,
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
  credentialId: "crd_01ARZ3NDEKTSV4RRFFQ69G5FAX",
  sellerId: seller.sellerId,
  label: "Primary coding agent",
  scopes: ["read", "configure", "validate", "publish"],
  version: 1,
  token: "agp_live_once_only",
};

const emptySnapshot: OnboardingSnapshot = {
  seller: null,
  paymentDestinations: [],
  credentials: [],
};

// createActions returns deterministic onboarding actions for user-visible tests.
function createActions(): OnboardingActions {
  return {
    createStorefront: vi.fn().mockResolvedValue({ ok: true, value: seller }),
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
    verifyPaymentDestination: vi.fn().mockResolvedValue({
      ok: true,
      value: activeDestination,
    }),
    createIntegrationCredential: vi.fn().mockResolvedValue({
      ok: true,
      value: createdCredential,
    }),
  };
}

describe("seller onboarding", () => {
  it("blocks setup controls for a suspended seller", () => {
    render(
      <SellerOnboarding
        initialSnapshot={{
          seller: { ...seller, status: "suspended" },
          paymentDestinations: [],
          credentials: [],
        }}
        actions={createActions()}
      />,
    );

    expect(screen.getByText("Seller account suspended")).toBeVisible();
    expect(
      screen.queryByRole("button", { name: "Connect browser wallet" }),
    ).not.toBeInTheDocument();
  });

  it("creates a storefront before exposing wallet and agent setup", async () => {
    const actions = createActions();
    render(
      <SellerOnboarding initialSnapshot={emptySnapshot} actions={actions} />,
    );

    expect(
      screen.getByRole("heading", { name: "Launch your storefront" }),
    ).toBeVisible();
    expect(
      screen.getByRole("region", { name: "Storefront launch sequence" }),
    ).toBeVisible();
    expect(
      screen.getByRole("region", { name: "01 Create your storefront" }),
    ).toBeVisible();
    expect(screen.getByText("1 of 5 complete")).toBeVisible();

    fireEvent.change(screen.getByLabelText("Storefront name"), {
      target: { value: seller.name },
    });
    fireEvent.change(screen.getByLabelText("Storefront URL name"), {
      target: { value: seller.slug },
    });
    fireEvent.change(screen.getByLabelText("Service API URL"), {
      target: { value: seller.upstreamBaseUrl },
    });
    fireEvent.submit(screen.getByRole("form", { name: "Create storefront" }));

    await waitFor(() =>
      expect(actions.createStorefront).toHaveBeenCalledTimes(1),
    );
    expect((await screen.findAllByText("Storefront created"))[0]).toBeVisible();
    expect(screen.getByText("/store/northstar-research")).toBeVisible();
    expect(
      screen.getByRole("button", { name: "Connect browser wallet" }),
    ).toBeEnabled();
  });

  it("creates a scoped key and presents copyable MCP configuration and setup prompt", async () => {
    const actions = createActions();
    render(
      <SellerOnboarding
        initialSnapshot={{
          seller,
          paymentDestinations: [activeDestination],
          credentials: [],
        }}
        actions={actions}
      />,
    );

    fireEvent.click(
      screen.getByRole("button", { name: "Create project connection key" }),
    );

    expect(
      await screen.findByText(
        "Save this project connection key now. It is shown only once.",
      ),
    ).toBeVisible();
    expect(screen.getByText(createdCredential.token)).toBeVisible();
    expect(
      screen.getByRole("button", { name: "Copy MCP configuration" }),
    ).toBeEnabled();
    expect(
      screen.getByRole("button", { name: "Copy setup prompt" }),
    ).toBeEnabled();
    expect(screen.getByText(/ready for product validation/i)).toBeVisible();

    const checklist = screen.getByRole("list", {
      name: "Onboarding checklist",
    });
    expect(
      within(checklist).getByText("Payment destination verified"),
    ).toBeVisible();
    expect(within(checklist).getByText("Coding agent connected")).toBeVisible();
  });
});
