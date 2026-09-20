import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import AnalyticsPage from "@/app/dashboard/analytics/page";
import ProductRoutesPage from "@/app/dashboard/products/page";
import TransactionsPage from "@/app/dashboard/transactions/page";
import { loadAnalyticsSnapshot } from "@/features/analytics/controller";
import { getSellerSession } from "@/features/auth/server/session";
import { loadProductRouteSnapshot } from "@/features/products/controller";
import { loadTransactionList } from "@/features/transactions/controller";

const sellerId = "sel_session_owner";

vi.mock("@/features/auth/server/session", () => ({
  getSellerSession: vi.fn(),
}));
vi.mock("@/features/products/controller", () => ({
  archiveRoute: vi.fn(),
  createDraft: vi.fn(),
  emergencyDisableRoute: vi.fn(),
  loadProductRouteSnapshot: vi.fn(),
  pauseRoute: vi.fn(),
  publishRoute: vi.fn(),
  updateDraft: vi.fn(),
  validateRoute: vi.fn(),
}));
vi.mock("@/features/analytics/controller", () => ({
  loadAnalyticsSnapshot: vi.fn(),
}));
vi.mock("@/features/transactions/controller", () => ({
  loadTransactionList: vi.fn(),
}));
vi.mock("@/features/products/view/product-route-workspace", () => ({
  ProductRouteWorkspace: ({
    initialSnapshot,
  }: {
    initialSnapshot: { sellerId: string };
  }) => <p>Products for {initialSnapshot.sellerId}</p>,
}));
vi.mock("@/features/analytics/view/seller-analytics-dashboard", () => ({
  SellerAnalyticsDashboard: ({
    snapshot,
  }: {
    snapshot: { sellerId: string };
  }) => <p>Analytics for {snapshot.sellerId}</p>,
}));

describe("dashboard seller ownership", () => {
  beforeEach(() => {
    vi.mocked(getSellerSession).mockResolvedValue({
      sessionId: "opaque-session",
      accessToken: "server-token",
      expiresAt: "2099-09-18T00:00:00Z",
      principal: {
        subject: "owner-subject",
        email: "owner@example.com",
        name: "Northstar Research",
        sellerId,
        onboardingComplete: true,
      },
    });
    vi.mocked(loadProductRouteSnapshot).mockResolvedValue({
      sellerId,
      routes: [],
      auditEvents: [],
    });
    vi.mocked(loadAnalyticsSnapshot).mockResolvedValue({
      sellerId,
      transactionCount: 0,
      aggregates: [],
      routes: [],
    });
    vi.mocked(loadTransactionList).mockResolvedValue({
      sellerId,
      transactions: [
        {
          transactionId: "txn_123",
          intentId: "int_123",
          sellerId,
          routeId: "rte_123",
          buyerId: "buyer",
          status: "FULFILLED",
          amount: "1000000",
          asset: "USDC",
          network: "eip155:84532",
          priceBreakdown: {
            calculation: "fixed_single_product",
            quantity: 1,
            unitAmount: "1000000",
            subtotal: "1000000",
            adjustments: "0",
            total: "1000000",
            asset: "USDC",
            network: "eip155:84532",
          },
          commerceLifecycle: {
            externalReference: "txn_123",
            commerceState: "fulfilled",
            paymentState: "finalized",
            fulfillmentState: "succeeded",
            refundState: "not_requested",
            recoveryAction: "none",
          },
          paymentFinality: "finalized",
          paymentReference: "payment-reference",
          upstreamStatus: 200,
          responseHash: null,
          failureCode: null,
          createdAt: "2026-09-18T00:00:00Z",
          updatedAt: "2026-09-18T00:00:00Z",
        },
      ],
    });
  });

  it("ignores attacker-controlled seller IDs on products and analytics", async () => {
    render(await ProductRoutesPage());
    expect(loadProductRouteSnapshot).toHaveBeenCalledWith();
    expect(screen.getByText(`Products for ${sellerId}`)).toBeVisible();

    render(await AnalyticsPage());
    expect(loadAnalyticsSnapshot).toHaveBeenCalledWith();
  });

  it("builds transaction links without seller identity in the URL", async () => {
    render(await TransactionsPage());
    expect(loadTransactionList).toHaveBeenCalledWith();
    expect(
      screen.getByRole("link", {
        name: "Open Digital purchase transaction",
      }),
    ).toHaveAttribute("href", "/dashboard/transactions/txn_123");
  });
});
