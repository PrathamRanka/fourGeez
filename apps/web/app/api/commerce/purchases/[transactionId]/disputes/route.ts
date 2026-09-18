import { NextResponse } from "next/server";
import { createBrowserDispute } from "@/features/commerce/server/commerce-gateway";

type DisputeRouteContext = {
  params: Promise<{ transactionId: string }>;
};

export async function POST(request: Request, context: DisputeRouteContext) {
  let input: unknown;
  try {
    input = await request.json();
  } catch {
    return NextResponse.json(
      { error: { code: "invalid_request", message: "Invalid dispute request." } },
      { status: 400, headers: { "Cache-Control": "no-store" } },
    );
  }
  const { transactionId } = await context.params;
  const disputeInput =
    typeof input === "object" && input !== null
      ? { ...(input as Record<string, unknown>), transactionId }
      : input;
  const result = await createBrowserDispute(
    disputeInput,
    request.headers.get("cookie") ?? "",
  );
  return result.ok
    ? NextResponse.json(result.value, {
        status: 201,
        headers: { "Cache-Control": "no-store" },
      })
    : NextResponse.json(
        { error: { code: result.code, message: result.message } },
        { status: result.status, headers: { "Cache-Control": "no-store" } },
      );
}
