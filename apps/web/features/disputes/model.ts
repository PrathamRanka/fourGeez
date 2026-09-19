import type { ActionResult } from "@/lib/agentpay-api";

export type DisputeReason =
  | "unauthorized"
  | "duplicate"
  | "wrong_amount"
  | "not_delivered"
  | "quality_or_output";

export type DisputeStatus =
  "open" | "refund_recommended" | "seller_review" | "denied" | "resolved";

export type Dispute = {
  disputeId: string;
  transactionId: string;
  reason: DisputeReason;
  statement?: string;
  status: DisputeStatus;
  ruleVersion: "dispute-rules-v1";
  classificationCode: string;
  explanation: string;
  createdAt: string;
};

export type ManualRefundRecord = {
  disputeId: string;
  transactionId: string;
  sellerId: string;
  amount: string;
  asset: string;
  network: string;
  reference: string;
  verificationState: "seller_reported";
  recordedBy: string;
  recordedAt: string;
};

export type CreateDisputeInput = {
  transactionId: string;
  reason: DisputeReason;
  statement: string;
};

export type DisputeAction = (
  input: CreateDisputeInput,
) => Promise<ActionResult<Dispute>>;

export function disputeStatusLabel(status: DisputeStatus): string {
  const labels: Record<DisputeStatus, string> = {
    open: "Open",
    refund_recommended: "Refund recommended",
    seller_review: "Seller review",
    denied: "Denied",
    resolved: "Resolved",
  };
  return labels[status];
}

export function disputeReasonLabel(reason: DisputeReason): string {
  const labels: Record<DisputeReason, string> = {
    unauthorized: "Unauthorized purchase",
    duplicate: "Duplicate payment",
    wrong_amount: "Wrong amount",
    not_delivered: "Not delivered",
    quality_or_output: "Quality or output",
  };
  return labels[reason];
}
