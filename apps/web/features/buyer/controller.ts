"use server";

import type {
  BuyerActivityInput,
  BuyerActivityResult,
} from "@/features/buyer/model";
import { loadStorefrontDiscovery } from "@/features/storefront/controller";
import type { ActionResult } from "@/lib/agentpay-api";

// runBuyerActivity performs transparent deterministic discovery without model inference.
export async function runBuyerActivity(
  input: BuyerActivityInput,
): Promise<ActionResult<BuyerActivityResult>> {
  const slug = input.slug.trim();
  const prompt = input.prompt.trim();
  if (
    !/^[a-z0-9-]{3,48}$/.test(slug) ||
    prompt.length < 3 ||
    prompt.length > 500
  ) {
    return {
      ok: false,
      error: "Enter a valid storefront slug and buyer request.",
    };
  }
  const discovery = await loadStorefrontDiscovery(slug);
  if (discovery.status !== "active") {
    return {
      ok: false,
      error: "AgentPay could not confirm an active storefront.",
      code:
        discovery.status === "inactive"
          ? "seller_inactive"
          : discovery.status === "expired"
            ? "discovery_expired"
            : discovery.reason,
    };
  }
  const products = discovery.manifest.products;
  const selectedProduct = products[0];
  return {
    ok: true,
    value: {
      mode: "deterministic",
      response: selectedProduct
        ? `I found ${products.length} published product${products.length === 1 ? "" : "s"}. The deterministic fallback selected ${selectedProduct.displayName} by catalog order; review its price before purchasing.`
        : "I found no published products in this storefront.",
      activities: [
        {
          tool: "getStorefrontManifest",
          status: "completed",
          detail: `Loaded ${products.length} signed product${products.length === 1 ? "" : "s"}`,
        },
        {
          tool: "rankEligibleOffers",
          status: selectedProduct ? "completed" : "blocked",
          detail: selectedProduct
            ? `Selected ${selectedProduct.displayName} using deterministic catalog order`
            : "No eligible published route was available",
        },
      ],
    },
  };
}
