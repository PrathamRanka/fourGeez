import { readFileSync } from "node:fs";
import path from "node:path";
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { Integration, IntegrationCard } from "@/components/ui/integration-card";
import { SiteFooter } from "@/components/site/site-footer";
import { StickyBanner } from "@/components/ui/sticky-banner";

describe("AgentPay web foundation", () => {
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
      screen.getByRole("navigation", { name: "Developer links" }),
    ).toBeVisible();
    expect(
      screen.getByRole("link", { name: "Create your storefront" }),
    ).toHaveAttribute("href", "/sign-up");
    expect(
      screen.getByRole("link", { name: "Meet the developers" }),
    ).toHaveAttribute("href", "/developers");
    expect(screen.getByRole("link", { name: "Contact us" })).toHaveAttribute(
      "href",
      "/contact",
    );
    expect(
      screen.queryByRole("link", { name: "GitHub repository" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: "Pratham Ranka on LinkedIn" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: "Agent Checkout" }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByText(/built by pratham ranka and ayush garg/i),
    ).toBeVisible();
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

  it("uses the landing palette across public decorative surfaces", () => {
    const stylesheet = readFileSync(
      path.resolve(process.cwd(), "app/globals.css"),
      "utf8",
    );
    const marketingStylesheet = readFileSync(
      path.resolve(
        process.cwd(),
        "features/marketing/view/marketing-page.module.css",
      ),
      "utf8",
    );
    expect(stylesheet).toContain("--brand-blue: #2979ff");
    expect(stylesheet).toContain("--brand-pink: #ff5aa5");
    expect(stylesheet).toContain("--brand-orange: #ff6d00");
    expect(marketingStylesheet).toContain("--marketing-blue: #2979ff");
    expect(marketingStylesheet).toContain("--marketing-pink: #ff5aa5");
    expect(marketingStylesheet).toContain("--marketing-orange: #ff6d00");
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
    render(<IntegrationCard />);

    expect(screen.getByLabelText("AgentPay integration network")).toBeVisible();
    expect(screen.queryByText("Connector active")).not.toBeInTheDocument();
    expect(
      screen.queryByText("One MCP. Every surface."),
    ).not.toBeInTheDocument();
    expect(screen.getByText("Repo")).toBeInTheDocument();
    expect(screen.getByText("MCP")).toBeInTheDocument();
    expect(screen.getByText("AI-ready")).toBeInTheDocument();
    expect(screen.getByText("Ship")).toBeInTheDocument();

    render(<Integration />);
    expect(
      screen.getAllByLabelText("AgentPay integration network"),
    ).toHaveLength(2);
  });

  it("serves one permanent dark appearance without theme persistence", () => {
    const layout = readFileSync(
      path.resolve(process.cwd(), "app/layout.tsx"),
      "utf8",
    );
    const authStyles = readFileSync(
      path.resolve(process.cwd(), "features/auth/view/auth-surface.module.css"),
      "utf8",
    );

    expect(layout).toContain('<html lang="en" className="dark">');
    expect(layout).not.toContain("agentpay-theme");
    expect(layout).not.toContain("themeInitializationScript");
    expect(authStyles).toContain("background: #050506");
  });

  it("uses the footer ink black for every shared application surface", () => {
    const stylesheet = readFileSync(
      path.resolve(process.cwd(), "app/globals.css"),
      "utf8",
    );

    expect(stylesheet).toContain("--background: #050506");
    expect(stylesheet).toContain("--dashboard-canvas: #050506");
    expect(stylesheet).toContain("--status-success: #8ce1bb");
    expect(stylesheet).toContain("--status-warning: #e1cd7e");
    expect(stylesheet).toContain("--status-danger: #ffaaaa");
    expect(stylesheet).not.toContain("#07101f");
    expect(stylesheet).not.toContain("#0b1628");
    expect(stylesheet).not.toContain("#10203a");
    const bodyRule = stylesheet.match(/body\s*\{([^}]*)\}/)?.[1] ?? "";
    expect(bodyRule).not.toContain("background-image");
  });

  it("keeps non-marketing pages and panels neutral black", () => {
    const neutralSurfaceFiles = [
      "app/docs/docs.module.css",
      "components/dashboard/dashboard-shell.module.css",
      "app/dashboard/analytics/analytics-page.module.css",
      "features/analytics/view/seller-analytics-dashboard.module.css",
      "features/analytics/view/daily-activity-chart.module.css",
      "features/auth/view/auth-surface.module.css",
      "features/buyer/view/buyer-activity.module.css",
      "features/commerce/view/buyer-purchase-detail.module.css",
      "features/storefront/view/storefront.module.css",
    ];
    const neutralSurfaceStyles = neutralSurfaceFiles
      .map((fileName) =>
        readFileSync(path.resolve(process.cwd(), fileName), "utf8"),
      )
      .join("\n");

    expect(neutralSurfaceStyles).not.toMatch(
      /background(?:-image)?:\s*(?:radial|conic)-gradient/i,
    );
    expect(neutralSurfaceStyles).not.toMatch(
      /background:[^;]*(?:--blue-soft|--blue-glow|--brand-(?:blue|pink|orange))/i,
    );
    expect(neutralSurfaceStyles).not.toMatch(
      /--(?:dash|analytics)-(?:canvas|surface|panel|raised|soft|bg):[^;]*#(?:fff(?:fff)?|fafaf8|f4f4f1)/i,
    );
    expect(neutralSurfaceStyles).not.toContain("color-scheme: light");
    expect(neutralSurfaceStyles).not.toContain("#08090b");
    expect(neutralSurfaceStyles).not.toContain("#090a0d");
  });

  it("does not use colored washes for application status surfaces", () => {
    const stylesheet = readFileSync(
      path.resolve(process.cwd(), "app/globals.css"),
      "utf8",
    );

    expect(stylesheet).not.toContain("--accent: #171a24");
    expect(stylesheet).not.toMatch(
      /background:\s*(?:#0b1a16|#1c0d0d|#19170d|oklch\(0\.9[45]\s+0\.0(?:35|4|45|55|6))/i,
    );
    expect(stylesheet).not.toMatch(
      /background:[^;]*color-mix\([^;]*(?:--blue-soft|--destructive)[^;]*\)/i,
    );
  });

  it("keeps shared registry components local and deterministic", () => {
    const sharedSources = [
      "components/ui/cloud-shader.tsx",
      "components/ui/sticky-banner.tsx",
      "components/ui/integration-card.tsx",
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
