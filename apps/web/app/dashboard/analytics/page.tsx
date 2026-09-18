import type { Metadata } from "next";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import { getSellerSession } from "@/features/auth/server/session";
import { loadAnalyticsSnapshot } from "@/features/analytics/controller";
import { SellerAnalyticsDashboard } from "@/features/analytics/view/seller-analytics-dashboard";
import styles from "./analytics-page.module.css";

export const metadata: Metadata = {
  title: "Revenue Lens",
};

// AnalyticsPage loads authoritative seller reporting without exposing credentials.
export default async function AnalyticsPage() {
  const session = await getSellerSession();
  const sellerId = session?.principal.sellerId;

  if (!sellerId) {
    return (
      <section className={styles.missingContext}>
        <p>Seller reporting / setup required</p>
        <h1>Choose a storefront first</h1>
        <span>
          Complete onboarding before opening asset-separated sales reporting.
        </span>
        <Button
          className={styles.action}
          render={<Link href="/dashboard/onboarding" />}
        >
          Open seller onboarding
        </Button>
      </section>
    );
  }

  const snapshot = await loadAnalyticsSnapshot();
  return <SellerAnalyticsDashboard snapshot={snapshot} />;
}
