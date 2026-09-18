import { readFileSync } from "node:fs";
import path from "node:path";
import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { SiteFooter } from "@/components/site/site-footer";
import { SiteHeader } from "@/components/site/site-header";

describe("AgentPay web foundation", () => {
  it("renders accessible primary navigation and account actions", () => {
    render(<SiteHeader />);

    expect(screen.getByRole("link", { name: "AgentPay home" })).toBeVisible();
    const primaryNavigation = screen.getByRole("navigation", {
      name: "Primary navigation",
    });
    expect(primaryNavigation).toBeVisible();
    expect(
      within(primaryNavigation).getByRole("link", { name: "Product" }),
    ).toHaveAttribute("href", "/#product");
    expect(screen.getAllByRole("link", { name: "Sign in" })[0]).toHaveAttribute(
      "href",
      "/sign-in",
    );
    expect(
      screen.getAllByRole("link", { name: "Get started" })[0],
    ).toHaveAttribute("href", "/sign-up");
  });

  it("opens the mobile menu with an announced state", () => {
    render(<SiteHeader />);

    const menuButton = screen.getByRole("button", { name: "Open navigation" });
    expect(menuButton).toHaveAttribute("aria-expanded", "false");
    fireEvent.click(menuButton);
    expect(
      screen.getByRole("button", { name: "Close navigation" }),
    ).toHaveAttribute("aria-expanded", "true");
    expect(
      screen.getByRole("navigation", { name: "Mobile navigation" }),
    ).toBeVisible();
  });

  it("renders product, company, and legal footer navigation", () => {
    render(<SiteFooter />);

    expect(
      screen.getByRole("navigation", { name: "Product links" }),
    ).toBeVisible();
    expect(
      screen.getByRole("navigation", { name: "Company links" }),
    ).toBeVisible();
    expect(
      screen.getByRole("navigation", { name: "Legal links" }),
    ).toBeVisible();
  });

  it("defines fluid layout tokens and a reduced-motion mode", () => {
    const stylesheet = readFileSync(
      path.resolve(process.cwd(), "app/globals.css"),
      "utf8",
    );

    expect(stylesheet).toContain("--font-display");
    expect(stylesheet).toContain("--page-gutter");
    expect(stylesheet).toContain("clamp(");
    expect(stylesheet).toContain("@media (prefers-reduced-motion: reduce)");
    expect(stylesheet).toContain(":focus-visible");
  });

  it("contains the dashboard navigation inside narrow viewports", () => {
    const stylesheet = readFileSync(
      path.resolve(process.cwd(), "app/globals.css"),
      "utf8",
    );

    expect(stylesheet).toContain("max-width: 100vw");
    expect(stylesheet).not.toContain("min-width: 31rem");
  });
});
