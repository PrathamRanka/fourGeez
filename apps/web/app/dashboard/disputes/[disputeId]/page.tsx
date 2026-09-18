import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { loadDispute } from "@/features/disputes/controller";
import { DisputeDetail } from "@/features/disputes/view/dispute-workspace";
import { loadTransactionEvidenceValidity } from "@/features/transactions/controller";

export const metadata: Metadata = { title: "Dispute detail" };

type DisputePageProps = {
  params: Promise<{ disputeId: string }>;
  searchParams: Promise<{ sellerId?: string }>;
};

export default async function DisputePage({ params, searchParams }: DisputePageProps) {
  const [{ disputeId }, query] = await Promise.all([params, searchParams]);
  const sellerId = query.sellerId ?? process.env.AGENTPAY_DEMO_SELLER_ID;
  if (!sellerId) {
    notFound();
  }
  const disputeResult = await loadDispute(disputeId);
  if (!disputeResult.ok) {
    notFound();
  }
  const evidenceValid = await loadTransactionEvidenceValidity(disputeResult.value.transactionId);
  if (evidenceValid === null) {
    notFound();
  }
  return <DisputeDetail dispute={disputeResult.value} evidenceValid={evidenceValid} sellerId={sellerId} />;
}
