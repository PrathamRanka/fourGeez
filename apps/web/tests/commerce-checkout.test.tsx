import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { PublicProduct } from "@/features/storefront/model";
import { CommerceCheckout } from "@/features/commerce/view/commerce-checkout";

const product: PublicProduct = {
  schemaVersion: "agentpay.product-contract.v1",
  sellerId: "sel_01ARZ3NDEKTSV4RRFFQ69G5FAV",
  routeId: "rte_01ARZ3NDEKTSV4RRFFQ69G5FAW",
  routeVersion: 2,
  displayName: "Research Report",
  productSlug: "research-report",
  description: "Generate a source-backed market brief.",
  mimeType: "application/json",
  inputSchema: { type: "object", properties: {}, additionalProperties: false },
  outputSchema: { type: "object", properties: {}, additionalProperties: false },
  amount: "35000000",
  asset: "USDC",
  network: "eip155:84532",
  paymentProtocol: "x402",
  paymentScheme: "exact",
  availability: "active",
  fulfillmentMode: "synchronous_https",
  fulfillmentTimeoutSeconds: 20,
  updatedAt: "2026-09-18T11:55:00Z",
  authoritativeForPurchase: false,
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
    window.ethereum = undefined;
  });

  it("renders the published input schema as buyer fields and blocks invalid input before checkout", async () => {
    const schemaProduct: PublicProduct = {
      ...product,
      inputSchema: {
        type: "object",
        additionalProperties: false,
        required: ["productName", "audience"],
        properties: {
          productName: {
            type: "string",
            description: "Name the product to analyze.",
            minLength: 2,
            maxLength: 20,
          },
          audience: {
            type: "string",
            description: "Choose who will read the report.",
            enum: ["Founders", "Developers"],
          },
          includeSources: {
            type: "boolean",
            description: "Include source links in the result.",
          },
        },
      },
    };
    const fetchMock = vi.fn().mockResolvedValue(
      jsonResponse(
        {
          error: {
            code: "payment_unavailable",
            message: "Checkout paused for this test.",
          },
        },
        503,
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    render(
      <CommerceCheckout
        channel="browser"
        product={schemaProduct}
        sellerSlug="northstar"
      />,
    );

    expect(
      screen.queryByLabelText("Request body (JSON)"),
    ).not.toBeInTheDocument();
    expect(screen.getByLabelText("Product name (required)")).toBeVisible();
    expect(screen.getByLabelText("Audience (required)")).toBeVisible();
    expect(screen.getByLabelText("Include sources")).toBeVisible();

    fireEvent.click(screen.getByRole("checkbox", { name: /confirm/i }));
    fireEvent.click(
      screen.getByRole("button", { name: "Review exact payment" }),
    );

    expect(await screen.findByText("Product name is required.")).toBeVisible();
    expect(screen.getByText("Audience is required.")).toBeVisible();
    expect(fetchMock).not.toHaveBeenCalled();

    fireEvent.change(screen.getByLabelText("Product name (required)"), {
      target: { value: "A product name that is too long" },
    });
    fireEvent.change(screen.getByLabelText("Audience (required)"), {
      target: { value: "Founders" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Review exact payment" }),
    );

    expect(
      await screen.findByText("Product name must be at most 20 characters."),
    ).toBeVisible();
    expect(fetchMock).not.toHaveBeenCalled();

    fireEvent.change(screen.getByLabelText("Product name (required)"), {
      target: { value: "AgentPay" },
    });
    fireEvent.click(screen.getByLabelText("Include sources"));
    fireEvent.click(
      screen.getByRole("button", { name: "Review exact payment" }),
    );

    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
    expect(JSON.parse(String(fetchMock.mock.calls[0][1]?.body))).toMatchObject({
      requestBody: JSON.stringify({
        audience: "Founders",
        includeSources: true,
        productName: "AgentPay",
      }),
    });
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
    expect(
      screen.getByText(
        /your wallet authorizes the seller's exact 35 USDC quote/i,
      ),
    ).toBeVisible();
    expect(screen.queryByText(/AgentPay charges/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/manager approval/i)).not.toBeInTheDocument();

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

  it("retries a cancelled wallet connection against the same frozen checkout", async () => {
    const buyerAddress = "0x2222222222222222222222222222222222222222";
    let connectionAttempts = 0;
    window.ethereum = {
      request: vi.fn().mockImplementation(({ method }) => {
        switch (method) {
          case "eth_chainId":
            return Promise.resolve("0x14a34");
          case "eth_accounts":
            return Promise.resolve([buyerAddress]);
          case "eth_requestAccounts":
            connectionAttempts += 1;
            return connectionAttempts === 1
              ? Promise.reject({ code: 4001 })
              : Promise.resolve([buyerAddress]);
          case "wallet_switchEthereumChain":
            return Promise.resolve(null);
          case "eth_signTypedData_v4":
            return Promise.resolve("0xsigned");
          default:
            return Promise.reject(new Error(`unexpected method ${method}`));
        }
      }),
    };
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
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
      )
      .mockResolvedValueOnce(
        jsonResponse({
          status: "fulfilled",
          transactionId: "txn_01ARZ3NDEKTSV4RRFFQ69G5FB0",
          settlementReference: "0xtestnet",
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
    fireEvent.click(screen.getByRole("checkbox", { name: /confirm/i }));
    fireEvent.click(
      screen.getByRole("button", { name: "Review exact payment" }),
    );
    fireEvent.click(
      await screen.findByRole("button", {
        name: "Confirm 35 USDC in wallet",
      }),
    );

    expect(
      await screen.findByText(/wallet connection was cancelled/i),
    ).toBeVisible();
    fireEvent.click(
      screen.getByRole("button", { name: "Connect wallet and retry" }),
    );

    expect(await screen.findByText("Paid and fulfilled")).toBeVisible();
    expect(fetchMock).toHaveBeenCalledTimes(2);
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

  it("keeps an unknown settlement on the same signed payment", async () => {
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
        jsonResponse(
          {
            error: {
              code: "payment_outcome_unknown",
              message: "Settlement confirmation is temporarily unavailable.",
              details: { recoveryAction: "retry_same_payment" },
            },
          },
          503,
        ),
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
    fireEvent.click(screen.getByRole("checkbox", { name: /confirm/i }));
    fireEvent.click(
      screen.getByRole("button", { name: "Review exact payment" }),
    );
    fireEvent.click(
      await screen.findByRole("button", {
        name: "Complete local demo payment",
      }),
    );

    expect(await screen.findByText(/same signed payment/i)).toBeVisible();
    fireEvent.click(
      screen.getByRole("button", { name: "Retry settlement check" }),
    );

    expect(await screen.findByText("Paid and fulfilled")).toBeVisible();
    expect(fetchMock).toHaveBeenCalledTimes(3);
    expect(JSON.parse(String(fetchMock.mock.calls[1][1]?.body))).toEqual(
      JSON.parse(String(fetchMock.mock.calls[2][1]?.body)),
    );
  });
});
