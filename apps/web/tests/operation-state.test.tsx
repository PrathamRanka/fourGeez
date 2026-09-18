import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { OperationState } from "@/components/dashboard/operation-state";
import { operationStateFromFailure } from "@/features/operations/model";

describe("operation states", () => {
  it("renders loading, empty, disabled, and terminal states", () => {
    const { rerender } = render(<OperationState kind="loading" />);
    expect(screen.getByRole("status")).toHaveTextContent("Loading seller workspace");

    rerender(<OperationState kind="empty" title="No transactions yet" />);
    expect(screen.getByText("No transactions yet")).toBeVisible();

    rerender(<OperationState kind="disabled" />);
    expect(screen.getByText("Action unavailable")).toBeVisible();

    rerender(<OperationState kind="terminal_error" />);
    expect(screen.getByText("This page cannot be opened")).toBeVisible();
  });

  it("renders retry, permission, quota, and suspended-seller guidance", () => {
    const retry = vi.fn();
    const { rerender } = render(<OperationState kind="retryable_error" onRetry={retry} />);
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(retry).toHaveBeenCalledTimes(1);

    rerender(<OperationState kind="permission_denied" />);
    expect(screen.getByText("Plan permission required")).toBeVisible();

    rerender(<OperationState kind="quota_exhausted" retryAfterSeconds={60} />);
    expect(screen.getByText("Monthly quota reached")).toBeVisible();
    expect(screen.getByText(/60 seconds/)).toBeVisible();

    rerender(<OperationState kind="seller_suspended" />);
    expect(screen.getByText("Seller account suspended")).toBeVisible();
  });

  it("maps stable API failures without relying on message text", () => {
    expect(operationStateFromFailure({ code: "rate_limited", status: 429 })).toBe("quota_exhausted");
    expect(operationStateFromFailure({ code: "permission_denied", status: 403 })).toBe("permission_denied");
    expect(operationStateFromFailure({ code: "dependency_unavailable", status: 503 })).toBe("retryable_error");
    expect(operationStateFromFailure({ code: "not_found", status: 404 })).toBe("terminal_error");
  });
});
