"use client";

import {
  AlertTriangle,
  Archive,
  Check,
  CircleCheck,
  CirclePause,
  CirclePlay,
  LoaderCircle,
  Package,
  Plus,
  RefreshCw,
  ShieldAlert,
  X,
} from "lucide-react";
import Link from "next/link";
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
  canonicalOrigin?: string;
  initialSnapshot: ProductRouteSnapshot;
  sellerSlug?: string;
};

// ProductRouteWorkspace gives sellers one controlled surface for route configuration.
export function ProductRouteWorkspace({
  actions,
  canonicalOrigin,
  initialSnapshot,
  sellerSlug,
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
  const [isCreateOpen, setIsCreateOpen] = useState(
    initialSnapshot.routes.length === 0,
  );
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
  const isBusy = pendingAction !== null;

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
    setIsCreateOpen(false);
    setStatusMessage("Draft created. Validate it before publishing.");
    form.reset();
  }

  // submitPrice updates the authoritative quote for future purchase intents.
  async function submitPrice(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!selectedRoute) return;

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
    if (!selectedRoute) return;

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
    if (!selectedRoute) return;
    if (
      actionName === "publish" &&
      (!validation?.valid || validation.version !== selectedRoute.version)
    ) {
      setErrorMessage(
        "Validate the current product version before publishing.",
      );
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
        ? await actions.publishRoute({
            ...input,
            contractHash: validation!.contractHash,
          })
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
      aria-busy={isBusy}
    >
      <header className="product-page-header">
        <div className="product-page-intro">
          <p className="dashboard-eyebrow">Storefront catalog</p>
          <h1>Products</h1>
          <p>Control what buyers see, pay, and receive.</p>
        </div>
        <div className="product-header-actions">
          <div className="product-counts" aria-label="Product status summary">
            <span>{routeCounts.published} published</span>
            <span>{routeCounts.draft} draft</span>
            <span>{routeCounts.attention} attention</span>
          </div>
          <Button
            type="button"
            variant={isCreateOpen ? "outline" : "default"}
            aria-expanded={isCreateOpen}
            aria-controls="new-product-panel"
            onClick={() => setIsCreateOpen((open) => !open)}
          >
            {isCreateOpen ? (
              <X aria-hidden="true" />
            ) : (
              <Plus aria-hidden="true" />
            )}
            {isCreateOpen ? "Close new product" : "New product"}
          </Button>
        </div>
      </header>

      {errorMessage ? (
        <section className="product-error-state" role="alert">
          <div>
            <span className="product-state-icon">
              <AlertTriangle aria-hidden="true" />
            </span>
            <div>
              <h2>Catalog unavailable</h2>
              <p>{errorMessage}</p>
            </div>
          </div>
          <Button
            type="button"
            variant="outline"
            onClick={() => window.location.reload()}
          >
            <RefreshCw aria-hidden="true" />
            Retry catalog
          </Button>
        </section>
      ) : null}
      <p className="dashboard-status" aria-live="polite">
        {statusMessage}
      </p>

      {isCreateOpen ? (
        <CreateProductPanel
          isFirstProduct={routes.length === 0}
          isPending={pendingAction === "create"}
          onSubmit={submitDraft}
        />
      ) : null}

      <div className="product-management-grid">
        <aside className="product-route-list" aria-label="Product catalog">
          <div className="product-catalog-heading">
            <div>
              <p className="product-section-kicker">Catalog</p>
              <h2 id="route-list-title">Your products</h2>
            </div>
            <span>{routes.length}</span>
          </div>
          {routes.length === 0 ? (
            <div className="product-empty-state product-empty-catalog">
              <span className="product-state-icon">
                <Package aria-hidden="true" />
              </span>
              <h2>Nothing is published yet</h2>
              <p>Your first draft is ready to be configured above.</p>
            </div>
          ) : (
            <div className="product-route-items">
              {routes.map((route) => (
                <button
                  key={route.routeId}
                  type="button"
                  className="product-route-item"
                  data-selected={route.routeId === selectedRouteId}
                  aria-pressed={route.routeId === selectedRouteId}
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

        <section className="product-inspector" aria-label="Product editor">
          {selectedRoute ? (
            <RouteInspector
              actionsPending={pendingAction}
              auditEvents={initialSnapshot.auditEvents}
              confirmEmergencyDisable={confirmEmergencyDisable}
              canonicalOrigin={canonicalOrigin}
              route={selectedRoute}
              sellerSlug={sellerSlug}
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
              <span className="product-state-icon">
                <Package aria-hidden="true" />
              </span>
              <h2 id="route-detail-title">Product editor ready</h2>
              <p>
                Add the buyer-facing offer first. Technical delivery remains
                tucked away.
              </p>
            </div>
          )}
        </section>
      </div>
    </div>
  );
}

type CreateProductPanelProps = {
  isFirstProduct: boolean;
  isPending: boolean;
  onSubmit: (event: React.FormEvent<HTMLFormElement>) => void;
};

function CreateProductPanel({
  isFirstProduct,
  isPending,
  onSubmit,
}: CreateProductPanelProps) {
  return (
    <section
      id="new-product-panel"
      className="product-create-panel"
      aria-labelledby="new-product-title"
    >
      <div className="product-panel-heading">
        <span className="product-panel-index">01</span>
        <div>
          <p className="product-section-kicker">New catalog entry</p>
          <h2 id="new-product-title">
            {isFirstProduct ? "Create your first product" : "Create a product"}
          </h2>
          <p>
            Start with what buyers need. Publish only after validation passes.
          </p>
        </div>
      </div>
      <form
        aria-label="Create product draft"
        className="product-create-form"
        onSubmit={onSubmit}
      >
        <p className="product-testnet-notice" role="note">
          Testnet only · Base Sepolia USDC · no real-money production
        </p>
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
          <input name="asset" required readOnly value="USDC" />
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
                    readOnly
                    value="eip155:84532"
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
        <Button type="submit" disabled={isPending}>
          {isPending ? (
            <LoaderCircle className="animate-spin" aria-hidden="true" />
          ) : (
            <Plus aria-hidden="true" />
          )}
          {isPending ? "Creating draft…" : "Create draft"}
        </Button>
      </form>
    </section>
  );
}

function RouteStatusBadge({ route }: { route: PaidRoute }) {
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
  canonicalOrigin?: string;
  route: PaidRoute;
  sellerSlug?: string;
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

function RouteInspector({
  actionsPending,
  auditEvents,
  confirmEmergencyDisable,
  canonicalOrigin,
  route,
  sellerSlug,
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
  const isBusy = actionsPending !== null;
  const validationState = validation
    ? validation.valid
      ? "Verified"
      : "Blocked"
    : "Not checked";
  const availabilityState =
    route.lifecycleStatus === "published" ? "Live" : "Offline";
  const canonicalURL =
    route.lifecycleStatus === "published" && canonicalOrigin && sellerSlug
      ? `${canonicalOrigin.replace(/\/$/, "")}/store/${encodeURIComponent(sellerSlug)}/products/${encodeURIComponent(route.productSlug)}`
      : null;

  return (
    <>
      <header className="product-detail-heading">
        <div>
          <p className="product-section-kicker">Selected product</p>
          <h2 id="route-detail-title">{route.displayName}</h2>
          <p>{route.description}</p>
          {canonicalURL ? (
            <Link href={canonicalURL} target="_blank" rel="noreferrer">
              Open canonical storefront product
            </Link>
          ) : null}
        </div>
        <div className="product-detail-meta">
          <RouteStatusBadge route={route} />
          <span>Version {route.version}</span>
        </div>
      </header>

      <ol className="product-readiness" aria-label="Product readiness">
        <ReadinessStep label="Catalog record" state="Registered" complete />
        <ReadinessStep
          label="Publication checks"
          state={validationState}
          complete={validation?.valid === true}
          blocked={validation?.valid === false}
        />
        <ReadinessStep
          label="Storefront availability"
          state={availabilityState}
          complete={route.lifecycleStatus === "published"}
          blocked={route.lifecycleStatus === "emergency_disabled"}
        />
      </ol>

      <div className="product-editor-grid">
        <section className="product-detail-section product-price-section">
          <div className="product-section-title">
            <div>
              <p className="product-section-kicker">Commercial</p>
              <h3>Price</h3>
              <p>Used for purchase intents created after the update.</p>
            </div>
            <strong>{formatAtomicPrice(route.amount, route.asset)}</strong>
          </div>
          <dl className="product-customer-details">
            <Definition
              label="Storefront path"
              value={`/products/${route.productSlug}`}
            />
            <Definition label="Settlement destination" value={route.payTo} />
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
                disabled={route.lifecycleStatus === "archived" || isBusy}
              />
            </label>
            <Button
              type="submit"
              variant="outline"
              disabled={route.lifecycleStatus === "archived" || isBusy}
            >
              {actionsPending === "price" ? (
                <LoaderCircle className="animate-spin" aria-hidden="true" />
              ) : null}
              {actionsPending === "price" ? "Updating…" : "Update price"}
            </Button>
          </form>
        </section>

        <section className="product-detail-section product-publication-section">
          <div className="product-section-title">
            <div>
              <p className="product-section-kicker">Readiness</p>
              <h3>Publication checks</h3>
              <p>Refresh the authoritative checks before going live.</p>
            </div>
            <Button
              type="button"
              variant="outline"
              onClick={onValidate}
              disabled={route.lifecycleStatus === "archived" || isBusy}
            >
              {actionsPending === "validate" ? (
                <LoaderCircle className="animate-spin" aria-hidden="true" />
              ) : (
                <RefreshCw aria-hidden="true" />
              )}
              {actionsPending === "validate" ? "Checking…" : "Validate product"}
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
            <div className="product-inline-empty">
              <CircleCheck aria-hidden="true" />
              <p>No current validation result.</p>
              <span>Run checks after changing price or delivery settings.</span>
            </div>
          )}
        </section>
      </div>

      <section className="product-detail-section product-controls-section">
        <div className="product-section-title">
          <div>
            <p className="product-section-kicker">Availability</p>
            <h3>Product controls</h3>
            <p>
              Change whether new buyers can discover and purchase this product.
            </p>
          </div>
        </div>
        <div className="product-control-row">
          {canPublish ? (
            <Button type="button" onClick={onPublish} disabled={isBusy}>
              {actionsPending === "publish" ? (
                <LoaderCircle className="animate-spin" aria-hidden="true" />
              ) : (
                <CirclePlay aria-hidden="true" />
              )}
              {actionsPending === "publish" ? "Publishing…" : "Publish product"}
            </Button>
          ) : null}
          {route.lifecycleStatus === "published" ? (
            <Button
              type="button"
              variant="outline"
              onClick={onPause}
              disabled={isBusy}
            >
              {actionsPending === "pause" ? (
                <LoaderCircle className="animate-spin" aria-hidden="true" />
              ) : (
                <CirclePause aria-hidden="true" />
              )}
              {actionsPending === "pause" ? "Pausing…" : "Pause product"}
            </Button>
          ) : null}
          {canArchive ? (
            <Button
              type="button"
              variant="outline"
              onClick={onArchive}
              disabled={isBusy}
            >
              <Archive aria-hidden="true" />
              {actionsPending === "archive" ? "Archiving…" : "Archive product"}
            </Button>
          ) : null}
          {route.lifecycleStatus === "published" ? (
            <AlertDialog
              open={confirmEmergencyDisable}
              onOpenChange={(open) =>
                open ? onRequestEmergencyDisable() : onCancelEmergencyDisable()
              }
            >
              <AlertDialogTrigger
                render={
                  <Button
                    type="button"
                    variant="destructive"
                    disabled={isBusy}
                  />
                }
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
      </section>

      <div className="product-lower-grid">
        <section className="product-detail-section">
          <div className="product-section-title">
            <div>
              <p className="product-section-kicker">Audit trail</p>
              <h3>Version history</h3>
              <p>Recorded publication and pricing changes.</p>
            </div>
          </div>
          <RouteHistory events={routeHistory} />
        </section>

        <section className="product-detail-section product-technical-section">
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
        </section>
      </div>
    </>
  );
}

type ReadinessStepProps = {
  label: string;
  state: string;
  complete?: boolean;
  blocked?: boolean;
};

function ReadinessStep({
  label,
  state,
  complete = false,
  blocked = false,
}: ReadinessStepProps) {
  return (
    <li data-state={blocked ? "blocked" : complete ? "complete" : "pending"}>
      <span className="product-readiness-marker" aria-hidden="true">
        {complete ? <Check /> : blocked ? <AlertTriangle /> : null}
      </span>
      <span>
        <small>{label}</small>
        <strong>{state}</strong>
      </span>
    </li>
  );
}

function RouteHistory({ events }: { events: RouteAuditEvent[] }) {
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
