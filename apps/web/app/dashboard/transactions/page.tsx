import type { Metadata } from "next";
import Link from "next/link";
import { buttonVariants } from "@/components/ui/button";
import { OperationState } from "@/components/dashboard/operation-state";
import { getSellerSession } from "@/features/auth/server/session";
import { operationStateFromFailure } from "@/features/operations/model";
import { loadTransactionList } from "@/features/transactions/controller";
import { transactionStatusLabel } from "@/features/transactions/model";
import { formatAtomicPrice } from "@/lib/money";

export const metadata: Metadata = { title: "Transactions" };

export default async function TransactionsPage() {
  const session = await getSellerSession();
  const sellerId = session?.principal.sellerId;
  if (!sellerId) {
    return (
      <OperationState
        kind="disabled"
        title="Choose a storefront first"
        description="Complete onboarding before opening transaction history."
      />
    );
  }
  const snapshot = await loadTransactionList();
  if (snapshot.failure) {
    const state = operationStateFromFailure(snapshot.failure);
    return (
      <OperationState
        kind={state}
        description={
          state === "retryable_error" || state === "terminal_error"
            ? snapshot.error
            : undefined
        }
        retryAfterSeconds={snapshot.failure.retryAfterSeconds}
      />
    );
  }
  return (
    <div className="transaction-list-workspace">
      <header className="transaction-list-header">
        <p className="dashboard-eyebrow">Proof Stream</p>
        <h1>Transactions</h1>
        <p>
          Open a purchase to verify its payment, fulfillment, and evidence
          chain.
        </p>
      </header>
      {snapshot.transactions.length === 0 ? (
        <OperationState
          kind="empty"
          title="No transactions recorded yet"
          description="Completed buyer and agent purchases will appear here with their evidence history."
        />
      ) : (
        <div className="transaction-list">
          {snapshot.transactions.map((transaction) => (
            <article key={transaction.transactionId}>
              <div>
                <strong>{transaction.transactionId}</strong>
                <span>
                  {transactionStatusLabel(transaction.status)} ·{" "}
                  {transaction.network}
                </span>
              </div>
              <strong>
                {formatAtomicPrice(transaction.amount, transaction.asset)}
              </strong>
              <Link
                className={buttonVariants({ variant: "outline" })}
                href={`/dashboard/transactions/${transaction.transactionId}`}
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
