import {
  buildRecentCountSparkline,
  buildRecentSumSparkline,
  buildSparklineTrend,
} from "./metric-sparkline";

describe("metric sparkline helpers", () => {
  const now = "2026-04-01T12:00:00.000Z";

  it("builds seven-day count series from timestamps", () => {
    const sparkline = buildRecentCountSparkline(
      [
        { timestamp: "2026-04-01T08:00:00.000Z" },
        { timestamp: "2026-03-31T08:00:00.000Z" },
        { timestamp: "2026-03-31T16:00:00.000Z" },
        { timestamp: "2026-03-28T11:00:00.000Z" },
      ],
      { now },
    );

    expect(sparkline).toEqual([
      { label: "2026-03-26", value: 0 },
      { label: "2026-03-27", value: 0 },
      { label: "2026-03-28", value: 1 },
      { label: "2026-03-29", value: 0 },
      { label: "2026-03-30", value: 0 },
      { label: "2026-03-31", value: 2 },
      { label: "2026-04-01", value: 1 },
    ]);
  });

  it("builds seven-day sum series from amounts", () => {
    const sparkline = buildRecentSumSparkline(
      [
        { timestamp: "2026-04-01T08:00:00.000Z", amount: 10.25 },
        { timestamp: "2026-03-31T16:00:00.000Z", amount: 3.5 },
        { timestamp: "2026-03-31T09:00:00.000Z", amount: 1.25 },
      ],
      { now },
    );

    expect(sparkline.at(-2)).toEqual({ label: "2026-03-31", value: 4.75 });
    expect(sparkline.at(-1)).toEqual({ label: "2026-04-01", value: 10.25 });
  });

  it("derives trend direction and percentage from sparkline points", () => {
    expect(
      buildSparklineTrend([
        { label: "2026-03-31", value: 4 },
        { label: "2026-04-01", value: 6 },
      ]),
    ).toEqual({ direction: "up", value: 50 });

    expect(
      buildSparklineTrend([
        { label: "2026-03-31", value: 6 },
        { label: "2026-04-01", value: 3 },
      ]),
    ).toEqual({ direction: "down", value: 50 });

    expect(
      buildSparklineTrend([
        { label: "2026-03-31", value: 6 },
        { label: "2026-04-01", value: 6 },
      ]),
    ).toEqual({ direction: "flat", value: 0 });
  });
});
