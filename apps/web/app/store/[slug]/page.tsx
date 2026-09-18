import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { loadStorefrontManifest } from "@/features/storefront/controller";
import { StorefrontHome } from "@/features/storefront/view/storefront";

type StorefrontPageProps = { params: Promise<{ slug: string }> };

export async function generateMetadata({ params }: StorefrontPageProps): Promise<Metadata> {
  const { slug } = await params;
  const manifest = await loadStorefrontManifest(slug);
  if (!manifest) return {};
  const origin = process.env.AGENTPAY_WEB_ORIGIN ?? "http://localhost:3000";
  return {
    title: `${manifest.seller.name} storefront`,
    description: `Browse verified digital services from ${manifest.seller.name} and purchase them through AgentPay's x402 checkout flow.`,
    alternates: { canonical: `${origin.replace(/\/$/, "")}/store/${slug}` },
  };
}

export default async function StorefrontPage({ params }: StorefrontPageProps) {
  const { slug } = await params;
  const manifest = await loadStorefrontManifest(slug);
  if (!manifest) notFound();
  return <StorefrontHome manifest={manifest} />;
}
