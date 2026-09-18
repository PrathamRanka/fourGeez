import { beforeEach, describe, expect, it, vi } from "vitest";
import { loadStorefrontDiscovery } from "@/features/storefront/controller";
import { runBuyerActivity } from "@/features/buyer/controller";

vi.mock("@/features/storefront/controller", () => ({
  loadStorefrontDiscovery: vi.fn(),
}));

const product = {
  sellerId: "sel_01ARZ3NDEKTSV4RRFFQ69G5FAV",
  routeId: "rte_01ARZ3NDEKTSV4RRFFQ69G5FAW",
  displayName: "Market Snapshot",
  productSlug: "market-snapshot",
  description: "Generate a market snapshot.",
  mimeType: "application/json",
  amount: "2500000",
  asset: "USDC",
  network: "eip155:84532",
  availability: "active" as const,
  canonicalUrl:
    "https://shop.agentpay.example/store/northstar/products/market-snapshot",
  purchaseSessionEndpoint:
    "https://api.agentpay.example/v1/storefronts/northstar/products/market-snapshot/purchase-sessions",
};

describe("buyer controller", () => {
  beforeEach(() => {
    vi.mocked(loadStorefrontDiscovery).mockReset();
  });

  it("selects the supported Base Sepolia USDC offer for the agent demo", async () => {
    vi.mocked(loadStorefrontDiscovery).mockResolvedValue({
      status: "active",
      manifest: {
        schemaVersion: "agentpay.discovery.v1",
        sellerId: product.sellerId,
        seller: { name: "Northstar", slug: "northstar" },
        availability: "active",
        publicationRevision: 1,
        issuedAt: "2026-09-18T12:00:00Z",
        expiresAt: "2026-09-18T13:00:00Z",
        canonicalOrigin: "https://shop.agentpay.example",
        products: [
          {
            ...product,
            routeId: "rte_01ARZ3NDEKTSV4RRFFQ69G5FB1",
            displayName: "Board Research Brief",
            productSlug: "board-research-brief",
            asset: "EURC",
          },
          product,
        ],
      },
      signature: {
        alg: "ES256",
        kid: "discovery-key",
        canonicalization: "RFC8785",
        domainSeparator: "agentpay.discovery.v1",
        value: "signature",
      },
    });

    const result = await runBuyerActivity({
      slug: "northstar",
      prompt: "A market brief under 3 USDC",
    });

    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value.selectedProduct?.productSlug).toBe("market-snapshot");
      expect(result.value.activities[1].detail).toContain("Market Snapshot");
    }
  });
});
