import { downloadAgentPayFile } from "@/lib/agentpay-api";

type ReceiptRouteContext = {
  params: Promise<{ transactionId: string }>;
};

// GET proxies a bounded receipt download without exposing seller credentials.
export async function GET(_request: Request, context: ReceiptRouteContext) {
  const { transactionId } = await context.params;
  if (!/^txn_[A-Za-z0-9]+$/.test(transactionId)) {
    return Response.json({ error: "Receipt not found." }, { status: 404 });
  }
  const result = await downloadAgentPayFile(
    `/v1/transactions/${encodeURIComponent(transactionId)}/receipt`,
  );
  if (!result.ok) {
    return Response.json(
      { error: result.error },
      {
        status: result.status ?? 409,
        headers: { "Cache-Control": "no-store" },
      },
    );
  }
  return new Response(result.value.body, {
    headers: {
      "Cache-Control": "no-store",
      "Content-Disposition": `attachment; filename="${transactionId}-receipt.json"`,
      "Content-Type": result.value.contentType,
    },
  });
}
