import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { BuyerActivityAction } from "@/features/buyer/model";
import { BuyerActivity } from "@/features/buyer/view/buyer-activity";

const selectedProduct = {
  sellerId: "sel_01ARZ3NDEKTSV4RRFFQ69G5FAV",
  routeId: "rte_01ARZ3NDEKTSV4RRFFQ69G5FAW",
  displayName: "Research Report",
  productSlug: "research-report",
  description: "Generate a source-backed market brief.",
  mimeType: "application/json",
  amount: "35000000",
  asset: "USDC",
  network: "eip155:84532",
  availability: "active" as const,
  canonicalUrl:
    "https://shop.agentpay.example/store/northstar/products/research-report",
  purchaseSessionEndpoint:
    "https://api.agentpay.example/v1/storefronts/northstar/products/research-report/purchase-sessions",
};

describe("buyer activity", () => {
  it("runs storefront discovery and labels deterministic fallback", async () => {
    const run: BuyerActivityAction = vi.fn().mockResolvedValue({
      ok: true,
      value: {
        mode: "deterministic",
        sellerSlug: "northstar",
        selectedProduct,
        response: "I found one published product that matches your request.",
        activities: [
          {
            tool: "getStorefrontManifest",
            status: "completed",
            detail: "Loaded 1 published route",
          },
          {
            tool: "rankEligibleOffers",
            status: "completed",
            detail: "Selected /research within the stated budget",
          },
        ],
      },
    });
    render(<BuyerActivity run={run} />);

    expect(screen.getByText("Agent trace")).toBeVisible();
    fireEvent.change(screen.getByLabelText("Storefront slug"), {
      target: { value: "northstar" },
    });
    fireEvent.change(screen.getByLabelText("What do you need?"), {
      target: { value: "A market brief under 40 USDC" },
    });
    fireEvent.submit(screen.getByRole("form", { name: "Buyer request" }));

    await waitFor(() => expect(run).toHaveBeenCalledTimes(1));
    expect(screen.getByText("Deterministic fallback active")).toBeVisible();
    expect(screen.getByText("getStorefrontManifest")).toBeVisible();
    expect(screen.getByText("rankEligibleOffers")).toBeVisible();
    expect(screen.getByText(/one published product/i)).toBeVisible();
    expect(
      screen.getByRole("heading", { name: "Agent-selected checkout" }),
    ).toBeVisible();
    expect(screen.getByText("35 USDC")).toBeVisible();
  });
});
