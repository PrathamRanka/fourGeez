import type { OperationFailure } from "@/features/operations/model";

export type TransactionStatus =
  | "PROPOSED"
  | "APPROVAL_PENDING"
  | "APPROVED"
  | "PAYMENT_REQUIRED"
  | "PAYMENT_VERIFIED"
  | "FORWARDED"
  | "FULFILLED"
  | "FAILED"
  | "DISPUTED"
  | "REFUND_RECOMMENDED"
  | "RESOLVED";

export type PaymentFinality = "confirmed" | "finalized" | "failed";

export type ExactPriceBreakdown = {
  calculation: "fixed_single_product";
  quantity: 1;
  unitAmount: string;
  subtotal: string;
  adjustments: string;
  total: string;
  asset: string;
  network: string;
};

export type CommerceLifecycle = {
  externalReference: string;
  commerceState:
    | "awaiting_payment"
    | "payment_processing"
    | "paid"
    | "fulfilling"
    | "fulfilled"
    | "failed"
    | "disputed"
    | "refund_recommended"
    | "resolved";
  paymentState: "pending" | "confirmed" | "finalized" | "failed";
  fulfillmentState: "not_started" | "in_progress" | "succeeded" | "failed";
  refundState: "not_requested" | "disputed" | "recommended" | "seller_reported";
  recoveryAction:
    | "retry_same_request"
    | "await_reconciliation"
    | "create_new_intent"
    | "open_dispute"
    | "await_resolution"
    | "record_external_refund"
    | "none";
};

export type Transaction = {
  transactionId: string;
  intentId: string;
  sellerId: string;
  routeId: string;
  productDisplayName?: string;
  productSlug?: string;
  buyerId: string;
  status: TransactionStatus;
  amount: string;
  asset: string;
  network: string;
  priceBreakdown: ExactPriceBreakdown;
  commerceLifecycle: CommerceLifecycle;
  paymentFinality?: PaymentFinality;
  paymentReference?: string;
  reconciledAt?: string;
  upstreamStatus: number | null;
  responseHash: string | null;
  failureCode: string | null;
  createdAt: string;
  updatedAt: string;
};

export type EvidenceEvent = {
  eventId: string;
  transactionId: string;
  sequence: number;
  eventType: string;
  actorType: "system" | "seller" | "buyer" | "approver";
  actorId: string | null;
  payload: Record<string, unknown>;
  previousEventHash: string | null;
  eventHash: string;
  kmsKeyId: string;
  kmsSignature: string;
  createdAt: string;
};

export type WebhookDeliveryStatus =
  "pending" | "retry_scheduled" | "delivered" | "dead_letter";

export type WebhookDelivery = {
  deliveryId: string;
  sellerId: string;
  subscriptionId: string;
  eventId: string;
  eventType: string;
  payloadHash: string;
  status: WebhookDeliveryStatus;
  attemptCount: number;
  nextAttemptAt: string | null;
  lastAttemptAt: string | null;
  deliveredAt: string | null;
  responseStatusCode: number | null;
  responseBodyHash: string | null;
  errorCode: string | null;
  createdAt: string;
  updatedAt: string;
  version: number;
};

export type TransactionDetailSnapshot = {
  sellerId: string;
  transaction: Transaction;
  evidence: {
    valid: boolean;
    events: EvidenceEvent[];
  };
  webhookDeliveries: WebhookDelivery[];
  error?: string;
};

export type TransactionListSnapshot = {
  sellerId: string;
  transactions: Transaction[];
  error?: string;
  failure?: OperationFailure;
};

export function transactionStatusLabel(status: TransactionStatus): string {
  return status
    .toLowerCase()
    .split("_")
    .map((word) => word[0].toUpperCase() + word.slice(1))
    .join(" ");
}

export function webhookDeliveryStatusLabel(
  status: WebhookDeliveryStatus,
): string {
  const labels: Record<WebhookDeliveryStatus, string> = {
    pending: "Pending",
    retry_scheduled: "Retry scheduled",
    delivered: "Delivered",
    dead_letter: "Dead letter",
  };
  return labels[status];
}
