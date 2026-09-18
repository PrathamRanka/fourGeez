import type {
  ApprovalDecision,
  ApprovalSessionSnapshot,
} from "@/features/approvals/model";
import { requestApprovalInvitation } from "@/lib/agentpay-api";

export async function loadApprovalSession(
  sessionId: string,
  invitationToken: string,
) {
  return requestApprovalInvitation<ApprovalSessionSnapshot>(
    `/v1/approval-sessions/${encodeURIComponent(sessionId)}`,
    invitationToken,
    { method: "GET" },
  );
}

export async function decideApprovalSession(
  sessionId: string,
  invitationToken: string,
  decision: ApprovalDecision,
) {
  return requestApprovalInvitation<ApprovalSessionSnapshot>(
    `/v1/approval-sessions/${encodeURIComponent(sessionId)}/decisions`,
    invitationToken,
    { method: "POST", body: { decision } },
  );
}

