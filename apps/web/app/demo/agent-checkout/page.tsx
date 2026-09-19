import type { Metadata } from "next";
import { runBuyerActivity } from "@/features/buyer/controller";
import { BuyerActivity } from "@/features/buyer/view/buyer-activity";
import { agentCheckoutDemoMetadata } from "@/features/marketing/seo";

export const metadata: Metadata = agentCheckoutDemoMetadata;

export default function AgentCheckoutDemoPage() {
  return <BuyerActivity run={runBuyerActivity} />;
}
