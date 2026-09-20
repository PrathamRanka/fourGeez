import { beforeEach, describe, expect, it, vi } from "vitest";
import { loadStorefrontLLMSText } from "@/features/storefront/controller";
import { GET } from "@/app/store/[slug]/llms.txt/route";

vi.mock("@/features/storefront/controller", () => ({
  loadStorefrontLLMSText: vi.fn(),
}));

describe("seller-hosted llms.txt", () => {
  beforeEach(() => vi.resetAllMocks());

  it("serves the current publication without a stale framework cache", async () => {
    vi.mocked(loadStorefrontLLMSText).mockResolvedValue({
      ok: true,
      value: "# Northstar\n\nPublication revision: 7\n",
    });

    const response = await GET(new Request("https://agentpay.example"), {
      params: Promise.resolve({ slug: "northstar" }),
    });

    expect(response.status).toBe(200);
    expect(response.headers.get("Cache-Control")).toBe(
      "public, max-age=0, must-revalidate",
    );
    await expect(response.text()).resolves.toContain("Publication revision: 7");
  });

  it("preserves inactive and dependency failure status without caching", async () => {
    vi.mocked(loadStorefrontLLMSText)
      .mockResolvedValueOnce({
        ok: false,
        status: 410,
        code: "gone",
        error: "inactive",
      })
      .mockResolvedValueOnce({
        ok: false,
        status: 503,
        code: "dependency_unavailable",
        error: "unavailable",
      });

    const context = { params: Promise.resolve({ slug: "northstar" }) };
    const inactive = await GET(
      new Request("https://agentpay.example"),
      context,
    );
    const unavailable = await GET(
      new Request("https://agentpay.example"),
      context,
    );

    expect(inactive.status).toBe(410);
    expect(unavailable.status).toBe(503);
    expect(inactive.headers.get("Cache-Control")).toBe("no-store");
  });
});
