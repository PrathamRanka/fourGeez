import { NextResponse } from "next/server";
import { loadBrowserPurchase } from "@/features/commerce/server/commerce-gateway";

type PurchaseRouteContext = {
  params: Promise<{ transactionId: string }>;
};

export async function GET(request: Request, context: PurchaseRouteContext) {
  const { transactionId } = await context.params;
  const result = await loadBrowserPurchase(
    transactionId,
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
