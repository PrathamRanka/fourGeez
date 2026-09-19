"use client";

import { useId } from "react";
import {
  Area,
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
import type { DailyActivity } from "@/features/analytics/model";
import styles from "./daily-activity-chart.module.css";

type DailyActivityTrend = DailyActivity & { total: number };

function buildSevenDayTrend(activity: DailyActivity[]): DailyActivityTrend[] {
  if (activity.length === 0) {
    return [];
  }

  const activityByDate = new Map(
    activity.map((dailyActivity) => [dailyActivity.bucketDate, dailyActivity]),
  );
  const latestBucketDate = activity.reduce(
    (latestDate, dailyActivity) =>
      dailyActivity.bucketDate > latestDate
        ? dailyActivity.bucketDate
        : latestDate,
    activity[0].bucketDate,
  );
  const latestDate = new Date(`${latestBucketDate}T00:00:00.000Z`);

  return Array.from({ length: 7 }, (_, index) => {
    const bucketDateValue = new Date(latestDate);
    bucketDateValue.setUTCDate(latestDate.getUTCDate() - (6 - index));
    const bucketDate = bucketDateValue.toISOString().slice(0, 10);
    const recordedActivity = activityByDate.get(bucketDate) ?? {
      bucketDate,
      fulfilled: 0,
      processing: 0,
      failed: 0,
      disputed: 0,
    };

    return {
      ...recordedActivity,
      total:
        recordedActivity.fulfilled +
        recordedActivity.processing +
        recordedActivity.failed +
        recordedActivity.disputed,
    };
  });
}

const chartConfig = {
  total: { label: "All activity", color: "var(--chart-total)" },
  fulfilled: { label: "Fulfilled", color: "var(--chart-fulfilled)" },
} satisfies ChartConfig;

const legendItems = [
  { key: "total", label: "All activity" },
  { key: "fulfilled", label: "Fulfilled" },
] as const;

type DailyActivityChartProps = {
  activity: DailyActivity[];
};

// DailyActivityChart is the isolated client boundary for the Recharts runtime.
export function DailyActivityChart({ activity }: DailyActivityChartProps) {
  const trend = buildSevenDayTrend(activity);
  const gradientId = `activity-fill-${useId().replace(/:/g, "")}`;
  const totals = trend.reduce(
    (summary, day) => ({
      activity: summary.activity + day.total,
      fulfilled: summary.fulfilled + day.fulfilled,
    }),
    { activity: 0, fulfilled: 0 },
  );
  const fulfillmentRate =
    totals.activity === 0
      ? 0
      : Math.round((totals.fulfilled / totals.activity) * 100);
  const latestBucket =
    trend.at(-1)?.bucketDate.slice(5).replace("-", "/") ?? "--";

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
            <span>Latest close</span>
            <strong>{latestBucket}</strong>
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
            margin={{ left: 0, right: 8, top: 10, bottom: 0 }}
          >
            <defs>
              <linearGradient id={gradientId} x1="0" y1="0" x2="0" y2="1">
                <stop offset="0%" stopColor="var(--color-total)" stopOpacity={0.22} />
                <stop offset="100%" stopColor="var(--color-total)" stopOpacity={0} />
              </linearGradient>
            </defs>
            <CartesianGrid
              vertical={false}
              stroke="var(--chart-grid)"
              strokeDasharray="2 6"
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
                  indicator="line"
                />
              }
            />
            <Area
              type="monotone"
              dataKey="total"
              stroke="var(--color-total)"
              fill={`url(#${gradientId})`}
              strokeWidth={2}
              dot={false}
              activeDot={{ r: 3, strokeWidth: 0 }}
              isAnimationActive={false}
            />
            <Line
              type="monotone"
              dataKey="fulfilled"
              stroke="var(--color-fulfilled)"
              strokeWidth={1.5}
              strokeDasharray="4 5"
              dot={false}
              activeDot={{ r: 3, strokeWidth: 0 }}
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
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
