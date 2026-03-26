import { render, screen, waitFor } from "@testing-library/react";
import AgentsPage from "./page";

const fetchAgents = jest.fn();
const fetchPool = jest.fn();
const searchParamsState = {
  member: "member-2",
};

const agentState = {
  agents: [
    {
      id: "agent-1",
      taskId: "task-1",
      taskTitle: "Timeline work",
      memberId: "member-1",
      roleId: "role-1",
      roleName: "Planner",
      status: "running" as const,
      runtime: "codex",
      provider: "openai",
      model: "gpt-5.4",
      turns: 3,
      cost: 1,
      budget: 5,
      worktreePath: "",
      branchName: "",
      sessionId: "",
      lastActivity: "2026-03-26T10:00:00.000Z",
      startedAt: "2026-03-26T09:00:00.000Z",
      createdAt: "2026-03-26T09:00:00.000Z",
      canResume: false,
      memoryStatus: "none" as const,
    },
    {
      id: "agent-2",
      taskId: "task-2",
      taskTitle: "Review queue",
      memberId: "member-2",
      roleId: "role-2",
      roleName: "Reviewer",
      status: "starting" as const,
      runtime: "codex",
      provider: "openai",
      model: "gpt-5.4",
      turns: 1,
      cost: 0.5,
      budget: 5,
      worktreePath: "",
      branchName: "",
      sessionId: "",
      lastActivity: "2026-03-26T10:05:00.000Z",
      startedAt: "2026-03-26T10:00:00.000Z",
      createdAt: "2026-03-26T10:00:00.000Z",
      canResume: false,
      memoryStatus: "none" as const,
    },
  ],
  fetchAgents,
  fetchPool,
  pool: null,
  loading: false,
};

jest.mock("next/navigation", () => ({
  useSearchParams: () => ({
    get: (key: string) => (key === "member" ? searchParamsState.member : null),
  }),
}));

jest.mock("@/lib/stores/agent-store", () => ({
  useAgentStore: (selector?: (state: typeof agentState) => unknown) =>
    selector ? selector(agentState) : agentState,
}));

describe("AgentsPage", () => {
  beforeEach(() => {
    fetchAgents.mockReset();
    fetchPool.mockReset();
    searchParamsState.member = "member-2";
  });

  it("filters the visible agent list when a member query parameter is present", async () => {
    render(<AgentsPage />);

    await waitFor(() => expect(fetchAgents).toHaveBeenCalled());
    expect(fetchPool).toHaveBeenCalled();
    expect(screen.getByText("Review queue")).toBeInTheDocument();
    expect(screen.queryByText("Timeline work")).not.toBeInTheDocument();
  });
});
