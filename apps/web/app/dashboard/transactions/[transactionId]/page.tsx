import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { loadTransactionDetail } from "@/features/transactions/controller";
import { TransactionDetail } from "@/features/transactions/view/transaction-detail";

export const metadata: Metadata = { title: "Transaction detail" };

type TransactionDetailPageProps = {
  params: Promise<{ transactionId: string }>;
  searchParams: Promise<{ sellerId?: string }>;
};

export default async function TransactionDetailPage({
  params,
  searchParams,
}: TransactionDetailPageProps) {
  const [{ transactionId }, query] = await Promise.all([params, searchParams]);
  const sellerId = query.sellerId ?? process.env.AGENTPAY_DEMO_SELLER_ID;
  if (!sellerId) {
    notFound();
  }
  const snapshot = await loadTransactionDetail(sellerId, transactionId);
  if (!snapshot) {
    notFound();
  }
  return <TransactionDetail snapshot={snapshot} />;
}
