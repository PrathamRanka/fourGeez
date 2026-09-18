import type { ActionResult } from "@/lib/agentpay-api";
import type { PublicProduct } from "@/features/storefront/model";

export type BuyerActivityEntry = {
  tool: string;
  status: "completed" | "blocked";
  detail: string;
};

export type BuyerActivityResult = {
  mode: "deterministic";
  sellerSlug: string;
  selectedProduct: PublicProduct | null;
  response: string;
  activities: BuyerActivityEntry[];
};

export type BuyerActivityInput = {
  slug: string;
  prompt: string;
};

export type BuyerActivityAction = (
  input: BuyerActivityInput,
) => Promise<ActionResult<BuyerActivityResult>>;
