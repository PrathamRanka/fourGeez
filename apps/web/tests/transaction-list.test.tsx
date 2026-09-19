import { render, screen, within } from "@testing-library/react";
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
    status: "FULFILLED",
    amount: "35000000",
    asset: "USDC",
    network: "eip155:84532",
    paymentFinality: "finalized",
    paymentReference: "0xabc123",
    reconciledAt: "2026-09-18T10:01:00Z",
    upstreamStatus: 200,
    responseHash: "b".repeat(64),
    failureCode: null,
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
    status: "DISPUTED",
    amount: "12000000",
    asset: "USDC",
    network: "eip155:84532",
    paymentFinality: "finalized",
    paymentReference: "0xdef456",
    reconciledAt: "2026-09-18T11:01:00Z",
    upstreamStatus: 200,
    responseHash: "c".repeat(64),
    failureCode: null,
    createdAt: "2026-09-18T11:00:00Z",
    updatedAt: "2026-09-18T11:02:00Z",
  },
];

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
    expect(within(table).getAllByText("Finalized")).toHaveLength(2);
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
