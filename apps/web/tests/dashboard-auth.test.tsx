import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { DashboardNavigation } from "@/components/dashboard/dashboard-navigation";
import { DashboardShell } from "@/components/dashboard/dashboard-shell";

vi.mock("next/navigation", () => ({
  usePathname: () => "/dashboard/products",
  useRouter: () => ({ push: vi.fn(), refresh: vi.fn() }),
}));

describe("authenticated seller dashboard", () => {
  it("does not carry seller identity in dashboard links", () => {
    render(<DashboardNavigation />);
    expect(
      screen.getByRole("navigation", { name: "Seller command navigation" }),
    ).toBeVisible();
    expect(screen.getByRole("link", { name: "Products" })).toHaveAttribute(
      "href",
      "/dashboard/products",
    );
    expect(screen.getByRole("link", { name: "Disputes" })).toHaveAttribute(
      "href",
      "/dashboard/disputes",
    );
    expect(screen.getByRole("link", { name: "Settings" })).toHaveAttribute(
      "href",
      "/dashboard/settings",
    );
    expect(
      screen.getByRole("link", { name: "Transactions" }),
    ).not.toHaveAttribute("href", expect.stringContaining("sellerId"));
    expect(screen.getByRole("link", { name: "Products" })).toHaveAttribute(
      "aria-current",
      "page",
    );
    expect(
      screen.queryByRole("link", { name: "Docs" }),
    ).not.toBeInTheDocument();
  });

  it("shows the authenticated workspace context and a real sign-out action", () => {
    render(
      <DashboardShell
        seller={{ email: "owner@example.com", name: "Northstar Research" }}
      >
        <p>Protected content</p>
      </DashboardShell>,
    );
    expect(
      screen.getByRole("complementary", {
        name: "AgentPay seller workspace",
      }),
    ).toBeVisible();
    expect(
      screen.getByRole("status", { name: "Environment" }),
    ).toHaveTextContent("Local testnet");
    expect(screen.getByText("Northstar Research")).toBeVisible();
    expect(screen.getByText("owner@example.com")).toBeVisible();
    expect(screen.getByRole("button", { name: "Sign out" })).toBeVisible();
    expect(
      screen.queryByRole("button", { name: "Switch appearance" }),
    ).not.toBeInTheDocument();
    expect(screen.getByRole("main")).toHaveAttribute("id", "main-content");
  });
});
