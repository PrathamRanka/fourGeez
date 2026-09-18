import type { Metadata } from "next";
import { loadPublicProduct } from "@/features/storefront/controller";
import {
  buildProductStructuredData,
  ProductDetail,
  StorefrontAvailability,
} from "@/features/storefront/view/storefront";

type ProductPageProps = {
  params: Promise<{ slug: string; productSlug: string }>;
};

export async function generateMetadata({
  params,
}: ProductPageProps): Promise<Metadata> {
  const { slug, productSlug } = await params;
  const state = await loadPublicProduct(slug, productSlug);
  if (state.status !== "active") {
    return { robots: { index: false, follow: false } };
  }
  return {
    title: `${state.document.product.displayName} | AgentPay`,
    description: state.document.product.description,
    alternates: { canonical: state.document.product.canonicalUrl },
    robots: { index: true, follow: true },
  };
}

export default async function ProductPage({ params }: ProductPageProps) {
  const { slug, productSlug } = await params;
  const state = await loadPublicProduct(slug, productSlug);
  if (state.status !== "active") {
    return <StorefrontAvailability state={state} subject="product" />;
  }
  const structuredData = buildProductStructuredData(state.document.product);
  return (
    <>
      <script
        type="application/ld+json"
        dangerouslySetInnerHTML={{
          __html: JSON.stringify(structuredData).replace(/</g, "\\u003c"),
        }}
      />
      <ProductDetail
        sellerSlug={state.document.sellerSlug}
        product={state.document.product}
        signature={state.signature}
      />
    </>
  );
}
