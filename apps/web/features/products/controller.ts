"use server";

import type {
  CreateRouteDraftInput,
  PaidRoute,
  ProductRouteSnapshot,
  RouteAuditEvent,
  RouteIdentityInput,
  RouteValidationResult,
  RouteVersionInput,
  UpdateRoutePriceInput,
} from "@/features/products/model";
import { requestAgentPay, type ActionResult } from "@/lib/agentpay-api";

// loadProductRouteSnapshot loads independent route and history data in parallel.
export async function loadProductRouteSnapshot(
  sellerId: string,
): Promise<ProductRouteSnapshot> {
  const encodedSellerId = encodeURIComponent(sellerId);
  const [routeResult, auditResult] = await Promise.all([
    requestAgentPay<{ items: PaidRoute[] }>(
      `/v1/sellers/${encodedSellerId}/routes`,
      { method: "GET" },
    ),
    requestAgentPay<{ items: RouteAuditEvent[] }>(
      `/v1/sellers/${encodedSellerId}/audit-events?limit=100`,
      { method: "GET" },
    ),
  ]);

  const error = !routeResult.ok
    ? routeResult.error
    : !auditResult.ok
      ? auditResult.error
      : undefined;

  return {
    sellerId,
    routes: routeResult.ok ? routeResult.value.items : [],
    auditEvents: auditResult.ok ? auditResult.value.items : [],
    error,
  };
}

// createDraft creates a route that remains unavailable until explicit publication.
export async function createDraft(
  input: CreateRouteDraftInput,
): Promise<ActionResult<PaidRoute>> {
  return requestAgentPay<PaidRoute>(
    `/v1/sellers/${encodeURIComponent(input.sellerId)}/routes`,
    {
      method: "POST",
      body: {
        method: input.method,
        pathPattern: input.pathPattern,
        description: input.description,
        mimeType: input.mimeType,
        amount: input.amount,
        asset: input.asset,
        network: input.network,
        payTo: input.payTo,
        upstreamTimeoutSeconds: input.upstreamTimeoutSeconds,
        publishImmediately: false,
      },
    },
  );
}

// updatePrice changes only the quote used by future purchase intents.
export async function updatePrice(
  input: UpdateRoutePriceInput,
): Promise<ActionResult<PaidRoute>> {
  return requestAgentPay<PaidRoute>(routePath(input), {
    method: "PATCH",
    body: {
      amount: input.amount,
      expectedVersion: input.expectedVersion,
    },
  });
}

// validateRoute returns current deterministic publication checks.
export async function validateRoute(
  input: RouteIdentityInput,
): Promise<ActionResult<RouteValidationResult>> {
  return requestAgentPay<RouteValidationResult>(
    `${routePath(input)}/validation`,
    { method: "GET" },
  );
}

// publishRoute publishes a draft or resumes a stopped route.
export async function publishRoute(
  input: RouteVersionInput,
): Promise<ActionResult<PaidRoute>> {
  return mutateLifecycle(input, "publish");
}

// pauseRoute performs a planned reversible stop.
export async function pauseRoute(
  input: RouteVersionInput,
): Promise<ActionResult<PaidRoute>> {
  return mutateLifecycle(input, "pause");
}

// archiveRoute permanently retires a non-published route.
export async function archiveRoute(
  input: RouteVersionInput,
): Promise<ActionResult<PaidRoute>> {
  return mutateLifecycle(input, "archive");
}

// emergencyDisableRoute immediately removes a route from new purchase flows.
export async function emergencyDisableRoute(
  input: RouteVersionInput,
): Promise<ActionResult<PaidRoute>> {
  return mutateLifecycle(input, "emergency-disable");
}

// mutateLifecycle sends one version-guarded route lifecycle command.
async function mutateLifecycle(
  input: RouteVersionInput,
  action: "archive" | "emergency-disable" | "pause" | "publish",
): Promise<ActionResult<PaidRoute>> {
  return requestAgentPay<PaidRoute>(`${routePath(input)}/${action}`, {
    method: "POST",
    body: { expectedVersion: input.expectedVersion },
  });
}

// routePath builds an encoded seller-owned route resource path.
function routePath(input: RouteIdentityInput): string {
  return `/v1/sellers/${encodeURIComponent(input.sellerId)}/routes/${encodeURIComponent(input.routeId)}`;
}
