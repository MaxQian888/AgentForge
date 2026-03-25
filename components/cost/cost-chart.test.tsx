jest.mock("recharts", () => ({
  ResponsiveContainer: ({
    children,
    width,
    height,
  }: {
    children: React.ReactNode;
    width: string;
    height: number;
  }) => (
    <div data-testid="responsive-container" data-height={height} data-width={width}>
      {children}
    </div>
  ),
  LineChart: ({
    children,
    data,
  }: {
    children: React.ReactNode;
    data: unknown[];
  }) => <div data-testid="line-chart" data-points={data.length}>{children}</div>,
  CartesianGrid: ({ strokeDasharray }: { strokeDasharray: string }) => (
    <div data-testid="grid" data-stroke-dasharray={strokeDasharray} />
  ),
  XAxis: ({ dataKey }: { dataKey: string }) => (
    <div data-testid="x-axis" data-key={dataKey} />
  ),
  YAxis: ({ tickFormatter }: { tickFormatter: (value: number) => string }) => (
    <div data-testid="y-axis">{tickFormatter(5)}</div>
  ),
  Tooltip: ({
    formatter,
  }: {
    formatter: (value: number) => [string, string];
  }) => <div data-testid="tooltip">{formatter(3.2)[0]}</div>,
  Line: ({ dataKey, stroke }: { dataKey: string; stroke: string }) => (
    <div data-testid="line" data-key={dataKey} data-stroke={stroke} />
  ),
}));

import { render, screen } from "@testing-library/react";
import { CostChart } from "./cost-chart";

describe("CostChart", () => {
  it("wires the chart data and formatters into recharts primitives", () => {
    render(
      <CostChart
        data={[
          { date: "2026-03-24", cost: 3.5 },
          { date: "2026-03-25", cost: 4.25 },
        ]}
      />,
    );

    expect(screen.getByTestId("responsive-container")).toHaveAttribute(
      "data-width",
      "100%",
    );
    expect(screen.getByTestId("line-chart")).toHaveAttribute("data-points", "2");
    expect(screen.getByTestId("grid")).toHaveAttribute(
      "data-stroke-dasharray",
      "3 3",
    );
    expect(screen.getByTestId("x-axis")).toHaveAttribute("data-key", "date");
    expect(screen.getByTestId("y-axis")).toHaveTextContent("$5");
    expect(screen.getByTestId("tooltip")).toHaveTextContent("$3.20");
    expect(screen.getByTestId("line")).toHaveAttribute("data-key", "cost");
  });
});
