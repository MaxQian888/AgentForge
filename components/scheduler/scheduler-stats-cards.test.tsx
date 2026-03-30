jest.mock("next-intl", () => ({
  useTranslations: () => (key: string) => {
    const map: Record<string, string> = {
      "stats.totalJobs": "Total Jobs",
      "stats.enabled": "Enabled",
      "stats.disabled": "Disabled",
      "stats.activeRuns": "Active Runs",
      "stats.failed24h": "Failed 24h",
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

    expect(container.querySelectorAll('[data-slot="skeleton"]')).toHaveLength(6);
  });

  it("renders computed success rate and accent colors from stats", () => {
    render(
      <SchedulerStatsCards
        loading={false}
        stats={{
          totalJobs: 6,
          enabledJobs: 5,
          disabledJobs: 1,
          failedJobs: 1,
          activeRuns: 2,
          totalRuns24h: 10,
          failedRuns24h: 3,
        }}
      />,
    );

    expect(screen.getByText("6")).toBeInTheDocument();
    expect(screen.getByText("5")).toBeInTheDocument();
    expect(screen.getByText("1")).toBeInTheDocument();
    expect(screen.getByText("2")).toHaveClass("text-blue-600");
    expect(screen.getByText("3")).toHaveClass("text-red-600");
    expect(screen.getByText("70%")).toHaveClass("text-red-600");
  });
});
