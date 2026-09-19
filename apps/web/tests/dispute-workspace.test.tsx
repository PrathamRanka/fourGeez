import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { DisputeAction } from "@/features/disputes/model";
import type { Dispute } from "@/features/disputes/model";
import {
  DisputeDetail,
  DisputePanel,
} from "@/features/disputes/view/dispute-workspace";

const dispute: Dispute = {
  disputeId: "dsp_01ARZ3NDEKTSV4RRFFQ69G5FAV",
  transactionId: "txn_01ARZ3NDEKTSV4RRFFQ69G5FAW",
  reason: "not_delivered" as const,
  statement: "The upstream request timed out.",
  status: "refund_recommended" as const,
  ruleVersion: "dispute-rules-v1",
  classificationCode: "delivery_not_confirmed",
  explanation:
    "Successful seller delivery was not recorded; a refund is recommended.",
  createdAt: "2026-09-18T10:00:00Z",
};

describe("dispute workspace", () => {
  it("creates a dispute and shows its deterministic resolution", async () => {
    const createDispute: DisputeAction = vi
      .fn()
      .mockResolvedValue({ ok: true, value: dispute });
    render(
      <DisputePanel
        transactionId={dispute.transactionId}
        transactionStatus="FAILED"
        evidenceValid
        createDispute={createDispute}
      />,
    );

    fireEvent.change(screen.getByLabelText("Dispute reason"), {
      target: { value: "not_delivered" },
    });
    fireEvent.change(screen.getByLabelText("Statement"), {
      target: { value: dispute.statement },
    });
    fireEvent.submit(screen.getByRole("form", { name: "Open dispute" }));

    await waitFor(() =>
      expect(createDispute).toHaveBeenCalledWith({
        transactionId: dispute.transactionId,
        reason: "not_delivered",
        statement: dispute.statement,
      }),
    );
    expect(await screen.findByText("Refund recommended")).toBeVisible();
    expect(screen.getByText("delivery_not_confirmed")).toBeVisible();
    expect(
      screen.getByRole("link", { name: "View dispute record" }),
    ).toHaveAttribute("href", `/dashboard/disputes/${dispute.disputeId}`);
  });

  it("shows evidence verification and a resolved dispute record", () => {
    render(
      <DisputeDetail
        dispute={{ ...dispute, status: "resolved" }}
        evidenceValid
        refundRecord={{
          disputeId: dispute.disputeId,
          transactionId: dispute.transactionId,
          sellerId: "sel_01ARZ3NDEKTSV4RRFFQ69G5FAX",
          amount: "100000",
          asset: "USDC",
          network: "eip155:84532",
          reference: "0xrefund",
          verificationState: "seller_reported",
          recordedBy: "seller-owner",
          recordedAt: "2026-09-18T11:00:00Z",
        }}
      />,
    );

    expect(screen.getByText("Evidence chain verified")).toBeVisible();
    expect(screen.getByRole("heading", { name: "Resolved" })).toBeVisible();
    expect(
      screen.getByRole("region", { name: "Dispute classification" }),
    ).toBeVisible();
    expect(screen.getByText("Recorded facts")).toBeVisible();
    expect(screen.getByText(dispute.explanation)).toBeVisible();
    expect(screen.getByText("dispute-rules-v1")).toBeVisible();
    expect(screen.getByText("Seller-reported external refund")).toBeVisible();
    expect(screen.getByText("Not network-verified by AgentPay.")).toBeVisible();
    expect(screen.getByText("0xrefund")).toBeVisible();
  });
});
