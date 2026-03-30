import { Activity } from "lucide-react";
import { render, screen } from "@testing-library/react";
import { MetricCard } from "./metric-card";

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
});
