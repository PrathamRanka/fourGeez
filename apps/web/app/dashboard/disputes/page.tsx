import type { Metadata } from "next";
import { OperationState } from "@/components/dashboard/operation-state";
import { getSellerSession } from "@/features/auth/server/session";
import { operationStateFromFailure } from "@/features/operations/model";
import { loadTransactionList } from "@/features/transactions/controller";
import { TransactionList } from "@/features/transactions/view/transaction-list";

export const metadata: Metadata = { title: "Disputes" };

export default async function DisputesPage() {
  const session = await getSellerSession();
  if (!session?.principal.sellerId) {
    return (
      <OperationState
        kind="disabled"
        title="Choose a storefront first"
        description="Complete onboarding before reviewing disputes."
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
    <TransactionList transactions={snapshot.transactions} view="disputes" />
  );
}
