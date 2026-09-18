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
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from "@/components/ui/accordion";
import { Button } from "@/components/ui/button";
import type {
  PaidRoute,
  ProductRouteActions,
  ProductRouteSnapshot,
  RouteAuditEvent,
  RouteValidationResult,
} from "@/features/products/model";
import { routeLifecycleLabel } from "@/features/products/model";
import {
  decimalToAtomicUnits,
  formatAtomicPrice,
  formatAtomicUnits,
} from "@/lib/money";
import styles from "./product-route-workspace.module.css";

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
  const [confirmEmergencyDisable, setConfirmEmergencyDisable] = useState(false);
  const selectedRoute =
    routes.find((route) => route.routeId === selectedRouteId) ?? null;
  const routeCounts = useMemo(
    () => ({
      published: routes.filter((route) => route.lifecycleStatus === "published")
        .length,
      draft: routes.filter((route) => route.lifecycleStatus === "draft").length,
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
    let amount: string;
    try {
      amount = decimalToAtomicUnits(String(fields.get("price") ?? ""));
    } catch (error) {
      setPendingAction(null);
      setErrorMessage(
        error instanceof Error ? error.message : "Enter a valid decimal price.",
      );
      return;
    }
    const result = await actions.createDraft({
      sellerId: initialSnapshot.sellerId,
      displayName: String(fields.get("displayName") ?? "").trim(),
      productSlug: String(fields.get("productSlug") ?? "").trim(),
      method: String(fields.get("method")) === "GET" ? "GET" : "POST",
      pathPattern: String(fields.get("pathPattern") ?? "").trim(),
      description: String(fields.get("description") ?? "").trim(),
      mimeType: String(fields.get("mimeType") ?? "").trim(),
      amount,
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
    let amount: string;
    try {
      amount = decimalToAtomicUnits(String(fields.get("price") ?? ""));
    } catch (error) {
      setPendingAction(null);
      setErrorMessage(
        error instanceof Error ? error.message : "Enter a valid decimal price.",
      );
      return;
    }
    const result = await actions.updatePrice({
      sellerId: initialSnapshot.sellerId,
      routeId: selectedRoute.routeId,
      expectedVersion: selectedRoute.version,
      amount,
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
      `Product is now ${routeLifecycleLabel(result.value.lifecycleStatus).toLowerCase()}.`,
    );
  }

  return (
    <div
      className={styles.workspace}
      role="region"
      aria-label="Catalog workspace"
    >
      <header className="product-page-header">
        <div>
          <p className="dashboard-eyebrow">Storefront catalog</p>
          <h1>Products</h1>
          <p>
            Name, price, review, and publish what buyers can purchase from your
            service. Technical delivery settings stay available when you need
            them.
          </p>
        </div>
        <div className="product-counts" aria-label="Product status summary">
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

      <section
        className="product-create-panel"
        aria-labelledby="new-product-title"
      >
        <div className="product-panel-heading">
          <div>
            <span className="product-panel-icon">
              <Plus aria-hidden="true" />
            </span>
            <div>
              <h2 id="new-product-title">Add a product</h2>
              <p>
                Save a draft, review its checks, then publish it when ready.
              </p>
            </div>
          </div>
        </div>
        <form
          aria-label="Create product draft"
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
            <span>Product URL name</span>
            <input
              name="productSlug"
              required
              minLength={3}
              maxLength={80}
              pattern="[A-Za-z0-9][A-Za-z0-9 _-]*[A-Za-z0-9]"
              placeholder="board-ready-market-report"
            />
          </label>
          <label className="product-field-description">
            <span>What buyers receive</span>
            <input
              name="description"
              required
              maxLength={500}
              placeholder="Generate a source-backed market brief"
            />
          </label>
          <label>
            <span>Price</span>
            <input
              name="price"
              required
              inputMode="decimal"
              pattern="[0-9]+(?:\.[0-9]{1,6})?"
              placeholder="35.00"
            />
          </label>
          <label>
            <span>Price asset</span>
            <input name="asset" required defaultValue="USDC" />
          </label>
          <label className="product-field-route">
            <span>Verified payment destination</span>
            <input
              name="payTo"
              required
              placeholder="0x verified wallet address"
            />
          </label>
          <Accordion className="product-advanced-fields">
            <AccordionItem value="technical-setup">
              <AccordionTrigger>Advanced product setup</AccordionTrigger>
              <AccordionContent>
                <div className="product-advanced-grid">
                  <label>
                    <span>HTTP method</span>
                    <select name="method" defaultValue="POST">
                      <option value="POST">POST</option>
                      <option value="GET">GET</option>
                    </select>
                  </label>
                  <label>
                    <span>API path</span>
                    <input
                      name="pathPattern"
                      required
                      pattern="/[A-Za-z0-9/_-]+"
                      placeholder="/reports/market-brief"
                    />
                  </label>
                  <label>
                    <span>Output MIME type</span>
                    <input
                      name="mimeType"
                      required
                      defaultValue="application/json"
                    />
                  </label>
                  <label>
                    <span>Payment network</span>
                    <input
                      name="network"
                      required
                      defaultValue="eip155:84532"
                    />
                  </label>
                  <label>
                    <span>Service timeout in seconds</span>
                    <input
                      name="upstreamTimeoutSeconds"
                      required
                      inputMode="numeric"
                      pattern="[0-9]+"
                      defaultValue="20"
                    />
                  </label>
                </div>
              </AccordionContent>
            </AccordionItem>
          </Accordion>
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
        <aside
          className="product-route-list"
          aria-label="Product catalog"
        >
          <div className="product-panel-heading">
            <div>
              <h2 id="route-list-title">Storefront catalog</h2>
              <p>{routes.length} configured products</p>
            </div>
          </div>
          {routes.length === 0 ? (
            <div className="product-empty-state">
              <p>No products yet.</p>
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
                  <span className="product-route-copy">
                    <strong>{route.displayName}</strong>
                    <small>/products/{route.productSlug}</small>
                  </span>
                  <span className="product-route-price">
                    {formatAtomicPrice(route.amount, route.asset)}
                  </span>
                  <RouteStatusBadge route={route} />
                </button>
              ))}
            </div>
          )}
        </aside>

        <section
          className="product-inspector"
          aria-label="Product editor"
        >
          {selectedRoute ? (
            <RouteInspector
              actionsPending={pendingAction}
              auditEvents={initialSnapshot.auditEvents}
              confirmEmergencyDisable={confirmEmergencyDisable}
              route={selectedRoute}
              validation={validation}
              onArchive={() => runLifecycleAction("archive")}
              onCancelEmergencyDisable={() => setConfirmEmergencyDisable(false)}
              onConfirmEmergencyDisable={() => runLifecycleAction("emergency")}
              onPause={() => runLifecycleAction("pause")}
              onPublish={() => runLifecycleAction("publish")}
              onRequestEmergencyDisable={() => setConfirmEmergencyDisable(true)}
              onSubmitPrice={submitPrice}
              onValidate={validateSelectedRoute}
            />
          ) : (
            <div className="product-empty-state product-empty-inspector">
              <p id="route-detail-title">Select a product</p>
              <span>Price, publication controls, and history appear here.</span>
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
          <span>Product</span>
          <h2 id="route-detail-title">{route.displayName}</h2>
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
        <dl className="product-customer-details">
          <div>
            <dt>Product URL</dt>
            <dd>/products/{route.productSlug}</dd>
          </div>
          <div>
            <dt>Verified payment destination</dt>
            <dd>{route.payTo}</dd>
          </div>
        </dl>
        <form
          aria-label="Update product price"
          className="product-price-form"
          onSubmit={onSubmitPrice}
        >
          <label>
            <span>Price</span>
            <input
              key={`${route.routeId}-${route.version}`}
              name="price"
              required
              inputMode="decimal"
              pattern="[0-9]+(?:\.[0-9]{1,6})?"
              defaultValue={formatAtomicUnits(route.amount)}
              disabled={route.lifecycleStatus === "archived"}
            />
          </label>
          <Button
            type="submit"
            variant="outline"
            disabled={
              route.lifecycleStatus === "archived" || actionsPending === "price"
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
            <p>Checks are refreshed before this product can be published.</p>
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
            Validate product
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
            Run validation before publishing or resuming this product.
          </p>
        )}
      </div>

      <div className="product-detail-section">
        <div className="product-section-title">
          <div>
            <h3>Product controls</h3>
            <p>Publish, pause, or retire this storefront product.</p>
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
              Publish product
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
              Pause product
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
              Archive product
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
                Emergency disable product
              </AlertDialogTrigger>
              <AlertDialogContent>
                <AlertDialogHeader>
                  <AlertDialogMedia>
                    <ShieldAlert aria-hidden="true" />
                  </AlertDialogMedia>
                  <AlertDialogTitle>Disable this product now?</AlertDialogTitle>
                  <AlertDialogDescription>
                    New purchase attempts stop immediately. Existing payment and
                    evidence records remain unchanged.
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
            <p>Recorded publication and pricing changes for this product.</p>
          </div>
        </div>
        <RouteHistory events={routeHistory} />
      </div>

      <div className="product-detail-section">
        <Accordion className="product-technical-accordion">
          <AccordionItem value="technical-details">
            <AccordionTrigger>Advanced technical details</AccordionTrigger>
            <AccordionContent>
              <dl className="product-technical-details">
                <Definition label="Route ID" value={route.routeId} />
                <Definition label="API path" value={route.pathPattern} />
                <Definition label="HTTP method" value={route.method} />
                <Definition label="Output MIME type" value={route.mimeType} />
                <Definition label="Payment network" value={route.network} />
                <Definition
                  label="Service timeout"
                  value={`${route.upstreamTimeoutSeconds} seconds`}
                />
                <Definition label="Atomic amount" value={route.amount} />
              </dl>
            </AccordionContent>
          </AccordionItem>
        </Accordion>
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
        No recorded changes for this product yet.
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
    "route.published": "Published product",
    "route.paused": "Paused product",
    "route.archived": "Archived product",
    "route.emergency_disabled": "Emergency disabled product",
  };
  return labels[action] ?? action;
}

function Definition({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <dt>{label}</dt>
      <dd>{value}</dd>
    </div>
  );
}
