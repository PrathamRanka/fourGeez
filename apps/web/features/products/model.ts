import type { ActionResult } from "@/lib/agentpay-api";

export type RouteLifecycleStatus =
  "draft" | "published" | "paused" | "archived" | "emergency_disabled";

export type PaidRoute = {
  routeId: string;
  sellerId: string;
  displayName: string;
  productSlug: string;
  method: "GET" | "POST";
  pathPattern: string;
  description: string;
  mimeType: string;
  inputSchema: Record<string, unknown>;
  outputSchema: Record<string, unknown>;
  amount: string;
  asset: string;
  network: string;
  payTo: string;
  approvalThresholdAmount: string | null;
  upstreamTimeoutSeconds: number;
  lifecycleStatus: RouteLifecycleStatus;
  enabled: boolean;
  createdAt: string;
  updatedAt: string;
  version: number;
};

export type RouteValidationCheck = {
  name: string;
  passed: boolean;
  message: string;
};

export type RouteValidationResult = {
  sellerId: string;
  routeId: string;
  valid: boolean;
  checks: RouteValidationCheck[];
  version: number;
  contractHash: string;
};

export type RouteAuditEvent = {
  auditEventId: string;
  sellerId: string;
  actorType: string;
  actorId: string;
  action: string;
  targetType: string;
  targetId: string;
  outcome: string;
  requestId: string;
  changedFields: string[];
  occurredAt: string;
};

export type ProductRouteSnapshot = {
  sellerId: string;
  routes: PaidRoute[];
  auditEvents: RouteAuditEvent[];
  activePaymentDestination?: {
    address: string;
    asset: string;
    network: string;
  };
  error?: string;
};

export type CreateRouteDraftInput = {
  sellerId: string;
  displayName: string;
  productSlug: string;
  method: "GET" | "POST";
  pathPattern: string;
  description: string;
  mimeType: string;
  inputSchema: Record<string, unknown>;
  outputSchema: Record<string, unknown>;
  amount: string;
  asset: string;
  network: string;
  payTo: string;
  upstreamTimeoutSeconds: number;
};

export type RouteIdentityInput = {
  sellerId: string;
  routeId: string;
};

export type RouteVersionInput = RouteIdentityInput & {
  expectedVersion: number;
};

export type PublishRouteInput = RouteVersionInput & {
  contractHash: string;
};

export type UpdateRouteDraftInput = RouteVersionInput & {
  displayName: string;
  method: "GET" | "POST";
  pathPattern: string;
  description: string;
  mimeType: string;
  inputSchema: Record<string, unknown>;
  outputSchema: Record<string, unknown>;
  amount: string;
  upstreamTimeoutSeconds: number;
};

export type ProductRouteActions = {
  createDraft: (
    input: CreateRouteDraftInput,
  ) => Promise<ActionResult<PaidRoute>>;
  updateDraft: (
    input: UpdateRouteDraftInput,
  ) => Promise<ActionResult<PaidRoute>>;
  validateRoute: (
    input: RouteIdentityInput,
  ) => Promise<ActionResult<RouteValidationResult>>;
  publishRoute: (input: PublishRouteInput) => Promise<ActionResult<PaidRoute>>;
  pauseRoute: (input: RouteVersionInput) => Promise<ActionResult<PaidRoute>>;
  archiveRoute: (input: RouteVersionInput) => Promise<ActionResult<PaidRoute>>;
  emergencyDisableRoute: (
    input: RouteVersionInput,
  ) => Promise<ActionResult<PaidRoute>>;
};

const lifecycleLabels: Record<RouteLifecycleStatus, string> = {
  draft: "Draft",
  published: "Published",
  paused: "Paused",
  archived: "Archived",
  emergency_disabled: "Emergency disabled",
};

// routeLifecycleLabel returns the seller-facing name for one route state.
export function routeLifecycleLabel(status: RouteLifecycleStatus): string {
  return lifecycleLabels[status];
}
