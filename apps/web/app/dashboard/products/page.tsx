import type { Metadata } from "next";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import {
  archiveRoute,
  createDraft,
  emergencyDisableRoute,
  loadProductRouteSnapshot,
  pauseRoute,
  publishRoute,
  updatePrice,
  validateRoute,
} from "@/features/products/controller";
import { ProductRouteWorkspace } from "@/features/products/view/product-route-workspace";

export const metadata: Metadata = {
  title: "Product routes",
};

type ProductRoutesPageProps = {
  searchParams: Promise<{ sellerId?: string }>;
};

const productRouteActions = {
  createDraft,
  updatePrice,
  validateRoute,
  publishRoute,
  pauseRoute,
  archiveRoute,
  emergencyDisableRoute,
};

// ProductRoutesPage loads seller routes server-side without exposing API credentials.
export default async function ProductRoutesPage({
  searchParams,
}: ProductRoutesPageProps) {
  const parameters = await searchParams;
  const sellerId = parameters.sellerId ?? process.env.AGENTPAY_DEMO_SELLER_ID;

  if (!sellerId) {
    return (
      <section className="dashboard-missing-context">
        <p className="dashboard-eyebrow">Catalog control</p>
        <h1>Choose a storefront first</h1>
        <p>
          Complete seller onboarding to create a storefront, verify its payment
          destination, and open product management.
        </p>
        <Button render={<Link href="/dashboard/onboarding" />}>
          Open seller onboarding
        </Button>
      </section>
    );
  }

  const snapshot = await loadProductRouteSnapshot(sellerId);
  return (
    <ProductRouteWorkspace
      actions={productRouteActions}
      initialSnapshot={snapshot}
    />
  );
}
