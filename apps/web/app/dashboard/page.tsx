import type { Metadata } from "next";
import { ArrowUpRight, Boxes, FileCheck2, Settings2 } from "lucide-react";
import Link from "next/link";
import { redirect } from "next/navigation";
import { loadAnalyticsSnapshot } from "@/features/analytics/controller";
import {
  buildDailyActivity,
  buildPaymentPairSummaries,
} from "@/features/analytics/model";
import { DailyActivityChart } from "@/features/analytics/view/daily-activity-chart";
import { getSellerSession } from "@/features/auth/server/session";
import { formatAtomicPrice } from "@/lib/money";
import styles from "./overview.module.css";

export const metadata: Metadata = { title: "Seller dashboard" };

const operatingLinks = [
  {
    href: "/dashboard/products",
    label: "Manage products",
    description: "Price, verify, and publish offers.",
    icon: Boxes,
  },
  {
    href: "/dashboard/transactions",
    label: "Review transactions",
    description: "Track settlement and fulfillment.",
    icon: FileCheck2,
  },
  {
    href: "/dashboard/settings",
    label: "Open settings",
    description: "Account, appearance, and store setup.",
    icon: Settings2,
  },
] as const;

export default async function DashboardPage() {
  const session = await getSellerSession();
  if (!session) redirect("/sign-in?returnTo=%2Fdashboard");
  if (!session.principal.sellerId || !session.principal.onboardingComplete) {
    redirect("/dashboard/onboarding");
  }

  const snapshot = await loadAnalyticsSnapshot();
  const paymentPairs = buildPaymentPairSummaries(snapshot.aggregates);
  const dailyActivity = buildDailyActivity(snapshot.aggregates);

  return (
    <div className={styles.workspace}>
      <header className={styles.pageHeader}>
        <div>
          <p className={styles.eyebrow}>Seller workspace</p>
          <h1>Commerce overview</h1>
          <p className={styles.welcome}>
            <strong>{session.principal.name}</strong>
            <span>Monitor sales, settlement, and delivery.</span>
          </p>
        </div>
        <div className={styles.networkStatus}>
          <span aria-hidden="true" />
          {snapshot.error ? "Reporting degraded" : "Systems operational"}
        </div>
      </header>

      <section className={styles.metrics} aria-label="Commerce metrics">
        <article>
          <span>Transactions</span>
          <strong>{snapshot.transactionCount}</strong>
          <small>All recorded sales</small>
        </article>
        <article>
          <span>Products</span>
          <strong>{snapshot.routes.length}</strong>
          <small>Configured paid routes</small>
        </article>
        <article>
          <span>Payment pairs</span>
          <strong>{paymentPairs.length}</strong>
          <small>Asset and network separated</small>
        </article>
      </section>

      {snapshot.error ? (
        <div className={styles.errorState} role="alert">
          <div>
            <strong>Live reporting is unavailable</strong>
            <p>{snapshot.error}</p>
          </div>
          <Link href="/dashboard">Retry overview</Link>
        </div>
      ) : null}

      <section className={styles.commerceGrid} aria-label="Settlement overview">
        <article className={styles.activityPanel}>
          <div className={styles.panelHeading}>
            <div>
              <p>Performance</p>
              <h2>Settlement activity</h2>
            </div>
            <span>Daily stage count · UTC</span>
          </div>
          {dailyActivity.length > 0 ? (
            <DailyActivityChart activity={dailyActivity} />
          ) : (
            <div className={styles.compactEmpty}>
              Activity appears after the first verified purchase.
            </div>
          )}
        </article>

        <aside className={styles.settlementPanel} aria-label="Payment pairs">
          <div className={styles.settlementHeading}>
            <div>
              <p>Settlements</p>
              <h2>Payment pairs</h2>
            </div>
            <div
              className={styles.transactionCount}
              role="status"
              aria-label={`${snapshot.transactionCount} ${
                snapshot.transactionCount === 1 ? "transaction" : "transactions"
              }`}
            >
              <strong>{snapshot.transactionCount}</strong>
              <span>Total</span>
            </div>
          </div>

          {paymentPairs.length > 0 ? (
            <div className={styles.pairList}>
              {paymentPairs.map((paymentPair) => (
                <section
                  key={`${paymentPair.asset}:${paymentPair.network}`}
                  className={styles.pairCard}
                  aria-label={`${paymentPair.asset} on ${paymentPair.network}`}
                >
                  <div className={styles.pairIdentity}>
                    <strong>{paymentPair.asset}</strong>
                    <span>{paymentPair.network}</span>
                  </div>
                  <dl>
                    <div>
                      <dt>Verified</dt>
                      <dd>
                        {formatAtomicPrice(
                          paymentPair.grossVerifiedAmount,
                          paymentPair.asset,
                        )}
                      </dd>
                    </div>
                    <div>
                      <dt>Fulfilled</dt>
                      <dd>
                        {formatAtomicPrice(
                          paymentPair.fulfilledAmount,
                          paymentPair.asset,
                        )}
                      </dd>
                    </div>
                  </dl>
                </section>
              ))}
            </div>
          ) : (
            <div className={styles.emptyPairs}>
              <strong>No settlements yet</strong>
              <span>Verified payments will appear here.</span>
            </div>
          )}

          <Link href="/dashboard/analytics" aria-label="Open analytics">
            Open analytics <ArrowUpRight aria-hidden="true" />
          </Link>
        </aside>
      </section>

      <section
        aria-labelledby="seller-operations"
        className={styles.operations}
      >
        <div className={styles.sectionHeading}>
          <div>
            <p>Quick actions</p>
            <h2 id="seller-operations">Keep commerce moving.</h2>
          </div>
        </div>
        <div className={styles.operationList}>
          {operatingLinks.map((operation) => {
            const Icon = operation.icon;
            return (
              <Link
                key={operation.href}
                href={operation.href}
                aria-label={operation.label}
                className={styles.operationLink}
              >
                <span className={styles.operationIcon}>
                  <Icon aria-hidden="true" />
                </span>
                <span>
                  <strong>{operation.label}</strong>
                  <small>{operation.description}</small>
                </span>
                <ArrowUpRight aria-hidden="true" />
              </Link>
            );
          })}
        </div>
      </section>
    </div>
  );
}
