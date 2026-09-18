import {
  CheckCircle2,
  Download,
  FileKey2,
  ShieldAlert,
  Webhook,
} from "lucide-react";
import Link from "next/link";
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from "@/components/ui/accordion";
import { buttonVariants } from "@/components/ui/button";
import type { TransactionDetailSnapshot } from "@/features/transactions/model";
import {
  transactionStatusLabel,
  webhookDeliveryStatusLabel,
} from "@/features/transactions/model";
import { formatAtomicPrice } from "@/lib/money";
import type { DisputeAction } from "@/features/disputes/model";
import { DisputePanel } from "@/features/disputes/view/dispute-workspace";

const dateFormatter = new Intl.DateTimeFormat("en", {
  dateStyle: "medium",
  timeStyle: "short",
  timeZone: "UTC",
});

type TransactionDetailProps = {
  snapshot: TransactionDetailSnapshot;
  createDispute?: DisputeAction;
};

// TransactionDetail presents payment, fulfillment, evidence, delivery, and receipt facts.
export function TransactionDetail({
  snapshot,
  createDispute,
}: TransactionDetailProps) {
  const { transaction, evidence } = snapshot;
  return (
    <div className="transaction-detail-workspace">
      <header className="transaction-detail-header">
        <div>
          <p className="dashboard-eyebrow">Proof Stream</p>
          <h1>{transaction.productDisplayName ?? "Purchase details"}</h1>
          <span>{transaction.transactionId}</span>
          <p>
            Payment, delivery proof, receipt, and notification history for one
            purchase.
          </p>
        </div>
        {evidence.valid && transaction.paymentFinality === "finalized" ? (
          <Link
            className={buttonVariants()}
            href={`/dashboard/transactions/${encodeURIComponent(transaction.transactionId)}/receipt`}
          >
            <Download aria-hidden="true" />
            Download receipt
          </Link>
        ) : null}
      </header>

      {snapshot.error ? (
        <div className="dashboard-error" role="alert">
          {snapshot.error}
        </div>
      ) : null}

      <section className="transaction-facts" aria-label="Transaction summary">
        <Fact
          label="Amount"
          value={formatAtomicPrice(transaction.amount, transaction.asset)}
        />
        <Fact
          label="Payment status"
          value={
            transaction.paymentFinality
              ? titleCase(transaction.paymentFinality)
              : "Awaiting payment"
          }
        />
        <Fact
          label="Delivery status"
          value={transactionStatusLabel(transaction.status)}
        />
        <Fact
          label="Product URL"
          value={
            transaction.productSlug
              ? `/products/${transaction.productSlug}`
              : "Not available"
          }
        />
      </section>

      <div className="transaction-detail-grid">
        <section className="transaction-panel" aria-labelledby="evidence-title">
          <div className="transaction-panel-heading">
            <div>
              {evidence.valid ? (
                <CheckCircle2 aria-hidden="true" />
              ) : (
                <ShieldAlert aria-hidden="true" />
              )}
              <div>
                <h2 id="evidence-title">
                  {evidence.valid
                    ? "Payment and delivery proof verified"
                    : "Payment and delivery proof verification failed"}
                </h2>
                <p>{evidence.events.length} append-only events</p>
              </div>
            </div>
          </div>
          {!evidence.valid ? (
            <div className="transaction-evidence-warning" role="alert">
              Payment and delivery proof verification failed. Receipt download
              is unavailable.
            </div>
          ) : null}
          <ol
            className="transaction-evidence-list"
            aria-label="Evidence events"
          >
            {evidence.events.map((event) => (
              <li key={event.eventId}>
                <span>{event.sequence}</span>
                <div>
                  <strong>{event.eventType}</strong>
                  <p>
                    <span>Sequence {event.sequence}</span> · {event.actorType} ·{" "}
                    {dateFormatter.format(new Date(event.createdAt))} UTC
                  </p>
                  <code>{event.eventHash}</code>
                </div>
              </li>
            ))}
          </ol>
        </section>

        <section className="transaction-panel" aria-labelledby="payment-title">
          <div className="transaction-panel-heading">
            <div>
              <FileKey2 aria-hidden="true" />
              <div>
                <h2 id="payment-title">Purchase record</h2>
                <p>
                  Safe payment and delivery references; raw proofs are never
                  displayed.
                </p>
              </div>
            </div>
          </div>
          <dl className="transaction-definition-list">
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
          </dl>
          <Accordion className="transaction-technical-details">
            <AccordionItem value="technical-details">
              <AccordionTrigger>Advanced technical details</AccordionTrigger>
              <AccordionContent>
                <dl className="transaction-definition-list">
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

      <section className="transaction-panel" aria-labelledby="delivery-title">
        <div className="transaction-panel-heading">
          <div>
            <Webhook aria-hidden="true" />
            <div>
              <h2 id="delivery-title">Webhook delivery history</h2>
              <p>Seller-wide delivery attempts, newest page.</p>
            </div>
          </div>
        </div>
        {snapshot.webhookDeliveries.length === 0 ? (
          <p className="transaction-empty-copy">
            No webhook deliveries recorded.
          </p>
        ) : (
          <div className="transaction-table-wrap">
            <table aria-label="Webhook delivery history">
              <thead>
                <tr>
                  <th>Event</th>
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
                      <small>{delivery.deliveryId}</small>
                    </td>
                    <td>
                      <span data-delivery-status={delivery.status}>
                        {webhookDeliveryStatusLabel(delivery.status)}
                      </span>
                    </td>
                    <td>{delivery.attemptCount}</td>
                    <td>
                      {delivery.responseStatusCode ?? delivery.errorCode ?? "—"}
                    </td>
                    <td>
                      {dateFormatter.format(new Date(delivery.updatedAt))} UTC
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

function titleCase(value: string): string {
  return value[0].toUpperCase() + value.slice(1);
}
