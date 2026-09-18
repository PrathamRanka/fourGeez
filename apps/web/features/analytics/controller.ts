"use server";

import type {
  AnalyticsRoute,
  AnalyticsSnapshot,
  SalesAggregate,
} from "@/features/analytics/model";
import { authenticatedSellerId } from "@/features/auth/server/authorization";
import { requestAgentPay } from "@/lib/agentpay-api";

// loadAnalyticsSnapshot starts independent summary and route requests together.
export async function loadAnalyticsSnapshot(): Promise<AnalyticsSnapshot> {
  const sellerId = await authenticatedSellerId();
  if (!sellerId) {
    return {
      sellerId: "",
      transactionCount: 0,
      aggregates: [],
      routes: [],
      error: "Your seller session has expired. Sign in again.",
    };
  }
  const encodedSellerId = encodeURIComponent(sellerId);
  const [summaryResult, routeResult] = await Promise.all([
    requestAgentPay<{
      transactionCount: number;
      aggregates: SalesAggregate[];
    }>(`/v1/sellers/${encodedSellerId}/dashboard-summary`, { method: "GET" }),
    requestAgentPay<{ items: AnalyticsRoute[] }>(
      `/v1/sellers/${encodedSellerId}/routes`,
      { method: "GET" },
    ),
  ]);

  return {
    sellerId,
    transactionCount: summaryResult.ok
      ? summaryResult.value.transactionCount
      : 0,
    aggregates: summaryResult.ok ? summaryResult.value.aggregates : [],
    routes: routeResult.ok ? routeResult.value.items : [],
    error: !summaryResult.ok
      ? summaryResult.error
      : !routeResult.ok
        ? routeResult.error
        : undefined,
  };
}
