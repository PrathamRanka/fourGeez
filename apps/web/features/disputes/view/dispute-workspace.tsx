"use client";

import {
  ArrowLeft,
  CheckCircle2,
  FileWarning,
  LoaderCircle,
  Scale,
  ShieldAlert,
} from "lucide-react";
import Link from "next/link";
import { useState } from "react";
import { Button, buttonVariants } from "@/components/ui/button";
import type {
  Dispute,
  DisputeAction,
  DisputeReason,
} from "@/features/disputes/model";
import {
  disputeReasonLabel,
  disputeStatusLabel,
} from "@/features/disputes/model";
import type { TransactionStatus } from "@/features/transactions/model";
import styles from "./disputes.module.css";

const disputeReasons: DisputeReason[] = [
  "unauthorized",
  "duplicate",
  "wrong_amount",
  "not_delivered",
  "quality_or_output",
];

type DisputePanelProps = {
  transactionId: string;
  transactionStatus: TransactionStatus;
  evidenceValid: boolean;
  createDispute: DisputeAction;
};

export function DisputePanel({
  transactionId,
  transactionStatus,
  evidenceValid,
  createDispute,
}: DisputePanelProps) {
  const [dispute, setDispute] = useState<Dispute | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);
  const canDispute =
    transactionStatus === "FULFILLED" || transactionStatus === "FAILED";

  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setPending(true);
    setError(null);
    const fields = new FormData(event.currentTarget);
    const result = await createDispute({
      transactionId,
      reason: String(fields.get("reason")) as DisputeReason,
      statement: String(fields.get("statement") ?? ""),
    });
    setPending(false);
    if (!result.ok) {
      setError(result.error);
      return;
    }
    setDispute(result.value);
  }

  if (dispute) {
    return (
      <DisputeOutcome
        dispute={dispute}
        evidenceValid={evidenceValid}
        detailLink={`/dashboard/disputes/${dispute.disputeId}`}
      />
    );
  }

  return (
    <section className={styles.openPanel} aria-labelledby="open-dispute-title">
      <header className={styles.panelHeader}>
        <span className={styles.iconFrame}>
          <FileWarning aria-hidden="true" />
        </span>
        <div>
          <p>Resolution desk</p>
          <h2 id="open-dispute-title">Open a dispute</h2>
        </div>
        <EvidenceBadge valid={evidenceValid} />
      </header>
      <div className={styles.openGrid}>
        <div className={styles.openNote}>
          <span>Deterministic review</span>
          <strong>Recorded facts decide the first classification.</strong>
          <p>No model can change payment, fulfillment, or evidence state.</p>
        </div>
        <form
          aria-label="Open dispute"
          onSubmit={submit}
          className={styles.form}
        >
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
              placeholder="Add concise facts for seller review."
            />
          </label>
          {!canDispute ? (
            <p className={styles.disabledCopy}>
              Disputes become available after fulfillment succeeds or fails.
            </p>
          ) : null}
          {error ? (
            <p className={styles.error} role="alert">
              {error}
            </p>
          ) : null}
          <Button
            type="submit"
            disabled={!canDispute || pending}
            className={styles.submitButton}
          >
            {pending ? (
              <LoaderCircle className="animate-spin" aria-hidden="true" />
            ) : (
              <Scale aria-hidden="true" />
            )}
            Open and classify dispute
          </Button>
        </form>
      </div>
    </section>
  );
}

export function DisputeDetail({
  dispute,
  evidenceValid,
}: {
  dispute: Dispute;
  evidenceValid: boolean;
}) {
  return (
    <div className={styles.workspace}>
      <header className={styles.detailHeader}>
        <div>
          <Link
            href={`/dashboard/transactions/${dispute.transactionId}`}
            className={styles.backLink}
          >
            <ArrowLeft aria-hidden="true" />
            Transaction
          </Link>
          <p className={styles.eyebrow}>Proof stream / dispute</p>
          <h1>Dispute record</h1>
          <code>{dispute.disputeId}</code>
        </div>
        <Link
          className={`${buttonVariants({ variant: "outline" })} ${styles.outlineAction}`}
          href={`/dashboard/transactions/${dispute.transactionId}`}
        >
          View transaction
        </Link>
      </header>
      <DisputeOutcome dispute={dispute} evidenceValid={evidenceValid} />
    </div>
  );
}

function DisputeOutcome({
  dispute,
  evidenceValid,
  detailLink,
}: {
  dispute: Dispute;
  evidenceValid: boolean;
  detailLink?: string;
}) {
  return (
    <section className={styles.outcome} aria-label="Dispute classification">
      <header className={styles.outcomeHeader}>
        <div>
          <p className={styles.eyebrow}>Classification</p>
          <h2>{disputeStatusLabel(dispute.status)}</h2>
        </div>
        <EvidenceBadge valid={evidenceValid} />
      </header>
      <div className={styles.outcomeGrid}>
        <article className={styles.explanation}>
          <span>Recorded facts</span>
          <p>{dispute.explanation}</p>
          {dispute.statement ? (
            <blockquote>{dispute.statement}</blockquote>
          ) : null}
        </article>
        <dl className={styles.facts}>
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
          <div>
            <dt>Transaction</dt>
            <dd>{dispute.transactionId}</dd>
          </div>
        </dl>
      </div>
      {detailLink ? (
        <footer className={styles.outcomeFooter}>
          <Link
            className={`${buttonVariants({ variant: "outline" })} ${styles.outlineAction}`}
            href={detailLink}
          >
            View dispute record
          </Link>
        </footer>
      ) : null}
    </section>
  );
}

function EvidenceBadge({ valid }: { valid: boolean }) {
  return (
    <span className={styles.evidenceBadge} data-valid={valid}>
      {valid ? (
        <CheckCircle2 aria-hidden="true" />
      ) : (
        <ShieldAlert aria-hidden="true" />
      )}
      {valid ? "Evidence chain verified" : "Evidence verification failed"}
    </span>
  );
}

function formatUTC(value: string): string {
  return `${new Intl.DateTimeFormat("en", { dateStyle: "medium", timeStyle: "short", timeZone: "UTC" }).format(new Date(value))} UTC`;
}
