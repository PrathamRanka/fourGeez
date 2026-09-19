import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { SellerTestPurchase } from "@/features/onboarding/view/seller-test-purchase";

const route = {
  routeId: "rte_01ARZ3NDEKTSV4RRFFQ69G5FAW",
  displayName: "Research report",
  productSlug: "research-report",
  method: "POST" as const,
  pathPattern: "/research/basic",
  description: "Generate a source-backed research report.",
  mimeType: "application/json",
  amount: "100000",
  asset: "USDC",
  network: "eip155:84532",
  lifecycleStatus: "published" as const,
  enabled: true,
};

const purchaseIntent = {
  intentId: "int_01ARZ3NDEKTSV4RRFFQ69G5FAX",
  routeId: route.routeId,
  sellerId: "sel_01ARZ3NDEKTSV4RRFFQ69G5FAV",
  buyerId: "browser:bps_01ARZ3NDEKTSV4RRFFQ69G5FAY",
  purchaseChannel: "browser" as const,
  productDisplayName: route.displayName,
  productSlug: route.productSlug,
  paymentDestinationId: "dst_01ARZ3NDEKTSV4RRFFQ69G5FAZ",
  payTo: "0x1111111111111111111111111111111111111111",
  requestMethod: route.method,
  requestPath: route.pathPattern,
  amount: route.amount,
  asset: route.asset,
  network: route.network,
  requestBodyHash: "a".repeat(64),
  intentHash: "b".repeat(64),
  maximumAmount: route.amount,
  status: "ready" as const,
  expiresAt: "2026-09-19T13:00:00Z",
  createdAt: "2026-09-19T12:55:00Z",
};

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

describe("seller test purchase", () => {
  afterEach(() => vi.unstubAllGlobals());

  it("stays hidden until every prerequisite and a published route are ready", () => {
    const { rerender } = render(
      <SellerTestPurchase
        eligible={false}
        route={route}
        sellerSlug="northstar"
        verifyPurchase={vi.fn()}
      />,
    );
    expect(
      screen.queryByRole("region", { name: "Run test purchase" }),
    ).not.toBeInTheDocument();

    rerender(
      <SellerTestPurchase
        eligible
        route={null}
        sellerSlug="northstar"
        verifyPurchase={vi.fn()}
      />,
    );
    expect(
      screen.queryByRole("region", { name: "Run test purchase" }),
    ).not.toBeInTheDocument();
  });

  it("proves the complete local mock journey and reconciles one transaction", async () => {
    const transactionId = "txn_01ARZ3NDEKTSV4RRFFQ69G5FB0";
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        jsonResponse(
          {
            purchaseIntent,
            transactionId,
            paymentRequired: btoa(
              JSON.stringify({
                scheme: "exact",
                network: route.network,
                asset: route.asset,
                amount: route.amount,
                payTo: purchaseIntent.payTo,
                resource: "http://localhost:8080/pay/northstar/research/basic",
                maxTimeoutSeconds: 30,
              }),
            ),
            paymentMode: "mock",
          },
          201,
        ),
      )
      .mockResolvedValueOnce(
        jsonResponse({
          status: "fulfilled",
          transactionId,
          settlementReference: "mock-settlement",
          contentType: "application/json",
          fulfillment: { reportId: "report_demo" },
        }),
      );
    vi.stubGlobal("fetch", fetchMock);
    const verifyPurchase = vi.fn().mockResolvedValue({
      ok: true,
      value: {
        transactionId,
        dashboardOccurrenceCount: 1,
        evidenceEventCount: 5,
        evidenceValid: true,
        fulfillmentExactlyOnce: true,
        receiptAvailable: true,
      },
    });

    render(
      <SellerTestPurchase
        eligible
        route={route}
        sellerSlug="northstar"
        verifyPurchase={verifyPurchase}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Run test purchase" }));

    expect(await screen.findByText("Launch test passed")).toBeVisible();
    expect(screen.getByText("Local mock payment")).toBeVisible();
    for (const label of [
      "Intent created",
      "Payment verified",
      "Exactly-once fulfillment",
      "Receipt and evidence verified",
      "Dashboard reconciled",
    ]) {
      expect(
        within(screen.getByRole("listitem", { name: label })).getByText(
          "Passed",
        ),
      ).toBeVisible();
    }
    expect(verifyPurchase).toHaveBeenCalledWith({ transactionId });
    expect(fetchMock).toHaveBeenCalledTimes(2);
    expect(
      screen.getByRole("link", { name: "Open verified transaction" }),
    ).toHaveAttribute("href", `/dashboard/transactions/${transactionId}`);
  });

  it("reports a failed checkpoint and allows a clean retry without claiming success", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        jsonResponse(
          {
            error: {
              code: "payment_unavailable",
              message: "The test payment service is temporarily unavailable.",
            },
          },
          503,
        ),
      )
      .mockResolvedValueOnce(
        jsonResponse(
          {
            purchaseIntent,
            transactionId: "txn_01ARZ3NDEKTSV4RRFFQ69G5FB0",
            paymentRequired: btoa(JSON.stringify({ scheme: "exact" })),
            paymentMode: "mock",
          },
          201,
        ),
      );
    vi.stubGlobal("fetch", fetchMock);

    render(
      <SellerTestPurchase
        eligible
        route={route}
        sellerSlug="northstar"
        verifyPurchase={vi.fn()}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Run test purchase" }));

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "temporarily unavailable",
    );
    expect(screen.queryByText("Launch test passed")).not.toBeInTheDocument();
    fireEvent.click(
      screen.getByRole("button", { name: "Retry test purchase" }),
    );
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(2));
  });
});
