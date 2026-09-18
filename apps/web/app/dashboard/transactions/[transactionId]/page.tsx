import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { loadTransactionDetail } from "@/features/transactions/controller";
import { TransactionDetail } from "@/features/transactions/view/transaction-detail";
import { createDispute } from "@/features/disputes/controller";

export const metadata: Metadata = { title: "Transaction detail" };

type TransactionDetailPageProps = {
  params: Promise<{ transactionId: string }>;
};

export default async function TransactionDetailPage({
  params,
}: TransactionDetailPageProps) {
  const { transactionId } = await params;
  const snapshot = await loadTransactionDetail(transactionId);
  if (!snapshot) {
    notFound();
  }
  return (
    <TransactionDetail snapshot={snapshot} createDispute={createDispute} />
  );
}
