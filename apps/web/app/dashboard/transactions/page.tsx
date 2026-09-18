import type { Metadata } from "next";
import Link from "next/link";
import { buttonVariants } from "@/components/ui/button";
import { loadTransactionList } from "@/features/transactions/controller";
import { transactionStatusLabel } from "@/features/transactions/model";
import { formatAtomicPrice } from "@/lib/money";

export const metadata: Metadata = { title: "Transactions" };

type TransactionsPageProps = {
  searchParams: Promise<{ sellerId?: string }>;
};

export default async function TransactionsPage({ searchParams }: TransactionsPageProps) {
  const parameters = await searchParams;
  const sellerId = parameters.sellerId ?? process.env.AGENTPAY_DEMO_SELLER_ID;
  if (!sellerId) {
    return <p className="dashboard-error">Choose a storefront in onboarding first.</p>;
  }
  const snapshot = await loadTransactionList(sellerId);
  return (
    <div className="transaction-list-workspace">
      <header className="transaction-list-header">
        <p className="dashboard-eyebrow">Proof Stream</p>
        <h1>Transactions</h1>
        <p>Open a purchase to verify its payment, fulfillment, and evidence chain.</p>
      </header>
      {snapshot.error ? <div className="dashboard-error" role="alert">{snapshot.error}</div> : null}
      {snapshot.transactions.length === 0 ? (
        <div className="transaction-panel transaction-empty-copy">No transactions recorded yet.</div>
      ) : (
        <div className="transaction-list">
          {snapshot.transactions.map((transaction) => (
            <article key={transaction.transactionId}>
              <div>
                <strong>{transaction.transactionId}</strong>
                <span>{transactionStatusLabel(transaction.status)} · {transaction.network}</span>
              </div>
              <strong>{formatAtomicPrice(transaction.amount, transaction.asset)}</strong>
              <Link
                className={buttonVariants({ variant: "outline" })}
                href={`/dashboard/transactions/${transaction.transactionId}?sellerId=${encodeURIComponent(sellerId)}`}
              >
                View details
              </Link>
            </article>
          ))}
        </div>
      )}
    </div>
  );
}
