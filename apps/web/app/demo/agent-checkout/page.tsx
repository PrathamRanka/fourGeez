import type { Metadata } from "next";
import { runBuyerActivity } from "@/features/buyer/controller";
import { BuyerActivity } from "@/features/buyer/view/buyer-activity";

export const metadata: Metadata = {
  title: "Agent checkout demo",
  description:
    "See a deterministic buyer agent discover an offer and enter AgentPay's shared x402 checkout.",
};

export default function AgentCheckoutDemoPage() {
  return <BuyerActivity run={runBuyerActivity} />;
}
