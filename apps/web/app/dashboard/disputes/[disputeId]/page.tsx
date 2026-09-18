import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { getSellerSession } from "@/features/auth/server/session";
import { loadDispute } from "@/features/disputes/controller";
import { DisputeDetail } from "@/features/disputes/view/dispute-workspace";
import { loadTransactionEvidenceValidity } from "@/features/transactions/controller";

export const metadata: Metadata = { title: "Dispute detail" };

type DisputePageProps = {
  params: Promise<{ disputeId: string }>;
};

export default async function DisputePage({ params }: DisputePageProps) {
  const [{ disputeId }, session] = await Promise.all([
    params,
    getSellerSession(),
  ]);
  const sellerId = session?.principal.sellerId;
  if (!sellerId) {
    notFound();
  }
  const disputeResult = await loadDispute(disputeId);
  if (!disputeResult.ok) {
    notFound();
  }
  const evidenceValid = await loadTransactionEvidenceValidity(
    disputeResult.value.transactionId,
  );
  if (evidenceValid === null) {
    notFound();
  }
  return (
    <DisputeDetail
      dispute={disputeResult.value}
      evidenceValid={evidenceValid}
    />
  );
}
