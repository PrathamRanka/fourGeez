import { NextResponse } from "next/server";
import type { ApprovalDecision } from "@/features/approvals/model";
import { decideApprovalSession } from "@/features/approvals/controller";

export async function POST(
  request: Request,
  context: { params: Promise<{ sessionId: string }> },
) {
  const token = new URL(request.url).searchParams.get("token");
  if (!token) {
    return NextResponse.json({ error: "Invitation token is required." }, { status: 401 });
  }
  const body = (await request.json()) as { decision?: ApprovalDecision };
  if (body.decision !== "approve" && body.decision !== "veto") {
    return NextResponse.json({ error: "Decision must be approve or veto." }, { status: 422 });
  }
  const { sessionId } = await context.params;
  const result = await decideApprovalSession(sessionId, token, body.decision);
  return NextResponse.json(result.ok ? result.value : { error: result.error }, {
    status: result.ok ? 200 : 502,
    headers: { "Cache-Control": "no-store" },
  });
}

