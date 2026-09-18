import { NextResponse } from "next/server";
import { loadBrowserDispute } from "@/features/commerce/server/commerce-gateway";

type DisputeRouteContext = {
  params: Promise<{ disputeId: string }>;
};

export async function GET(request: Request, context: DisputeRouteContext) {
  const { disputeId } = await context.params;
  const result = await loadBrowserDispute(
    disputeId,
    request.headers.get("cookie") ?? "",
  );
  return result.ok
    ? NextResponse.json(result.value, {
        headers: { "Cache-Control": "no-store" },
      })
    : NextResponse.json(
        { error: { code: result.code, message: result.message } },
        { status: result.status, headers: { "Cache-Control": "no-store" } },
      );
}
