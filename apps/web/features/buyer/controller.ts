"use server";

import type {
  BuyerActivityInput,
  BuyerActivityResult,
} from "@/features/buyer/model";
import type { StorefrontManifest } from "@/features/storefront/model";
import { requestPublicAgentPay, type ActionResult } from "@/lib/agentpay-api";

// runBuyerActivity performs transparent deterministic discovery without model inference.
export async function runBuyerActivity(
  input: BuyerActivityInput,
): Promise<ActionResult<BuyerActivityResult>> {
  const slug = input.slug.trim();
  const prompt = input.prompt.trim();
  if (!/^[a-z0-9-]{3,48}$/.test(slug) || prompt.length < 3 || prompt.length > 500) {
    return { ok: false, error: "Enter a valid storefront slug and buyer request." };
  }
  const manifestResult = await requestPublicAgentPay<StorefrontManifest>(
    `/store/${encodeURIComponent(slug)}/manifest.json`,
  );
  if (!manifestResult.ok) return manifestResult;
  const routes = manifestResult.value.routes;
  const selectedRoute = routes[0];
  return {
    ok: true,
    value: {
      mode: "deterministic",
      response: selectedRoute
        ? `I found ${routes.length} published product${routes.length === 1 ? "" : "s"}. The deterministic fallback selected ${selectedRoute.pathPattern} by catalog order; review its price before purchasing.`
        : "I found no published products in this storefront.",
      activities: [
        {
          tool: "getStorefrontManifest",
          status: "completed",
          detail: `Loaded ${routes.length} published route${routes.length === 1 ? "" : "s"}`,
        },
        {
          tool: "rankEligibleOffers",
          status: selectedRoute ? "completed" : "blocked",
          detail: selectedRoute
            ? `Selected ${selectedRoute.pathPattern} using deterministic catalog order`
            : "No eligible published route was available",
        },
      ],
    },
  };
}
