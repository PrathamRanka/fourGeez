import { describe, expect, it } from "vitest";
import { buildSevenDayActivity } from "@/features/analytics/model";

describe("analytics activity window", () => {
  it("pads a single recorded day into a truthful seven-day trend", () => {
    const trend = buildSevenDayActivity([
      {
        bucketDate: "2026-09-17",
        fulfilled: 1,
        processing: 2,
        failed: 0,
        disputed: 1,
      },
    ]);

    expect(trend).toHaveLength(7);
    expect(trend[0]).toMatchObject({ bucketDate: "2026-09-11", total: 0 });
    expect(trend[6]).toMatchObject({
      bucketDate: "2026-09-17",
      fulfilled: 1,
      total: 4,
    });
  });
});
