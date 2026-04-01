import { Activity } from "lucide-react";
import { render, screen } from "@testing-library/react";
import { MetricCard } from "./metric-card";

jest.mock("recharts", () => ({
  ResponsiveContainer: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="metric-sparkline-container">{children}</div>
  ),
  AreaChart: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="metric-sparkline-chart">{children}</div>
  ),
  Area: () => <div data-testid="metric-sparkline-area" />,
}));

describe("MetricCard", () => {
  it("renders linked metric cards with trend styling", () => {
    render(
      <MetricCard
        label="Velocity"
        value="24"
        icon={Activity}
        trend={{ value: 35, direction: "up" }}
        href="/project/dashboard"
        className="metric-card"
      />,
    );

    const link = screen.getByRole("link");
    expect(link).toHaveAttribute("href", "/project/dashboard");
    expect(link).toHaveClass("cursor-pointer");
    expect(link).toHaveClass("metric-card");
    expect(screen.getByText("Velocity")).toBeInTheDocument();
    expect(screen.getByText("24")).toBeInTheDocument();
    expect(screen.getByText("35%")).toHaveClass("text-emerald-600");
  });

  it("renders plain metric cards without links", () => {
    const { container } = render(<MetricCard label="Tasks" value={8} />);

    expect(screen.queryByRole("link")).not.toBeInTheDocument();
    expect(container.firstChild?.nodeName).toBe("DIV");
    expect(screen.getByText("8")).toBeInTheDocument();
  });

  it("renders a sparkline when historical points are provided", () => {
    render(
      <MetricCard
        label="Weekly Cost"
        value="$42.00"
        sparkline={[
          { label: "2026-03-30", value: 4 },
          { label: "2026-03-31", value: 5 },
          { label: "2026-04-01", value: 6 },
        ]}
      />,
    );

    expect(screen.getByTestId("metric-sparkline-chart")).toBeInTheDocument();
    expect(screen.getByTestId("metric-sparkline-area")).toBeInTheDocument();
  });
});
