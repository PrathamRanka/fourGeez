import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type { TransactionDetailSnapshot } from "@/features/transactions/model";
import { TransactionDetail } from "@/features/transactions/view/transaction-detail";

const sellerId = "sel_01ARZ3NDEKTSV4RRFFQ69G5FAV";
const transactionId = "txn_01ARZ3NDEKTSV4RRFFQ69G5FAW";

const snapshot: TransactionDetailSnapshot = {
  sellerId,
  transaction: {
    transactionId,
    intentId: "int_01ARZ3NDEKTSV4RRFFQ69G5FAW",
    sellerId,
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
  evidence: {
    valid: true,
    events: [
      {
        eventId: "evt_01ARZ3NDEKTSV4RRFFQ69G5FAY",
        transactionId,
        sequence: 1,
        eventType: "payment.verified",
        actorType: "system",
        actorId: null,
        payload: { amount: "35000000", asset: "USDC" },
        previousEventHash: null,
        eventHash: "a".repeat(64),
        kmsKeyId: "evidence-key-v1",
        kmsSignature: "signature",
        createdAt: "2026-09-18T10:01:00Z",
      },
    ],
  },
  webhookDeliveries: [
    {
      deliveryId: "whd_01ARZ3NDEKTSV4RRFFQ69G5FAZ",
      sellerId,
      subscriptionId: "whk_01ARZ3NDEKTSV4RRFFQ69G5FB0",
      eventId: "evt_01ARZ3NDEKTSV4RRFFQ69G5FAY",
      eventType: "fulfillment.succeeded",
      payloadHash: "c".repeat(64),
      status: "delivered",
      attemptCount: 1,
      nextAttemptAt: null,
      lastAttemptAt: "2026-09-18T10:03:00Z",
      deliveredAt: "2026-09-18T10:03:00Z",
      responseStatusCode: 204,
      responseBodyHash: null,
      errorCode: null,
      createdAt: "2026-09-18T10:02:00Z",
      updatedAt: "2026-09-18T10:03:00Z",
      version: 2,
    },
  ],
};

describe("transaction detail", () => {
  it("shows payment, fulfillment, verified evidence, delivery history, and receipt download", async () => {
    render(<TransactionDetail snapshot={snapshot} />);

    expect(
      screen.getByRole("heading", { name: "Research Report" }),
    ).toBeVisible();
    expect(screen.getByText(transactionId)).toBeVisible();
    expect(screen.getByText("35 USDC")).toBeVisible();
    expect(screen.getByText("Finalized")).toBeVisible();
    expect(screen.getAllByText("Fulfilled")).toHaveLength(2);
    expect(
      screen.getByRole("region", { name: "Transaction lifecycle" }),
    ).toBeVisible();
    expect(screen.getByText("Payment verified")).toBeVisible();
    expect(screen.getByText("Fulfillment complete")).toBeVisible();
    expect(
      screen.getByText("Payment and delivery proof verified"),
    ).toBeVisible();

    expect(screen.queryByText("Route ID")).not.toBeInTheDocument();
    fireEvent.click(
      screen.getByRole("button", { name: "Advanced technical details" }),
    );
    expect(await screen.findByText("Route ID")).toBeVisible();
    expect(screen.getByText(snapshot.transaction.routeId)).toBeVisible();

    const evidenceList = screen.getByRole("list", { name: "Evidence events" });
    expect(within(evidenceList).getByText("payment.verified")).toBeVisible();
    expect(within(evidenceList).getByText("Sequence 1")).toBeVisible();

    const deliveryTable = screen.getByRole("table", {
      name: "Webhook delivery history",
    });
    expect(
      within(deliveryTable).getByText("fulfillment.succeeded"),
    ).toBeVisible();
    expect(within(deliveryTable).getByText("Delivered")).toBeVisible();

    expect(
      screen.getByRole("link", { name: "Download receipt" }),
    ).toHaveAttribute(
      "href",
      `/dashboard/transactions/${transactionId}/receipt`,
    );
  });

  it("warns when evidence verification fails and withholds the receipt action", () => {
    render(
      <TransactionDetail
        snapshot={{
          ...snapshot,
          evidence: { ...snapshot.evidence, valid: false },
        }}
      />,
    );

    expect(screen.getByRole("alert")).toHaveTextContent(
      "Payment and delivery proof verification failed",
    );
    expect(
      screen.queryByRole("link", { name: "Download receipt" }),
    ).not.toBeInTheDocument();
  });
});
