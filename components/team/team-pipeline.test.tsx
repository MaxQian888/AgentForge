const agentStoreState = {
  agents: [
    {
      id: "planner-1",
      taskId: "task-1",
      taskTitle: "Review queue",
      memberId: "member-1",
      roleId: "role-planner",
      roleName: "Planner",
      status: "completed",
      runtime: "codex",
      provider: "openai",
      model: "gpt-5.4",
      turns: 4,
      cost: 1.25,
      budget: 3,
      worktreePath: "",
      branchName: "",
      sessionId: "",
      lastActivity: "",
      startedAt: "",
      createdAt: "",
      canResume: false,
      memoryStatus: "none" as const,
    },
    {
      id: "coder-1",
      taskId: "task-1",
      taskTitle: "Review queue",
      memberId: "member-2",
      roleId: "role-coder",
      roleName: "Coder",
      status: "running",
      runtime: "codex",
      provider: "openai",
      model: "gpt-5.4",
      turns: 6,
      cost: 2.5,
      budget: 4,
      worktreePath: "",
      branchName: "",
      sessionId: "",
      lastActivity: "",
      startedAt: "",
      createdAt: "",
      canResume: false,
      memoryStatus: "none" as const,
    },
  ],
};

jest.mock("@/lib/stores/agent-store", () => ({
  useAgentStore: (selector: (state: typeof agentStoreState) => unknown) =>
    selector(agentStoreState),
}));

import { render, screen } from "@testing-library/react";
import { TeamPipeline } from "./team-pipeline";
import type { AgentTeam } from "@/lib/stores/team-store";

const team: AgentTeam = {
  id: "team-1",
  projectId: "project-1",
  taskId: "task-1",
  taskTitle: "Review queue",
  name: "Review squad",
  status: "executing",
  strategy: "",
  runtime: "codex",
  provider: "openai",
  model: "gpt-5.4",
  plannerRunId: "planner-1",
  reviewerRunId: undefined,
  coderRunIds: ["coder-1"],
  totalBudget: 10,
  totalSpent: 5,
  errorMessage: "",
  createdAt: "2026-03-25T08:00:00.000Z",
  updatedAt: "2026-03-25T08:30:00.000Z",
};

describe("TeamPipeline", () => {
  it("renders planner/coder progress and pending reviewer state", () => {
    render(<TeamPipeline team={team} />);

    expect(screen.getByText("Plan")).toBeInTheDocument();
    expect(screen.getByText("Planner")).toBeInTheDocument();
    expect(screen.getByText("Coder")).toBeInTheDocument();
    expect(screen.getByText("Turns: 4")).toBeInTheDocument();
    expect(screen.getByText("$2.50")).toBeInTheDocument();
    expect(screen.getByText("Waiting...")).toBeInTheDocument();
  });
});
