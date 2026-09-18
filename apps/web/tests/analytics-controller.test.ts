import { beforeEach, describe, expect, it, vi } from "vitest";
import { authenticatedSellerId } from "@/features/auth/server/authorization";
import { loadAnalyticsSnapshot } from "@/features/analytics/controller";
import { requestAgentPay } from "@/lib/agentpay-api";

vi.mock("@/features/auth/server/authorization", () => ({
  authenticatedSellerId: vi.fn(),
}));

vi.mock("@/lib/agentpay-api", () => ({
  requestAgentPay: vi.fn(),
}));

describe("analytics controller", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-09-19T12:00:00.000Z"));
    vi.mocked(authenticatedSellerId).mockResolvedValue("sel_owner");
    vi.mocked(requestAgentPay)
      .mockResolvedValueOnce({
        ok: true,
        value: { transactionCount: 0, aggregates: [] },
      })
      .mockResolvedValueOnce({ ok: true, value: { items: [] } });
  });

  it("requests the bounded 30-day UTC analytics window required by the API", async () => {
    await loadAnalyticsSnapshot();

    expect(requestAgentPay).toHaveBeenNthCalledWith(
      1,
      "/v1/sellers/sel_owner/dashboard-summary?from=2026-08-20T12%3A00%3A00.000Z&to=2026-09-19T12%3A00%3A00.000Z",
      { method: "GET" },
    );
  });
});
