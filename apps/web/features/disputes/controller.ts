"use server";

import type {
  CreateDisputeInput,
  Dispute,
  ManualRefundRecord,
} from "@/features/disputes/model";
import { requestAgentPay } from "@/lib/agentpay-api";

export async function createDispute(input: CreateDisputeInput) {
  return requestAgentPay<Dispute>("/v1/disputes", {
    method: "POST",
    body: input,
  });
}

export async function loadManualRefundRecord(
  sellerId: string,
  disputeId: string,
) {
  return requestAgentPay<ManualRefundRecord>(
    `/v1/sellers/${encodeURIComponent(sellerId)}/disputes/${encodeURIComponent(disputeId)}/refund-records/current`,
    { method: "GET" },
  );
}

export async function loadDispute(disputeId: string) {
  return requestAgentPay<Dispute>(
    `/v1/disputes/${encodeURIComponent(disputeId)}`,
    { method: "GET" },
  );
}
