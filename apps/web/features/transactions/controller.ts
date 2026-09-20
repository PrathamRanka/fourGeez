"use server";

import type {
  Transaction,
  TransactionDetailSnapshot,
  TransactionListSnapshot,
  WebhookDelivery,
} from "@/features/transactions/model";
import { authenticatedSellerId } from "@/features/auth/server/authorization";
import { requestAgentPay } from "@/lib/agentpay-api";

// loadTransactionList returns the newest bounded seller transaction page.
export async function loadTransactionList(): Promise<TransactionListSnapshot> {
  const sellerId = await authenticatedSellerId();
  if (!sellerId) {
    return {
      sellerId: "",
      transactions: [],
      error: "Your seller session has expired. Sign in again.",
      failure: { code: "unauthorized", status: 401 },
    };
  }
  const encodedSellerId = encodeURIComponent(sellerId);
  const [liveResult, testResult] = await Promise.all([
    requestAgentPay<{ items: Transaction[] }>(
      `/v1/sellers/${encodedSellerId}/transactions?limit=50&activityMode=live`,
      { method: "GET" },
    ),
    requestAgentPay<{ items: Transaction[] }>(
      `/v1/sellers/${encodedSellerId}/transactions?limit=50&activityMode=test`,
      { method: "GET" },
    ),
  ]);
  const failedResult = !liveResult.ok
    ? liveResult
    : !testResult.ok
      ? testResult
      : null;
  const transactions =
    liveResult.ok && testResult.ok
      ? [...liveResult.value.items, ...testResult.value.items].toSorted(
          (left, right) => right.updatedAt.localeCompare(left.updatedAt),
        )
      : [];
  return {
    sellerId,
    transactions,
    error: failedResult?.error,
    failure: failedResult
      ? {
          code: failedResult.code,
          status: failedResult.status,
          retryAfterSeconds: failedResult.retryAfterSeconds,
        }
      : undefined,
  };
}

// loadTransactionDetail fetches independent evidence detail and delivery history in parallel.
export async function loadTransactionDetail(
  transactionId: string,
): Promise<TransactionDetailSnapshot | null> {
  const sellerId = await authenticatedSellerId();
  if (!sellerId) return null;
  const [detailResult, deliveryResult] = await Promise.all([
    requestAgentPay<
      Pick<TransactionDetailSnapshot, "transaction" | "evidence">
    >(`/v1/transactions/${encodeURIComponent(transactionId)}`, {
      method: "GET",
    }),
    requestAgentPay<{ items: WebhookDelivery[] }>(
      `/v1/sellers/${encodeURIComponent(sellerId)}/webhook-deliveries?limit=50`,
      { method: "GET" },
    ),
  ]);

  if (!detailResult.ok) {
    return null;
  }
  return {
    sellerId,
    ...detailResult.value,
    webhookDeliveries: deliveryResult.ok ? deliveryResult.value.items : [],
    error: deliveryResult.ok ? undefined : deliveryResult.error,
  };
}

// loadTransactionEvidenceValidity reads only the transaction evidence needed by dispute detail.
export async function loadTransactionEvidenceValidity(
  transactionId: string,
): Promise<boolean | null> {
  const result = await requestAgentPay<
    Pick<TransactionDetailSnapshot, "evidence">
  >(`/v1/transactions/${encodeURIComponent(transactionId)}`, {
    method: "GET",
  });
  return result.ok ? result.value.evidence.valid : null;
}
