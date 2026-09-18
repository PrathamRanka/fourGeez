import { NextResponse } from "next/server";
import { loadApprovalSession } from "@/features/approvals/controller";

export async function GET(
  request: Request,
  context: { params: Promise<{ sessionId: string }> },
) {
  const token = new URL(request.url).searchParams.get("token");
  if (!token) {
    return NextResponse.json({ error: "Invitation token is required." }, { status: 401 });
  }
  const { sessionId } = await context.params;
  const result = await loadApprovalSession(sessionId, token);
  return NextResponse.json(result.ok ? result.value : { error: result.error }, {
    status: result.ok ? 200 : 502,
    headers: { "Cache-Control": "no-store" },
  });
}

