import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { BuyerPurchaseDetail } from "@/features/commerce/view/buyer-purchase-detail";

export const metadata: Metadata = {
  title: "Purchase details",
  robots: { index: false, follow: false },
};

type BuyerPurchasePageProps = {
  params: Promise<{ transactionId: string }>;
  searchParams: Promise<{ dispute?: string | string[] }>;
};

export default async function BuyerPurchasePage({
  params,
  searchParams,
}: BuyerPurchasePageProps) {
  const [{ transactionId }, query] = await Promise.all([params, searchParams]);
  if (!/^txn_[A-Za-z0-9]+$/.test(transactionId)) {
    notFound();
  }
  const disputeId = Array.isArray(query.dispute)
    ? query.dispute[0]
    : query.dispute;
  return (
    <BuyerPurchaseDetail
      transactionId={transactionId}
      initialDisputeId={
        disputeId && /^dsp_[A-Za-z0-9]+$/.test(disputeId)
          ? disputeId
          : undefined
      }
    />
  );
}
