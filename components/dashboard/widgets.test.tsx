jest.mock("recharts", () => ({
  ResponsiveContainer: ({ children }: { children?: React.ReactNode }) => (
    <div data-testid="responsive-container">{children}</div>
  ),
  BarChart: ({ children }: { children?: React.ReactNode }) => (
    <div data-testid="bar-chart">{children}</div>
  ),
  LineChart: ({ children }: { children?: React.ReactNode }) => (
    <div data-testid="line-chart">{children}</div>
  ),
  CartesianGrid: () => <div data-testid="cartesian-grid" />,
  XAxis: ({ dataKey }: { dataKey?: string }) => <div data-testid={`x-axis-${dataKey}`} />,
  YAxis: () => <div data-testid="y-axis" />,
  Tooltip: () => <div data-testid="tooltip" />,
  Bar: ({ dataKey }: { dataKey?: string }) => <div data-testid={`bar-${dataKey}`} />,
  Line: ({ dataKey }: { dataKey?: string }) => <div data-testid={`line-${dataKey}`} />,
}));

import { render, screen } from "@testing-library/react";
import { BurndownChartWidget, MetricCard, ThroughputChart } from "./widgets";

describe("dashboard widgets", () => {
  it("renders the throughput chart scaffold", () => {
    render(<ThroughputChart data={[{ date: "2026-03-30", count: 3 }]} />);

    expect(screen.getByTestId("bar-chart")).toBeInTheDocument();
    expect(screen.getByTestId("bar-count")).toBeInTheDocument();
    expect(screen.getByTestId("x-axis-date")).toBeInTheDocument();
  });

  it("renders the burndown chart scaffold", () => {
    render(
      <BurndownChartWidget
        data={[
          { date: "2026-03-30", remainingTasks: 5, completedTasks: 2 },
        ]}
      />,
    );

    expect(screen.getByTestId("line-chart")).toBeInTheDocument();
    expect(screen.getByTestId("line-remainingTasks")).toBeInTheDocument();
    expect(screen.getByTestId("line-completedTasks")).toBeInTheDocument();
  });

  it("renders metric card secondary text when provided", () => {
    render(<MetricCard label="Blocked" value={4} secondary="Down 2 from yesterday" />);

    expect(screen.getByText("Blocked")).toBeInTheDocument();
    expect(screen.getByText("4")).toBeInTheDocument();
    expect(screen.getByText("Down 2 from yesterday")).toBeInTheDocument();
  });
});
