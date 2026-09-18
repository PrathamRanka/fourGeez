"use server";

import type {
  CreateDisputeInput,
  Dispute,
} from "@/features/disputes/model";
import { requestAgentPay } from "@/lib/agentpay-api";

export async function createDispute(input: CreateDisputeInput) {
  return requestAgentPay<Dispute>("/v1/disputes", {
    method: "POST",
    body: input,
  });
}

export async function loadDispute(disputeId: string) {
  return requestAgentPay<Dispute>(
    `/v1/disputes/${encodeURIComponent(disputeId)}`,
    { method: "GET" },
  );
}

