"use client";

import {
  Bar,
  CartesianGrid,
  ComposedChart,
  Line,
  XAxis,
  YAxis,
} from "recharts";
import {
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
  type ChartConfig,
} from "@/components/ui/chart";
import {
  buildSevenDayActivity,
  type DailyActivity,
} from "@/features/analytics/model";
import styles from "./daily-activity-chart.module.css";

const chartConfig = {
  total: { label: "All activity", color: "var(--chart-total)" },
  fulfilled: { label: "Fulfilled", color: "var(--chart-fulfilled)" },
  processing: { label: "Processing", color: "var(--chart-processing)" },
  failed: { label: "Failed", color: "var(--chart-failed)" },
  disputed: { label: "Disputed", color: "var(--chart-disputed)" },
} satisfies ChartConfig;

const legendItems = [
  { key: "total", label: "All activity" },
  { key: "fulfilled", label: "Fulfilled" },
  { key: "processing", label: "Processing" },
  { key: "failed", label: "Failed" },
  { key: "disputed", label: "Disputed" },
] as const;

type DailyActivityChartProps = {
  activity: DailyActivity[];
};

// DailyActivityChart is the isolated client boundary for the Recharts runtime.
export function DailyActivityChart({ activity }: DailyActivityChartProps) {
  const trend = buildSevenDayActivity(activity);
  const totals = trend.reduce(
    (summary, day) => ({
      activity: summary.activity + day.total,
      fulfilled: summary.fulfilled + day.fulfilled,
      exceptions: summary.exceptions + day.failed + day.disputed,
    }),
    { activity: 0, fulfilled: 0, exceptions: 0 },
  );
  const fulfillmentRate =
    totals.activity === 0
      ? 0
      : Math.round((totals.fulfilled / totals.activity) * 100);

  return (
    <div className={styles.frame}>
      <div className={styles.chartTopline}>
        <div
          className={styles.summary}
          role="group"
          aria-label="Seven-day activity summary"
        >
          <div>
            <span>7-day activity</span>
            <strong>{totals.activity}</strong>
          </div>
          <div>
            <span>Fulfillment rate</span>
            <strong>{fulfillmentRate}%</strong>
          </div>
          <div>
            <span>Exceptions</span>
            <strong>{totals.exceptions}</strong>
          </div>
        </div>
        <div
          className={styles.legend}
          role="list"
          aria-label="Reconciliation stages"
        >
          {legendItems.map((item) => (
            <span role="listitem" key={item.key}>
              <i data-series={item.key} aria-hidden="true" />
              {item.label}
            </span>
          ))}
        </div>
      </div>

      <div role="img" aria-label="Daily sales activity">
        <ChartContainer
          config={chartConfig}
          className={styles.chart}
          initialDimension={{ width: 720, height: 320 }}
        >
          <ComposedChart
            accessibilityLayer
            data={trend}
            margin={{ left: 0, right: 12, top: 16, bottom: 0 }}
          >
            <CartesianGrid
              vertical={false}
              stroke="var(--chart-grid)"
            />
            <XAxis
              dataKey="bucketDate"
              tickLine={false}
              axisLine={false}
              tickMargin={12}
              minTickGap={24}
              tickFormatter={(value: string) => value.slice(5).replace("-", "/")}
            />
            <YAxis
              allowDecimals={false}
              tickLine={false}
              axisLine={false}
              width={28}
            />
            <ChartTooltip
              cursor={{ stroke: "var(--chart-crosshair)", strokeWidth: 1 }}
              content={
                <ChartTooltipContent
                  className={styles.tooltip}
                  labelKey="bucketDate"
                  indicator="dot"
                />
              }
            />
            <Bar
              dataKey="fulfilled"
              stackId="activity"
              fill="var(--color-fulfilled)"
              maxBarSize={28}
              isAnimationActive={false}
            />
            <Bar
              dataKey="processing"
              stackId="activity"
              fill="var(--color-processing)"
              maxBarSize={28}
              isAnimationActive={false}
            />
            <Bar
              dataKey="failed"
              stackId="activity"
              fill="var(--color-failed)"
              maxBarSize={28}
              isAnimationActive={false}
            />
            <Bar
              dataKey="disputed"
              stackId="activity"
              fill="var(--color-disputed)"
              maxBarSize={28}
              isAnimationActive={false}
            />
            <Line
              type="monotone"
              dataKey="total"
              stroke="var(--color-total)"
              strokeWidth={2}
              dot={false}
              activeDot={{ r: 3, strokeWidth: 1 }}
              isAnimationActive={false}
            />
          </ComposedChart>
        </ChartContainer>
      </div>

      <table
        className={styles.accessibleTable}
        aria-label="Daily reconciliation totals"
      >
        <thead>
          <tr>
            <th>Date</th>
            <th>Fulfilled</th>
            <th>Processing</th>
            <th>Failed</th>
            <th>Disputed</th>
            <th>Total</th>
          </tr>
        </thead>
        <tbody>
          {trend.map((day) => (
            <tr key={day.bucketDate}>
              <th scope="row">{day.bucketDate}</th>
              <td>{day.fulfilled}</td>
              <td>{day.processing}</td>
              <td>{day.failed}</td>
              <td>{day.disputed}</td>
              <td>{day.total}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
