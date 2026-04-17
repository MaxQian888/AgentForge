import { render, screen } from "@testing-library/react";
import { BudgetForecastCard } from "./budget-forecast-card";

describe("BudgetForecastCard", () => {
  it("projects spend based on recent burn rate and shows on-track badge", () => {
    render(
      <BudgetForecastCard
        input={{
          dailyCosts: [
            { date: "2026-04-10", costUsd: 1 },
            { date: "2026-04-11", costUsd: 1 },
            { date: "2026-04-12", costUsd: 1 },
          ],
          budgetUsd: 100,
          spentUsd: 3,
          daysRemaining: 10,
        }}
      />,
    );

    expect(screen.getByTestId("budget-forecast-card")).toBeInTheDocument();
    expect(screen.getByText("On Track")).toBeInTheDocument();
    // Burn rate ~= 1/day → projected spend = 3 + 1*10 = 13
    expect(screen.getByText("$13.00")).toBeInTheDocument();
  });

  it("shows over-budget warning when projection exceeds budget", () => {
    render(
      <BudgetForecastCard
        input={{
          dailyCosts: [
            { date: "2026-04-10", costUsd: 20 },
            { date: "2026-04-11", costUsd: 20 },
          ],
          budgetUsd: 50,
          spentUsd: 40,
          daysRemaining: 5,
        }}
      />,
    );

    expect(screen.getByText("Over Budget")).toBeInTheDocument();
    expect(screen.getByText(/exhausted/i)).toBeInTheDocument();
  });

  it("handles missing budget gracefully", () => {
    render(
      <BudgetForecastCard
        input={{
          dailyCosts: [],
          budgetUsd: null,
          spentUsd: 0,
          daysRemaining: 7,
        }}
      />,
    );
    expect(screen.getByText("—")).toBeInTheDocument();
    expect(screen.queryByText("On Track")).not.toBeInTheDocument();
    expect(screen.queryByText("Over Budget")).not.toBeInTheDocument();
  });
});
