import { AlertTriangle, ArrowUpRight, CircleCheck, Scale } from "lucide-react";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  buildDailyActivity,
  buildPaymentPairSummaries,
  buildRoutePerformance,
  type AnalyticsSnapshot,
} from "@/features/analytics/model";
import { DailyActivityChart } from "@/features/analytics/view/daily-activity-chart";
import { formatAtomicPrice } from "@/lib/money";

type SellerAnalyticsDashboardProps = {
  snapshot: AnalyticsSnapshot;
};

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
    <div className="analytics-workspace">
      <header className="analytics-header">
        <div>
          <p className="dashboard-eyebrow">Seller reporting</p>
          <h1>Revenue Lens</h1>
          <p>
            Verified payment and fulfillment facts, separated by asset and
            network. Unlike currencies are never combined.
          </p>
        </div>
        <span>{snapshot.transactionCount} transactions in this window</span>
      </header>

      {snapshot.error ? (
        <div className="dashboard-error" role="alert">
          {snapshot.error}
        </div>
      ) : null}

      {paymentPairs.length === 0 ? (
        <Card className="analytics-empty-state">
          <CardHeader>
            <CardTitle>No sales in this window yet</CardTitle>
            <CardDescription>
              Publish a validated product, then completed purchases will appear
              here once the gateway records them.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <Button
              render={
                <Link
                  href={`/dashboard/products?sellerId=${encodeURIComponent(snapshot.sellerId)}`}
                />
              }
            >
              Review products
              <ArrowUpRight aria-hidden="true" />
            </Button>
          </CardContent>
        </Card>
      ) : (
        <>
          <div className="analytics-pair-grid">
            {paymentPairs.map((summary) => (
              <section
                key={`${summary.asset}:${summary.network}`}
                className="analytics-pair"
                aria-label={`${summary.asset} on ${summary.network}`}
              >
                <header>
                  <div>
                    <strong>{summary.asset}</strong>
                    <span>{summary.network}</span>
                  </div>
                  <span>Settlement pair</span>
                </header>
                <div className="analytics-metric-grid">
                  <Metric
                    label="Gross verified"
                    value={formatAtomicPrice(
                      summary.grossVerifiedAmount,
                      summary.asset,
                    )}
                    icon={<ArrowUpRight aria-hidden="true" />}
                  />
                  <Metric
                    label="Fulfilled sales"
                    value={formatAtomicPrice(
                      summary.fulfilledAmount,
                      summary.asset,
                    )}
                    icon={<CircleCheck aria-hidden="true" />}
                  />
                  <Metric
                    label="Failures"
                    value={formatAtomicPrice(
                      summary.failedAmount,
                      summary.asset,
                    )}
                    icon={<AlertTriangle aria-hidden="true" />}
                  />
                  <Metric
                    label="Disputes"
                    value={formatAtomicPrice(
                      summary.disputedAmount,
                      summary.asset,
                    )}
                    icon={<Scale aria-hidden="true" />}
                  />
                </div>
              </section>
            ))}
          </div>

          <section className="analytics-chart-panel">
            <div className="analytics-section-heading">
              <div>
                <h2>Daily sales activity</h2>
                <p>Transaction counts by current reconciliation stage.</p>
              </div>
              <span>UTC</span>
            </div>
            <DailyActivityChart activity={dailyActivity} />
          </section>

          <section className="analytics-route-panel">
            <div className="analytics-section-heading">
              <div>
                <h2>Product performance</h2>
                <p>Fulfillment outcomes remain separated by payment pair.</p>
              </div>
            </div>
            {routePerformance.length === 0 ? (
              <p className="analytics-muted-state">
                Route-level performance appears after the first paid request.
              </p>
            ) : (
              <div className="analytics-table-wrap">
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
                          {route.asset}
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

type MetricProps = {
  label: string;
  value: string;
  icon: React.ReactNode;
};

function Metric({ label, value, icon }: MetricProps) {
  return (
    <div className="analytics-metric">
      <span>{icon}</span>
      <p>{label}</p>
      <strong>{value}</strong>
    </div>
  );
}
