import { render, screen } from "@testing-library/react";
import { TaskProgressSummary } from "./task-progress-summary";

describe("TaskProgressSummary", () => {
  it("renders health, dependency, and spend details for degraded realtime", () => {
    render(
      <TaskProgressSummary
        counts={{ healthy: 3, warning: 2, stalled: 1, unscheduled: 4 }}
        dependencySummary={{ blocked: 2, readyToUnblock: 1 }}
        costSummary={{
          totalSpentUsd: 12.5,
          totalBudgetUsd: 20,
          activeRunCount: 3,
          activeRunCostUsd: 4.25,
          activeRunBudgetUsd: 10,
          budgetedTaskCount: 6,
          overBudgetTaskCount: 2,
        }}
        realtimeState="degraded"
      />,
    );

    expect(screen.getByText("Progress health")).toBeInTheDocument();
    expect(screen.getByText("Realtime degraded")).toBeInTheDocument();
    expect(screen.getByText("Healthy 3")).toBeInTheDocument();
    expect(screen.getByText("Blocked 2")).toBeInTheDocument();
    expect(screen.getByText("Ready to unblock 1")).toBeInTheDocument();
    expect(screen.getByText("Task spend $12.50 / $20.00")).toBeInTheDocument();
    expect(
      screen.getByText("Active runs 3 using $4.25 / $10.00"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Budgeted tasks 6, over budget 2"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Realtime updates unavailable."),
    ).toBeInTheDocument();
  });

  it("omits the degraded note when realtime is live", () => {
    render(
      <TaskProgressSummary
        counts={{ healthy: 1, warning: 0, stalled: 0, unscheduled: 0 }}
        dependencySummary={{ blocked: 0, readyToUnblock: 0 }}
        costSummary={{
          totalSpentUsd: 1,
          totalBudgetUsd: 5,
          activeRunCount: 1,
          activeRunCostUsd: 0.5,
          activeRunBudgetUsd: 2,
          budgetedTaskCount: 1,
          overBudgetTaskCount: 0,
        }}
        realtimeState="live"
      />,
    );

    expect(screen.getByText("Realtime live")).toBeInTheDocument();
    expect(
      screen.queryByText("Realtime updates unavailable."),
    ).not.toBeInTheDocument();
  });
});
