import { render, screen } from "@testing-library/react";
import { AgentCard } from "./agent-card";
import type { Agent } from "@/lib/stores/agent-store";

function makeAgent(overrides: Partial<Agent> = {}): Agent {
  return {
    id: "agent-1",
    taskId: "task-1",
    taskTitle: "Implement review queue",
    memberId: "member-1",
    roleId: "role-reviewer",
    roleName: "Reviewer",
    status: "running",
    runtime: "codex",
    provider: "openai",
    model: "gpt-5.4",
    turns: 8,
    cost: 4.25,
    budget: 5,
    worktreePath: "D:/Project/AgentForge",
    branchName: "agent/task-1",
    sessionId: "session-1",
    lastActivity: "2026-03-25T09:00:00.000Z",
    startedAt: "2026-03-25T08:00:00.000Z",
    createdAt: "2026-03-25T07:55:00.000Z",
    canResume: true,
    memoryStatus: "available",
    ...overrides,
  };
}

describe("AgentCard", () => {
  it("shows runtime, spend, and high-budget usage state", () => {
    render(<AgentCard agent={makeAgent()} />);

    expect(screen.getByText("Reviewer")).toBeInTheDocument();
    expect(screen.getByText("Implement review queue")).toBeInTheDocument();
    expect(
      screen.getByText("Runtime: codex / openai / gpt-5.4"),
    ).toBeInTheDocument();
    expect(screen.getByText("Turns: 8")).toBeInTheDocument();
    expect(screen.getByText("Cost: $4.25 / $5.00")).toBeInTheDocument();
    expect(screen.getByRole("progressbar")).toHaveAttribute(
      "aria-valuenow",
      "85",
    );
  });

  it("falls back to placeholder runtime values and zero-width budget bar", () => {
    render(
      <AgentCard
        agent={makeAgent({
          runtime: "",
          provider: "",
          model: "",
          budget: 0,
          cost: 0,
          status: "budget_exceeded",
        })}
      />,
    );

    expect(screen.getByText("Runtime: - / - / -")).toBeInTheDocument();
    expect(screen.getByText("budget_exceeded")).toBeInTheDocument();
    expect(screen.getByRole("progressbar")).toHaveAttribute(
      "aria-valuenow",
      "0",
    );
  });
});
