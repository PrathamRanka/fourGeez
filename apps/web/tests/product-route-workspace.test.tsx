import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
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
  displayName: "Research Report",
  productSlug: "research-report",
  method: "POST",
  pathPattern: "/research",
  description: "Generate a source-backed market brief",
  mimeType: "application/json",
  inputSchema: {
    type: "object",
    properties: {},
    additionalProperties: false,
  },
  outputSchema: {
    type: "object",
    properties: {},
    additionalProperties: false,
  },
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
        displayName: "Executive Summaries",
        productSlug: "executive-summaries",
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
        contractHash: "a".repeat(64),
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
  it("leads with product language and keeps route details advanced", async () => {
    const actions = createActions();
    render(
      <ProductRouteWorkspace
        actions={actions}
        initialSnapshot={initialSnapshot}
        canonicalOrigin="https://agentpay.example"
        sellerSlug="northstar-research"
      />,
    );

    expect(screen.getByRole("heading", { name: "Products" })).toBeVisible();
    expect(
      screen.getByRole("region", { name: "Catalog workspace" }),
    ).toBeVisible();
    expect(
      screen.getByRole("complementary", { name: "Product catalog" }),
    ).toBeVisible();
    expect(
      screen.getByRole("region", { name: "Product editor" }),
    ).toBeVisible();
    expect(screen.getByText("1 published")).toBeVisible();
    expect(screen.getAllByText("35 USDC")[0]).toBeVisible();
    expect(
      screen.getByRole("heading", { name: "Research Report" }),
    ).toBeVisible();
    expect(screen.getByText("Version 2")).toBeVisible();
    expect(screen.getAllByText("Published")[0]).toBeVisible();
    expect(
      screen.getByRole("link", { name: "Open canonical storefront product" }),
    ).toHaveAttribute(
      "href",
      "https://agentpay.example/store/northstar-research/products/research-report",
    );
    const readiness = screen.getByRole("list", {
      name: "Product readiness",
    });
    expect(within(readiness).getByText("Registered")).toBeVisible();
    expect(within(readiness).getByText("Not checked")).toBeVisible();
    expect(within(readiness).getByText("Live")).toBeVisible();
    expect(screen.queryByText("Route ID")).not.toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Advanced technical details" }),
    );
    expect(screen.getByText("Route ID")).toBeVisible();
    expect(screen.getAllByText(publishedRoute.routeId)[0]).toBeVisible();
    expect(screen.getByText("API path")).toBeVisible();
    expect(screen.getByText("/research")).toBeVisible();

    fireEvent.click(screen.getByRole("button", { name: "Validate product" }));

    expect(actions.validateRoute).toHaveBeenCalledWith({
      sellerId,
      routeId: publishedRoute.routeId,
    });
    expect(await screen.findByText("Ready to publish")).toBeVisible();
    const validationList = screen.getByRole("list", {
      name: "Publication checks",
    });
    expect(within(validationList).getAllByText("Passed")).toHaveLength(2);
    expect(within(readiness).getByText("Verified")).toBeVisible();
    expect(screen.getByText("Published product")).toBeVisible();
  });

  it("keeps product creation secondary until the seller asks for it", () => {
    const actions = createActions();
    render(
      <ProductRouteWorkspace
        actions={actions}
        initialSnapshot={initialSnapshot}
      />,
    );

    expect(
      screen.queryByRole("form", { name: "Create product draft" }),
    ).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "New product" }));

    expect(
      screen.getByRole("form", { name: "Create product draft" }),
    ).toBeVisible();
    expect(
      screen.getByRole("button", { name: "Close new product" }),
    ).toBeVisible();
    expect(
      screen.getByText(/testnet only.*no real-money production/i),
    ).toBeVisible();
    expect(screen.getByLabelText("Price asset")).toHaveAttribute("readonly");

    fireEvent.click(
      screen.getByRole("button", { name: "Advanced product setup" }),
    );
    expect(screen.getByLabelText("Payment network")).toHaveAttribute(
      "readonly",
    );
  });

  it("turns an empty catalog into a direct creation path", () => {
    const actions = createActions();
    render(
      <ProductRouteWorkspace
        actions={actions}
        initialSnapshot={{ ...initialSnapshot, routes: [] }}
      />,
    );

    expect(
      screen.getByRole("heading", { name: "Create your first product" }),
    ).toBeVisible();
    expect(
      screen.getByRole("form", { name: "Create product draft" }),
    ).toBeVisible();
    expect(screen.getByText("Nothing is published yet")).toBeVisible();
  });

  it("shows a clear retry action when the catalog snapshot fails", () => {
    const actions = createActions();
    render(
      <ProductRouteWorkspace
        actions={actions}
        initialSnapshot={{
          ...initialSnapshot,
          routes: [],
          error: "The catalog service is temporarily unavailable.",
        }}
      />,
    );

    expect(
      screen.getByRole("heading", { name: "Catalog unavailable" }),
    ).toBeVisible();
    expect(
      screen.getByText("The catalog service is temporarily unavailable."),
    ).toBeVisible();
    expect(screen.getByRole("button", { name: "Retry catalog" })).toBeVisible();
  });

  it("creates a draft and pauses a published route through real actions", async () => {
    const actions = createActions();
    render(
      <ProductRouteWorkspace
        actions={actions}
        initialSnapshot={initialSnapshot}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "New product" }));

    fireEvent.change(screen.getByLabelText("Product name"), {
      target: { value: "Executive Summaries" },
    });
    fireEvent.change(screen.getByLabelText("Product URL name"), {
      target: { value: "executive-summaries" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Advanced product setup" }),
    );
    fireEvent.change(screen.getByLabelText("API path"), {
      target: { value: "/summaries" },
    });
    fireEvent.change(screen.getByLabelText("What buyers receive"), {
      target: { value: "Summarize a supplied document" },
    });
    fireEvent.change(
      within(
        screen.getByRole("form", { name: "Create product draft" }),
      ).getByLabelText("Price"),
      {
        target: { value: "12" },
      },
    );
    fireEvent.submit(
      screen.getByRole("form", { name: "Create product draft" }),
    );

    await waitFor(() => expect(actions.createDraft).toHaveBeenCalledTimes(1));
    expect(actions.createDraft).toHaveBeenCalledWith(
      expect.objectContaining({ amount: "12000000" }),
    );
    expect(screen.getByText("1 draft")).toBeVisible();
    expect(
      await screen.findByRole("heading", { name: "Executive Summaries" }),
    ).toBeVisible();

    fireEvent.click(screen.getByRole("button", { name: /Research Report/ }));
    fireEvent.click(screen.getByRole("button", { name: "Pause product" }));
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

    const priceForm = screen.getByRole("form", {
      name: "Update product price",
    });
    fireEvent.change(within(priceForm).getByLabelText("Price"), {
      target: { value: "40" },
    });
    fireEvent.submit(priceForm);

    await waitFor(() =>
      expect(actions.updatePrice).toHaveBeenCalledWith({
        sellerId,
        routeId: draftRoute.routeId,
        expectedVersion: 1,
        amount: "40000000",
      }),
    );
    expect(await screen.findByText("Version 2")).toBeVisible();

    vi.mocked(actions.validateRoute).mockResolvedValue({
      ok: true,
      value: {
        sellerId,
        routeId: draftRoute.routeId,
        valid: true,
        version: 2,
        contractHash: "b".repeat(64),
        checks: [],
      },
    });
    fireEvent.click(screen.getByRole("button", { name: "Validate product" }));
    await waitFor(() => expect(actions.validateRoute).toHaveBeenCalledTimes(1));

    fireEvent.click(screen.getByRole("button", { name: "Publish product" }));
    await waitFor(() =>
      expect(actions.publishRoute).toHaveBeenCalledWith({
        sellerId,
        routeId: draftRoute.routeId,
        expectedVersion: 2,
        contractHash: "b".repeat(64),
      }),
    );
    expect((await screen.findAllByText("Published"))[0]).toBeVisible();
    expect(screen.getByText("Version 3")).toBeVisible();
  });

  it("warns that editing a published price requires republication", async () => {
    const actions = createActions();
    vi.mocked(actions.updatePrice).mockResolvedValue({
      ok: true,
      value: {
        ...publishedRoute,
        amount: "40000000",
        lifecycleStatus: "paused",
        enabled: false,
        version: publishedRoute.version + 1,
      },
    });

    render(
      <ProductRouteWorkspace
        actions={actions}
        initialSnapshot={{ ...initialSnapshot, routes: [publishedRoute] }}
      />,
    );

    expect(
      screen.getByText(/Changing this price pauses the live product/),
    ).toBeVisible();
    const priceForm = screen.getByRole("form", {
      name: "Update product price",
    });
    fireEvent.change(within(priceForm).getByLabelText("Price"), {
      target: { value: "40" },
    });
    fireEvent.submit(priceForm);

    expect((await screen.findAllByText("Paused"))[0]).toBeVisible();
    expect(
      screen.getByRole("button", { name: "Validate product" }),
    ).toBeEnabled();
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

    fireEvent.click(screen.getByRole("button", { name: "Archive product" }));

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
      screen.getByRole("button", { name: "Validate product" }),
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
      screen.getByRole("button", { name: "Emergency disable product" }),
    );
    expect(
      screen.getByRole("heading", { name: "Disable this product now?" }),
    ).toBeVisible();
    expect(actions.emergencyDisableRoute).not.toHaveBeenCalled();

    fireEvent.click(
      screen.getByRole("button", { name: "Confirm emergency disable" }),
    );
    await waitFor(() =>
      expect(actions.emergencyDisableRoute).toHaveBeenCalledTimes(1),
    );
    expect((await screen.findAllByText("Emergency disabled"))[0]).toBeVisible();
  });
});
