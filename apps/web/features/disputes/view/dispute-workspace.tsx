"use client";

import {
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

const disputeReasons: DisputeReason[] = [
  "unauthorized",
  "duplicate",
  "wrong_amount",
  "not_delivered",
  "quality_or_output",
];

type DisputePanelProps = {
  sellerId?: string;
  transactionId: string;
  transactionStatus: TransactionStatus;
  evidenceValid: boolean;
  createDispute: DisputeAction;
};

export function DisputePanel({
  transactionId,
  sellerId,
  transactionStatus,
  evidenceValid,
  createDispute,
}: DisputePanelProps) {
  const [dispute, setDispute] = useState<Dispute | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);
  const canDispute = transactionStatus === "FULFILLED" || transactionStatus === "FAILED";

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
        detailLink={`/dashboard/disputes/${dispute.disputeId}${sellerId ? `?sellerId=${encodeURIComponent(sellerId)}` : ""}`}
      />
    );
  }

  return (
    <section className="dispute-panel" aria-labelledby="open-dispute-title">
      <header>
        <FileWarning aria-hidden="true" />
        <div>
          <h2 id="open-dispute-title">Open a dispute</h2>
          <p>Recorded transaction facts determine the initial classification.</p>
        </div>
        <EvidenceBadge valid={evidenceValid} />
      </header>
      <form aria-label="Open dispute" onSubmit={submit}>
        <label>
          <span>Dispute reason</span>
          <select name="reason" defaultValue="" required disabled={!canDispute || pending}>
            <option value="" disabled>Select a reason</option>
            {disputeReasons.map((reason) => (
              <option key={reason} value={reason}>{disputeReasonLabel(reason)}</option>
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
          <p className="dispute-disabled-copy">
            Disputes become available after fulfillment succeeds or fails.
          </p>
        ) : null}
        {error ? <p className="dispute-error" role="alert">{error}</p> : null}
        <Button type="submit" disabled={!canDispute || pending}>
          {pending ? <LoaderCircle className="animate-spin" aria-hidden="true" /> : <Scale aria-hidden="true" />}
          Open and classify dispute
        </Button>
      </form>
    </section>
  );
}

export function DisputeDetail({
  dispute,
  evidenceValid,
  sellerId,
}: {
  dispute: Dispute;
  evidenceValid: boolean;
  sellerId?: string;
}) {
  return (
    <div className="dispute-detail-workspace">
      <header className="dispute-detail-header">
        <div>
          <p className="dashboard-eyebrow">Proof Stream</p>
          <h1>{dispute.disputeId}</h1>
          <p>Deterministic classification and recorded resolution state.</p>
        </div>
        <Link className={buttonVariants({ variant: "outline" })} href={`/dashboard/transactions/${dispute.transactionId}${sellerId ? `?sellerId=${encodeURIComponent(sellerId)}` : ""}`}>
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
    <section className="dispute-outcome" aria-labelledby="dispute-outcome-title">
      <header>
        <Scale aria-hidden="true" />
        <div>
          <p>Classification</p>
          <h2 id="dispute-outcome-title">{disputeStatusLabel(dispute.status)}</h2>
        </div>
        <EvidenceBadge valid={evidenceValid} />
      </header>
      <p className="dispute-explanation">{dispute.explanation}</p>
      <dl>
        <div><dt>Reason</dt><dd>{disputeReasonLabel(dispute.reason)}</dd></div>
        <div><dt>Classification code</dt><dd>{dispute.classificationCode}</dd></div>
        <div><dt>Rule version</dt><dd>{dispute.ruleVersion}</dd></div>
        <div><dt>Created</dt><dd>{formatUTC(dispute.createdAt)}</dd></div>
      </dl>
      {dispute.statement ? <blockquote>{dispute.statement}</blockquote> : null}
      {detailLink ? <Link className={buttonVariants({ variant: "outline" })} href={detailLink}>View dispute record</Link> : null}
    </section>
  );
}

function EvidenceBadge({ valid }: { valid: boolean }) {
  return (
    <span className="dispute-evidence-badge" data-valid={valid}>
      {valid ? <CheckCircle2 aria-hidden="true" /> : <ShieldAlert aria-hidden="true" />}
      {valid ? "Evidence chain verified" : "Evidence verification failed"}
    </span>
  );
}

function formatUTC(value: string): string {
  return `${new Intl.DateTimeFormat("en", {
    dateStyle: "medium",
    timeStyle: "short",
    timeZone: "UTC",
  }).format(new Date(value))} UTC`;
}
