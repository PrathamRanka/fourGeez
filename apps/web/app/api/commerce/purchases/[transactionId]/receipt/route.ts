import { loadBrowserPurchaseReceipt } from "@/features/commerce/server/commerce-gateway";

type ReceiptRouteContext = {
  params: Promise<{ transactionId: string }>;
};

export async function GET(request: Request, context: ReceiptRouteContext) {
  const { transactionId } = await context.params;
  const result = await loadBrowserPurchaseReceipt(
    transactionId,
    request.headers.get("cookie") ?? "",
  );
  if (!result.ok) {
    return Response.json(
      { error: { code: result.code, message: result.message } },
      { status: result.status, headers: { "Cache-Control": "no-store" } },
    );
  }
  return new Response(result.value.body, {
    headers: {
      "Cache-Control": "no-store",
      "Content-Disposition": result.value.contentDisposition,
      "Content-Type": result.value.contentType,
    },
  });
}
