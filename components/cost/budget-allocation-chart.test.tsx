jest.mock("recharts", () => ({
  ResponsiveContainer: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="responsive-container">{children}</div>
  ),
  PieChart: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="pie-chart">{children}</div>
  ),
  Pie: ({
    children,
    data,
    dataKey,
  }: {
    children: React.ReactNode;
    data: unknown[];
    dataKey: string;
  }) => (
    <div data-testid="pie" data-key={dataKey} data-slices={data.length}>
      {children}
    </div>
  ),
  Cell: ({ fill }: { fill: string }) => (
    <div data-testid="cell" data-fill={fill} />
  ),
  Tooltip: () => <div data-testid="tooltip" />,
  Legend: () => <div data-testid="legend" />,
}));

import { render, screen, fireEvent } from "@testing-library/react";
import { BudgetAllocationChart } from "./budget-allocation-chart";

describe("BudgetAllocationChart", () => {
  it("renders pie chart with one Cell per data slice", () => {
    render(
      <BudgetAllocationChart
        data={[
          { category: "agents", amountUsd: 50 },
          { category: "compute", amountUsd: 30 },
        ]}
      />,
    );
    expect(screen.getByTestId("pie")).toHaveAttribute("data-slices", "2");
    expect(screen.getAllByTestId("cell")).toHaveLength(2);
  });

  it("shows configure prompt when no data is available", () => {
    const onConfigureBudget = jest.fn();
    render(
      <BudgetAllocationChart data={[]} onConfigureBudget={onConfigureBudget} />,
    );

    expect(screen.getByTestId("budget-allocation-empty")).toHaveTextContent(
      "No budget configured for this project.",
    );

    fireEvent.click(screen.getByText("Configure budget in settings"));
    expect(onConfigureBudget).toHaveBeenCalledTimes(1);
  });

  it("treats all-zero amounts as empty", () => {
    render(
      <BudgetAllocationChart
        data={[{ category: "agents", amountUsd: 0 }]}
      />,
    );
    expect(screen.getByTestId("budget-allocation-empty")).toBeInTheDocument();
  });
});
