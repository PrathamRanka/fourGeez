import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import SettingsPage from "@/app/dashboard/settings/page";
import { getSellerSession } from "@/features/auth/server/session";

vi.mock("@/features/auth/server/session", () => ({
  getSellerSession: vi.fn(),
}));

describe("seller dashboard settings", () => {
  it("keeps workspace configuration in a dedicated settings page", async () => {
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

    render(await SettingsPage());

    expect(screen.getByRole("heading", { name: "Settings" })).toBeVisible();
    expect(
      screen.queryByRole("button", { name: "Switch appearance" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("heading", { name: "Appearance" }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Manage store setup" }),
    ).toHaveAttribute("href", "/dashboard/onboarding");
    expect(screen.getByText("owner@example.com")).toBeVisible();
  });
});
