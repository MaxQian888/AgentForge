jest.mock("next-intl", () => ({
  useTranslations: () => (key: string) => {
    const map: Record<string, string> = {
      "agentFleet.title": "Agent Fleet",
      "agentFleet.empty": "No active agents.",
      "agentFleet.name": "Name",
      "agentFleet.role": "Role",
      "agentFleet.task": "Task",
      "agentFleet.status": "Status",
      "agentFleet.cost": "Cost",
      "status.running": "running",
    };
    return map[key] ?? key;
  },
}));

import { render, screen } from "@testing-library/react";
import { AgentFleetWidget } from "./agent-fleet-widget";

describe("AgentFleetWidget", () => {
  it("renders an empty state when there are no agents", () => {
    render(<AgentFleetWidget agents={[]} />);

    expect(screen.getByText("No active agents.")).toBeInTheDocument();
  });

  it("renders agent rows with formatted costs", () => {
    render(
      <AgentFleetWidget
        agents={[
          {
            id: "agent-1",
            memberId: "alice",
            roleName: "Reviewer",
            taskTitle: "Review release plan",
            status: "running",
            cost: 12.5,
          } as never,
        ]}
      />,
    );

    expect(screen.getByText("alice")).toBeInTheDocument();
    expect(screen.getByText("Reviewer")).toBeInTheDocument();
    expect(screen.getByText("Review release plan")).toBeInTheDocument();
    expect(screen.getByText("running")).toBeInTheDocument();
    expect(screen.getByText("$12.50")).toBeInTheDocument();
  });
});
