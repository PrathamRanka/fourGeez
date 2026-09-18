import { describe, expect, it, vi } from "vitest";
import DashboardLayout from "@/app/dashboard/layout";
import { getSellerSession } from "@/features/auth/server/session";
import { headers } from "next/headers";
import { redirect } from "next/navigation";

vi.mock("@/features/auth/server/session", () => ({
  getSellerSession: vi.fn(),
}));
vi.mock("next/headers", () => ({ headers: vi.fn() }));
vi.mock("next/navigation", () => ({ redirect: vi.fn() }));

describe("dashboard layout protection", () => {
  it("returns an expired session to sign-in with only a safe relative destination", async () => {
    vi.mocked(getSellerSession).mockResolvedValue(null);
    vi.mocked(headers).mockResolvedValue(
      new Headers({
        "x-agentpay-return-path": "/dashboard/transactions?status=failed",
      }) as never,
    );
    vi.mocked(redirect).mockImplementation(() => {
      throw new Error("NEXT_REDIRECT");
    });

    await expect(
      DashboardLayout({ children: <p>Protected</p> }),
    ).rejects.toThrow("NEXT_REDIRECT");
    expect(redirect).toHaveBeenCalledWith(
      "/sign-in?returnTo=%2Fdashboard%2Ftransactions%3Fstatus%3Dfailed",
    );
  });
});
