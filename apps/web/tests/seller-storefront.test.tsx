import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type {
  DiscoverySignature,
  PublicProduct,
  StorefrontManifest,
} from "@/features/storefront/model";
import {
  ProductDetail,
  StorefrontAvailability,
  StorefrontHome,
  buildProductStructuredData,
} from "@/features/storefront/view/storefront";

const manifest: StorefrontManifest = {
  schemaVersion: "agentpay.discovery.v1",
  sellerId: "sel_01ARZ3NDEKTSV4RRFFQ69G5FAV",
  seller: { name: "Northstar Research", slug: "northstar" },
  availability: "active",
  publicationRevision: 4,
  issuedAt: "2026-09-18T11:59:00Z",
  expiresAt: "2026-09-18T12:05:00Z",
  canonicalOrigin: "https://shop.agentpay.example",
  products: [],
};

const product: PublicProduct = {
  sellerId: manifest.sellerId,
  routeId: "rte_01ARZ3NDEKTSV4RRFFQ69G5FAW",
  displayName: "Research Report",
  productSlug: "research-report",
  description: "Generate a source-backed market brief for a defined topic.",
  mimeType: "application/json",
  amount: "35000000",
  asset: "USDC",
  network: "eip155:84532",
  availability: "active",
  canonicalUrl:
    "https://shop.agentpay.example/store/northstar/products/research-report",
  purchaseSessionEndpoint:
    "https://api.agentpay.example/v1/storefronts/northstar/products/research-report/purchase-sessions",
};

manifest.products = [product];

const signature: DiscoverySignature = {
  alg: "ES256",
  kid: "discovery-2026-09",
  canonicalization: "RFC8785",
  domainSeparator: "agentpay.discovery.v1",
  value: "signed-value",
};

describe("seller storefront", () => {
  it("renders seller branding and published products with discovery links", () => {
    render(<StorefrontHome manifest={manifest} signature={signature} />);

    expect(
      screen.getByRole("heading", { name: "Northstar Research" }),
    ).toBeVisible();
    expect(screen.getByText("/store/northstar")).toBeVisible();
    expect(
      screen.getByRole("heading", { name: "Research Report" }),
    ).toBeVisible();
    expect(screen.getByText(product.description)).toBeVisible();
    expect(screen.getByText("35 USDC")).toBeVisible();
    expect(screen.getByText("Active on AgentPay")).toBeVisible();
    expect(screen.getByRole("link", { name: "View product" })).toHaveAttribute(
      "href",
      "/store/northstar/products/research-report",
    );
    expect(
      screen.getByRole("link", { name: "Signed manifest" }),
    ).toHaveAttribute("href", "/store/northstar/manifest.json");
    expect(screen.getByRole("link", { name: "llms.txt" })).toHaveAttribute(
      "href",
      "/store/northstar/llms.txt",
    );
  });

  it("shows exact x402 purchase instructions and matching structured data", async () => {
    render(
      <ProductDetail
        sellerSlug="northstar"
        product={product}
        signature={signature}
      />,
    );

    expect(
      screen.getByRole("heading", { name: "Research Report" }),
    ).toBeVisible();
    expect(screen.getByText("Pay exactly 35 USDC")).toBeVisible();
    expect(
      screen.getByText("/store/northstar/products/research-report"),
    ).toBeVisible();
    expect(screen.queryByText("eip155:84532")).not.toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Advanced technical details" }),
    );
    expect(await screen.findByText("eip155:84532")).toBeVisible();
    expect(screen.getByText(product.purchaseSessionEndpoint)).toBeVisible();
    expect(screen.queryByText(/card/i)).not.toBeInTheDocument();

    expect(buildProductStructuredData(product)).toMatchObject({
      "@type": "Service",
      name: "Research Report",
      description: product.description,
      url: product.canonicalUrl,
    });
  });

  it.each([
    ["inactive", "This storefront is not accepting new purchases"],
    ["expired", "This storefront status has expired"],
    ["unavailable", "This storefront is temporarily unavailable"],
  ] as const)(
    "renders the %s storefront state without products",
    (status, heading) => {
      render(
        <StorefrontAvailability
          state={{
            status,
            sellerSlug: "northstar",
            ...(status === "inactive" ? { reason: "cancelled" as const } : {}),
            ...(status === "expired"
              ? { expiresAt: "2026-09-18T11:55:00Z" }
              : {}),
            ...(status === "unavailable"
              ? { reason: "dependency_unavailable" }
              : {}),
          }}
        />,
      );

      expect(screen.getByRole("heading", { name: heading })).toBeVisible();
      expect(
        screen.queryByRole("link", { name: "View product" }),
      ).not.toBeInTheDocument();
    },
  );
});
