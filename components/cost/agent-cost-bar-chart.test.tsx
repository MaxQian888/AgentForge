jest.mock("recharts", () => ({
  ResponsiveContainer: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="responsive-container">{children}</div>
  ),
  BarChart: ({
    children,
    data,
  }: {
    children: React.ReactNode;
    data: Array<{ label: string; totalCostUsd: number }>;
  }) => (
    <div data-testid="bar-chart" data-count={data.length} data-first-label={data[0]?.label ?? ""}>
      {children}
    </div>
  ),
  CartesianGrid: () => <div data-testid="grid" />,
  XAxis: () => <div data-testid="x-axis" />,
  YAxis: () => <div data-testid="y-axis" />,
  Tooltip: () => <div data-testid="tooltip" />,
  Bar: ({ dataKey }: { dataKey: string }) => (
    <div data-testid="bar" data-key={dataKey} />
  ),
}));

import { render, screen } from "@testing-library/react";
import { AgentCostBarChart } from "./agent-cost-bar-chart";

describe("AgentCostBarChart", () => {
  it("sorts agents by cost descending and excludes zero-cost entries", () => {
    render(
      <AgentCostBarChart
        data={[
          { label: "planner", totalCostUsd: 2 },
          { label: "architect", totalCostUsd: 10 },
          { label: "reviewer", totalCostUsd: 0 },
          { label: "coder", totalCostUsd: 5 },
        ]}
      />,
    );

    const chart = screen.getByTestId("bar-chart");
    expect(chart).toHaveAttribute("data-count", "3");
    expect(chart).toHaveAttribute("data-first-label", "architect");
    expect(screen.getByTestId("bar")).toHaveAttribute("data-key", "totalCostUsd");
  });

  it("shows empty state when there are no cost entries", () => {
    render(<AgentCostBarChart data={[]} />);
    expect(screen.getByTestId("agent-cost-empty")).toHaveTextContent(
      "No agent cost data available yet.",
    );
  });
});
