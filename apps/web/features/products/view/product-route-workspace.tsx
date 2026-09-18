"use client";

import {
  AlertTriangle,
  Archive,
  Check,
  CirclePause,
  CirclePlay,
  LoaderCircle,
  Plus,
  RefreshCw,
  ShieldAlert,
} from "lucide-react";
import { useMemo, useState } from "react";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogMedia,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import { Button } from "@/components/ui/button";
import type {
  PaidRoute,
  ProductRouteActions,
  ProductRouteSnapshot,
  RouteAuditEvent,
  RouteValidationResult,
} from "@/features/products/model";
import { routeLifecycleLabel } from "@/features/products/model";
import { formatAtomicPrice } from "@/lib/money";

const historyDateFormatter = new Intl.DateTimeFormat("en", {
  dateStyle: "medium",
  timeStyle: "short",
  timeZone: "UTC",
});

type ProductRouteWorkspaceProps = {
  actions: ProductRouteActions;
  initialSnapshot: ProductRouteSnapshot;
};

// ProductRouteWorkspace gives sellers one controlled surface for route configuration.
export function ProductRouteWorkspace({
  actions,
  initialSnapshot,
}: ProductRouteWorkspaceProps) {
  const [routes, setRoutes] = useState(initialSnapshot.routes);
  const [selectedRouteId, setSelectedRouteId] = useState(
    initialSnapshot.routes[0]?.routeId ?? null,
  );
  const [validation, setValidation] = useState<RouteValidationResult | null>(
    null,
  );
  const [pendingAction, setPendingAction] = useState<string | null>(null);
  const [errorMessage, setErrorMessage] = useState<string | null>(
    initialSnapshot.error ?? null,
  );
  const [statusMessage, setStatusMessage] = useState<string | null>(null);
  const [confirmEmergencyDisable, setConfirmEmergencyDisable] =
    useState(false);
  const selectedRoute =
    routes.find((route) => route.routeId === selectedRouteId) ?? null;
  const routeCounts = useMemo(
    () => ({
      published: routes.filter(
        (route) => route.lifecycleStatus === "published",
      ).length,
      draft: routes.filter((route) => route.lifecycleStatus === "draft")
        .length,
      attention: routes.filter((route) =>
        ["paused", "emergency_disabled"].includes(route.lifecycleStatus),
      ).length,
    }),
    [routes],
  );

  // replaceRoute applies a server-confirmed route version to local workspace state.
  function replaceRoute(updatedRoute: PaidRoute) {
    setRoutes((currentRoutes) =>
      currentRoutes.map((route) =>
        route.routeId === updatedRoute.routeId ? updatedRoute : route,
      ),
    );
    setValidation(null);
  }

  // submitDraft creates an unpublished product route from seller-entered fields.
  async function submitDraft(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setErrorMessage(null);
    setStatusMessage(null);
    setPendingAction("create");
    const form = event.currentTarget;
    const fields = new FormData(form);
    const result = await actions.createDraft({
      sellerId: initialSnapshot.sellerId,
      displayName: String(fields.get("displayName") ?? "").trim(),
      productSlug: String(fields.get("productSlug") ?? "").trim(),
      method: String(fields.get("method")) === "GET" ? "GET" : "POST",
      pathPattern: String(fields.get("pathPattern") ?? "").trim(),
      description: String(fields.get("description") ?? "").trim(),
      mimeType: String(fields.get("mimeType") ?? "").trim(),
      amount: String(fields.get("amount") ?? "").trim(),
      asset: String(fields.get("asset") ?? "").trim(),
      network: String(fields.get("network") ?? "").trim(),
      payTo: String(fields.get("payTo") ?? "").trim(),
      upstreamTimeoutSeconds: Number(fields.get("upstreamTimeoutSeconds")),
    });
    setPendingAction(null);

    if (!result.ok) {
      setErrorMessage(result.error);
      return;
    }

    setRoutes((currentRoutes) => [...currentRoutes, result.value]);
    setSelectedRouteId(result.value.routeId);
    setValidation(null);
    setStatusMessage("Draft created. Validate it before publishing.");
    form.reset();
  }

  // submitPrice updates the authoritative quote for future purchase intents.
  async function submitPrice(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!selectedRoute) {
      return;
    }

    setErrorMessage(null);
    setStatusMessage(null);
    setPendingAction("price");
    const fields = new FormData(event.currentTarget);
    const result = await actions.updatePrice({
      sellerId: initialSnapshot.sellerId,
      routeId: selectedRoute.routeId,
      expectedVersion: selectedRoute.version,
      amount: String(fields.get("amount") ?? "").trim(),
    });
    setPendingAction(null);

    if (!result.ok) {
      setErrorMessage(result.error);
      return;
    }

    replaceRoute(result.value);
    setStatusMessage("Price updated for future purchases.");
  }

  // validateSelectedRoute refreshes deterministic publication checks.
  async function validateSelectedRoute() {
    if (!selectedRoute) {
      return;
    }

    setErrorMessage(null);
    setStatusMessage(null);
    setPendingAction("validate");
    const result = await actions.validateRoute({
      sellerId: initialSnapshot.sellerId,
      routeId: selectedRoute.routeId,
    });
    setPendingAction(null);

    if (!result.ok) {
      setErrorMessage(result.error);
      return;
    }

    setValidation(result.value);
  }

  // runLifecycleAction executes one guarded route transition using its current version.
  async function runLifecycleAction(
    actionName: "archive" | "emergency" | "pause" | "publish",
  ) {
    if (!selectedRoute) {
      return;
    }

    setErrorMessage(null);
    setStatusMessage(null);
    setPendingAction(actionName);
    const input = {
      sellerId: initialSnapshot.sellerId,
      routeId: selectedRoute.routeId,
      expectedVersion: selectedRoute.version,
    };
    const result =
      actionName === "publish"
        ? await actions.publishRoute(input)
        : actionName === "pause"
          ? await actions.pauseRoute(input)
          : actionName === "archive"
            ? await actions.archiveRoute(input)
            : await actions.emergencyDisableRoute(input);
    setPendingAction(null);
    setConfirmEmergencyDisable(false);

    if (!result.ok) {
      setErrorMessage(result.error);
      return;
    }

    replaceRoute(result.value);
    setStatusMessage(
      `Route is now ${routeLifecycleLabel(result.value.lifecycleStatus).toLowerCase()}.`,
    );
  }

  return (
    <div className="product-workspace">
      <header className="product-page-header">
        <div>
          <p className="dashboard-eyebrow">Catalog control</p>
          <h1>Product routes</h1>
          <p>
            Draft, validate, price, and publish the API operations customers can
            buy. Every change is version guarded and recorded.
          </p>
        </div>
        <div className="product-counts" aria-label="Route status summary">
          <span>{routeCounts.published} published</span>
          <span>{routeCounts.draft} draft</span>
          <span>{routeCounts.attention} need attention</span>
        </div>
      </header>

      {errorMessage ? (
        <div className="dashboard-error" role="alert">
          {errorMessage}
        </div>
      ) : null}
      <p className="dashboard-status" aria-live="polite">
        {statusMessage}
      </p>

      <section className="product-create-panel" aria-labelledby="new-route-title">
        <div className="product-panel-heading">
          <div>
            <span className="product-panel-icon">
              <Plus aria-hidden="true" />
            </span>
            <div>
              <h2 id="new-route-title">Create a route draft</h2>
              <p>Nothing goes live until validation passes and you publish it.</p>
            </div>
          </div>
        </div>
        <form
          aria-label="Create route draft"
          className="product-create-form"
          onSubmit={submitDraft}
        >
          <label>
            <span>Product name</span>
            <input
              name="displayName"
              required
              maxLength={120}
              placeholder="Board-ready market report"
            />
          </label>
          <label>
            <span>Product slug</span>
            <input
              name="productSlug"
              required
              minLength={3}
              maxLength={80}
              pattern="[A-Za-z0-9][A-Za-z0-9 _-]*[A-Za-z0-9]"
              placeholder="board-ready-market-report"
            />
          </label>
          <label>
            <span>Method</span>
            <select name="method" defaultValue="POST">
              <option value="POST">POST</option>
              <option value="GET">GET</option>
            </select>
          </label>
          <label className="product-field-route">
            <span>Route path</span>
            <input
              name="pathPattern"
              required
              pattern="/[A-Za-z0-9/_-]+"
              placeholder="/reports/market-brief"
            />
          </label>
          <label className="product-field-description">
            <span>Product description</span>
            <input
              name="description"
              required
              maxLength={500}
              placeholder="Generate a source-backed market brief"
            />
          </label>
          <label>
            <span>Price in atomic units</span>
            <input
              name="amount"
              required
              inputMode="numeric"
              pattern="[0-9]+"
              placeholder="35000000"
            />
          </label>
          <label>
            <span>Asset</span>
            <input name="asset" required defaultValue="USDC" />
          </label>
          <label>
            <span>Network</span>
            <input name="network" required defaultValue="eip155:84532" />
          </label>
          <label className="product-field-route">
            <span>Payment destination</span>
            <input
              name="payTo"
              required
              placeholder="0x verified wallet address"
            />
          </label>
          <input name="mimeType" type="hidden" value="application/json" />
          <input name="upstreamTimeoutSeconds" type="hidden" value="20" />
          <Button type="submit" disabled={pendingAction === "create"}>
            {pendingAction === "create" ? (
              <LoaderCircle className="animate-spin" aria-hidden="true" />
            ) : (
              <Plus aria-hidden="true" />
            )}
            Create draft
          </Button>
        </form>
      </section>

      <div className="product-management-grid">
        <section className="product-route-list" aria-labelledby="route-list-title">
          <div className="product-panel-heading">
            <div>
              <h2 id="route-list-title">Catalog</h2>
              <p>{routes.length} configured routes</p>
            </div>
          </div>
          {routes.length === 0 ? (
            <div className="product-empty-state">
              <p>No product routes yet.</p>
              <span>Create a draft above or connect your coding agent.</span>
            </div>
          ) : (
            <div className="product-route-items">
              {routes.map((route) => (
                <button
                  key={route.routeId}
                  type="button"
                  className="product-route-item"
                  data-selected={route.routeId === selectedRouteId}
                  onClick={() => {
                    setSelectedRouteId(route.routeId);
                    setValidation(null);
                  }}
                >
                  <span className="product-route-method">{route.method}</span>
                  <span className="product-route-copy">
                    <strong>{route.pathPattern}</strong>
                    <small>{route.description}</small>
                  </span>
                  <span className="product-route-price">
                    {formatAtomicPrice(route.amount, route.asset)}
                  </span>
                  <RouteStatusBadge route={route} />
                </button>
              ))}
            </div>
          )}
        </section>

        <section className="product-inspector" aria-labelledby="route-detail-title">
          {selectedRoute ? (
            <RouteInspector
              actionsPending={pendingAction}
              auditEvents={initialSnapshot.auditEvents}
              confirmEmergencyDisable={confirmEmergencyDisable}
              route={selectedRoute}
              validation={validation}
              onArchive={() => runLifecycleAction("archive")}
              onCancelEmergencyDisable={() =>
                setConfirmEmergencyDisable(false)
              }
              onConfirmEmergencyDisable={() =>
                runLifecycleAction("emergency")
              }
              onPause={() => runLifecycleAction("pause")}
              onPublish={() => runLifecycleAction("publish")}
              onRequestEmergencyDisable={() =>
                setConfirmEmergencyDisable(true)
              }
              onSubmitPrice={submitPrice}
              onValidate={validateSelectedRoute}
            />
          ) : (
            <div className="product-empty-state product-empty-inspector">
              <p id="route-detail-title">Select a product route</p>
              <span>Configuration, controls, and history appear here.</span>
            </div>
          )}
        </section>
      </div>
    </div>
  );
}

type RouteStatusBadgeProps = {
  route: PaidRoute;
};

// RouteStatusBadge renders one consistent lifecycle label.
function RouteStatusBadge({ route }: RouteStatusBadgeProps) {
  return (
    <span className="route-status-badge" data-status={route.lifecycleStatus}>
      {routeLifecycleLabel(route.lifecycleStatus)}
    </span>
  );
}

type RouteInspectorProps = {
  actionsPending: string | null;
  auditEvents: RouteAuditEvent[];
  confirmEmergencyDisable: boolean;
  route: PaidRoute;
  validation: RouteValidationResult | null;
  onArchive: () => void;
  onCancelEmergencyDisable: () => void;
  onConfirmEmergencyDisable: () => void;
  onPause: () => void;
  onPublish: () => void;
  onRequestEmergencyDisable: () => void;
  onSubmitPrice: (event: React.FormEvent<HTMLFormElement>) => void;
  onValidate: () => void;
};

// RouteInspector presents one route's controls, validation, and audit-backed history.
function RouteInspector({
  actionsPending,
  auditEvents,
  confirmEmergencyDisable,
  route,
  validation,
  onArchive,
  onCancelEmergencyDisable,
  onConfirmEmergencyDisable,
  onPause,
  onPublish,
  onRequestEmergencyDisable,
  onSubmitPrice,
  onValidate,
}: RouteInspectorProps) {
  const routeHistory = auditEvents.filter(
    (event) => event.targetId === route.routeId,
  );
  const canPublish = ["draft", "paused", "emergency_disabled"].includes(
    route.lifecycleStatus,
  );
  const canArchive = ["draft", "paused", "emergency_disabled"].includes(
    route.lifecycleStatus,
  );

  return (
    <>
      <div className="product-detail-heading">
        <div>
          <span>{route.method}</span>
          <h2 id="route-detail-title">{route.pathPattern}</h2>
          <p>{route.description}</p>
        </div>
        <div>
          <RouteStatusBadge route={route} />
          <span>Version {route.version}</span>
        </div>
      </div>

      <div className="product-detail-section">
        <div className="product-section-title">
          <div>
            <h3>Price</h3>
            <p>Applies only to purchase intents created after this update.</p>
          </div>
          <strong>{formatAtomicPrice(route.amount, route.asset)}</strong>
        </div>
        <form
          aria-label="Update route price"
          className="product-price-form"
          onSubmit={onSubmitPrice}
        >
          <label>
            <span>Atomic amount</span>
            <input
              key={`${route.routeId}-${route.version}`}
              name="amount"
              required
              inputMode="numeric"
              pattern="[0-9]+"
              defaultValue={route.amount}
              disabled={route.lifecycleStatus === "archived"}
            />
          </label>
          <Button
            type="submit"
            variant="outline"
            disabled={
              route.lifecycleStatus === "archived" ||
              actionsPending === "price"
            }
          >
            Update price
          </Button>
        </form>
      </div>

      <div className="product-detail-section">
        <div className="product-section-title">
          <div>
            <h3>Publication checks</h3>
            <p>Checks are refreshed from the backend and never cached.</p>
          </div>
          <Button
            type="button"
            variant="outline"
            onClick={onValidate}
            disabled={
              route.lifecycleStatus === "archived" ||
              actionsPending === "validate"
            }
          >
            <RefreshCw aria-hidden="true" />
            Validate route
          </Button>
        </div>
        {validation ? (
          <div className="product-validation" data-valid={validation.valid}>
            <strong>
              {validation.valid ? "Ready to publish" : "Needs attention"}
            </strong>
            <ul aria-label="Publication checks">
              {validation.checks.map((check) => (
                <li key={check.name}>
                  {check.passed ? (
                    <Check aria-hidden="true" />
                  ) : (
                    <AlertTriangle aria-hidden="true" />
                  )}
                  <span>{check.message}</span>
                  <small>{check.passed ? "Passed" : "Blocked"}</small>
                </li>
              ))}
            </ul>
          </div>
        ) : (
          <p className="product-muted-copy">
            Run validation before publishing or resuming this route.
          </p>
        )}
      </div>

      <div className="product-detail-section">
        <div className="product-section-title">
          <div>
            <h3>Route controls</h3>
            <p>Every action uses the version shown above.</p>
          </div>
        </div>
        <div className="product-control-row">
          {canPublish ? (
            <Button
              type="button"
              onClick={onPublish}
              disabled={actionsPending === "publish"}
            >
              <CirclePlay aria-hidden="true" />
              Publish route
            </Button>
          ) : null}
          {route.lifecycleStatus === "published" ? (
            <Button
              type="button"
              variant="outline"
              onClick={onPause}
              disabled={actionsPending === "pause"}
            >
              <CirclePause aria-hidden="true" />
              Pause route
            </Button>
          ) : null}
          {canArchive ? (
            <Button
              type="button"
              variant="outline"
              onClick={onArchive}
              disabled={actionsPending === "archive"}
            >
              <Archive aria-hidden="true" />
              Archive route
            </Button>
          ) : null}
          {route.lifecycleStatus === "published" ? (
            <AlertDialog
              open={confirmEmergencyDisable}
              onOpenChange={(open) => {
                if (open) {
                  onRequestEmergencyDisable();
                } else {
                  onCancelEmergencyDisable();
                }
              }}
            >
              <AlertDialogTrigger
                render={<Button type="button" variant="destructive" />}
              >
                <ShieldAlert aria-hidden="true" />
                Emergency disable route
              </AlertDialogTrigger>
              <AlertDialogContent>
                <AlertDialogHeader>
                  <AlertDialogMedia>
                    <ShieldAlert aria-hidden="true" />
                  </AlertDialogMedia>
                  <AlertDialogTitle>Disable this route now?</AlertDialogTitle>
                  <AlertDialogDescription>
                    New purchase attempts stop immediately. Existing payment
                    and evidence records remain unchanged.
                  </AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                  <AlertDialogCancel>Cancel</AlertDialogCancel>
                  <AlertDialogAction
                    variant="destructive"
                    onClick={onConfirmEmergencyDisable}
                    disabled={actionsPending === "emergency"}
                  >
                    Confirm emergency disable
                  </AlertDialogAction>
                </AlertDialogFooter>
              </AlertDialogContent>
            </AlertDialog>
          ) : null}
        </div>
      </div>

      <div className="product-detail-section">
        <div className="product-section-title">
          <div>
            <h3>Version history</h3>
            <p>Immutable seller audit events for this route.</p>
          </div>
        </div>
        <RouteHistory events={routeHistory} />
      </div>

    </>
  );
}

type RouteHistoryProps = {
  events: RouteAuditEvent[];
};

// RouteHistory translates route audit actions into concise seller-visible entries.
function RouteHistory({ events }: RouteHistoryProps) {
  if (events.length === 0) {
    return (
      <p className="product-muted-copy">
        No recorded changes for this route yet.
      </p>
    );
  }

  return (
    <ol className="product-history">
      {events.map((event) => (
        <li key={event.auditEventId}>
          <span aria-hidden="true" />
          <div>
            <strong>{auditActionLabel(event.action)}</strong>
            <small>
              {historyDateFormatter.format(new Date(event.occurredAt))} UTC ·{" "}
              {event.changedFields.length} fields changed
            </small>
          </div>
        </li>
      ))}
    </ol>
  );
}

// auditActionLabel converts fixed audit vocabulary into readable history copy.
function auditActionLabel(action: string): string {
  const labels: Record<string, string> = {
    "route.draft_created": "Draft created",
    "route.price_changed": "Price changed",
    "route.published": "Published route",
    "route.paused": "Paused route",
    "route.archived": "Archived route",
    "route.emergency_disabled": "Emergency disabled route",
  };
  return labels[action] ?? action;
}
