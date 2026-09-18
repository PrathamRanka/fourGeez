"use client";

import { OperationState } from "@/components/dashboard/operation-state";

export default function DashboardError({ reset }: { error: Error; reset: () => void }) {
  return <OperationState kind="retryable_error" onRetry={reset} />;
}

