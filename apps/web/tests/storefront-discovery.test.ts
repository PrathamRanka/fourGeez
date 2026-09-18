import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  loadPublicProduct,
  loadStorefrontDiscovery,
} from "@/features/storefront/controller";
import { requestPublicAgentPay } from "@/lib/agentpay-api";

vi.mock("@/lib/agentpay-api", () => ({
  requestPublicAgentPay: vi.fn(),
  requestPublicAgentPayText: vi.fn(),
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

describe("authoritative storefront discovery", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-09-18T12:00:00Z"));
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("loads the signed AgentPay storefront contract as active", async () => {
    vi.mocked(requestPublicAgentPay).mockResolvedValue({
      ok: true,
      value: {
        document: {
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
      },
    });

    const result = await loadStorefrontDiscovery("northstar");

    expect(requestPublicAgentPay).toHaveBeenCalledWith(
      "/store/northstar/manifest.json",
    );
    expect(result).toMatchObject({
      status: "active",
      manifest: { publicationRevision: 4, products: [product] },
      signature: { alg: "ES256", kid: "discovery-2026-09" },
    });
  });

  it("preserves a signed inactive tombstone returned with 410", async () => {
    vi.mocked(requestPublicAgentPay).mockResolvedValue({
      ok: false,
      status: 410,
      code: "seller_inactive",
      error: "This seller is inactive.",
      responseBody: {
        document: {
          schemaVersion: "agentpay.discovery.v1",
          sellerId: product.sellerId,
          sellerSlug: "northstar",
          availability: "inactive",
          reason: "cancelled",
          publicationRevision: 5,
          issuedAt: "2026-09-18T11:59:00Z",
          expiresAt: "2026-09-18T12:05:00Z",
          canonicalOrigin: "https://shop.agentpay.example",
        },
        signature,
      },
    });

    await expect(loadStorefrontDiscovery("northstar")).resolves.toMatchObject({
      status: "inactive",
      tombstone: { reason: "cancelled", sellerSlug: "northstar" },
    });
  });

  it("fails closed when a signed manifest has expired", async () => {
    vi.mocked(requestPublicAgentPay).mockResolvedValue({
      ok: true,
      value: {
        document: {
          schemaVersion: "agentpay.discovery.v1",
          sellerId: product.sellerId,
          seller: { name: "Northstar Research", slug: "northstar" },
          availability: "active",
          publicationRevision: 3,
          issuedAt: "2026-09-18T11:50:00Z",
          expiresAt: "2026-09-18T11:55:00Z",
          canonicalOrigin: "https://shop.agentpay.example",
          products: [product],
        },
        signature,
      },
    });

    await expect(loadStorefrontDiscovery("northstar")).resolves.toMatchObject({
      status: "expired",
      sellerSlug: "northstar",
      expiresAt: "2026-09-18T11:55:00Z",
    });
  });

  it("loads one product from the dedicated AgentPay product endpoint", async () => {
    vi.mocked(requestPublicAgentPay).mockResolvedValue({
      ok: true,
      value: {
        document: {
          schemaVersion: "agentpay.discovery.v1",
          sellerId: product.sellerId,
          sellerSlug: "northstar",
          publicationRevision: 4,
          issuedAt: "2026-09-18T11:59:00Z",
          expiresAt: "2026-09-18T12:05:00Z",
          canonicalOrigin: "https://shop.agentpay.example",
          product,
        },
        signature,
      },
    });

    const result = await loadPublicProduct("northstar", "research-report");

    expect(requestPublicAgentPay).toHaveBeenCalledWith(
      "/v1/storefronts/northstar/products/research-report",
    );
    expect(result).toMatchObject({
      status: "active",
      document: { product: { productSlug: "research-report" } },
    });
  });

  it("returns unavailable for malformed or failed discovery", async () => {
    vi.mocked(requestPublicAgentPay)
      .mockResolvedValueOnce({ ok: true, value: { seller: "untrusted" } })
      .mockResolvedValueOnce({
        ok: false,
        status: 503,
        code: "dependency_unavailable",
        error: "Discovery is unavailable.",
      });

    await expect(loadStorefrontDiscovery("northstar")).resolves.toMatchObject({
      status: "unavailable",
    });
    await expect(
      loadPublicProduct("northstar", "research-report"),
    ).resolves.toMatchObject({
      status: "unavailable",
      reason: "dependency_unavailable",
    });
  });
});
