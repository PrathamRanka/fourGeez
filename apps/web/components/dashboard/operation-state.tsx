"use client";

import {
  Ban,
  CircleOff,
  ClockAlert,
  Inbox,
  LoaderCircle,
  LockKeyhole,
  RefreshCw,
  ShieldX,
  TriangleAlert,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import type { OperationStateKind } from "@/features/operations/model";

const stateContent: Record<
  OperationStateKind,
  { title: string; description: string; icon: typeof Inbox }
> = {
  loading: {
    title: "Loading seller workspace",
    description: "Fetching the latest authoritative AgentPay records.",
    icon: LoaderCircle,
  },
  empty: {
    title: "Nothing here yet",
    description: "New records will appear here after the first completed action.",
    icon: Inbox,
  },
  retryable_error: {
    title: "AgentPay is temporarily unavailable",
    description: "The request did not complete. Your saved records were not changed.",
    icon: TriangleAlert,
  },
  terminal_error: {
    title: "This page cannot be opened",
    description: "The record may not exist or your link may no longer be valid.",
    icon: ShieldX,
  },
  disabled: {
    title: "Action unavailable",
    description: "Complete the required earlier step before continuing.",
    icon: CircleOff,
  },
  permission_denied: {
    title: "Plan permission required",
    description: "The current plan does not allow this operation. Review plan access before retrying.",
    icon: LockKeyhole,
  },
  quota_exhausted: {
    title: "Monthly quota reached",
    description: "This seller has used the current plan allowance for this operation.",
    icon: ClockAlert,
  },
  seller_suspended: {
    title: "Seller account suspended",
    description: "Storefront changes and paid operations are blocked until the account is reactivated.",
    icon: Ban,
  },
};

type OperationStateProps = {
  kind: OperationStateKind;
  title?: string;
  description?: string;
  retryAfterSeconds?: number;
  onRetry?: () => void;
};

export function OperationState({
  kind,
  title,
  description,
  retryAfterSeconds,
  onRetry,
}: OperationStateProps) {
  const content = stateContent[kind];
  const Icon = content.icon;
  const isLoading = kind === "loading";
  return (
    <section
      className="operation-state"
      data-state={kind}
      role={isLoading ? "status" : kind.includes("error") ? "alert" : undefined}
      aria-live={isLoading ? "polite" : undefined}
    >
      <span className="operation-state-icon">
        <Icon className={isLoading ? "animate-spin" : undefined} aria-hidden="true" />
      </span>
      <div>
        <h2>{title ?? content.title}</h2>
        <p>{description ?? content.description}</p>
        {kind === "quota_exhausted" && retryAfterSeconds ? (
          <small>Retry after at least {retryAfterSeconds} seconds.</small>
        ) : null}
      </div>
      {kind === "retryable_error" && onRetry ? (
        <Button type="button" variant="outline" onClick={onRetry}>
          <RefreshCw aria-hidden="true" />
          Try again
        </Button>
      ) : null}
    </section>
  );
}

