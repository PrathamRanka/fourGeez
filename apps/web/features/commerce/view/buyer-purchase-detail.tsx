"use client";

import {
  CheckCircle2,
  Download,
  FileCheck2,
  FileWarning,
  LoaderCircle,
  RotateCcw,
  Scale,
  ShieldAlert,
} from "lucide-react";
import Link from "next/link";
import { useEffect, useState } from "react";
import { Button, buttonVariants } from "@/components/ui/button";
import type {
  BrowserDispute,
  BuyerPurchaseSnapshot,
  PurchaseReceipt,
} from "@/features/commerce/model";
import type { DisputeReason } from "@/features/disputes/model";
import {
  disputeReasonLabel,
  disputeStatusLabel,
} from "@/features/disputes/model";
import { transactionStatusLabel } from "@/features/transactions/model";
import { formatAtomicPrice } from "@/lib/money";
import styles from "./buyer-purchase-detail.module.css";

const disputeReasons: DisputeReason[] = [
  "unauthorized",
  "duplicate",
  "wrong_amount",
  "not_delivered",
  "quality_or_output",
];

const dateFormatter = new Intl.DateTimeFormat("en", {
  dateStyle: "medium",
  timeStyle: "short",
  timeZone: "UTC",
});

type ReceiptState =
  | { status: "idle" }
  | { status: "verifying" }
  | { status: "verified"; receipt: PurchaseReceipt }
  | { status: "error"; message: string };

export function BuyerPurchaseDetail({
  transactionId,
  initialDisputeId,
}: {
  transactionId: string;
  initialDisputeId?: string;
}) {
  const [purchase, setPurchase] = useState<BuyerPurchaseSnapshot | null>(null);
  const [dispute, setDispute] = useState<BrowserDispute | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [reloadVersion, setReloadVersion] = useState(0);
  const [receiptState, setReceiptState] = useState<ReceiptState>({
    status: "idle",
  });

  useEffect(() => {
    const abortController = new AbortController();
    const purchaseRequest = requestCommerce<BuyerPurchaseSnapshot>(
      `/api/commerce/purchases/${encodeURIComponent(transactionId)}`,
      { method: "GET", signal: abortController.signal },
    );
    const disputeRequest = initialDisputeId
      ? requestCommerce<BrowserDispute>(
          `/api/commerce/disputes/${encodeURIComponent(initialDisputeId)}`,
          { method: "GET", signal: abortController.signal },
        )
      : Promise.resolve(null);

    void Promise.all([purchaseRequest, disputeRequest]).then(
      ([purchaseResult, disputeResult]) => {
        if (abortController.signal.aborted) return;
        if (!purchaseResult.ok) {
          setError(purchaseResult.error);
          setLoading(false);
          return;
        }
        if (disputeResult && !disputeResult.ok) {
          setError(disputeResult.error);
          setLoading(false);
          return;
        }
        setPurchase(purchaseResult.value);
        setDispute(disputeResult?.value ?? null);
        setLoading(false);
      },
    );

    return () => abortController.abort();
  }, [initialDisputeId, reloadVersion, transactionId]);

  async function verifyReceipt() {
    setReceiptState({ status: "verifying" });
    const result = await requestCommerce<PurchaseReceipt>(
      `/api/commerce/purchases/${encodeURIComponent(transactionId)}/receipt`,
      { method: "GET" },
    );
    if (!result.ok) {
      setReceiptState({ status: "error", message: result.error });
      return;
    }
    if (!receiptMatchesTransaction(result.value, transactionId)) {
      setReceiptState({
        status: "error",
        message: "The receipt did not match this purchase record.",
      });
      return;
    }
    setReceiptState({ status: "verified", receipt: result.value });
  }

  if (loading) {
    return (
      <main id="main-content" className={styles.statePage}>
        <div className={styles.emptyCopy} role="status">
          Loading purchase record…
        </div>
      </main>
    );
  }

  if (error || !purchase) {
    return (
      <main id="main-content" className={styles.statePage}>
        <div className={styles.errorBox} role="alert">
          {error ?? "Purchase record not found."}
        </div>
        <Button
          className={styles.action}
          type="button"
          variant="outline"
          onClick={() => {
            setLoading(true);
            setError(null);
            setReloadVersion((version) => version + 1);
          }}
        >
          <RotateCcw aria-hidden="true" /> Try again
        </Button>
      </main>
    );
  }

  const { transaction, evidence } = purchase;
  const receiptAvailable =
    evidence.valid && transaction.paymentFinality === "finalized";

  return (
    <main id="main-content" className={styles.page}>
      <Link href="/">← AgentPay home</Link>
      <div className={styles.workspace}>
        <header className={styles.header}>
          <div>
            <p className={styles.eyebrow}>Purchase ledger</p>
            <h1>{transaction.productDisplayName ?? "Purchase details"}</h1>
            <span>{transaction.transactionId}</span>
            <p>
              {transactionOutcome(transaction.status)} Payment, fulfillment,
              receipt, and support records remain bound to this purchase.
            </p>
          </div>
          <span className={styles.evidenceBadge} data-valid={evidence.valid}>
            {evidence.valid ? (
              <CheckCircle2 aria-hidden="true" />
            ) : (
              <ShieldAlert aria-hidden="true" />
            )}
            {evidence.valid
              ? "Evidence chain verified"
              : "Evidence verification failed"}
          </span>
        </header>

        <section className={styles.facts} aria-label="Purchase summary">
          <Fact
            label="Outcome"
            value={transactionOutcomeLabel(transaction.status)}
          />
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
          <Fact label="Network" value={transaction.network} />
        </section>

        <div className={styles.detailGrid}>
          <section className={styles.panel} aria-labelledby="evidence-title">
            <div className={styles.panelHeading}>
              <div>
                {evidence.valid ? (
                  <CheckCircle2 aria-hidden="true" />
                ) : (
                  <ShieldAlert aria-hidden="true" />
                )}
                <div>
                  <h2 id="evidence-title">Evidence timeline</h2>
                  <p>{evidence.events.length} append-only events</p>
                </div>
              </div>
            </div>
            {!evidence.valid ? (
              <div className={styles.warning} role="alert">
                Evidence verification failed. Receipt verification and dispute
                classification may be unavailable.
              </div>
            ) : null}
            {evidence.events.length === 0 ? (
              <p className={styles.emptyCopy}>
                No evidence events have been recorded yet.
              </p>
            ) : (
              <ol className={styles.evidenceList} aria-label="Evidence events">
                {evidence.events.map((event) => (
                  <li key={event.eventId}>
                    <span>{event.sequence}</span>
                    <div>
                      <strong>{event.eventType}</strong>
                      <p>
                        {event.actorType} · {formatUTC(event.createdAt)}
                      </p>
                      <code>{event.eventHash}</code>
                    </div>
                  </li>
                ))}
              </ol>
            )}
          </section>

          <section className={styles.panel} aria-labelledby="receipt-title">
            <div className={styles.panelHeading}>
              <div>
                <FileCheck2 aria-hidden="true" />
                <div>
                  <h2 id="receipt-title">Verified receipt</h2>
                  <p>Machine-readable proof bound to this transaction.</p>
                </div>
              </div>
            </div>
            <dl className={styles.definitionList}>
              <Definition
                label="Status"
                value={receiptAvailable ? "Ready" : "Not available"}
              />
              <Definition
                label="Payment reference"
                value={transaction.paymentReference ?? "Not available"}
              />
              <Definition
                label="Delivery status"
                value={transactionStatusLabel(transaction.status)}
              />
            </dl>
            <div className={styles.actions}>
              {receiptAvailable ? (
                <>
                  <a
                    className={`${buttonVariants()} ${styles.action}`}
                    download
                    href={`/api/commerce/purchases/${encodeURIComponent(transactionId)}/receipt`}
                  >
                    <Download aria-hidden="true" /> Download receipt
                  </a>
                  <Button
                    className={styles.action}
                    type="button"
                    variant="outline"
                    disabled={receiptState.status === "verifying"}
                    onClick={verifyReceipt}
                  >
                    {receiptState.status === "verifying" ? (
                      <LoaderCircle
                        className="animate-spin"
                        aria-hidden="true"
                      />
                    ) : (
                      <FileCheck2 aria-hidden="true" />
                    )}
                    Verify receipt
                  </Button>
                </>
              ) : (
                <p className={styles.emptyCopy}>
                  A receipt becomes available after finalized payment and valid
                  evidence.
                </p>
              )}
            </div>
            <ReceiptVerification state={receiptState} />
          </section>
        </div>

        <BuyerDisputePanel
          dispute={dispute}
          evidenceValid={evidence.valid}
          transactionId={transactionId}
          transactionStatus={transaction.status}
          onCreated={setDispute}
        />
      </div>
    </main>
  );
}

function BuyerDisputePanel({
  dispute,
  evidenceValid,
  transactionId,
  transactionStatus,
  onCreated,
}: {
  dispute: BrowserDispute | null;
  evidenceValid: boolean;
  transactionId: string;
  transactionStatus: BuyerPurchaseSnapshot["transaction"]["status"];
  onCreated: (dispute: BrowserDispute) => void;
}) {
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const canDispute =
    evidenceValid &&
    (transactionStatus === "FULFILLED" || transactionStatus === "FAILED");

  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setPending(true);
    setError(null);
    const fields = new FormData(event.currentTarget);
    const result = await requestCommerce<BrowserDispute>(
      `/api/commerce/purchases/${encodeURIComponent(transactionId)}/disputes`,
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          reason: String(fields.get("reason")),
          statement: String(fields.get("statement") ?? ""),
        }),
      },
    );
    setPending(false);
    if (!result.ok) {
      setError(result.error);
      return;
    }
    const disputePath = `/purchases/${transactionId}?dispute=${result.value.disputeId}`;
    window.history.replaceState(window.history.state, "", disputePath);
    onCreated(result.value);
  }

  if (dispute) {
    const disputePath = `/purchases/${transactionId}?dispute=${dispute.disputeId}`;
    return (
      <section
        className={styles.disputeOutcome}
        aria-labelledby="dispute-outcome-title"
      >
        <header>
          <Scale aria-hidden="true" />
          <div>
            <p>Dispute classification</p>
            <h2 id="dispute-outcome-title">
              {disputeStatusLabel(dispute.status)}
            </h2>
          </div>
          <span className={styles.evidenceBadge} data-valid={evidenceValid}>
            {evidenceValid ? (
              <CheckCircle2 aria-hidden="true" />
            ) : (
              <ShieldAlert aria-hidden="true" />
            )}
            {evidenceValid
              ? "Evidence chain verified"
              : "Evidence verification failed"}
          </span>
        </header>
        <p className={styles.disputeExplanation}>{dispute.explanation}</p>
        <dl>
          <div>
            <dt>Reason</dt>
            <dd>{disputeReasonLabel(dispute.reason)}</dd>
          </div>
          <div>
            <dt>Classification code</dt>
            <dd>{dispute.classificationCode}</dd>
          </div>
          <div>
            <dt>Rule version</dt>
            <dd>{dispute.ruleVersion}</dd>
          </div>
          <div>
            <dt>Created</dt>
            <dd>{formatUTC(dispute.createdAt)}</dd>
          </div>
        </dl>
        <div className={styles.actions}>
          <Link
            className={`${buttonVariants({ variant: "outline" })} ${styles.action}`}
            href={disputePath}
          >
            Open saved dispute link
          </Link>
        </div>
      </section>
    );
  }

  return (
    <section
      className={styles.disputePanel}
      aria-labelledby="buyer-dispute-title"
    >
      <header>
        <FileWarning aria-hidden="true" />
        <div>
          <h2 id="buyer-dispute-title">Need help with this purchase?</h2>
          <p>Open a dispute using the verified transaction record.</p>
        </div>
        <span className={styles.evidenceBadge} data-valid={evidenceValid}>
          {evidenceValid ? (
            <CheckCircle2 aria-hidden="true" />
          ) : (
            <ShieldAlert aria-hidden="true" />
          )}
          {evidenceValid
            ? "Evidence chain verified"
            : "Evidence verification failed"}
        </span>
      </header>
      <form aria-label="Open dispute" onSubmit={submit}>
        <label>
          <span>Dispute reason</span>
          <select
            name="reason"
            defaultValue=""
            required
            disabled={!canDispute || pending}
          >
            <option value="" disabled>
              Select a reason
            </option>
            {disputeReasons.map((reason) => (
              <option key={reason} value={reason}>
                {disputeReasonLabel(reason)}
              </option>
            ))}
          </select>
        </label>
        <label>
          <span>Statement</span>
          <textarea
            name="statement"
            maxLength={2000}
            disabled={!canDispute || pending}
            placeholder="Add concise facts that help the seller review this purchase."
          />
        </label>
        {!canDispute ? (
          <p className={styles.disabledCopy}>
            Disputes become available after fulfillment succeeds or fails with a
            verified evidence chain.
          </p>
        ) : null}
        {error ? (
          <p className={styles.errorCopy} role="alert">
            {error}
          </p>
        ) : null}
        <Button
          className={styles.action}
          type="submit"
          disabled={!canDispute || pending}
        >
          {pending ? (
            <LoaderCircle className="animate-spin" aria-hidden="true" />
          ) : (
            <Scale aria-hidden="true" />
          )}
          Open and classify dispute
        </Button>
      </form>
    </section>
  );
}

function ReceiptVerification({ state }: { state: ReceiptState }) {
  if (state.status === "idle" || state.status === "verifying") return null;
  if (state.status === "error") {
    return (
      <div
        className={`${styles.receiptStatus} ${styles.receiptError}`}
        role="alert"
      >
        <ShieldAlert aria-hidden="true" />
        <div>
          <strong>Receipt verification failed</strong>
          <span>{state.message}</span>
        </div>
      </div>
    );
  }
  const eventLabel =
    state.receipt.evidence.eventCount === 1 ? "event" : "events";
  return (
    <div
      className={`${styles.receiptStatus} ${styles.receiptReady}`}
      role="status"
    >
      <CheckCircle2 aria-hidden="true" />
      <div>
        <strong>Receipt verified</strong>
        <span>
          Schema version {state.receipt.schemaVersion} ·{" "}
          {state.receipt.evidence.eventCount} evidence {eventLabel}
        </span>
      </div>
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

function transactionOutcomeLabel(
  status: BuyerPurchaseSnapshot["transaction"]["status"],
): string {
  if (status === "PROPOSED") return "Started";
  if (status === "APPROVAL_PENDING") return "Pending";
  if (status === "APPROVED") return "Authorized";
  if (status === "PAYMENT_REQUIRED") return "Payment due";
  if (status === "PAYMENT_VERIFIED") return "Paid";
  if (status === "FORWARDED") return "Fulfilling";
  if (status === "FULFILLED") return "Fulfilled";
  if (status === "FAILED") return "Failed";
  if (status === "DISPUTED") return "Disputed";
  if (status === "REFUND_RECOMMENDED") return "Refund advised";
  if (status === "RESOLVED") return "Resolved";
  return transactionStatusLabel(status);
}

function transactionOutcome(
  status: BuyerPurchaseSnapshot["transaction"]["status"],
): string {
  if (status === "FULFILLED") return "The seller completed this purchase.";
  if (status === "FAILED") return "Payment completed, but fulfillment failed.";
  return `This purchase is ${transactionStatusLabel(status).toLowerCase()}.`;
}

function receiptMatchesTransaction(
  receipt: PurchaseReceipt,
  transactionId: string,
): boolean {
  return (
    (receipt.schemaVersion === "1" || receipt.schemaVersion === "2") &&
    receipt.transaction.transactionId === transactionId &&
    receipt.evidence.verified === true &&
    receipt.evidence.eventCount === receipt.evidence.events.length &&
    receipt.evidence.eventCount > 0
  );
}

async function requestCommerce<Value>(
  path: string,
  init: RequestInit,
): Promise<{ ok: true; value: Value } | { ok: false; error: string }> {
  try {
    const response = await fetch(path, { ...init, cache: "no-store" });
    const responseBody: unknown = await response.json();
    if (!response.ok) {
      const apiError =
        isRecord(responseBody) && isRecord(responseBody.error)
          ? responseBody.error
          : null;
      return {
        ok: false,
        error:
          apiError && typeof apiError.message === "string"
            ? apiError.message
            : "AgentPay could not load this purchase record.",
      };
    }
    return { ok: true, value: responseBody as Value };
  } catch (error) {
    if (error instanceof DOMException && error.name === "AbortError") {
      return { ok: false, error: "Purchase loading was cancelled." };
    }
    return {
      ok: false,
      error: "Purchase records are temporarily unavailable. Try again.",
    };
  }
}

function formatUTC(value: string): string {
  return `${dateFormatter.format(new Date(value))} UTC`;
}

function titleCase(value: string): string {
  return value[0].toUpperCase() + value.slice(1);
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}
