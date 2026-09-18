"use client";

import {
  Ban,
  Check,
  CheckCircle2,
  Clock3,
  LoaderCircle,
  Radio,
  ShieldCheck,
  WifiOff,
} from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import { Button } from "@/components/ui/button";
import type {
  ApprovalDecision,
  ApprovalEvent,
  ApprovalSessionSnapshot,
} from "@/features/approvals/model";

const refreshIntervalMilliseconds = 2_000;

type ApprovalSessionProps = {
  initialSnapshot: ApprovalSessionSnapshot;
  now: string;
  websocketOrigin?: string;
};

export function ApprovalSession({
  initialSnapshot,
  now,
  websocketOrigin,
}: ApprovalSessionProps) {
  const [snapshot, setSnapshot] = useState(initialSnapshot);
  const [currentTime, setCurrentTime] = useState(() => new Date(now).getTime());
  const [connection, setConnection] = useState<"socket" | "rest">(
    websocketOrigin ? "socket" : "rest",
  );
  const [pendingDecision, setPendingDecision] =
    useState<ApprovalDecision | null>(null);
  const [error, setError] = useState<string | null>(null);

  const invitationToken = useCallback(
    () => new URLSearchParams(window.location.search).get("token") ?? "",
    [],
  );

  const refresh = useCallback(async () => {
    const token = invitationToken();
    if (!token) {
      setError("This approval invitation is missing its access token.");
      return;
    }
    try {
      const response = await fetch(
        `/approve/${encodeURIComponent(snapshot.sessionId)}/snapshot?token=${encodeURIComponent(token)}`,
        { cache: "no-store" },
      );
      const responseBody = (await response.json()) as
        | ApprovalSessionSnapshot
        | { error?: string };
      if (!response.ok || !("sessionId" in responseBody)) {
        setError(
          "error" in responseBody && responseBody.error
            ? responseBody.error
            : "Approval status could not be refreshed.",
        );
        return;
      }
      setSnapshot(responseBody);
      setError(null);
    } catch {
      setError("Approval status could not be refreshed. Retrying automatically.");
    }
  }, [invitationToken, snapshot.sessionId]);

  useEffect(() => {
    const timer = window.setInterval(
      () => setCurrentTime(Date.now()),
      1_000,
    );
    return () => window.clearInterval(timer);
  }, []);

  useEffect(() => {
    if (snapshot.status !== "pending") {
      return;
    }
    if (!websocketOrigin) {
      return;
    }

    const token = invitationToken();
    if (!token) {
      return;
    }
    const socket = new WebSocket(
      `${websocketOrigin.replace(/\/$/, "")}/ws/approval-sessions/${encodeURIComponent(snapshot.sessionId)}?token=${encodeURIComponent(token)}`,
    );
    socket.addEventListener("open", () => setConnection("socket"));
    socket.addEventListener("message", (message) => {
      try {
        const event = JSON.parse(String(message.data)) as ApprovalEvent;
        if (event.sessionId === snapshot.sessionId) {
          void refresh();
        }
      } catch {
        void refresh();
      }
    });
    socket.addEventListener("close", () => setConnection("rest"));
    socket.addEventListener("error", () => setConnection("rest"));
    return () => socket.close();
  }, [invitationToken, refresh, snapshot.sessionId, snapshot.status, websocketOrigin]);

  useEffect(() => {
    if (snapshot.status !== "pending" || connection !== "rest") {
      return;
    }
    const timer = window.setInterval(refresh, refreshIntervalMilliseconds);
    return () => window.clearInterval(timer);
  }, [connection, refresh, snapshot.status]);

  const locallyExpired =
    snapshot.status === "pending" &&
    currentTime >= new Date(snapshot.expiresAt).getTime();
  const resolved = snapshot.status !== "pending" || locallyExpired;
  const approvals = snapshot.decisions.filter(
    (decision) => decision.decision === "approve",
  ).length;

  async function decide(decision: ApprovalDecision) {
    const token = invitationToken();
    if (!token || resolved) {
      setError(
        token
          ? "This approval session can no longer accept decisions."
          : "This approval invitation is missing its access token.",
      );
      return;
    }
    setPendingDecision(decision);
    setError(null);
    try {
      const response = await fetch(
        `/approve/${encodeURIComponent(snapshot.sessionId)}/decision?token=${encodeURIComponent(token)}`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ decision }),
        },
      );
      const responseBody = (await response.json()) as
        | ApprovalSessionSnapshot
        | { error?: string };
      if (!response.ok || !("sessionId" in responseBody)) {
        setError(
          "error" in responseBody && responseBody.error
            ? responseBody.error
            : "Your decision could not be recorded.",
        );
        return;
      }
      setSnapshot(responseBody);
    } catch {
      setError("Your decision could not be recorded. Check the connection and retry.");
    } finally {
      setPendingDecision(null);
    }
  }

  const statusCopy = locallyExpired
    ? "Approval window expired"
    : snapshot.status === "approved"
      ? "Purchase approved"
      : snapshot.status === "vetoed"
        ? "Purchase vetoed"
        : snapshot.status === "expired"
          ? "Approval window expired"
          : "Decision requested";

  return (
    <main id="main-content" className="approval-page">
      <header className="approval-page-header">
        <div className="approval-brand"><ShieldCheck aria-hidden="true" /><span>AgentPay Trust Gate</span></div>
        <p>Two-person approval</p>
        <h1>{statusCopy}</h1>
        <span>
          Review the immutable purchase intent and record one decision before
          the approval window closes.
        </span>
      </header>

      <section className="approval-live-card" aria-labelledby="approval-progress-title">
        <div className="approval-live-heading">
          <div>
            <Radio aria-hidden="true" />
            <div>
              <h2 id="approval-progress-title">
                {approvals} of {snapshot.requiredApprovals} approvals recorded
              </h2>
              <p aria-live="polite">
                {connection === "socket"
                  ? "Live updates connected"
                  : "Live updates via REST fallback"}
              </p>
            </div>
          </div>
          <span data-approval-status={locallyExpired ? "expired" : snapshot.status}>
            {statusCopy}
          </span>
        </div>

        <ol className="approval-quorum-rail" aria-label="Approval progress">
          {Array.from({ length: snapshot.requiredApprovals }, (_, index) => {
            const decision = snapshot.decisions[index];
            return (
              <li key={decision?.label ?? `open-${index}`} data-decision={decision?.decision ?? "open"}>
                <span>{decision ? (decision.decision === "approve" ? <Check aria-hidden="true" /> : <Ban aria-hidden="true" />) : index + 1}</span>
                <div>
                  <strong>{decision?.label ?? "Awaiting approver"}</strong>
                  <small>{decision ? `${decision.decision === "approve" ? "Approved" : "Vetoed"} · ${formatUTC(decision.decidedAt)}` : "Invitation open"}</small>
                </div>
              </li>
            );
          })}
        </ol>

        <dl className="approval-intent-facts">
          <div><dt>Purchase intent</dt><dd>{snapshot.intentId}</dd></div>
          <div><dt>Intent hash</dt><dd>{snapshot.intentHash}</dd></div>
          <div><dt>Expires</dt><dd>{formatUTC(snapshot.expiresAt)}</dd></div>
          <div><dt>Version</dt><dd>{snapshot.version}</dd></div>
        </dl>

        {error ? <p className="approval-error" role="alert">{error}</p> : null}

        <div className="approval-actions">
          <Button
            size="lg"
            disabled={resolved || pendingDecision !== null}
            onClick={() => void decide("approve")}
          >
            {pendingDecision === "approve" ? <LoaderCircle className="animate-spin" aria-hidden="true" /> : <CheckCircle2 aria-hidden="true" />}
            Approve purchase
          </Button>
          <AlertDialog>
            <AlertDialogTrigger
              render={<Button variant="destructive" size="lg" disabled={resolved || pendingDecision !== null} />}
            >
              <Ban aria-hidden="true" />
              Veto purchase
            </AlertDialogTrigger>
            <AlertDialogContent>
              <AlertDialogHeader>
                <AlertDialogTitle>Veto this purchase?</AlertDialogTitle>
                <AlertDialogDescription>
                  A veto resolves the session immediately and prevents this
                  purchase from proceeding.
                </AlertDialogDescription>
              </AlertDialogHeader>
              <AlertDialogFooter>
                <AlertDialogCancel>Keep reviewing</AlertDialogCancel>
                <AlertDialogAction variant="destructive" onClick={() => void decide("veto")}>
                  Confirm veto
                </AlertDialogAction>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>
        </div>

        <div className="approval-security-note">
          {connection === "socket" ? <Radio aria-hidden="true" /> : <WifiOff aria-hidden="true" />}
          <p><strong>REST is authoritative.</strong> This page refreshes the signed session snapshot after every live event and retries automatically if the socket is unavailable.</p>
        </div>
      </section>

      <p className="approval-expiration"><Clock3 aria-hidden="true" /> Invitation access ends at {formatUTC(snapshot.expiresAt)}.</p>
    </main>
  );
}

function formatUTC(value: string): string {
  return `${new Intl.DateTimeFormat("en", {
    dateStyle: "medium",
    timeStyle: "short",
    timeZone: "UTC",
  }).format(new Date(value))} UTC`;
}
