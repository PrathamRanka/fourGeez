import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type { Transaction } from "@/features/transactions/model";
import { TransactionList } from "@/features/transactions/view/transaction-list";

const transactions: Transaction[] = [
  {
    transactionId: "txn_01ARZ3NDEKTSV4RRFFQ69G5FAW",
    intentId: "int_01ARZ3NDEKTSV4RRFFQ69G5FAW",
    sellerId: "sel_01ARZ3NDEKTSV4RRFFQ69G5FAV",
    routeId: "rte_01ARZ3NDEKTSV4RRFFQ69G5FAX",
    productDisplayName: "Research Report",
    productSlug: "research-report",
    buyerId: "buyer-demo",
    activityMode: "live",
    checkoutExpiresAt: "2026-09-18T10:05:00Z",
    sellerOutcome: "fulfilled",
    purchaseChannel: "browser",
    status: "FULFILLED",
    amount: "35000000",
    asset: "USDC",
    network: "eip155:84532",
    priceBreakdown: {
      calculation: "fixed_single_product",
      quantity: 1,
      unitAmount: "35000000",
      subtotal: "35000000",
      adjustments: "0",
      total: "35000000",
      asset: "USDC",
      network: "eip155:84532",
    },
    commerceLifecycle: {
      externalReference: "txn_01ARZ3NDEKTSV4RRFFQ69G5FAW",
      commerceState: "fulfilled",
      paymentState: "finalized",
      fulfillmentState: "succeeded",
      refundState: "not_requested",
      recoveryAction: "none",
      recoveryState: "none",
    },
    paymentFinality: "finalized",
    paymentReference: "0xabc123",
    reconciledAt: "2026-09-18T10:01:00Z",
    upstreamStatus: 200,
    responseHash: "b".repeat(64),
    failureCode: null,
    fulfillmentAttempts: 1,
    createdAt: "2026-09-18T10:00:00Z",
    updatedAt: "2026-09-18T10:02:00Z",
  },
  {
    transactionId: "txn_01ARZ3NDEKTSV4RRFFQ69G5FB1",
    intentId: "int_01ARZ3NDEKTSV4RRFFQ69G5FB1",
    sellerId: "sel_01ARZ3NDEKTSV4RRFFQ69G5FAV",
    routeId: "rte_01ARZ3NDEKTSV4RRFFQ69G5FB2",
    productDisplayName: "Data Export",
    productSlug: "data-export",
    buyerId: "agent-ops",
    activityMode: "live",
    checkoutExpiresAt: "2026-09-18T11:05:00Z",
    sellerOutcome: "disputed",
    purchaseChannel: "agent",
    status: "DISPUTED",
    amount: "12000000",
    asset: "USDC",
    network: "eip155:84532",
    priceBreakdown: {
      calculation: "fixed_single_product",
      quantity: 1,
      unitAmount: "12000000",
      subtotal: "12000000",
      adjustments: "0",
      total: "12000000",
      asset: "USDC",
      network: "eip155:84532",
    },
    commerceLifecycle: {
      externalReference: "txn_01ARZ3NDEKTSV4RRFFQ69G5FB1",
      commerceState: "disputed",
      paymentState: "finalized",
      fulfillmentState: "succeeded",
      refundState: "disputed",
      recoveryAction: "await_resolution",
      recoveryState: "dispute_open",
    },
    paymentFinality: "finalized",
    paymentReference: "0xdef456",
    reconciledAt: "2026-09-18T11:01:00Z",
    upstreamStatus: 200,
    responseHash: "c".repeat(64),
    failureCode: null,
    fulfillmentAttempts: 1,
    createdAt: "2026-09-18T11:00:00Z",
    updatedAt: "2026-09-18T11:02:00Z",
  },
];

const abandonedTestTransaction: Transaction = {
  ...transactions[0],
  transactionId: "txn_01ARZ3NDEKTSV4RRFFQ69G5FB3",
  intentId: "int_01ARZ3NDEKTSV4RRFFQ69G5FB3",
  productDisplayName: "Test checkout",
  activityMode: "test",
  sellerOutcome: "abandoned",
  status: "PAYMENT_REQUIRED",
  paymentFinality: undefined,
  paymentReference: undefined,
  commerceLifecycle: {
    externalReference: "txn_01ARZ3NDEKTSV4RRFFQ69G5FB3",
    commerceState: "abandoned",
    paymentState: "expired",
    fulfillmentState: "not_started",
    refundState: "not_requested",
    recoveryAction: "create_new_intent",
    recoveryState: "none",
  },
};

describe("transaction list", () => {
  it("renders a dense, accessible ledger with status and settlement facts", () => {
    render(<TransactionList transactions={transactions} />);

    expect(screen.getByRole("heading", { name: "Transactions" })).toBeVisible();
    expect(screen.getByText("2 records")).toBeVisible();
    expect(screen.getByText("1 fulfilled")).toBeVisible();
    expect(screen.getByText("1 needs review")).toBeVisible();

    const table = screen.getByRole("table", { name: "Seller transactions" });
    expect(within(table).getByText("Research Report")).toBeVisible();
    expect(within(table).getByText("buyer-demo")).toBeVisible();
    expect(within(table).getAllByText("Payment finalized")).toHaveLength(2);
    expect(within(table).getByText("Browser")).toBeVisible();
    expect(within(table).getByText("External agent")).toBeVisible();
    expect(within(table).getByText("Disputed")).toBeVisible();
    expect(
      within(table).getByRole("link", {
        name: "Open Research Report transaction",
      }),
    ).toHaveAttribute(
      "href",
      "/dashboard/transactions/txn_01ARZ3NDEKTSV4RRFFQ69G5FAW",
    );
  });

  it("separates test activity and filters abandoned checkouts", () => {
    render(
      <TransactionList
        transactions={[...transactions, abandonedTestTransaction]}
      />,
    );

    expect(screen.queryByText("Test checkout")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Test activity" }));
    expect(screen.getByText("Test checkout")).toBeVisible();
    expect(screen.getByText("Abandoned checkout")).toBeVisible();
    expect(screen.getByText("Checkout expired")).toBeVisible();
    expect(screen.getByText("Not started")).toBeVisible();

    fireEvent.change(screen.getByRole("combobox", { name: "Outcome" }), {
      target: { value: "fulfilled" },
    });
    expect(screen.queryByText("Test checkout")).not.toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "No test activity" }),
    ).toBeVisible();
  });

  it("renders a focused dispute queue without unrelated sales", () => {
    render(<TransactionList transactions={transactions} view="disputes" />);

    expect(screen.getByRole("heading", { name: "Disputes" })).toBeVisible();
    expect(screen.getByText("1 case")).toBeVisible();
    expect(screen.queryByText("Research Report")).not.toBeInTheDocument();
    expect(screen.getByText("Data Export")).toBeVisible();
    expect(
      screen.getByRole("table", { name: "Seller disputes" }),
    ).toBeVisible();
  });
});
