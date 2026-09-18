import {
  AlertTriangle,
  ArrowUpRight,
  CircleCheck,
  RefreshCw,
  Scale,
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

const metrics = [
  { key: "grossVerifiedAmount", label: "Gross verified", icon: ArrowUpRight },
  { key: "fulfilledAmount", label: "Fulfilled sales", icon: CircleCheck },
  { key: "failedAmount", label: "Failures", icon: AlertTriangle },
  { key: "disputedAmount", label: "Disputes", icon: Scale },
] as const;

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

  return (
    <div className={styles.workspace}>
      <header className={styles.hero}>
        <div>
          <p className={styles.eyebrow}>Revenue intelligence / UTC</p>
          <h1>Revenue Lens</h1>
          <p>
            Verified commerce facts. Every asset and network stays distinct.
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
        <section className={styles.emptyState}>
          <div>
            <p className={styles.sectionIndex}>No recorded volume</p>
            <h2>No sales in this window yet</h2>
            <p>
              Publish a validated product. Verified purchases will appear here.
            </p>
          </div>
          <Link href="/dashboard/products">
            Review products <ArrowUpRight aria-hidden="true" />
          </Link>
        </section>
      ) : (
        <>
          <section aria-labelledby="settlement-pairs" className={styles.pairs}>
            <header className={styles.sectionHeading}>
              <div>
                <p className={styles.sectionIndex}>01 / Settlement pairs</p>
                <h2 id="settlement-pairs">Revenue without false totals.</h2>
              </div>
              <span>{paymentPairs.length} active pairs</span>
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
                      <span>{summary.network}</span>
                    </div>
                    <span>Settlement pair</span>
                  </header>
                  <div className={styles.metricGrid}>
                    {metrics.map((metric) => {
                      const Icon = metric.icon;
                      return (
                        <div className={styles.metric} key={metric.key}>
                          <Icon aria-hidden="true" />
                          <p>{metric.label}</p>
                          <strong>
                            {formatAtomicPrice(
                              summary[metric.key],
                              summary.asset,
                            )}
                          </strong>
                        </div>
                      );
                    })}
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
                <p className={styles.sectionIndex}>02 / Reconciliation flow</p>
                <h2 id="daily-activity">Daily sales activity</h2>
              </div>
              <span>Count by current stage / UTC</span>
            </div>
            <DailyActivityChart activity={dailyActivity} />
          </section>

          <section
            className={styles.routePanel}
            aria-labelledby="product-performance"
          >
            <div className={styles.sectionHeading}>
              <div>
                <p className={styles.sectionIndex}>03 / Products</p>
                <h2 id="product-performance">Product performance</h2>
              </div>
              <span>Fulfillment by exact payment pair</span>
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
                      <th>Failures</th>
                      <th>Disputes</th>
                    </tr>
                  </thead>
                  <tbody>
                    {routePerformance.map((route) => (
                      <tr
                        key={`${route.routeId}:${route.asset}:${route.network}`}
                      >
                        <td>
                          <strong>{route.routeLabel}</strong>
                          <small>{route.routeId}</small>
                        </td>
                        <td>
                          <strong>{route.asset}</strong>
                          <small>{route.network}</small>
                        </td>
                        <td>{route.fulfilledCount} fulfilled</td>
                        <td>
                          {formatAtomicPrice(
                            route.fulfilledAmount,
                            route.asset,
                          )}
                        </td>
                        <td>{route.failedCount}</td>
                        <td>{route.disputedCount}</td>
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
