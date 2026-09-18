import type { Metadata } from "next";
import { runBuyerActivity } from "@/features/buyer/controller";
import { BuyerActivity } from "@/features/buyer/view/buyer-activity";

export const metadata: Metadata = {
  title: "Buyer activity",
  description: "Inspect an AgentPay storefront with visible, deterministic tool activity.",
};

export default function BuyerPage() {
  return <BuyerActivity run={runBuyerActivity} />;
}
