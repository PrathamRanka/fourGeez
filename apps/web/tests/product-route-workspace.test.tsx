import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type {
  PaidRoute,
  ProductRouteActions,
  ProductRouteSnapshot,
} from "@/features/products/model";
import { ProductRouteWorkspace } from "@/features/products/view/product-route-workspace";

const sellerId = "sel_01ARZ3NDEKTSV4RRFFQ69G5FAV";

const publishedRoute: PaidRoute = {
  routeId: "rte_01ARZ3NDEKTSV4RRFFQ69G5FAW",
  sellerId,
  method: "POST",
  pathPattern: "/research",
  description: "Generate a source-backed market brief",
  mimeType: "application/json",
  amount: "35000000",
  asset: "USDC",
  network: "eip155:84532",
  payTo: "0x1111111111111111111111111111111111111111",
  approvalThresholdAmount: null,
  upstreamTimeoutSeconds: 20,
  lifecycleStatus: "published",
  enabled: true,
  createdAt: "2026-09-18T09:00:00Z",
  updatedAt: "2026-09-18T09:05:00Z",
  version: 2,
};

const initialSnapshot: ProductRouteSnapshot = {
  sellerId,
  routes: [publishedRoute],
  auditEvents: [
    {
      auditEventId: "aud_01ARZ3NDEKTSV4RRFFQ69G5FAX",
      sellerId,
      actorType: "seller_user",
      actorId: "seller-subject",
      action: "route.published",
      targetType: "paid_route",
      targetId: publishedRoute.routeId,
      outcome: "succeeded",
      requestId: "req_123",
      changedFields: ["lifecycleStatus", "enabled"],
      occurredAt: "2026-09-18T09:05:00Z",
    },
  ],
};

const draftRoute: PaidRoute = {
  ...publishedRoute,
  lifecycleStatus: "draft",
  enabled: false,
  version: 1,
};

// createActions returns deterministic product operations for visible behavior tests.
function createActions(): ProductRouteActions {
  return {
    createDraft: vi.fn().mockResolvedValue({
      ok: true,
      value: {
        ...publishedRoute,
        routeId: "rte_01ARZ3NDEKTSV4RRFFQ69G5FAY",
        pathPattern: "/summaries",
        description: "Summarize a supplied document",
        amount: "12000000",
        lifecycleStatus: "draft",
        enabled: false,
        version: 1,
      },
    }),
    updatePrice: vi.fn().mockResolvedValue({
      ok: true,
      value: { ...publishedRoute, amount: "40000000", version: 3 },
    }),
    validateRoute: vi.fn().mockResolvedValue({
      ok: true,
      value: {
        sellerId,
        routeId: publishedRoute.routeId,
        valid: true,
        version: publishedRoute.version,
        checks: [
          {
            name: "seller_active",
            passed: true,
            message: "seller must be active",
          },
          {
            name: "route_configuration_valid",
            passed: true,
            message: "stored route must satisfy current validation",
          },
        ],
      },
    }),
    publishRoute: vi.fn().mockResolvedValue({
      ok: true,
      value: publishedRoute,
    }),
    pauseRoute: vi.fn().mockResolvedValue({
      ok: true,
      value: {
        ...publishedRoute,
        lifecycleStatus: "paused",
        enabled: false,
        version: 3,
      },
    }),
    archiveRoute: vi.fn().mockResolvedValue({
      ok: true,
      value: {
        ...publishedRoute,
        lifecycleStatus: "archived",
        enabled: false,
        version: 3,
      },
    }),
    emergencyDisableRoute: vi.fn().mockResolvedValue({
      ok: true,
      value: {
        ...publishedRoute,
        lifecycleStatus: "emergency_disabled",
        enabled: false,
        version: 3,
      },
    }),
  };
}

describe("product route workspace", () => {
  it("shows route state, pricing, validation, and version history", async () => {
    const actions = createActions();
    render(
      <ProductRouteWorkspace
        actions={actions}
        initialSnapshot={initialSnapshot}
      />,
    );

    expect(
      screen.getByRole("heading", { name: "Product routes" }),
    ).toBeVisible();
    expect(screen.getByText("1 published")).toBeVisible();
    expect(screen.getAllByText("35 USDC")[0]).toBeVisible();
    expect(screen.getByText("Version 2")).toBeVisible();
    expect(screen.getAllByText("Published")[0]).toBeVisible();

    fireEvent.click(screen.getByRole("button", { name: "Validate route" }));

    expect(actions.validateRoute).toHaveBeenCalledWith({
      sellerId,
      routeId: publishedRoute.routeId,
    });
    expect(await screen.findByText("Ready to publish")).toBeVisible();
    const validationList = screen.getByRole("list", {
      name: "Publication checks",
    });
    expect(within(validationList).getAllByText("Passed")).toHaveLength(2);
    expect(screen.getByText("Published route")).toBeVisible();
  });

  it("creates a draft and pauses a published route through real actions", async () => {
    const actions = createActions();
    render(
      <ProductRouteWorkspace
        actions={actions}
        initialSnapshot={initialSnapshot}
      />,
    );

    fireEvent.change(screen.getByLabelText("Route path"), {
      target: { value: "/summaries" },
    });
    fireEvent.change(screen.getByLabelText("Product description"), {
      target: { value: "Summarize a supplied document" },
    });
    fireEvent.change(screen.getByLabelText("Price in atomic units"), {
      target: { value: "12000000" },
    });
    fireEvent.submit(screen.getByRole("form", { name: "Create route draft" }));

    await waitFor(() => expect(actions.createDraft).toHaveBeenCalledTimes(1));
    expect(screen.getByText("1 draft")).toBeVisible();
    expect(
      await screen.findByRole("heading", { name: "/summaries" }),
    ).toBeVisible();

    fireEvent.click(screen.getByText("/research"));
    fireEvent.click(screen.getByRole("button", { name: "Pause route" }));
    await waitFor(() => expect(actions.pauseRoute).toHaveBeenCalledTimes(1));
    expect((await screen.findAllByText("Paused"))[0]).toBeVisible();
  });

  it("updates a draft price, publishes it, and uses the returned route version", async () => {
    const actions = createActions();
    vi.mocked(actions.updatePrice).mockResolvedValue({
      ok: true,
      value: { ...draftRoute, amount: "40000000", version: 2 },
    });
    vi.mocked(actions.publishRoute).mockResolvedValue({
      ok: true,
      value: {
        ...draftRoute,
        amount: "40000000",
        lifecycleStatus: "published",
        enabled: true,
        version: 3,
      },
    });

    render(
      <ProductRouteWorkspace
        actions={actions}
        initialSnapshot={{ ...initialSnapshot, routes: [draftRoute] }}
      />,
    );

    fireEvent.change(screen.getByLabelText("Atomic amount"), {
      target: { value: "40000000" },
    });
    fireEvent.submit(screen.getByLabelText("Update route price"));

    await waitFor(() =>
      expect(actions.updatePrice).toHaveBeenCalledWith({
        sellerId,
        routeId: draftRoute.routeId,
        expectedVersion: 1,
        amount: "40000000",
      }),
    );
    expect(await screen.findByText("Version 2")).toBeVisible();

    fireEvent.click(screen.getByRole("button", { name: "Publish route" }));
    await waitFor(() =>
      expect(actions.publishRoute).toHaveBeenCalledWith({
        sellerId,
        routeId: draftRoute.routeId,
        expectedVersion: 2,
      }),
    );
    expect((await screen.findAllByText("Published"))[0]).toBeVisible();
    expect(screen.getByText("Version 3")).toBeVisible();
  });

  it("archives a stopped route and disables further edits", async () => {
    const actions = createActions();
    const pausedRoute: PaidRoute = {
      ...publishedRoute,
      lifecycleStatus: "paused",
      enabled: false,
      version: 4,
    };
    vi.mocked(actions.archiveRoute).mockResolvedValue({
      ok: true,
      value: {
        ...pausedRoute,
        lifecycleStatus: "archived",
        version: 5,
      },
    });

    render(
      <ProductRouteWorkspace
        actions={actions}
        initialSnapshot={{ ...initialSnapshot, routes: [pausedRoute] }}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Archive route" }));

    await waitFor(() =>
      expect(actions.archiveRoute).toHaveBeenCalledWith({
        sellerId,
        routeId: pausedRoute.routeId,
        expectedVersion: 4,
      }),
    );
    expect((await screen.findAllByText("Archived"))[0]).toBeVisible();
    expect(screen.getByRole("button", { name: "Update price" })).toBeDisabled();
    expect(
      screen.getByRole("button", { name: "Validate route" }),
    ).toBeDisabled();
  });

  it("requires confirmation before emergency disable", async () => {
    const actions = createActions();
    render(
      <ProductRouteWorkspace
        actions={actions}
        initialSnapshot={initialSnapshot}
      />,
    );

    fireEvent.click(
      screen.getByRole("button", { name: "Emergency disable route" }),
    );
    expect(
      screen.getByRole("heading", { name: "Disable this route now?" }),
    ).toBeVisible();
    expect(actions.emergencyDisableRoute).not.toHaveBeenCalled();

    fireEvent.click(
      screen.getByRole("button", { name: "Confirm emergency disable" }),
    );
    await waitFor(() =>
      expect(actions.emergencyDisableRoute).toHaveBeenCalledTimes(1),
    );
    expect(
      (await screen.findAllByText("Emergency disabled"))[0],
    ).toBeVisible();
  });
});
