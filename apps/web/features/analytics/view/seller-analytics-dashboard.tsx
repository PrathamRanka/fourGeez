import {
  ArrowUpRight,
  RefreshCw,
} from "lucide-react";
import Link from "next/link";
import {
  buildDailyActivity,
  buildPaymentPairSummaries,
  buildRoutePerformance,
  type AnalyticsSnapshot,
} from "@/features/analytics/model";
import { DailyActivityChart } from "@/features/analytics/view/daily-activity-chart";
import { formatAtomicPrice } from "@/lib/money";
import styles from "./seller-analytics-dashboard.module.css";

type SellerAnalyticsDashboardProps = {
  snapshot: AnalyticsSnapshot;
};

const paymentMetrics = [
  { key: "grossVerifiedAmount", label: "Gross verified" },
  { key: "fulfilledAmount", label: "Fulfilled volume" },
  { key: "failedAmount", label: "Failed volume" },
  { key: "disputedAmount", label: "Disputed volume" },
] as const;

const networkLabels: Readonly<Record<string, string>> = {
  "eip155:84532": "Base Sepolia",
  "eip155:11155111": "Ethereum Sepolia",
};

function networkLabel(network: string): string {
  return networkLabels[network] ?? network;
}

function routeFulfillmentRate(route: {
  fulfilledCount: number;
  failedCount: number;
  disputedCount: number;
}): number {
  const terminalCount =
    route.fulfilledCount + route.failedCount + route.disputedCount;
  return terminalCount === 0
    ? 0
    : Math.round((route.fulfilledCount / terminalCount) * 100);
}

// SellerAnalyticsDashboard presents authoritative, payment-pair-separated sales reporting.
export function SellerAnalyticsDashboard({
  snapshot,
}: SellerAnalyticsDashboardProps) {
  const paymentPairs = buildPaymentPairSummaries(snapshot.aggregates);
  const dailyActivity = buildDailyActivity(snapshot.aggregates);
  const routePerformance = buildRoutePerformance(
    snapshot.aggregates,
    snapshot.routes,
  );
  const activityTotals = dailyActivity.reduce(
    (totals, day) => ({
      fulfilled: totals.fulfilled + day.fulfilled,
      processing: totals.processing + day.processing,
      exceptions: totals.exceptions + day.failed + day.disputed,
    }),
    { fulfilled: 0, processing: 0, exceptions: 0 },
  );
  const trackedTransactions =
    activityTotals.fulfilled +
    activityTotals.processing +
    activityTotals.exceptions;
  const fulfillmentRate =
    trackedTransactions === 0
      ? 0
      : Math.round(
          (activityTotals.fulfilled / trackedTransactions) * 100,
        );
  const prioritizedRoutes = [...routePerformance].sort((left, right) => {
    const exceptionDifference =
      right.failedCount +
      right.disputedCount -
      (left.failedCount + left.disputedCount);
    if (exceptionDifference !== 0) {
      return exceptionDifference;
    }
    if (right.fulfilledCount !== left.fulfilledCount) {
      return right.fulfilledCount - left.fulfilledCount;
    }
    return left.routeLabel.localeCompare(right.routeLabel);
  });

  return (
    <div className={styles.workspace}>
      <header className={styles.hero}>
        <div>
          <p className={styles.eyebrow}>Revenue Lens / trailing 30 days / UTC</p>
          <h1>Revenue Lens</h1>
          <p>
            Read settlement health, exceptions, and product performance without
            combining unlike assets.
          </p>
        </div>
        <div
          className={styles.windowCount}
          role="status"
          aria-label={`${snapshot.transactionCount} transactions in this window`}
        >
          <strong>{snapshot.transactionCount}</strong>
          <span>transactions in this window</span>
        </div>
      </header>

      {snapshot.error ? (
        <section className={styles.errorState} role="alert">
          <div className={styles.errorIcon} aria-hidden="true">
            <RefreshCw />
          </div>
          <div>
            <p className={styles.sectionIndex}>Reporting unavailable</p>
            <h2>Live totals could not be loaded.</h2>
            <p>{snapshot.error}</p>
          </div>
          <Link href="/dashboard/analytics">Reload analytics</Link>
        </section>
      ) : paymentPairs.length === 0 ? (
        <section
          className={styles.emptyState}
          role="status"
          aria-label="Analytics empty state"
        >
          <div>
            <p className={styles.sectionIndex}>No recorded volume</p>
            <h2>No sales in this window yet</h2>
            <p>
              Publish and validate a product to begin tracking verified sales.
            </p>
          </div>
          <Link href="/dashboard/products">
            Review products <ArrowUpRight aria-hidden="true" />
          </Link>
        </section>
      ) : (
        <>
          <section
            className={styles.healthPanel}
            aria-labelledby="sales-health"
          >
            <div className={styles.healthHeading}>
              <div>
                <p className={styles.sectionIndex}>Operating pulse</p>
                <h2 id="sales-health">Sales health</h2>
              </div>
              <span>Current reconciliation state</span>
            </div>
            <div className={styles.healthMetrics}>
              <article>
                <span>Fulfillment rate</span>
                <strong>{fulfillmentRate}%</strong>
                <small>
                  {activityTotals.fulfilled} of {trackedTransactions} tracked
                </small>
              </article>
              <article>
                <span>In flight</span>
                <strong>{activityTotals.processing} in flight</strong>
                <small>Awaiting a terminal outcome</small>
              </article>
              <article data-attention={activityTotals.exceptions > 0}>
                <span>Exceptions</span>
                <strong>{activityTotals.exceptions} need review</strong>
                <small>Failed or disputed sales</small>
              </article>
              <Link
                className={styles.healthAction}
                href="/dashboard/transactions"
              >
                <span>
                  {activityTotals.exceptions > 0
                    ? `Review ${activityTotals.exceptions} exceptions`
                    : "Review transactions"}
                </span>
                <ArrowUpRight aria-hidden="true" />
              </Link>
            </div>
          </section>

          <section aria-labelledby="settlement-pairs" className={styles.pairs}>
            <header className={styles.sectionHeading}>
              <div>
                <p className={styles.sectionIndex}>Settlement comparison</p>
                <h2 id="settlement-pairs">Money by payment pair</h2>
              </div>
              <span>{paymentPairs.length} active payment pairs</span>
            </header>
            <div className={styles.pairGrid}>
              {paymentPairs.map((summary, pairIndex) => (
                <section
                  key={`${summary.asset}:${summary.network}`}
                  className={styles.pair}
                  data-accent={pairIndex === 0 ? "true" : undefined}
                  aria-label={`${summary.asset} on ${summary.network}`}
                >
                  <header>
                    <div>
                      <strong>{summary.asset}</strong>
                      <span>{networkLabel(summary.network)}</span>
                    </div>
                    <span>{summary.network}</span>
                  </header>
                  <div className={styles.metricGrid}>
                    {paymentMetrics.map((metric) => (
                      <div className={styles.metric} key={metric.key}>
                        <p>{metric.label}</p>
                        <strong>
                          {formatAtomicPrice(
                            summary[metric.key],
                            summary.asset,
                          )}
                        </strong>
                      </div>
                    ))}
                  </div>
                </section>
              ))}
            </div>
          </section>

          <section
            className={styles.chartPanel}
            aria-labelledby="daily-activity"
          >
            <div className={styles.sectionHeading}>
              <div>
                <p className={styles.sectionIndex}>Reconciliation flow</p>
                <h2 id="daily-activity">Daily sales activity</h2>
              </div>
              <span>Seven daily closes / current stage</span>
            </div>
            <DailyActivityChart activity={dailyActivity} />
          </section>

          <section
            className={styles.routePanel}
            aria-labelledby="product-performance"
          >
            <div className={styles.sectionHeading}>
              <div>
                <p className={styles.sectionIndex}>Product decisions</p>
                <h2 id="product-performance">Product performance</h2>
              </div>
              <span>Exceptions first / payment pairs remain separate</span>
            </div>
            {routePerformance.length === 0 ? (
              <div className={styles.mutedState}>
                Route-level performance appears after the first paid request.
              </div>
            ) : (
              <div className={styles.tableWrap}>
                <table aria-label="Product performance">
                  <thead>
                    <tr>
                      <th>Product</th>
                      <th>Payment pair</th>
                      <th>Fulfilled</th>
                      <th>Volume</th>
                      <th>Success rate</th>
                      <th>Failures</th>
                      <th>Disputes</th>
                    </tr>
                  </thead>
                  <tbody>
                    {prioritizedRoutes.map((route) => (
                      <tr
                        key={`${route.routeId}:${route.asset}:${route.network}`}
                      >
                        <td data-label="Product" className={styles.productCell}>
                          <strong>{route.routeLabel}</strong>
                          <small>{route.routeId}</small>
                        </td>
                        <td data-label="Payment pair" className={styles.pairCell}>
                          <strong>{route.asset}</strong>
                          <small>
                            {networkLabel(route.network)} / {route.network}
                          </small>
                        </td>
                        <td data-label="Fulfilled">
                          {route.fulfilledCount} fulfilled
                        </td>
                        <td data-label="Volume">
                          {formatAtomicPrice(
                            route.fulfilledAmount,
                            route.asset,
                          )}
                        </td>
                        <td data-label="Success rate">
                          {routeFulfillmentRate(route)}%
                        </td>
                        <td data-label="Failures">{route.failedCount}</td>
                        <td data-label="Disputes">{route.disputedCount}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </section>
        </>
      )}
    </div>
  );
}
