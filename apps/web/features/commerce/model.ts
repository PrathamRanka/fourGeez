import type { PublicProduct } from "@/features/storefront/model";
import type { Dispute } from "@/features/disputes/model";
import type {
  EvidenceEvent,
  Transaction,
} from "@/features/transactions/model";

export type CommerceChannel = "browser" | "agent";

export type PurchaseIntent = {
  intentId: string;
  routeId: string;
  sellerId: string;
  buyerId: string;
  purchaseSessionId?: string | null;
  purchaseChannel: CommerceChannel;
  productDisplayName: string;
  productSlug: string;
  paymentDestinationId: string;
  payTo: string;
  requestMethod: "GET" | "POST";
  requestPath: string;
  amount: string;
  asset: string;
  network: string;
  requestBodyHash: string;
  intentHash: string;
  maximumAmount: string;
  status: "ready" | "expired" | "executed";
  expiresAt: string;
  createdAt: string;
};

export type CheckoutStartResult = {
  purchaseIntent: PurchaseIntent;
  transactionId?: string;
  traceId: string;
  paymentRequired: string;
  paymentMode: "mock" | "x402";
};

export type CheckoutCompleteResult = {
  status: "fulfilled";
  transactionId: string;
  settlementReference?: string;
  contentType: string;
  fulfillment: unknown;
};

export type CommerceRequest = {
  channel: CommerceChannel;
  sellerSlug: string;
  productSlug: string;
  routeId: string;
  maximumAmount: string;
  requestBody: string;
};

export type CommerceSelection = {
  sellerSlug: string;
  product: PublicProduct;
};

export type BuyerPurchaseSnapshot = {
  transaction: Transaction & {
    purchaseSessionId?: string | null;
    purchaseChannel: CommerceChannel;
    paymentRail: "x402";
    paymentDestinationId: string;
  };
  evidence: {
    valid: boolean;
    events: EvidenceEvent[];
  };
};

export type PurchaseReceipt = {
  schemaVersion: "1" | "2";
  transaction: {
    transactionId: string;
  };
  evidence: {
    verified: true;
    eventCount: number;
    rootEventHash: string;
    headEventHash: string;
    events: EvidenceEvent[];
  };
};

export type BrowserDispute = Dispute;
