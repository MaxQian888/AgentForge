jest.mock("next-intl", () => ({
  useTranslations: () => (key: string) => {
    const map: Record<string, string> = {
      "stats.totalJobs": "Total Jobs",
      "stats.enabled": "Enabled",
      "stats.paused": "Paused",
      "stats.activeRuns": "Active Runs",
      "stats.queueDepth": "Queue Depth",
      "stats.failed24h": "Failed 24h",
      "stats.avgDuration": "Avg Duration",
      "stats.successRate24h": "Success Rate 24h",
    };
    return map[key] ?? key;
  },
}));

import { render, screen } from "@testing-library/react";
import { SchedulerStatsCards } from "./scheduler-stats-cards";

describe("SchedulerStatsCards", () => {
  it("renders loading placeholders when stats are pending", () => {
    const { container } = render(
      <SchedulerStatsCards stats={null} loading />,
    );

    expect(container.querySelectorAll('[data-slot="skeleton"]')).toHaveLength(8);
  });

  it("renders richer operator metrics and accent colors from stats", () => {
    render(
      <SchedulerStatsCards
        loading={false}
        stats={{
          totalJobs: 6,
          enabledJobs: 5,
          disabledJobs: 1,
          pausedJobs: 1,
          failedJobs: 1,
          activeRuns: 2,
          queueDepth: 3,
          totalRuns24h: 10,
          successfulRuns24h: 7,
          failedRuns24h: 3,
          averageDurationMs: 1500,
          successRate24h: 70,
        }}
      />,
    );

    expect(screen.getByText("6")).toBeInTheDocument();
    expect(screen.getByText("5")).toBeInTheDocument();
    expect(screen.getByText("1")).toBeInTheDocument();
    expect(screen.getByText("2")).toHaveClass("text-blue-600");
    expect(screen.getAllByText("3").some((element) => element.className.includes("text-red-600"))).toBe(true);
    expect(screen.getByText("1500ms")).toBeInTheDocument();
    expect(screen.getByText("70%")).toHaveClass("text-red-600");
  });
});
