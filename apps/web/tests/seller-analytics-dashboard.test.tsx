import { render, screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type { AnalyticsSnapshot } from "@/features/analytics/model";
import { SellerAnalyticsDashboard } from "@/features/analytics/view/seller-analytics-dashboard";

const sellerId = "sel_01ARZ3NDEKTSV4RRFFQ69G5FAV";

const snapshot: AnalyticsSnapshot = {
  sellerId,
  transactionCount: 5,
  routes: [
    {
      routeId: "rte_01ARZ3NDEKTSV4RRFFQ69G5FAW",
      displayName: "Research Report",
      pathPattern: "/research",
    },
  ],
  aggregates: [
    {
      sellerId,
      bucketDate: "2026-09-18",
      asset: "USDC",
      network: "eip155:84532",
      stage: "finalized",
      transactionCount: 1,
      amount: "15000000",
      lastTransactionAt: "2026-09-18T10:00:00Z",
    },
    {
      sellerId,
      bucketDate: "2026-09-18",
      asset: "USDC",
      network: "eip155:84532",
      stage: "disputed",
      transactionCount: 1,
      amount: "20000000",
      lastTransactionAt: "2026-09-18T11:00:00Z",
    },
    {
      sellerId,
      bucketDate: "2026-09-18",
      asset: "USDC",
      network: "eip155:84532",
      stage: "failed",
      transactionCount: 1,
      amount: "10000000",
      lastTransactionAt: "2026-09-18T12:00:00Z",
    },
    {
      sellerId,
      bucketDate: "2026-09-17",
      asset: "USDC",
      network: "eip155:84532",
      stage: "fulfilled",
      transactionCount: 1,
      amount: "35000000",
      lastTransactionAt: "2026-09-17T12:00:00Z",
    },
    {
      sellerId,
      bucketDate: "2026-09-17",
      asset: "USDC",
      network: "eip155:84532",
      routeId: "rte_01ARZ3NDEKTSV4RRFFQ69G5FAW",
      stage: "fulfilled",
      transactionCount: 1,
      amount: "35000000",
      lastTransactionAt: "2026-09-17T12:00:00Z",
    },
    {
      sellerId,
      bucketDate: "2026-09-18",
      asset: "EURC",
      network: "eip155:11155111",
      stage: "fulfilled",
      transactionCount: 1,
      amount: "9000000",
      lastTransactionAt: "2026-09-18T13:00:00Z",
    },
  ],
};

describe("seller analytics dashboard", () => {
  it("keeps unlike assets and networks in separate revenue summaries", () => {
    render(<SellerAnalyticsDashboard snapshot={snapshot} />);

    expect(screen.getByRole("heading", { name: "Revenue Lens" })).toBeVisible();
    expect(
      screen.getByRole("status", {
        name: "5 transactions in this window",
      }),
    ).toBeVisible();

    const usdcSummary = screen.getByRole("region", {
      name: "USDC on eip155:84532",
    });
    expect(within(usdcSummary).getByText("70 USDC")).toBeVisible();
    expect(within(usdcSummary).getByText("35 USDC")).toBeVisible();
    expect(within(usdcSummary).getByText("10 USDC")).toBeVisible();
    expect(within(usdcSummary).getByText("20 USDC")).toBeVisible();

    const eurcSummary = screen.getByRole("region", {
      name: "EURC on eip155:11155111",
    });
    expect(within(eurcSummary).getAllByText("9 EURC")).toHaveLength(2);
    expect(screen.queryByText("79")).not.toBeInTheDocument();
  });

  it("shows daily activity and route performance without counting route rows twice", () => {
    render(<SellerAnalyticsDashboard snapshot={snapshot} />);

    expect(
      screen.getByRole("img", { name: "Daily sales activity" }),
    ).toBeVisible();
    const legend = screen.getByRole("list", {
      name: "Reconciliation stages",
    });
    expect(within(legend).getByText("Fulfilled")).toBeVisible();
    expect(within(legend).getByText("Processing")).toBeVisible();
    expect(within(legend).getByText("Failed")).toBeVisible();
    expect(within(legend).getByText("Disputed")).toBeVisible();
    const accessibleSeries = screen.getByRole("table", {
      name: "Daily reconciliation totals",
    });
    expect(
      within(accessibleSeries).getByText("2026-09-17"),
    ).toBeInTheDocument();
    expect(
      within(accessibleSeries).getByText("2026-09-18"),
    ).toBeInTheDocument();
    const routeTable = screen.getByRole("table", {
      name: "Product performance",
    });
    expect(within(routeTable).getByText("Research Report")).toBeVisible();
    expect(within(routeTable).getByText("1 fulfilled")).toBeVisible();
    expect(within(routeTable).getByText("35 USDC")).toBeVisible();
  });

  it("renders an actionable empty state", () => {
    render(
      <SellerAnalyticsDashboard
        snapshot={{ ...snapshot, transactionCount: 0, aggregates: [] }}
      />,
    );

    expect(screen.getByText("No sales in this window yet")).toBeVisible();
    expect(
      screen.getByRole("link", { name: "Review products" }),
    ).toHaveAttribute("href", "/dashboard/products");
  });

  it("shows a distinct retryable reporting error without an empty-sales prompt", () => {
    render(
      <SellerAnalyticsDashboard
        snapshot={{
          ...snapshot,
          transactionCount: 0,
          aggregates: [],
          error: "The reporting service is unavailable.",
        }}
      />,
    );

    expect(screen.getByRole("alert")).toHaveTextContent(
      "The reporting service is unavailable.",
    );
    expect(
      screen.getByRole("link", { name: "Reload analytics" }),
    ).toHaveAttribute("href", "/dashboard/analytics");
    expect(
      screen.queryByText("No sales in this window yet"),
    ).not.toBeInTheDocument();
  });
});
