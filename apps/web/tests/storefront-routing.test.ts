import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { GET } from "@/app/store/[slug]/[artifact]/route";
import { generateMetadata } from "@/app/store/[slug]/products/[productSlug]/page";
import {
  loadPublicProduct,
  loadStorefrontDiscovery,
} from "@/features/storefront/controller";

vi.mock("@/features/storefront/controller", () => ({
  loadPublicProduct: vi.fn(),
  loadStorefrontDiscovery: vi.fn(),
}));

const signature = {
  alg: "ES256" as const,
  kid: "discovery-2026-09",
  canonicalization: "RFC8785" as const,
  domainSeparator: "agentpay.discovery.v1" as const,
  value: "signed-value",
};

const product = {
  sellerId: "sel_01ARZ3NDEKTSV4RRFFQ69G5FAV",
  routeId: "rte_01ARZ3NDEKTSV4RRFFQ69G5FAW",
  displayName: "Research Report",
  productSlug: "research-report",
  description: "Generate a source-backed market brief.",
  mimeType: "application/json",
  amount: "35000000",
  asset: "USDC",
  network: "eip155:84532",
  availability: "active" as const,
  canonicalUrl:
    "https://shop.agentpay.example/store/northstar/products/research-report",
  purchaseSessionEndpoint:
    "https://api.agentpay.example/v1/storefronts/northstar/products/research-report/purchase-sessions",
};

describe("storefront sitemap", () => {
  beforeEach(() => {
    vi.stubEnv("AGENTPAY_WEB_ORIGIN", "https://shop.agentpay.example");
    vi.mocked(loadStorefrontDiscovery).mockResolvedValue({
      status: "active",
      manifest: {
        schemaVersion: "agentpay.discovery.v1",
        sellerId: product.sellerId,
        seller: { name: "Northstar Research", slug: "northstar" },
        availability: "active",
        publicationRevision: 4,
        issuedAt: "2026-09-18T11:59:00Z",
        expiresAt: "2026-09-18T12:05:00Z",
        canonicalOrigin: "https://shop.agentpay.example",
        products: [product],
      },
      signature,
    });
  });

  afterEach(() => {
    vi.unstubAllEnvs();
  });

  it("uses the public product slug instead of the internal route ID", async () => {
    const response = await GET(new Request("https://shop.agentpay.example"), {
      params: Promise.resolve({ slug: "northstar", artifact: "sitemap.xml" }),
    });
    const sitemap = await response.text();

    expect(sitemap).toContain(
      "https://shop.agentpay.example/store/northstar/products/research-report",
    );
    expect(sitemap).not.toContain("/products/rte_01ARZ3NDEKTSV4RRFFQ69G5FAW");
  });

  it("does not resolve an internal route ID as a public product URL", async () => {
    vi.mocked(loadPublicProduct).mockResolvedValue({
      status: "unavailable",
      sellerSlug: "northstar",
      reason: "not_found",
    });

    const metadata = await generateMetadata({
      params: Promise.resolve({
        slug: "northstar",
        productSlug: "rte_01ARZ3NDEKTSV4RRFFQ69G5FAW",
      }),
    });

    expect(metadata).toEqual({ robots: { index: false, follow: false } });
    expect(loadPublicProduct).toHaveBeenCalledWith(
      "northstar",
      "rte_01ARZ3NDEKTSV4RRFFQ69G5FAW",
    );
  });
});
