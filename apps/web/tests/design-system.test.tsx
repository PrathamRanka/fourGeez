import { readFileSync } from "node:fs";
import path from "node:path";
import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import {
  AgentPayIntegrationMap,
  IntegrationCard,
} from "@/components/shadcn-space/card/integration-card";
import { SiteFooter } from "@/components/site/site-footer";
import { SiteHeader } from "@/components/site/site-header";
import { StickyBanner } from "@/components/ui/sticky-banner";

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
    expect(
      screen.getByRole("link", { name: "Create your storefront" }),
    ).toHaveAttribute("href", "/sign-up");
  });

  it("defines the approved dual-theme typography, bento, and motion tokens", () => {
    const stylesheet = readFileSync(
      path.resolve(process.cwd(), "app/globals.css"),
      "utf8",
    );

    expect(stylesheet).toContain("@fontsource-variable/bricolage-grotesque");
    expect(stylesheet).toContain("@fontsource-variable/manrope");
    expect(stylesheet).toContain(
      '--font-display: "Bricolage Grotesque Variable"',
    );
    expect(stylesheet).toContain('--font-body: "Manrope Variable"');
    expect(stylesheet).not.toContain("DM Sans Variable");
    expect(stylesheet).not.toContain("Plus Jakarta Sans Variable");
    expect(stylesheet).toContain("--page-gutter");
    expect(stylesheet).toContain("--surface-elevated");
    expect(stylesheet).toContain("--bento-card-compact");
    expect(stylesheet).toContain("--bento-card-feature");
    expect(stylesheet).toContain("clamp(");
    expect(stylesheet).toContain("@media (prefers-reduced-motion: reduce)");
    expect(stylesheet).toContain(":focus-visible");
  });

  it("uses the dismissible registry banner without demo behavior", () => {
    render(<StickyBanner>Network access is available.</StickyBanner>);

    expect(screen.getByText("Network access is available.")).toBeVisible();
    fireEvent.click(screen.getByRole("button", { name: "Dismiss banner" }));
    expect(
      screen.queryByText("Network access is available."),
    ).not.toBeInTheDocument();
  });

  it("renders a deterministic AgentPay integration card", () => {
    render(
      <IntegrationCard
        title="One commerce control plane"
        description="Connect storefronts, agents, and settlement."
        href="/docs"
        visual={<AgentPayIntegrationMap />}
      />,
    );

    expect(
      screen.getByRole("heading", { name: "One commerce control plane" }),
    ).toBeVisible();
    expect(
      screen.getByRole("link", { name: "View documentation" }),
    ).toHaveAttribute("href", "/docs");
    expect(screen.getByLabelText("AgentPay integration network")).toBeVisible();
  });

  it("keeps shared registry components local and deterministic", () => {
    const sharedSources = [
      "components/ui/cloud-shader.tsx",
      "components/ui/sticky-banner.tsx",
      "components/shadcn-space/card/integration-card.tsx",
    ]
      .map((fileName) =>
        readFileSync(path.resolve(process.cwd(), fileName), "utf8"),
      )
      .join("\n");

    expect(sharedSources).not.toContain("console.log");
    expect(sharedSources).not.toContain("Math.random");
    expect(sharedSources).not.toContain("images.shadcnspace.com");
    expect(sharedSources).not.toContain("images.unsplash.com");
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
