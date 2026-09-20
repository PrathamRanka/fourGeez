import type { Metadata } from "next";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import { getSellerSession } from "@/features/auth/server/session";
import {
  archiveRoute,
  createDraft,
  emergencyDisableRoute,
  loadProductRouteSnapshot,
  pauseRoute,
  publishRoute,
  updateDraft,
  validateRoute,
} from "@/features/products/controller";
import { ProductRouteWorkspace } from "@/features/products/view/product-route-workspace";

export const metadata: Metadata = {
  title: "Products",
};

const productRouteActions = {
  createDraft,
  updateDraft,
  validateRoute,
  publishRoute,
  pauseRoute,
  archiveRoute,
  emergencyDisableRoute,
};

// ProductRoutesPage loads seller routes server-side without exposing API credentials.
export default async function ProductRoutesPage() {
  const session = await getSellerSession();
  const sellerId = session?.principal.sellerId;

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

  const snapshot = await loadProductRouteSnapshot();
  return (
    <ProductRouteWorkspace
      actions={productRouteActions}
      canonicalOrigin={
        process.env.AGENTPAY_WEB_ORIGIN ?? "http://localhost:3000"
      }
      initialSnapshot={snapshot}
      sellerSlug={session.principal.storefront?.slug}
    />
  );
}
