import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import DashboardPage from "@/app/dashboard/page";
import { getSellerSession } from "@/features/auth/server/session";

vi.mock("@/features/auth/server/session", () => ({
  getSellerSession: vi.fn(),
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

    render(await DashboardPage());

    expect(
      screen.getByRole("heading", { name: "Northstar Research" }),
    ).toBeVisible();
    expect(
      screen.getByRole("link", { name: "Manage products" }),
    ).toHaveAttribute("href", "/dashboard/products");
    expect(
      screen.getByRole("link", { name: "Review transactions" }),
    ).toHaveAttribute("href", "/dashboard/transactions");
  });
});
