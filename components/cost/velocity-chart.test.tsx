import { render, screen } from "@testing-library/react";
import { VelocityChart } from "./velocity-chart";

describe("VelocityChart", () => {
  it("shows an empty-state message when there is no data", () => {
    render(<VelocityChart data={[]} />);

    expect(
      screen.getByText("No velocity data available yet."),
    ).toBeInTheDocument();
  });

  it("renders period totals, tooltips, and aggregate badges", () => {
    render(
      <VelocityChart
        data={[
          { period: "2026-03-01", tasksCompleted: 4, costUsd: 10 },
          { period: "2026-03-08", tasksCompleted: 8, costUsd: 12.5 },
        ]}
      />,
    );

    expect(
      screen.getByTitle("2026-03-01: 4 tasks, $10.00"),
    ).toBeInTheDocument();
    expect(
      screen.getByTitle("2026-03-08: 8 tasks, $12.50"),
    ).toBeInTheDocument();
    expect(screen.getByText("Avg: 6.0 tasks/period")).toBeInTheDocument();
    expect(screen.getByText("Total: $22.50")).toBeInTheDocument();
    expect(screen.getByText("03-01")).toBeInTheDocument();
    expect(screen.getByText("03-08")).toBeInTheDocument();
  });
});
