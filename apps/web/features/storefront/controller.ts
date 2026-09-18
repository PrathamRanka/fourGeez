import type { StorefrontManifest } from "@/features/storefront/model";
import {
  requestPublicAgentPay,
  requestPublicAgentPayText,
} from "@/lib/agentpay-api";

export async function loadStorefrontManifest(
  slug: string,
): Promise<StorefrontManifest | null> {
  const result = await requestPublicAgentPay<StorefrontManifest>(
    `/store/${encodeURIComponent(slug)}/manifest.json`,
  );
  return result.ok ? result.value : null;
}

export async function loadStorefrontLLMSText(
  slug: string,
): Promise<string | null> {
  const result = await requestPublicAgentPayText(
    `/store/${encodeURIComponent(slug)}/llms.txt`,
  );
  return result.ok ? result.value : null;
}
