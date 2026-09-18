import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import DashboardPage from "@/app/dashboard/page";
import { loadAnalyticsSnapshot } from "@/features/analytics/controller";
import { getSellerSession } from "@/features/auth/server/session";

vi.mock("@/features/auth/server/session", () => ({
  getSellerSession: vi.fn(),
}));
vi.mock("@/features/analytics/controller", () => ({
  loadAnalyticsSnapshot: vi.fn(),
}));

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

    render(await DashboardPage());

    expect(
      screen.getByRole("heading", { name: "Commerce command center" }),
    ).toBeVisible();
    expect(
      screen.getByRole("region", { name: "AgentPay MCP connection" }),
    ).toBeVisible();
    expect(
      screen.getByRole("img", { name: "AgentPay commerce network" }),
    ).toBeVisible();
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
});
