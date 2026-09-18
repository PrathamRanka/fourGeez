import type { Metadata } from "next";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import { getSellerSession } from "@/features/auth/server/session";
import { loadAnalyticsSnapshot } from "@/features/analytics/controller";
import { SellerAnalyticsDashboard } from "@/features/analytics/view/seller-analytics-dashboard";

export const metadata: Metadata = {
  title: "Revenue Lens",
};

// AnalyticsPage loads authoritative seller reporting without exposing credentials.
export default async function AnalyticsPage() {
  const session = await getSellerSession();
  const sellerId = session?.principal.sellerId;

  if (!sellerId) {
    return (
      <section className="dashboard-missing-context">
        <p className="dashboard-eyebrow">Seller reporting</p>
        <h1>Choose a storefront first</h1>
        <p>
          Complete onboarding before opening asset-separated sales reporting.
        </p>
        <Button render={<Link href="/dashboard/onboarding" />}>
          Open seller onboarding
        </Button>
      </section>
    );
  }

  const snapshot = await loadAnalyticsSnapshot();
  return <SellerAnalyticsDashboard snapshot={snapshot} />;
}
