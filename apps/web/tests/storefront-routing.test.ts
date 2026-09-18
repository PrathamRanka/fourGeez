import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { GET } from "@/app/store/[slug]/[artifact]/route";
import { generateMetadata } from "@/app/store/[slug]/products/[productSlug]/page";
import { loadStorefrontManifest } from "@/features/storefront/controller";

vi.mock("@/features/storefront/controller", () => ({
  loadStorefrontManifest: vi.fn(),
}));

describe("storefront sitemap", () => {
  beforeEach(() => {
    vi.stubEnv("AGENTPAY_WEB_ORIGIN", "https://shop.agentpay.example");
    vi.mocked(loadStorefrontManifest).mockResolvedValue({
      seller: { name: "Northstar Research", slug: "northstar" },
      routes: [
        {
          routeId: "rte_01ARZ3NDEKTSV4RRFFQ69G5FAW",
          sellerId: "sel_01ARZ3NDEKTSV4RRFFQ69G5FAV",
          displayName: "Research Report",
          productSlug: "research-report",
          method: "POST",
          pathPattern: "/research",
          description: "Generate a source-backed market brief.",
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
    const metadata = await generateMetadata({
      params: Promise.resolve({
        slug: "northstar",
        productSlug: "rte_01ARZ3NDEKTSV4RRFFQ69G5FAW",
      }),
    });

    expect(metadata).toEqual({});
  });
});
