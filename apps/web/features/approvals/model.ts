export type ApprovalDecision = "approve" | "veto";

export type ApprovalSessionStatus =
  | "pending"
  | "approved"
  | "vetoed"
  | "expired";

export type ApprovalSessionSnapshot = {
  sessionId: string;
  intentId: string;
  intentHash: string;
  requiredApprovals: number;
  decisions: Array<{
    label: string;
    decision: ApprovalDecision;
    decidedAt: string;
  }>;
  status: ApprovalSessionStatus;
  expiresAt: string;
  createdAt: string;
  updatedAt: string;
  version: number;
};

export type ApprovalEvent = {
  eventId: string;
  type:
    | "session.snapshot"
    | "approver.joined"
    | "approval.decided"
    | "session.resolved";
  sessionId: string;
  occurredAt: string;
  data: ApprovalSessionSnapshot;
};

