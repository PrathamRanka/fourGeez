import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { ApprovalSessionSnapshot } from "@/features/approvals/model";
import { ApprovalSession } from "@/features/approvals/view/approval-session";

const pendingSnapshot: ApprovalSessionSnapshot = {
  sessionId: "aps_01ARZ3NDEKTSV4RRFFQ69G5FAV",
  intentId: "int_01ARZ3NDEKTSV4RRFFQ69G5FAW",
  intentHash: "a".repeat(64),
  requiredApprovals: 2,
  decisions: [
    {
      label: "Finance",
      decision: "approve",
      decidedAt: "2026-09-18T09:58:00Z",
    },
  ],
  status: "pending",
  expiresAt: "2026-09-18T10:10:00Z",
  createdAt: "2026-09-18T09:55:00Z",
  updatedAt: "2026-09-18T09:58:00Z",
  version: 2,
};

afterEach(() => {
  vi.restoreAllMocks();
  window.history.replaceState({}, "", "/");
});

describe("approval session", () => {
  it("shows live progress and records an approval through the invitation route", async () => {
    window.history.replaceState({}, "", "/approve/aps_demo?token=invite-secret");
    const resolvedSnapshot = {
      ...pendingSnapshot,
      status: "approved" as const,
      decisions: [
        ...pendingSnapshot.decisions,
        {
          label: "Security",
          decision: "approve" as const,
          decidedAt: "2026-09-18T10:00:00Z",
        },
      ],
      version: 3,
    };
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(
        new Response(JSON.stringify(resolvedSnapshot), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
      );

    render(
      <ApprovalSession
        initialSnapshot={pendingSnapshot}
        now="2026-09-18T10:00:00Z"
      />,
    );

    expect(screen.getByText("1 of 2 approvals recorded")).toBeVisible();
    expect(screen.getByText("Live updates via REST fallback")).toBeVisible();
    fireEvent.click(screen.getByRole("button", { name: "Approve purchase" }));

    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
    expect(fetchMock).toHaveBeenCalledWith(
      "/approve/aps_01ARZ3NDEKTSV4RRFFQ69G5FAV/decision?token=invite-secret",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ decision: "approve" }),
      }),
    );
    expect(
      await screen.findByRole("heading", { name: "Purchase approved" }),
    ).toBeVisible();
    expect(screen.getByRole("button", { name: "Approve purchase" })).toBeDisabled();
  });

  it("prevents decisions after the session expires", () => {
    render(
      <ApprovalSession
        initialSnapshot={{
          ...pendingSnapshot,
          status: "expired",
          expiresAt: "2026-09-18T09:59:00Z",
        }}
        now="2026-09-18T10:00:00Z"
      />,
    );

    expect(
      screen.getByRole("heading", { name: "Approval window expired" }),
    ).toBeVisible();
    expect(screen.getByRole("button", { name: "Approve purchase" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Veto purchase" })).toBeDisabled();
  });
});
