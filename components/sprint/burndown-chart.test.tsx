jest.mock("next-intl", () => ({
  useTranslations: () => (key: string, params?: Record<string, unknown>) => {
    const map: Record<string, string> = {
      "burndown.empty": "No burndown data available yet.",
      "burndown.remaining": "Remaining: {remainingTasks}",
      "burndown.completed": "Completed: {completedTasks}",
    };
    let result = map[key] ?? key;
    if (params) {
      Object.entries(params).forEach(([k, v]) => {
        result = result.replace(`{${k}}`, String(v));
      });
    }
    return result;
  },
}));

import { render, screen } from "@testing-library/react";
import { BurndownChart } from "./burndown-chart";
import type { SprintBurndownPoint } from "@/lib/stores/sprint-store";

describe("BurndownChart", () => {
  const sampleBurndown: SprintBurndownPoint[] = [
    { date: "2026-03-24", remainingTasks: 10, completedTasks: 0 },
    { date: "2026-03-25", remainingTasks: 8, completedTasks: 2 },
    { date: "2026-03-26", remainingTasks: 5, completedTasks: 5 },
    { date: "2026-03-27", remainingTasks: 3, completedTasks: 7 },
    { date: "2026-03-28", remainingTasks: 1, completedTasks: 9 },
  ];

  it("renders an SVG chart with burndown data", () => {
    const { container } = render(
      <BurndownChart burndown={sampleBurndown} plannedTasks={10} />
    );

    const svg = container.querySelector("svg");
    expect(svg).toBeInTheDocument();

    // Should render a circle for each data point
    const circles = container.querySelectorAll("circle");
    expect(circles.length).toBe(sampleBurndown.length);
  });

  it("shows empty state when no burndown data", () => {
    render(<BurndownChart burndown={[]} plannedTasks={0} />);

    expect(screen.getByText(/no burndown data available/i)).toBeInTheDocument();
  });

  it("renders path elements for ideal and actual lines", () => {
    const { container } = render(
      <BurndownChart burndown={sampleBurndown} plannedTasks={10} />
    );

    const paths = container.querySelectorAll("path");
    // Ideal line + actual line = 2 paths
    expect(paths.length).toBe(2);
  });
});
