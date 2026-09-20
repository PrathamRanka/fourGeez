import type { Metadata } from "next";
import {
  ArrowRight,
  ArrowUpRight,
  Boxes,
  FileCheck2,
  Settings2,
} from "lucide-react";
import Link from "next/link";
import { redirect } from "next/navigation";
import { loadAnalyticsSnapshot } from "@/features/analytics/controller";
import {
  buildDailyActivity,
  buildPaymentPairSummaries,
} from "@/features/analytics/model";
import { DailyActivityChart } from "@/features/analytics/view/daily-activity-chart";
import { getSellerSession } from "@/features/auth/server/session";
import { loadOnboardingState } from "@/features/onboarding/controller";
import { getSellerNextAction } from "@/features/onboarding/model";
import { formatAtomicPrice } from "@/lib/money";
import styles from "./overview.module.css";

export const metadata: Metadata = { title: "Seller dashboard" };

const operatingLinks = [
  {
    href: "/dashboard/products",
    label: "Manage products",
    description: "Pricing, publishing, and fulfillment.",
    icon: Boxes,
  },
  {
    href: "/dashboard/transactions",
    label: "Review transactions",
    description: "Settlement, delivery, and disputes.",
    icon: FileCheck2,
  },
  {
    href: "/dashboard/settings",
    label: "Open settings",
    description: "Storefront, account, and appearance.",
    icon: Settings2,
  },
] as const;

export default async function DashboardPage() {
  const session = await getSellerSession();
  if (!session) redirect("/sign-in?returnTo=%2Fdashboard");
  if (!session.principal.sellerId || !session.principal.onboardingComplete) {
    redirect("/dashboard/onboarding");
  }

  const [snapshot, onboarding] = await Promise.all([
    loadAnalyticsSnapshot(),
    loadOnboardingState(),
  ]);
  const paymentPairs = buildPaymentPairSummaries(snapshot.aggregates);
  const dailyActivity = buildDailyActivity(snapshot.aggregates);
  const stageCounts = dailyActivity.reduce(
    (totals, day) => ({
      fulfilled: totals.fulfilled + day.fulfilled,
      processing: totals.processing + day.processing,
      needsAttention: totals.needsAttention + day.failed + day.disputed,
    }),
    { fulfilled: 0, processing: 0, needsAttention: 0 },
  );

  const setupAction = getSellerNextAction(onboarding);
  const recommendedAction = !onboarding.publication.allowed
    ? {
        title: setupAction.label,
        description: setupAction.description,
        href: setupAction.href,
        label: setupAction.label,
      }
    : snapshot.routes.length === 0
      ? {
          title: "Publish your first product",
          description:
            "Create one paid offer so buyer agents can discover your storefront.",
          href: "/dashboard/products",
          label: "Publish your first product",
        }
      : stageCounts.needsAttention > 0
        ? {
            title: "Resolve open sales",
            description:
              "Review failed or disputed transactions before they interrupt fulfillment.",
            href: "/dashboard/transactions",
            label: `Review ${stageCounts.needsAttention} ${
              stageCounts.needsAttention === 1 ? "transaction" : "transactions"
            }`,
          }
        : snapshot.transactionCount === 0
          ? {
              title: "Prepare for your first sale",
              description:
                "Confirm your product details and share the storefront with buyers.",
              href: "/dashboard/products",
              label: "Review published products",
            }
          : {
              title: "Review recent sales",
              description:
                "Reconcile settled payments with completed fulfillment.",
              href: "/dashboard/transactions",
              label: "Review recent sales",
            };

  return (
    <div className={styles.workspace}>
      <header className={styles.pageHeader}>
        <div>
          <p className={styles.eyebrow}>Seller workspace</p>
          <h1>Commerce overview</h1>
          <p className={styles.welcome}>
            <strong>{session.principal.name}</strong>
            <span>Sales, settlement, and delivery at a glance.</span>
          </p>
        </div>
        <div className={styles.networkStatus} data-degraded={!!snapshot.error}>
          <span aria-hidden="true" />
          {snapshot.error ? "Reporting degraded" : "Systems operational"}
        </div>
      </header>

      <section className={styles.storeHealth} aria-label="Store health">
        <div>
          <span>Publication</span>
          <strong>
            {onboarding.publication.allowed ? "Ready" : "Blocked"}
          </strong>
          <small>
            {onboarding.publication.allowed
              ? "Server checks allow an approved product to go live."
              : "Complete the next setup action before publishing."}
          </small>
        </div>
        <div>
          <span>Integration</span>
          <strong>
            {onboarding.integrationVerification
              ? onboarding.integrationVerification.valid
                ? "Pass"
                : "Fail"
              : "Not checked"}
          </strong>
          <small>
            {onboarding.integrationVerification?.valid
              ? `Verified for product version ${onboarding.integrationVerification.routeVersion}.`
              : "AgentPay has not recorded a passing check for the current product version."}
          </small>
        </div>
      </section>

      <section className={styles.metrics} aria-label="Commerce metrics">
        <article
          role="status"
          aria-label={`${snapshot.transactionCount} ${
            snapshot.transactionCount === 1 ? "transaction" : "transactions"
          }`}
        >
          <span>Transactions</span>
          <strong>{snapshot.transactionCount}</strong>
          <small>Recorded in this window</small>
        </article>
        <article>
          <span>Fulfilled</span>
          <strong>{stageCounts.fulfilled}</strong>
          <small>Delivered successfully</small>
        </article>
        <article>
          <span>Processing</span>
          <strong>{stageCounts.processing}</strong>
          <small>Moving through settlement</small>
        </article>
        <article data-attention={stageCounts.needsAttention > 0}>
          <span>Needs attention</span>
          <strong>{stageCounts.needsAttention}</strong>
          <small>Failed or disputed</small>
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

      <section className={styles.primaryGrid} aria-label="Operating overview">
        <section
          className={styles.briefPanel}
          aria-label="Recommended next step"
        >
          <div>
            <p className={styles.sectionLabel}>Recommended next step</p>
            <h2>{recommendedAction.title}</h2>
            <p className={styles.briefDescription}>
              {recommendedAction.description}
            </p>
          </div>
          <Link
            href={recommendedAction.href}
            aria-label={recommendedAction.label}
            className={styles.primaryAction}
          >
            {recommendedAction.label}
            <ArrowRight aria-hidden="true" />
          </Link>
          <dl className={styles.briefFacts}>
            <div>
              <dt>Products</dt>
              <dd>{snapshot.routes.length}</dd>
            </div>
            <div>
              <dt>Payment pairs</dt>
              <dd>{paymentPairs.length}</dd>
            </div>
          </dl>
        </section>

        <article className={styles.activityPanel}>
          <div className={styles.panelHeading}>
            <div>
              <p>Seven-day performance</p>
              <h2>Settlement activity</h2>
            </div>
            <span>Daily stage count · UTC</span>
          </div>
          {dailyActivity.length > 0 ? (
            <DailyActivityChart activity={dailyActivity} />
          ) : (
            <div className={styles.compactEmpty}>
              <strong>No activity yet</strong>
              <span>Your first verified purchase will appear here.</span>
            </div>
          )}
        </article>
      </section>

      <section className={styles.settlementPanel} aria-label="Payment pairs">
        <div className={styles.settlementHeading}>
          <div>
            <p>Settlement book</p>
            <h2>Funds by asset and network</h2>
          </div>
          <Link href="/dashboard/analytics" aria-label="Open analytics">
            Open analytics <ArrowUpRight aria-hidden="true" />
          </Link>
        </div>

        {paymentPairs.length > 0 ? (
          <div className={styles.pairList}>
            <div className={styles.pairColumns} aria-hidden="true">
              <span>Payment pair</span>
              <span>Verified</span>
              <span>Fulfilled</span>
              <span>Status</span>
            </div>
            {paymentPairs.map((paymentPair) => {
              const requiresReview =
                BigInt(paymentPair.failedAmount) > 0n ||
                BigInt(paymentPair.disputedAmount) > 0n;

              return (
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
                  <span
                    className={styles.pairStatus}
                    data-review={requiresReview}
                  >
                    {requiresReview ? "Review" : "Clear"}
                  </span>
                </section>
              );
            })}
          </div>
        ) : (
          <div className={styles.emptyPairs}>
            <strong>No settlements yet</strong>
            <span>
              Verified payments will appear here by asset and network.
            </span>
          </div>
        )}
      </section>

      <section
        aria-labelledby="seller-operations"
        className={styles.operations}
      >
        <div className={styles.sectionHeading}>
          <div>
            <p>Workspace shortcuts</p>
            <h2 id="seller-operations">Operate your storefront</h2>
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
