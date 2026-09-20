"use client";

import { ArrowUpRight } from "lucide-react";
import Link from "next/link";
import { useState } from "react";
import type {
  ActivityMode,
  SellerOutcome,
  Transaction,
} from "@/features/transactions/model";
import {
  fulfillmentStateLabel,
  paymentStateLabel,
  sellerOutcomeLabel,
} from "@/features/transactions/model";
import { formatAtomicPrice } from "@/lib/money";
import styles from "./transactions.module.css";

const dateFormatter = new Intl.DateTimeFormat("en", {
  dateStyle: "medium",
  timeStyle: "short",
  timeZone: "UTC",
});

const outcomeFilters: Array<{ value: "all" | SellerOutcome; label: string }> = [
  { value: "all", label: "All outcomes" },
  { value: "awaiting_payment", label: "Awaiting payment" },
  { value: "abandoned", label: "Abandoned" },
  { value: "payment_rejected", label: "Payment rejected" },
  { value: "payment_processing", label: "Payment processing" },
  { value: "fulfilling", label: "Delivery in progress" },
  { value: "fulfilled", label: "Fulfilled" },
  { value: "fulfillment_failed", label: "Delivery failed" },
  { value: "disputed", label: "Disputed" },
  { value: "refund_recommended", label: "Refund recommended" },
  { value: "resolved", label: "Resolved" },
];

export function TransactionList({
  transactions,
  view = "transactions",
}: {
  transactions: Transaction[];
  view?: "transactions" | "disputes";
}) {
  const [activityMode, setActivityMode] = useState<ActivityMode>("live");
  const [outcome, setOutcome] = useState<"all" | SellerOutcome>("all");
  const isDisputeView = view === "disputes";
  const scopedTransactions = transactions.filter(
    (transaction) =>
      transaction.activityMode === activityMode &&
      (isDisputeView
        ? ["disputed", "refund_recommended", "resolved"].includes(
            transaction.sellerOutcome,
          )
        : outcome === "all" || transaction.sellerOutcome === outcome),
  );
  const fulfilled = scopedTransactions.filter(
    ({ sellerOutcome }) => sellerOutcome === "fulfilled",
  ).length;
  const needsReview = scopedTransactions.filter(({ sellerOutcome }) =>
    [
      "payment_rejected",
      "fulfillment_failed",
      "disputed",
      "refund_recommended",
    ].includes(sellerOutcome),
  ).length;

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
              : "Payment and delivery stay separate. Live and test activity never share totals."}
          </p>
        </div>
        <dl
          className={styles.ledgerSummary}
          aria-label={isDisputeView ? "Dispute counts" : "Transaction counts"}
        >
          <div>
            <dt>{isDisputeView ? "Cases" : "Visible"}</dt>
            <dd>
              {scopedTransactions.length}{" "}
              {isDisputeView
                ? scopedTransactions.length === 1
                  ? "case"
                  : "cases"
                : "records"}
            </dd>
          </div>
          <div>
            <dt>{isDisputeView ? "Open review" : "Delivered"}</dt>
            <dd>
              {isDisputeView
                ? `${needsReview} active`
                : `${fulfilled} fulfilled`}
            </dd>
          </div>
          <div data-attention={needsReview > 0}>
            <dt>{isDisputeView ? "Source" : "Attention"}</dt>
            <dd>
              {isDisputeView
                ? "Verified ledger"
                : `${needsReview} needs review`}
            </dd>
          </div>
        </dl>
      </header>

      <section className={styles.filters} aria-label="Transaction filters">
        <fieldset>
          <legend>Activity</legend>
          {(["live", "test"] as const).map((mode) => (
            <button
              aria-pressed={activityMode === mode}
              key={mode}
              onClick={() => setActivityMode(mode)}
              type="button"
            >
              {mode === "live" ? "Live activity" : "Test activity"}
            </button>
          ))}
        </fieldset>
        {!isDisputeView ? (
          <label>
            Outcome
            <select
              value={outcome}
              onChange={(event) =>
                setOutcome(event.target.value as "all" | SellerOutcome)
              }
            >
              {outcomeFilters.map((filter) => (
                <option key={filter.value} value={filter.value}>
                  {filter.label}
                </option>
              ))}
            </select>
          </label>
        ) : null}
      </section>

      {scopedTransactions.length === 0 ? (
        <section
          className={styles.emptyState}
          aria-label={
            isDisputeView ? "No disputes" : "No matching transactions"
          }
        >
          <span>000</span>
          <h2>
            {isDisputeView ? "No disputes" : `No ${activityMode} activity`}
          </h2>
          <p>
            {isDisputeView
              ? "Cases appear here when a completed transaction is disputed."
              : activityMode === "live"
                ? "Test checkouts are kept in the separate test activity view."
                : "No test transactions match this outcome filter."}
          </p>
        </section>
      ) : (
        <div className={styles.tableFrame}>
          <div className={styles.tableCaption}>
            <span>
              {isDisputeView
                ? "Dispute record"
                : `${activityMode} activity only`}
            </span>
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
                  <th>Outcome</th>
                  <th>Payment</th>
                  <th>Delivery</th>
                  <th>Amount</th>
                  <th>Updated</th>
                  <th aria-label="Open transaction" />
                </tr>
              </thead>
              <tbody>
                {scopedTransactions.map((transaction) => {
                  const productName =
                    transaction.productDisplayName ?? "Digital purchase";
                  return (
                    <tr key={transaction.transactionId}>
                      <td data-label="Product">
                        <strong>{productName}</strong>
                        <code>{transaction.transactionId}</code>
                        <small>{transaction.buyerId}</small>
                        <small>
                          {transaction.purchaseChannel === "browser"
                            ? "Browser"
                            : "External agent"}
                        </small>
                        <small>
                          {transaction.activityMode === "live"
                            ? "Live"
                            : "Test"}
                        </small>
                      </td>
                      <td data-label="Outcome">
                        <span
                          className={styles.status}
                          data-state={transaction.sellerOutcome}
                        >
                          <span aria-hidden="true" />
                          {sellerOutcomeLabel(transaction.sellerOutcome)}
                        </span>
                      </td>
                      <td data-label="Payment">
                        <strong>
                          {paymentStateLabel(
                            transaction.commerceLifecycle.paymentState,
                          )}
                        </strong>
                        <small>{transaction.network}</small>
                      </td>
                      <td data-label="Delivery">
                        <strong>
                          {fulfillmentStateLabel(
                            transaction.commerceLifecycle.fulfillmentState,
                          )}
                        </strong>
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
