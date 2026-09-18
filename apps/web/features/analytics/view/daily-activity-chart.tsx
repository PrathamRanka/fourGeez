"use client";

import { Area, AreaChart, CartesianGrid, XAxis, YAxis } from "recharts";
import {
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
  type ChartConfig,
} from "@/components/ui/chart";
import type { DailyActivity } from "@/features/analytics/model";
import styles from "./daily-activity-chart.module.css";

const chartConfig = {
  fulfilled: { label: "Fulfilled", color: "#f2f2ef" },
  processing: { label: "Processing", color: "#779fff" },
  failed: { label: "Failed", color: "#a46772" },
  disputed: { label: "Disputed", color: "#c69af2" },
} satisfies ChartConfig;

const legendItems = [
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
  return (
    <div className={styles.frame}>
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

      <div role="img" aria-label="Daily sales activity">
        <ChartContainer
          config={chartConfig}
          className={styles.chart}
          initialDimension={{ width: 720, height: 320 }}
        >
          <AreaChart
            accessibilityLayer
            data={activity}
            margin={{ left: 2, right: 8, top: 8, bottom: 0 }}
          >
            <defs>
              {legendItems.map((item) => (
                <linearGradient
                  id={`${item.key}-fill`}
                  key={item.key}
                  x1="0"
                  y1="0"
                  x2="0"
                  y2="1"
                >
                  <stop
                    offset="0%"
                    stopColor={`var(--color-${item.key})`}
                    stopOpacity={item.key === "processing" ? 0.4 : 0.32}
                  />
                  <stop
                    offset="100%"
                    stopColor={`var(--color-${item.key})`}
                    stopOpacity={0.02}
                  />
                </linearGradient>
              ))}
            </defs>
            <CartesianGrid
              vertical={false}
              stroke="#292929"
              strokeDasharray="2 5"
            />
            <XAxis
              dataKey="bucketDate"
              tickLine={false}
              axisLine={false}
              tickMargin={12}
              minTickGap={24}
            />
            <YAxis
              allowDecimals={false}
              tickLine={false}
              axisLine={false}
              width={28}
            />
            <ChartTooltip
              cursor={{ stroke: "#55555c", strokeWidth: 1 }}
              content={
                <ChartTooltipContent
                  className={styles.tooltip}
                  labelKey="bucketDate"
                  indicator="line"
                />
              }
            />
            {legendItems.map((item) => (
              <Area
                key={item.key}
                type="monotone"
                dataKey={item.key}
                stackId="activity"
                stroke={`var(--color-${item.key})`}
                fill={`url(#${item.key}-fill)`}
                strokeWidth={item.key === "fulfilled" ? 1.6 : 1.3}
                dot={false}
                activeDot={{ r: 3, strokeWidth: 0 }}
              />
            ))}
          </AreaChart>
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
          {activity.map((day) => (
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
