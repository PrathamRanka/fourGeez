import {
  ArrowLeft,
  Check,
  CheckCircle2,
  Download,
  FileKey2,
  ShieldAlert,
  Webhook,
} from "lucide-react";
import Link from "next/link";
import type { ReactNode } from "react";
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from "@/components/ui/accordion";
import { buttonVariants } from "@/components/ui/button";
import type { DisputeAction } from "@/features/disputes/model";
import { DisputePanel } from "@/features/disputes/view/dispute-workspace";
import type { TransactionDetailSnapshot } from "@/features/transactions/model";
import {
  transactionStatusLabel,
  webhookDeliveryStatusLabel,
} from "@/features/transactions/model";
import { formatAtomicPrice } from "@/lib/money";
import styles from "./transactions.module.css";

const dateFormatter = new Intl.DateTimeFormat("en", {
  dateStyle: "medium",
  timeStyle: "short",
  timeZone: "UTC",
});

type TransactionDetailProps = {
  snapshot: TransactionDetailSnapshot;
  createDispute?: DisputeAction;
};

export function TransactionDetail({
  snapshot,
  createDispute,
}: TransactionDetailProps) {
  const { transaction, evidence } = snapshot;
  const canDownloadReceipt =
    evidence.valid && transaction.paymentFinality === "finalized";

  return (
    <div className={styles.workspace}>
      <header className={styles.detailHeader}>
        <div>
          <Link href="/dashboard/transactions" className={styles.backLink}>
            <ArrowLeft aria-hidden="true" />
            Transactions
          </Link>
          <p className={styles.eyebrow}>Proof stream / transaction</p>
          <h1>{transaction.productDisplayName ?? "Purchase details"}</h1>
          <code>{transaction.transactionId}</code>
        </div>
        <div className={styles.headerActions}>
          <StatusBadge status={transaction.status} />
          {canDownloadReceipt ? (
            <Link
              className={`${buttonVariants()} ${styles.primaryAction}`}
              href={`/dashboard/transactions/${encodeURIComponent(transaction.transactionId)}/receipt`}
            >
              <Download aria-hidden="true" />
              Download receipt
            </Link>
          ) : null}
        </div>
      </header>

      {snapshot.error ? (
        <div className={styles.error} role="alert">
          {snapshot.error}
        </div>
      ) : null}

      <section className={styles.lifecycle} aria-label="Transaction lifecycle">
        <LifecycleStep
          index="01"
          label={
            transaction.paymentFinality === "failed"
              ? "Payment failed"
              : transaction.paymentFinality
                ? "Payment verified"
                : "Payment pending"
          }
          complete={Boolean(
            transaction.paymentFinality &&
            transaction.paymentFinality !== "failed",
          )}
        />
        <LifecycleStep
          index="02"
          label={
            transaction.paymentFinality === "finalized"
              ? "Settlement finalized"
              : "Settlement pending"
          }
          complete={transaction.paymentFinality === "finalized"}
        />
        <LifecycleStep
          index="03"
          label={
            transaction.status === "FULFILLED"
              ? "Fulfillment complete"
              : transactionStatusLabel(transaction.status)
          }
          complete={transaction.status === "FULFILLED"}
        />
      </section>

      <section className={styles.facts} aria-label="Transaction summary">
        <Fact
          label="Amount"
          value={formatAtomicPrice(transaction.amount, transaction.asset)}
        />
        <Fact
          label="Payment"
          value={
            transaction.paymentFinality
              ? titleCase(transaction.paymentFinality)
              : "Awaiting payment"
          }
        />
        <Fact
          label="Delivery"
          value={transactionStatusLabel(transaction.status)}
        />
        <Fact label="Network" value={transaction.network} />
        <Fact label="Pricing" value="Exact fixed price" />
        <Fact
          label="Recovery"
          value={recoveryActionLabel(
            transaction.commerceLifecycle.recoveryAction,
          )}
        />
      </section>

      <p className={styles.emptyCopy}>
        No tax, shipping, discounts, or platform fees
      </p>

      <div className={styles.detailGrid}>
        <section className={styles.panel} aria-labelledby="evidence-title">
          <PanelHeading
            icon={
              evidence.valid ? (
                <CheckCircle2 aria-hidden="true" />
              ) : (
                <ShieldAlert aria-hidden="true" />
              )
            }
            title={
              evidence.valid
                ? "Payment and delivery proof verified"
                : "Payment and delivery proof verification failed"
            }
            detail={`${evidence.events.length} append-only events`}
            id="evidence-title"
          />
          {!evidence.valid ? (
            <div className={styles.evidenceWarning} role="alert">
              Payment and delivery proof verification failed. Receipt download
              is unavailable.
            </div>
          ) : null}
          <ol className={styles.evidenceList} aria-label="Evidence events">
            {evidence.events.map((event) => (
              <li key={event.eventId}>
                <span>{String(event.sequence).padStart(2, "0")}</span>
                <div>
                  <strong>{event.eventType}</strong>
                  <p>
                    <span>Sequence {event.sequence}</span>
                    <span>{event.actorType}</span>
                    <time dateTime={event.createdAt}>
                      {dateFormatter.format(new Date(event.createdAt))} UTC
                    </time>
                  </p>
                  <code>{event.eventHash}</code>
                </div>
              </li>
            ))}
          </ol>
        </section>

        <section className={styles.panel} aria-labelledby="payment-title">
          <PanelHeading
            icon={<FileKey2 aria-hidden="true" />}
            title="Purchase record"
            detail="Safe references only. Raw proofs stay private."
            id="payment-title"
          />
          <dl className={styles.definitionList}>
            <Definition
              label="Payment reference"
              value={transaction.paymentReference ?? "Not available"}
            />
            <Definition
              label="Delivery response"
              value={
                transaction.upstreamStatus === null
                  ? "Not called"
                  : String(transaction.upstreamStatus)
              }
            />
            <Definition
              label="Product path"
              value={
                transaction.productSlug
                  ? `/products/${transaction.productSlug}`
                  : "Not available"
              }
            />
          </dl>
          <Accordion className={styles.technicalDetails}>
            <AccordionItem value="technical-details">
              <AccordionTrigger className={styles.technicalTrigger}>
                Advanced technical details
              </AccordionTrigger>
              <AccordionContent>
                <dl className={styles.definitionList}>
                  <Definition
                    label="Purchase intent ID"
                    value={transaction.intentId}
                  />
                  <Definition label="Route ID" value={transaction.routeId} />
                  <Definition label="Buyer ID" value={transaction.buyerId} />
                  <Definition
                    label="Payment network"
                    value={transaction.network}
                  />
                  <Definition
                    label="Response hash"
                    value={transaction.responseHash ?? "Not available"}
                  />
                </dl>
              </AccordionContent>
            </AccordionItem>
          </Accordion>
        </section>
      </div>

      <section className={styles.panel} aria-labelledby="delivery-title">
        <PanelHeading
          icon={<Webhook aria-hidden="true" />}
          title="Webhook delivery history"
          detail="Seller notifications, newest page."
          id="delivery-title"
        />
        {snapshot.webhookDeliveries.length === 0 ? (
          <p className={styles.emptyCopy}>No webhook deliveries recorded.</p>
        ) : (
          <div className={styles.tableScroller}>
            <table
              aria-label="Webhook delivery history"
              className={styles.table}
            >
              <thead>
                <tr>
                  <th>Event / delivery</th>
                  <th>Status</th>
                  <th>Attempts</th>
                  <th>Response</th>
                  <th>Updated</th>
                </tr>
              </thead>
              <tbody>
                {snapshot.webhookDeliveries.map((delivery) => (
                  <tr key={delivery.deliveryId}>
                    <td>
                      <strong>{delivery.eventType}</strong>
                      <code>{delivery.deliveryId}</code>
                    </td>
                    <td>
                      <span
                        className={styles.status}
                        data-state={delivery.status}
                      >
                        <span aria-hidden="true" />
                        {webhookDeliveryStatusLabel(delivery.status)}
                      </span>
                    </td>
                    <td>{delivery.attemptCount}</td>
                    <td>
                      {delivery.responseStatusCode ?? delivery.errorCode ?? "—"}
                    </td>
                    <td>
                      <time dateTime={delivery.updatedAt}>
                        {dateFormatter.format(new Date(delivery.updatedAt))} UTC
                      </time>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>

      {createDispute ? (
        <DisputePanel
          transactionId={transaction.transactionId}
          transactionStatus={transaction.status}
          evidenceValid={evidence.valid}
          createDispute={createDispute}
        />
      ) : null}
    </div>
  );
}

function LifecycleStep({
  index,
  label,
  complete,
}: {
  index: string;
  label: string;
  complete: boolean;
}) {
  return (
    <div data-complete={complete}>
      <span>{complete ? <Check aria-hidden="true" /> : index}</span>
      <strong>{label}</strong>
    </div>
  );
}

function PanelHeading({
  icon,
  title,
  detail,
  id,
}: {
  icon: ReactNode;
  title: string;
  detail: string;
  id: string;
}) {
  return (
    <header className={styles.panelHeading}>
      <span>{icon}</span>
      <div>
        <h2 id={id}>{title}</h2>
        <p>{detail}</p>
      </div>
    </header>
  );
}

function Fact({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

function Definition({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <dt>{label}</dt>
      <dd>{value}</dd>
    </div>
  );
}

function StatusBadge({
  status,
}: {
  status: TransactionDetailSnapshot["transaction"]["status"];
}) {
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

function recoveryActionLabel(
  action: TransactionDetailSnapshot["transaction"]["commerceLifecycle"]["recoveryAction"],
): string {
  const labels = {
    retry_same_request: "Retry the same request",
    retry_same_payment: "Retry fulfillment with the same payment",
    await_reconciliation: "Await payment reconciliation",
    create_new_intent: "Create a new purchase intent",
    open_dispute: "Open a dispute",
    await_resolution: "Await dispute resolution",
    record_external_refund: "Record the external refund",
    request_seller_review: "Review delivery before resolving",
    none: "No recovery action required",
  } as const;
  return labels[action];
}
