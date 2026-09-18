import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type { StorefrontManifest } from "@/features/storefront/model";
import {
  ProductDetail,
  StorefrontHome,
  buildProductStructuredData,
} from "@/features/storefront/view/storefront";

const manifest: StorefrontManifest = {
  seller: { name: "Northstar Research", slug: "northstar" },
  routes: [
    {
      routeId: "rte_01ARZ3NDEKTSV4RRFFQ69G5FAW",
      sellerId: "sel_01ARZ3NDEKTSV4RRFFQ69G5FAV",
      displayName: "Research Report",
      productSlug: "research-report",
      method: "POST",
      pathPattern: "/research",
      description: "Generate a source-backed market brief for a defined topic.",
      mimeType: "application/json",
      amount: "35000000",
      asset: "USDC",
      network: "eip155:84532",
      payTo: "0x1111111111111111111111111111111111111111",
      approvalThresholdAmount: null,
      upstreamTimeoutSeconds: 20,
      lifecycleStatus: "published",
      enabled: true,
      createdAt: "2026-09-18T09:00:00Z",
      updatedAt: "2026-09-18T09:05:00Z",
      version: 2,
    },
  ],
};

describe("seller storefront", () => {
  it("renders seller branding and published products with discovery links", () => {
    render(<StorefrontHome manifest={manifest} />);

    expect(screen.getByRole("heading", { name: "Northstar Research" })).toBeVisible();
    expect(screen.getByText(manifest.routes[0].description)).toBeVisible();
    expect(screen.getByText("35 USDC")).toBeVisible();
    expect(screen.getByRole("link", { name: "View product" })).toHaveAttribute(
      "href",
      "/store/northstar/products/research-report",
    );
    expect(screen.getByRole("link", { name: "Manifest" })).toHaveAttribute(
      "href",
      "/store/northstar/manifest.json",
    );
    expect(screen.getByRole("link", { name: "llms.txt" })).toHaveAttribute(
      "href",
      "/store/northstar/llms.txt",
    );
  });

  it("shows exact x402 purchase instructions and matching structured data", () => {
    render(
      <ProductDetail
        manifest={manifest}
        route={manifest.routes[0]}
        apiOrigin="https://api.agentpay.example"
      />,
    );

    expect(screen.getByRole("heading", { name: "/research" })).toBeVisible();
    expect(screen.getByText("Pay exactly 35 USDC")).toBeVisible();
    expect(screen.getByText("eip155:84532")).toBeVisible();
    expect(
      screen.getByText("https://api.agentpay.example/pay/northstar/research"),
    ).toBeVisible();
    expect(screen.queryByText(/card/i)).not.toBeInTheDocument();

    expect(
      buildProductStructuredData(
        manifest,
        manifest.routes[0],
        "https://shop.agentpay.example/store/northstar/products/research-report",
      ),
    ).toMatchObject({
      "@type": "Service",
      name: "Research Report",
      description: manifest.routes[0].description,
      url: "https://shop.agentpay.example/store/northstar/products/research-report",
    });
  });
});
