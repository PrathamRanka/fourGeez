import { render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import DashboardPage from "@/app/dashboard/page";
import { loadAnalyticsSnapshot } from "@/features/analytics/controller";
import { getSellerSession } from "@/features/auth/server/session";
import { loadOnboardingState } from "@/features/onboarding/controller";

vi.mock("@/features/auth/server/session", () => ({
  getSellerSession: vi.fn(),
}));
vi.mock("@/features/analytics/controller", () => ({
  loadAnalyticsSnapshot: vi.fn(),
}));
vi.mock("@/features/onboarding/controller", () => ({
  loadOnboardingState: vi.fn(),
}));

const onboarding = {
  sellerId: "sel_session_owner",
  complete: true,
  currentStep: "storefront_previewed" as const,
  steps: [
    {
      name: "integration_verification" as const,
      status: "complete" as const,
      blocking: false,
      message: "Automated integration verification passed.",
    },
  ],
  publication: { allowed: true, blockers: [] },
  integrationVerification: {
    schemaVersion: "agentpay.sandbox.v2" as const,
    sellerId: "sel_session_owner",
    routeId: "rte_research",
    routeVersion: 2,
    completedAt: "2026-09-20T10:00:00Z",
    valid: true,
    checks: [],
  },
  version: 2,
};

describe("seller dashboard overview", () => {
  it("gives an established seller direct links to essential operations", async () => {
    vi.mocked(getSellerSession).mockResolvedValue({
      sessionId: "opaque-session",
      accessToken: "server-token",
      expiresAt: "2099-09-18T00:00:00Z",
      principal: {
        subject: "owner-subject",
        email: "owner@example.com",
        name: "Northstar Research",
        sellerId: "sel_session_owner",
        onboardingComplete: true,
      },
    });
    vi.mocked(loadAnalyticsSnapshot).mockResolvedValue({
      sellerId: "sel_session_owner",
      transactionCount: 3,
      routes: [],
      aggregates: [
        {
          sellerId: "sel_session_owner",
          bucketDate: "2026-09-19",
          asset: "USDC",
          network: "eip155:84532",
          stage: "fulfilled",
          transactionCount: 2,
          amount: "25000000",
          lastTransactionAt: "2026-09-19T08:00:00Z",
        },
        {
          sellerId: "sel_session_owner",
          bucketDate: "2026-09-19",
          asset: "EURC",
          network: "eip155:11155111",
          stage: "verified",
          transactionCount: 1,
          amount: "9000000",
          lastTransactionAt: "2026-09-19T09:00:00Z",
        },
      ],
    });
    vi.mocked(loadOnboardingState).mockResolvedValue(onboarding);

    render(await DashboardPage());

    expect(
      screen.getByRole("heading", { name: "Commerce overview" }),
    ).toBeVisible();
    expect(
      screen.getByRole("region", { name: "Recommended next step" }),
    ).toBeVisible();
    expect(
      screen.getByRole("link", { name: "Publish your first product" }),
    ).toHaveAttribute("href", "/dashboard/products");
    const commerceMetrics = screen.getByRole("region", {
      name: "Commerce metrics",
    });
    expect(within(commerceMetrics).getByText("Needs attention")).toBeVisible();
    expect(within(commerceMetrics).getByText("Processing")).toBeVisible();
    const storeHealth = screen.getByRole("region", { name: "Store health" });
    expect(storeHealth).toHaveTextContent("PublicationReady");
    expect(storeHealth).toHaveTextContent("IntegrationPass");
    expect(
      screen.queryByRole("region", { name: "AgentPay integration network" }),
    ).not.toBeInTheDocument();
    expect(screen.queryByText("Commerce network")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("img", { name: "AgentPay commerce network" }),
    ).not.toBeInTheDocument();
    expect(screen.getByText("Northstar Research")).toBeVisible();
    expect(
      screen.getByRole("status", { name: "3 transactions" }),
    ).toBeVisible();
    expect(
      screen.getByRole("region", { name: "USDC on eip155:84532" }),
    ).toBeVisible();
    expect(
      screen.getByRole("region", { name: "EURC on eip155:11155111" }),
    ).toBeVisible();
    expect(screen.queryByText("34")).not.toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Manage products" }),
    ).toHaveAttribute("href", "/dashboard/products");
    expect(
      screen.getByRole("link", { name: "Review transactions" }),
    ).toHaveAttribute("href", "/dashboard/transactions");
    expect(
      screen.getByRole("link", { name: "Open analytics" }),
    ).toHaveAttribute("href", "/dashboard/analytics");
  });

  it("prioritizes transaction review when recorded sales need attention", async () => {
    vi.mocked(getSellerSession).mockResolvedValue({
      sessionId: "opaque-session",
      accessToken: "server-token",
      expiresAt: "2099-09-18T00:00:00Z",
      principal: {
        subject: "owner-subject",
        email: "owner@example.com",
        name: "Northstar Research",
        sellerId: "sel_session_owner",
        onboardingComplete: true,
      },
    });
    vi.mocked(loadAnalyticsSnapshot).mockResolvedValue({
      sellerId: "sel_session_owner",
      transactionCount: 4,
      routes: [
        {
          routeId: "rte_research",
          displayName: "Research report",
          pathPattern: "/reports/research",
        },
      ],
      aggregates: [
        {
          sellerId: "sel_session_owner",
          bucketDate: "2026-09-19",
          asset: "USDC",
          network: "eip155:84532",
          stage: "fulfilled",
          transactionCount: 2,
          amount: "25000000",
          lastTransactionAt: "2026-09-19T08:00:00Z",
        },
        {
          sellerId: "sel_session_owner",
          bucketDate: "2026-09-19",
          asset: "USDC",
          network: "eip155:84532",
          stage: "failed",
          transactionCount: 1,
          amount: "5000000",
          lastTransactionAt: "2026-09-19T09:00:00Z",
        },
        {
          sellerId: "sel_session_owner",
          bucketDate: "2026-09-19",
          asset: "USDC",
          network: "eip155:84532",
          stage: "disputed",
          transactionCount: 1,
          amount: "7000000",
          lastTransactionAt: "2026-09-19T10:00:00Z",
        },
      ],
    });
    vi.mocked(loadOnboardingState).mockResolvedValue(onboarding);

    render(await DashboardPage());

    expect(
      screen.getByRole("link", { name: "Review 2 transactions" }),
    ).toHaveAttribute("href", "/dashboard/transactions");
    expect(
      screen.getByRole("region", { name: "USDC on eip155:84532" }),
    ).toHaveTextContent("Review");
  });
});
