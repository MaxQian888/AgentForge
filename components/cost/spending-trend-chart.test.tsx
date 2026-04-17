jest.mock("recharts", () => ({
  ResponsiveContainer: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="responsive-container">{children}</div>
  ),
  LineChart: ({
    children,
    data,
  }: {
    children: React.ReactNode;
    data: unknown[];
  }) => (
    <div data-testid="line-chart" data-points={data.length}>
      {children}
    </div>
  ),
  CartesianGrid: () => <div data-testid="grid" />,
  XAxis: ({ dataKey }: { dataKey: string }) => (
    <div data-testid="x-axis" data-key={dataKey} />
  ),
  YAxis: () => <div data-testid="y-axis" />,
  Tooltip: () => <div data-testid="tooltip" />,
  ReferenceLine: ({ y }: { y: number }) => (
    <div data-testid="reference-line" data-y={y} />
  ),
  Line: ({ dataKey }: { dataKey: string }) => (
    <div data-testid="line" data-key={dataKey} />
  ),
}));

import { render, screen } from "@testing-library/react";
import { SpendingTrendChart } from "./spending-trend-chart";

function buildSeries(days: number) {
  return Array.from({ length: days }, (_, i) => ({
    date: `2026-03-${String(i + 1).padStart(2, "0")}`,
    cost: i + 1,
  }));
}

describe("SpendingTrendChart", () => {
  it("renders chart primitives with average reference line", () => {
    render(<SpendingTrendChart data={buildSeries(10)} defaultPeriod="30d" />);

    expect(screen.getByTestId("line-chart")).toHaveAttribute(
      "data-points",
      "10",
    );
    expect(screen.getByTestId("reference-line")).toBeInTheDocument();
    expect(screen.getByText(/Average:/i)).toBeInTheDocument();
  });

  it("slices data to the 30-day window by default", () => {
    render(<SpendingTrendChart data={buildSeries(40)} defaultPeriod="30d" />);
    expect(screen.getByTestId("line-chart")).toHaveAttribute(
      "data-points",
      "30",
    );
  });

  it("slices data to the 7-day window when requested", () => {
    render(<SpendingTrendChart data={buildSeries(40)} defaultPeriod="7d" />);
    expect(screen.getByTestId("line-chart")).toHaveAttribute(
      "data-points",
      "7",
    );
  });

  it("shows empty state when no data is available", () => {
    render(<SpendingTrendChart data={[]} />);
    expect(screen.getByTestId("spending-trend-empty")).toHaveTextContent(
      "No daily cost data available yet.",
    );
  });
});
