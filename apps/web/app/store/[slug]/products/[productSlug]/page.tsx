import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { loadStorefrontManifest } from "@/features/storefront/controller";
import {
  buildProductStructuredData,
  ProductDetail,
} from "@/features/storefront/view/storefront";

type ProductPageProps = {
  params: Promise<{ slug: string; productSlug: string }>;
};

export async function generateMetadata({
  params,
}: ProductPageProps): Promise<Metadata> {
  const { slug, productSlug } = await params;
  const manifest = await loadStorefrontManifest(slug);
  const route = manifest?.routes.find(
    (candidate) => candidate.productSlug === productSlug,
  );
  if (!manifest || !route) return {};
  const origin = process.env.AGENTPAY_WEB_ORIGIN ?? "http://localhost:3000";
  return {
    title: `${route.displayName} by ${manifest.seller.name}`,
    description: route.description,
    alternates: {
      canonical: `${origin.replace(/\/$/, "")}/store/${slug}/products/${route.productSlug}`,
    },
    robots: { index: true, follow: true },
  };
}

export default async function ProductPage({ params }: ProductPageProps) {
  const { slug, productSlug } = await params;
  const manifest = await loadStorefrontManifest(slug);
  const route = manifest?.routes.find(
    (candidate) => candidate.productSlug === productSlug,
  );
  if (!manifest || !route) notFound();
  const webOrigin = process.env.AGENTPAY_WEB_ORIGIN ?? "http://localhost:3000";
  const canonicalUrl = `${webOrigin.replace(/\/$/, "")}/store/${slug}/products/${route.productSlug}`;
  const structuredData = buildProductStructuredData(
    manifest,
    route,
    canonicalUrl,
  );
  return (
    <>
      <script
        type="application/ld+json"
        dangerouslySetInnerHTML={{ __html: JSON.stringify(structuredData) }}
      />
      <ProductDetail
        manifest={manifest}
        route={route}
        apiOrigin={process.env.AGENTPAY_API_ORIGIN ?? "http://localhost:8080"}
      />
    </>
  );
}
