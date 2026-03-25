import { render, screen } from "@testing-library/react";
import { AgentPerformanceTable } from "./agent-performance-table";

describe("AgentPerformanceTable", () => {
  it("shows an empty-state message when no records exist", () => {
    render(<AgentPerformanceTable data={[]} />);

    expect(
      screen.getByText("No agent performance data available yet."),
    ).toBeInTheDocument();
  });

  it("renders tabular performance summaries and success badges", () => {
    render(
      <AgentPerformanceTable
        data={[
          {
            agentId: "agent-1",
            agentName: "Planner",
            taskCount: 4,
            successRate: 0.82,
            avgCostUsd: 1.25,
            avgDurationMinutes: 18,
            totalCostUsd: 5,
          },
          {
            agentId: "agent-2",
            agentName: "Reviewer",
            taskCount: 2,
            successRate: 0.4,
            avgCostUsd: 2.5,
            avgDurationMinutes: 30,
            totalCostUsd: 5,
          },
        ]}
      />,
    );

    expect(screen.getByRole("columnheader", { name: "Agent" })).toBeInTheDocument();
    expect(screen.getByText("Planner")).toBeInTheDocument();
    expect(screen.getByText("Reviewer")).toBeInTheDocument();
    expect(screen.getByText("82%")).toBeInTheDocument();
    expect(screen.getByText("40%")).toBeInTheDocument();
    expect(screen.getAllByText("$5.00")).toHaveLength(2);
    expect(screen.getByText("18m")).toBeInTheDocument();
  });
});
