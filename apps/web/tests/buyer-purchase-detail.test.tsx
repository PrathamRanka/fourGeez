import { fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { BuyerPurchaseDetail } from "@/features/commerce/view/buyer-purchase-detail";

const transactionId = "txn_01ARZ3NDEKTSV4RRFFQ69G5FAV";
const disputeId = "dsp_01ARZ3NDEKTSV4RRFFQ69G5FAW";

const purchase = {
  transaction: {
    transactionId,
    intentId: "int_01ARZ3NDEKTSV4RRFFQ69G5FAX",
    sellerId: "sel_01ARZ3NDEKTSV4RRFFQ69G5FAY",
    routeId: "rte_01ARZ3NDEKTSV4RRFFQ69G5FAZ",
    buyerId: "browser:bps_01ARZ3NDEKTSV4RRFFQ69G5FB0",
    purchaseSessionId: "bps_01ARZ3NDEKTSV4RRFFQ69G5FB0",
    purchaseChannel: "browser",
    paymentRail: "x402",
    productDisplayName: "Market Snapshot",
    productSlug: "market-snapshot",
    paymentDestinationId: "dst_01ARZ3NDEKTSV4RRFFQ69G5FB1",
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
        eventId: "evt_01ARZ3NDEKTSV4RRFFQ69G5FB2",
        transactionId,
        sequence: 1,
        eventType: "fulfillment.succeeded",
        actorType: "system",
        actorId: null,
        payload: { upstreamStatus: 200 },
        previousEventHash: null,
        eventHash: "a".repeat(64),
        kmsKeyId: "evidence-key-v1",
        kmsSignature: "signature",
        createdAt: "2026-09-18T10:02:00Z",
      },
    ],
  },
};

const receipt = {
  schemaVersion: "2",
  transaction: purchase.transaction,
  evidence: {
    verified: true,
    eventCount: 1,
    rootEventHash: "a".repeat(64),
    headEventHash: "a".repeat(64),
    events: purchase.evidence.events,
  },
};

const dispute = {
  disputeId,
  transactionId,
  reason: "not_delivered",
  statement: "The delivered file was empty.",
  status: "refund_recommended",
  ruleVersion: "dispute-rules-v1",
  classificationCode: "delivery_not_confirmed",
  explanation: "A refund is recommended from the recorded purchase facts.",
  createdAt: "2026-09-18T10:05:00Z",
};

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

describe("buyer purchase detail", () => {
  beforeEach(() => {
    vi.unstubAllGlobals();
    window.history.replaceState({}, "", "/");
  });

  it("shows the outcome, verifies and downloads the receipt, exposes evidence, and creates a durable dispute link", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(jsonResponse(purchase))
      .mockResolvedValueOnce(jsonResponse(receipt))
      .mockResolvedValueOnce(jsonResponse(dispute, 201));
    vi.stubGlobal("fetch", fetchMock);

    render(<BuyerPurchaseDetail transactionId={transactionId} />);

    expect(screen.getByRole("status")).toHaveTextContent(
      "Loading purchase record",
    );
    expect(
      await screen.findByRole("heading", { name: "Market Snapshot" }),
    ).toBeVisible();
    expect(
      within(screen.getByRole("region", { name: "Purchase summary" })).getByText(
        "Fulfilled",
      ),
    ).toBeVisible();
    expect(screen.getByText("35 USDC")).toBeVisible();
    expect(screen.getAllByText("Evidence chain verified")).toHaveLength(2);
    expect(screen.getByText("fulfillment.succeeded")).toBeVisible();

    const download = screen.getByRole("link", {
      name: "Download receipt",
    });
    expect(download).toHaveAttribute(
      "href",
      `/api/commerce/purchases/${transactionId}/receipt`,
    );

    fireEvent.click(screen.getByRole("button", { name: "Verify receipt" }));
    expect(await screen.findByText("Receipt verified")).toBeVisible();
    expect(screen.getByText("Schema version 2 · 1 evidence event")).toBeVisible();

    fireEvent.change(screen.getByLabelText("Dispute reason"), {
      target: { value: "not_delivered" },
    });
    fireEvent.change(screen.getByLabelText("Statement"), {
      target: { value: dispute.statement },
    });
    fireEvent.submit(screen.getByRole("form", { name: "Open dispute" }));

    expect(await screen.findByText("Refund recommended")).toBeVisible();
    expect(screen.getByText(dispute.explanation)).toBeVisible();
    const disputeLink = screen.getByRole("link", {
      name: "Open saved dispute link",
    });
    expect(disputeLink).toHaveAttribute(
      "href",
      `/purchases/${transactionId}?dispute=${disputeId}`,
    );
    expect(window.location.search).toBe(`?dispute=${disputeId}`);
    expect(fetchMock).toHaveBeenNthCalledWith(
      3,
      `/api/commerce/purchases/${transactionId}/disputes`,
      expect.objectContaining({ method: "POST" }),
    );
  });

  it("loads an existing dispute beside the purchase without hiding the transaction", async () => {
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockResolvedValueOnce(jsonResponse(purchase))
        .mockResolvedValueOnce(jsonResponse(dispute)),
    );

    render(
      <BuyerPurchaseDetail
        initialDisputeId={disputeId}
        transactionId={transactionId}
      />,
    );

    expect(
      await screen.findByRole("heading", { name: "Market Snapshot" }),
    ).toBeVisible();
    expect(screen.getByText("Refund recommended")).toBeVisible();
    expect(screen.getByText("delivery_not_confirmed")).toBeVisible();
  });

  it("gives a retry path when durable purchase access cannot be loaded", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        jsonResponse(
          {
            error: {
              code: "purchase_session_expired",
              message: "Purchase access expired. Recover it with the paying wallet.",
            },
          },
          401,
        ),
      )
      .mockResolvedValueOnce(jsonResponse(purchase));
    vi.stubGlobal("fetch", fetchMock);

    render(<BuyerPurchaseDetail transactionId={transactionId} />);

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Purchase access expired. Recover it with the paying wallet.",
    );
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(
      await screen.findByRole("heading", { name: "Market Snapshot" }),
    ).toBeVisible();
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });
});
