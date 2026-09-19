export type ReconciliationStage =
  "challenged" | "verified" | "finalized" | "fulfilled" | "failed" | "disputed";

export type SalesAggregate = {
  sellerId: string;
  bucketDate: string;
  asset: string;
  network: string;
  routeId?: string;
  stage: ReconciliationStage;
  transactionCount: number;
  amount: string;
  lastTransactionAt: string;
};

export type AnalyticsRoute = {
  routeId: string;
  displayName: string;
  pathPattern: string;
};

export type AnalyticsSnapshot = {
  sellerId: string;
  transactionCount: number;
  aggregates: SalesAggregate[];
  routes: AnalyticsRoute[];
  error?: string;
};

export type PaymentPairSummary = {
  asset: string;
  network: string;
  grossVerifiedAmount: string;
  fulfilledAmount: string;
  failedAmount: string;
  disputedAmount: string;
};

export type DailyActivity = {
  bucketDate: string;
  fulfilled: number;
  failed: number;
  disputed: number;
  processing: number;
};

export type DailyActivityTrend = DailyActivity & {
  total: number;
};

export type RoutePerformance = {
  routeId: string;
  routeLabel: string;
  asset: string;
  network: string;
  fulfilledAmount: string;
  fulfilledCount: number;
  failedCount: number;
  disputedCount: number;
};

const grossVerifiedStages = new Set<ReconciliationStage>([
  "verified",
  "finalized",
  "fulfilled",
  "disputed",
]);

function addAtomicAmount(current: string, amount: string): string {
  return (BigInt(current) + BigInt(amount)).toString();
}

// buildPaymentPairSummaries reduces only seller-wide rows so route buckets are not counted twice.
export function buildPaymentPairSummaries(
  aggregates: SalesAggregate[],
): PaymentPairSummary[] {
  const summaries = new Map<string, PaymentPairSummary>();

  for (const aggregate of aggregates) {
    if (aggregate.routeId) {
      continue;
    }
    const key = `${aggregate.asset}\u0000${aggregate.network}`;
    const summary = summaries.get(key) ?? {
      asset: aggregate.asset,
      network: aggregate.network,
      grossVerifiedAmount: "0",
      fulfilledAmount: "0",
      failedAmount: "0",
      disputedAmount: "0",
    };
    if (grossVerifiedStages.has(aggregate.stage)) {
      summary.grossVerifiedAmount = addAtomicAmount(
        summary.grossVerifiedAmount,
        aggregate.amount,
      );
    }
    if (aggregate.stage === "fulfilled") {
      summary.fulfilledAmount = addAtomicAmount(
        summary.fulfilledAmount,
        aggregate.amount,
      );
    } else if (aggregate.stage === "failed") {
      summary.failedAmount = addAtomicAmount(
        summary.failedAmount,
        aggregate.amount,
      );
    } else if (aggregate.stage === "disputed") {
      summary.disputedAmount = addAtomicAmount(
        summary.disputedAmount,
        aggregate.amount,
      );
    }
    summaries.set(key, summary);
  }

  return [...summaries.values()].sort((left, right) =>
    `${left.asset}:${left.network}`.localeCompare(
      `${right.asset}:${right.network}`,
    ),
  );
}

// buildDailyActivity prepares count-only chart values without converting money to floating point.
export function buildDailyActivity(
  aggregates: SalesAggregate[],
): DailyActivity[] {
  const days = new Map<string, DailyActivity>();
  for (const aggregate of aggregates) {
    if (aggregate.routeId) {
      continue;
    }
    const day = days.get(aggregate.bucketDate) ?? {
      bucketDate: aggregate.bucketDate,
      fulfilled: 0,
      failed: 0,
      disputed: 0,
      processing: 0,
    };
    if (aggregate.stage === "fulfilled") {
      day.fulfilled += aggregate.transactionCount;
    } else if (aggregate.stage === "failed") {
      day.failed += aggregate.transactionCount;
    } else if (aggregate.stage === "disputed") {
      day.disputed += aggregate.transactionCount;
    } else {
      day.processing += aggregate.transactionCount;
    }
    days.set(aggregate.bucketDate, day);
  }
  return [...days.values()].sort((left, right) =>
    left.bucketDate.localeCompare(right.bucketDate),
  );
}

// buildSevenDayActivity preserves recorded facts while filling unrecorded dates with zero activity.
export function buildSevenDayActivity(
  activity: DailyActivity[],
): DailyActivityTrend[] {
  if (activity.length === 0) {
    return [];
  }

  const byDate = new Map(activity.map((day) => [day.bucketDate, day]));
  const latestDate = activity.reduce((latest, day) =>
    day.bucketDate > latest ? day.bucketDate : latest,
  activity[0].bucketDate);
  const latest = new Date(`${latestDate}T00:00:00.000Z`);

  return Array.from({ length: 7 }, (_, index) => {
    const date = new Date(latest);
    date.setUTCDate(latest.getUTCDate() - (6 - index));
    const bucketDate = date.toISOString().slice(0, 10);
    const recorded = byDate.get(bucketDate) ?? {
      bucketDate,
      fulfilled: 0,
      processing: 0,
      failed: 0,
      disputed: 0,
    };

    return {
      ...recorded,
      total:
        recorded.fulfilled +
        recorded.processing +
        recorded.failed +
        recorded.disputed,
    };
  });
}

// buildRoutePerformance uses route-specific rows and keeps unlike payment pairs separate.
export function buildRoutePerformance(
  aggregates: SalesAggregate[],
  routes: AnalyticsRoute[],
): RoutePerformance[] {
  const routeLabels = new Map(
    routes.map((route) => [route.routeId, route.displayName]),
  );
  const performance = new Map<string, RoutePerformance>();

  for (const aggregate of aggregates) {
    if (!aggregate.routeId) {
      continue;
    }
    const key = `${aggregate.routeId}\u0000${aggregate.asset}\u0000${aggregate.network}`;
    const row = performance.get(key) ?? {
      routeId: aggregate.routeId,
      routeLabel: routeLabels.get(aggregate.routeId) ?? aggregate.routeId,
      asset: aggregate.asset,
      network: aggregate.network,
      fulfilledAmount: "0",
      fulfilledCount: 0,
      failedCount: 0,
      disputedCount: 0,
    };
    if (aggregate.stage === "fulfilled") {
      row.fulfilledAmount = addAtomicAmount(
        row.fulfilledAmount,
        aggregate.amount,
      );
      row.fulfilledCount += aggregate.transactionCount;
    } else if (aggregate.stage === "failed") {
      row.failedCount += aggregate.transactionCount;
    } else if (aggregate.stage === "disputed") {
      row.disputedCount += aggregate.transactionCount;
    }
    performance.set(key, row);
  }

  return [...performance.values()].sort((left, right) =>
    left.routeLabel.localeCompare(right.routeLabel),
  );
}
