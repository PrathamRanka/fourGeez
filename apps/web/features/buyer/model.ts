import type { ActionResult } from "@/lib/agentpay-api";

export type BuyerActivityEntry = {
  tool: string;
  status: "completed" | "blocked";
  detail: string;
};

export type BuyerActivityResult = {
  mode: "deterministic";
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
