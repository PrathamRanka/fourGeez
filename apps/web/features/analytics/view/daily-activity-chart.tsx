"use client";

import { Bar, BarChart, CartesianGrid, XAxis } from "recharts";
import {
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
  type ChartConfig,
} from "@/components/ui/chart";
import type { DailyActivity } from "@/features/analytics/model";

const chartConfig = {
  fulfilled: { label: "Fulfilled", color: "var(--chart-3)" },
  processing: { label: "Processing", color: "var(--chart-2)" },
  failed: { label: "Failed", color: "var(--chart-4)" },
  disputed: { label: "Disputed", color: "var(--chart-5)" },
} satisfies ChartConfig;

type DailyActivityChartProps = {
  activity: DailyActivity[];
};

// DailyActivityChart is the isolated client boundary for the Recharts runtime.
export function DailyActivityChart({ activity }: DailyActivityChartProps) {
  return (
    <div role="img" aria-label="Daily sales activity">
      <ChartContainer
        config={chartConfig}
        className="analytics-chart"
        initialDimension={{ width: 720, height: 280 }}
      >
        <BarChart accessibilityLayer data={activity}>
          <CartesianGrid vertical={false} />
          <XAxis
            dataKey="bucketDate"
            tickLine={false}
            axisLine={false}
            tickMargin={10}
          />
          <ChartTooltip content={<ChartTooltipContent />} />
          <Bar
            dataKey="fulfilled"
            stackId="activity"
            fill="var(--color-fulfilled)"
          />
          <Bar
            dataKey="processing"
            stackId="activity"
            fill="var(--color-processing)"
          />
          <Bar
            dataKey="failed"
            stackId="activity"
            fill="var(--color-failed)"
          />
          <Bar
            dataKey="disputed"
            stackId="activity"
            fill="var(--color-disputed)"
            radius={[4, 4, 0, 0]}
          />
        </BarChart>
      </ChartContainer>
    </div>
  );
}
