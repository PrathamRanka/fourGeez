import { ArrowUpRight } from "lucide-react";
import Link from "next/link";
import type { Transaction } from "@/features/transactions/model";
import { transactionStatusLabel } from "@/features/transactions/model";
import { formatAtomicPrice } from "@/lib/money";
import styles from "./transactions.module.css";

const dateFormatter = new Intl.DateTimeFormat("en", {
  dateStyle: "medium",
  timeStyle: "short",
  timeZone: "UTC",
});

export function TransactionList({
  transactions,
  view = "transactions",
}: {
  transactions: Transaction[];
  view?: "transactions" | "disputes";
}) {
  const visibleTransactions =
    view === "disputes"
      ? transactions.filter(({ status }) =>
          ["DISPUTED", "REFUND_RECOMMENDED", "RESOLVED"].includes(status),
        )
      : transactions;
  const fulfilled = transactions.filter(
    ({ status }) => status === "FULFILLED",
  ).length;
  const needsReview = transactions.filter(({ status }) =>
    ["FAILED", "DISPUTED", "REFUND_RECOMMENDED"].includes(status),
  ).length;
  const isDisputeView = view === "disputes";

  return (
    <div className={styles.workspace}>
      <header className={styles.listHeader}>
        <div>
          <p className={styles.eyebrow}>
            {isDisputeView ? "Resolution desk" : "Proof stream / ledger"}
          </p>
          <h1>{isDisputeView ? "Disputes" : "Transactions"}</h1>
          <p>
            {isDisputeView
              ? "Cases requiring review, evidence, or refund follow-up."
              : "Payment, fulfillment, evidence, and dispute state in one record."}
          </p>
        </div>
        {isDisputeView ? (
          <dl className={styles.ledgerSummary} aria-label="Dispute counts">
            <div>
              <dt>Cases</dt>
              <dd>
                {visibleTransactions.length}{" "}
                {visibleTransactions.length === 1 ? "case" : "cases"}
              </dd>
            </div>
            <div>
              <dt>Open review</dt>
              <dd>{needsReview} active</dd>
            </div>
            <div>
              <dt>Source</dt>
              <dd>Verified ledger</dd>
            </div>
          </dl>
        ) : (
          <dl className={styles.ledgerSummary} aria-label="Transaction counts">
            <div>
              <dt>Total</dt>
              <dd>{transactions.length} records</dd>
            </div>
            <div>
              <dt>Delivered</dt>
              <dd>{fulfilled} fulfilled</dd>
            </div>
            <div data-attention={needsReview > 0}>
              <dt>Attention</dt>
              <dd>{needsReview} needs review</dd>
            </div>
          </dl>
        )}
      </header>

      {visibleTransactions.length === 0 ? (
        <section
          className={styles.emptyState}
          aria-label={isDisputeView ? "No disputes" : "No transactions"}
        >
          <span>000</span>
          <h2>{isDisputeView ? "No disputes" : "No transactions yet"}</h2>
          <p>
            {isDisputeView
              ? "Cases appear here when a completed transaction is disputed."
              : "Completed browser and agent purchases will appear here."}
          </p>
        </section>
      ) : (
        <div className={styles.tableFrame}>
          <div className={styles.tableCaption}>
            <span>Live commerce record</span>
            <span>UTC / newest first</span>
          </div>
          <div className={styles.tableScroller}>
            <table
              aria-label={
                isDisputeView ? "Seller disputes" : "Seller transactions"
              }
              className={styles.table}
            >
              <thead>
                <tr>
                  <th>Product / reference</th>
                  <th>Buyer</th>
                  <th>Status</th>
                  <th>Settlement</th>
                  <th>Amount</th>
                  <th>Updated</th>
                  <th aria-label="Open transaction" />
                </tr>
              </thead>
              <tbody>
                {visibleTransactions.map((transaction) => {
                  const productName =
                    transaction.productDisplayName ?? "Digital purchase";
                  return (
                    <tr key={transaction.transactionId}>
                      <td data-label="Product">
                        <strong>{productName}</strong>
                        <code>{transaction.transactionId}</code>
                      </td>
                      <td data-label="Buyer">
                        <code>{transaction.buyerId}</code>
                      </td>
                      <td data-label="Status">
                        <StatusBadge status={transaction.status} />
                      </td>
                      <td data-label="Settlement">
                        <span
                          className={styles.settlement}
                          data-state={transaction.paymentFinality ?? "pending"}
                        >
                          {transaction.paymentFinality
                            ? titleCase(transaction.paymentFinality)
                            : "Pending"}
                        </span>
                        <small>{transaction.network}</small>
                      </td>
                      <td data-label="Amount" className={styles.amount}>
                        {formatAtomicPrice(
                          transaction.amount,
                          transaction.asset,
                        )}
                      </td>
                      <td data-label="Updated">
                        <time dateTime={transaction.updatedAt}>
                          {dateFormatter.format(
                            new Date(transaction.updatedAt),
                          )}
                        </time>
                        <small>UTC</small>
                      </td>
                      <td className={styles.openCell}>
                        <Link
                          href={`/dashboard/transactions/${transaction.transactionId}`}
                          aria-label={`Open ${productName} transaction`}
                        >
                          <ArrowUpRight aria-hidden="true" />
                        </Link>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  );
}

function StatusBadge({ status }: { status: Transaction["status"] }) {
  return (
    <span className={styles.status} data-state={status.toLowerCase()}>
      <span aria-hidden="true" />
      {transactionStatusLabel(status)}
    </span>
  );
}

function titleCase(value: string): string {
  return value[0].toUpperCase() + value.slice(1);
}
