import type { Metadata } from "next";
import { loadStorefrontDiscovery } from "@/features/storefront/controller";
import {
  StorefrontAvailability,
  StorefrontHome,
} from "@/features/storefront/view/storefront";

type StorefrontPageProps = { params: Promise<{ slug: string }> };

export async function generateMetadata({
  params,
}: StorefrontPageProps): Promise<Metadata> {
  const { slug } = await params;
  const state = await loadStorefrontDiscovery(slug);
  if (state.status !== "active") {
    return { robots: { index: false, follow: false } };
  }
  const origin = process.env.AGENTPAY_WEB_ORIGIN ?? "http://localhost:3000";
  return {
    title: `${state.manifest.seller.name} storefront`,
    description: `Browse verified digital services from ${state.manifest.seller.name} and purchase them through AgentPay's x402 checkout flow.`,
    alternates: { canonical: `${origin.replace(/\/$/, "")}/store/${slug}` },
  };
}

export default async function StorefrontPage({ params }: StorefrontPageProps) {
  const { slug } = await params;
  const state = await loadStorefrontDiscovery(slug);
  if (state.status !== "active") {
    return <StorefrontAvailability state={state} />;
  }
  return (
    <StorefrontHome manifest={state.manifest} signature={state.signature} />
  );
}
