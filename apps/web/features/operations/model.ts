export type OperationStateKind =
  | "loading"
  | "empty"
  | "retryable_error"
  | "terminal_error"
  | "disabled"
  | "permission_denied"
  | "quota_exhausted"
  | "seller_suspended";

export type OperationFailure = {
  code?: string;
  status?: number;
  retryAfterSeconds?: number;
};

// operationStateFromFailure maps stable wire metadata without inspecting human copy.
export function operationStateFromFailure(
  failure: OperationFailure,
): OperationStateKind {
  if (failure.code === "rate_limited" || failure.status === 429) {
    return "quota_exhausted";
  }
  if (failure.code === "permission_denied" || failure.status === 403) {
    return "permission_denied";
  }
  if (
    failure.code === "dependency_unavailable" ||
    (failure.status !== undefined && failure.status >= 500)
  ) {
    return "retryable_error";
  }
  return "terminal_error";
}

