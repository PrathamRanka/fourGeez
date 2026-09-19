import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { PublicProduct } from "@/features/storefront/model";
import { CommerceCheckout } from "@/features/commerce/view/commerce-checkout";

const product: PublicProduct = {
  sellerId: "sel_01ARZ3NDEKTSV4RRFFQ69G5FAV",
  routeId: "rte_01ARZ3NDEKTSV4RRFFQ69G5FAW",
  displayName: "Research Report",
  productSlug: "research-report",
  description: "Generate a source-backed market brief.",
  mimeType: "application/json",
  amount: "35000000",
  asset: "USDC",
  network: "eip155:84532",
  availability: "active",
  canonicalUrl:
    "https://shop.agentpay.example/store/northstar/products/research-report",
  purchaseSessionEndpoint:
    "https://api.agentpay.example/v1/storefronts/northstar/products/research-report/purchase-sessions",
};

const intent = {
  intentId: "int_01ARZ3NDEKTSV4RRFFQ69G5FAX",
  routeId: product.routeId,
  sellerId: product.sellerId,
  buyerId: "browser:bps_01ARZ3NDEKTSV4RRFFQ69G5FAY",
  purchaseChannel: "browser" as const,
  productDisplayName: product.displayName,
  productSlug: product.productSlug,
  paymentDestinationId: "dst_01ARZ3NDEKTSV4RRFFQ69G5FAZ",
  payTo: "0x1111111111111111111111111111111111111111",
  requestMethod: "POST" as const,
  requestPath: "/research/basic",
  amount: product.amount,
  asset: product.asset,
  network: product.network,
  requestBodyHash: "a".repeat(64),
  intentHash: "b".repeat(64),
  maximumAmount: product.amount,
  status: "ready" as const,
  expiresAt: "2026-09-18T13:00:00Z",
  createdAt: "2026-09-18T12:55:00Z",
};

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

describe("commerce checkout", () => {
  beforeEach(() => {
    vi.unstubAllGlobals();
  });

  it("completes the local x402 demo without a buyer approval step", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        jsonResponse({
          purchaseIntent: intent,
          transactionId: "txn_01ARZ3NDEKTSV4RRFFQ69G5FB0",
          paymentRequired: btoa(
            JSON.stringify({
              scheme: "exact",
              network: product.network,
              asset: product.asset,
              amount: product.amount,
              payTo: intent.payTo,
              resource: "http://localhost:8080/pay/northstar/research/basic",
              maxTimeoutSeconds: 30,
            }),
          ),
          paymentMode: "mock",
        }),
      )
      .mockResolvedValueOnce(
        jsonResponse({
          status: "fulfilled",
          transactionId: "txn_01ARZ3NDEKTSV4RRFFQ69G5FB0",
          settlementReference: "mock-settlement",
          contentType: "application/json",
          fulfillment: { reportId: "report_demo" },
        }),
      );
    vi.stubGlobal("fetch", fetchMock);

    render(
      <CommerceCheckout
        channel="browser"
        product={product}
        sellerSlug="northstar"
      />,
    );

    expect(screen.getByText("Protected x402 settlement")).toBeVisible();
    expect(
      screen.getByText(
        "Testnet only · Base Sepolia USDC · no real-money production",
      ),
    ).toBeVisible();
    expect(screen.getByLabelText("Maximum spend")).toHaveValue("35");
    expect(screen.queryByText(/manager approval/i)).not.toBeInTheDocument();

    fireEvent.change(screen.getByLabelText("Request body (JSON)"), {
      target: { value: '{"topic":"agent commerce"}' },
    });
    fireEvent.click(screen.getByRole("checkbox", { name: /confirm/i }));
    fireEvent.click(
      screen.getByRole("button", { name: "Review exact payment" }),
    );

    expect(await screen.findByText("Payment ready")).toBeVisible();
    fireEvent.click(
      screen.getByRole("button", { name: "Complete local demo payment" }),
    );

    expect(await screen.findByText("Paid and fulfilled")).toBeVisible();
    expect(screen.getByText(/report_demo/)).toBeVisible();
    expect(screen.getByRole("link", { name: "View receipt" })).toHaveAttribute(
      "href",
      "/purchases/txn_01ARZ3NDEKTSV4RRFFQ69G5FB0#receipt-title",
    );
    expect(screen.getByRole("link", { name: "View evidence" })).toHaveAttribute(
      "href",
      "/purchases/txn_01ARZ3NDEKTSV4RRFFQ69G5FB0#evidence-title",
    );
    expect(screen.getByRole("button", { name: "Open dispute" })).toBeEnabled();
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });

  it("shows human guidance when a Base Sepolia wallet is unavailable", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        jsonResponse({
          purchaseIntent: intent,
          transactionId: "txn_01ARZ3NDEKTSV4RRFFQ69G5FB0",
          paymentRequired: btoa(
            JSON.stringify({
              x402Version: 2,
              accepts: [
                {
                  scheme: "exact",
                  network: product.network,
                  asset: "0x036CbD53842c5426634e7929541eC2318f3dCF7e",
                  amount: product.amount,
                  payTo: intent.payTo,
                  maxTimeoutSeconds: 60,
                  extra: { name: "USDC", version: "2" },
                },
              ],
              resource: {
                url: "http://localhost:8080/pay/northstar/research/basic",
                description: product.description,
                mimeType: product.mimeType,
              },
            }),
          ),
          paymentMode: "x402",
        }),
      ),
    );

    render(
      <CommerceCheckout
        channel="browser"
        product={product}
        sellerSlug="northstar"
      />,
    );
    fireEvent.click(screen.getByRole("checkbox", { name: /confirm/i }));
    fireEvent.click(
      screen.getByRole("button", { name: "Review exact payment" }),
    );

    expect(await screen.findByText("Wallet required")).toBeVisible();
    expect(screen.getByText(/testnet only/i)).toBeVisible();
    expect(screen.getByText(/install or open an EVM wallet/i)).toBeVisible();
    expect(
      screen.getByRole("button", { name: "Check for wallet again" }),
    ).toBeEnabled();
  });

  it("announces a retryable checkout error and permits another attempt", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        jsonResponse(
          {
            error: {
              code: "payment_unavailable",
              message: "The x402 facilitator is temporarily unavailable.",
            },
          },
          503,
        ),
      ),
    );

    render(
      <CommerceCheckout
        channel="browser"
        product={product}
        sellerSlug="northstar"
      />,
    );
    fireEvent.click(screen.getByRole("checkbox", { name: /confirm/i }));
    fireEvent.click(
      screen.getByRole("button", { name: "Review exact payment" }),
    );

    await waitFor(() =>
      expect(screen.getByRole("alert")).toHaveTextContent(
        "The x402 facilitator is temporarily unavailable.",
      ),
    );
    expect(screen.getByRole("button", { name: "Try again" })).toBeEnabled();
  });
});
