import { getSellerSession } from "@/features/auth/server/session";
import type { ActionResult } from "@/lib/agentpay-api";

export async function authenticatedSellerId(): Promise<string | null> {
  return (await getSellerSession())?.principal.sellerId ?? null;
}

export function sellerSessionRequired<Value>(): ActionResult<Value> {
  return {
    ok: false,
    error: "Your seller session has expired. Sign in again.",
    code: "unauthorized",
    status: 401,
  };
}
