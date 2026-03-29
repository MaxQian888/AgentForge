jest.mock("./team-pipeline", () => ({
  TeamPipeline: ({ team }: { team: { id: string } }) => (
    <div data-testid="team-pipeline">{team.id}</div>
  ),
}));

jest.mock("@/components/agent/output-stream", () => ({
  OutputStream: ({ lines }: { lines: string[] }) => (
    <div data-testid="output-stream">{lines.join(" | ")}</div>
  ),
}));

const fetchTeam = jest.fn().mockResolvedValue(undefined);
const cancelTeam = jest.fn();
const retryTeam = jest.fn();
const fetchAgent = jest.fn().mockResolvedValue(undefined);

const teamState = {
  teams: [
    {
      id: "team-1",
      projectId: "project-1",
      taskId: "task-1",
      taskTitle: "Review queue",
      name: "Review squad",
      status: "planning",
      strategy: "",
      runtime: "codex",
      provider: "openai",
      model: "gpt-5.4",
      plannerRunId: "planner-1",
      reviewerRunId: "reviewer-1",
      coderRunIds: ["coder-1"],
      totalBudget: 10,
      totalSpent: 9,
      errorMessage: "",
      createdAt: "2026-03-25T08:30:00.000Z",
      updatedAt: "2026-03-25T09:00:00.000Z",
    },
  ],
  fetchTeam,
  cancelTeam,
  retryTeam,
  loadingById: {},
  errorById: {},
};

const agentState = {
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
      cost: 1.5,
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
      turns: 5,
      cost: 4,
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
    {
      id: "reviewer-1",
      taskId: "task-1",
      taskTitle: "Review queue",
      memberId: "member-3",
      roleId: "role-reviewer",
      roleName: "Reviewer",
      status: "starting",
      runtime: "codex",
      provider: "openai",
      model: "gpt-5.4",
      turns: 2,
      cost: 1,
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
  ],
  agentOutputs: new Map<string, string[]>([
    ["planner-1", ["planner ready"]],
    ["reviewer-1", ["review queued"]],
  ]),
  fetchAgent,
};

const useTeamStoreMock = jest.fn(
  (selector: (state: typeof teamState) => unknown) => selector(teamState),
);
const useAgentStoreMock = jest.fn(
  (selector: (state: typeof agentState) => unknown) => selector(agentState),
);

jest.mock("@/lib/stores/team-store", () => ({
  useTeamStore: (selector: (state: typeof teamState) => unknown) =>
    useTeamStoreMock(selector),
  getTeamStrategyLabel: () => "Planner → Coder → Reviewer",
}));

jest.mock("@/lib/stores/agent-store", () => ({
  useAgentStore: (selector: (state: typeof agentState) => unknown) =>
    useAgentStoreMock(selector),
}));

import userEvent from "@testing-library/user-event";
import { render, screen, waitFor } from "@testing-library/react";
import { TeamDetailView } from "./team-detail-view";

describe("TeamDetailView", () => {
  beforeEach(() => {
    jest.useFakeTimers();
    jest.setSystemTime(new Date("2026-03-25T09:30:00.000Z"));
    fetchTeam.mockClear();
    cancelTeam.mockClear();
    retryTeam.mockClear();
    fetchAgent.mockClear();
    useTeamStoreMock.mockImplementation(
      (selector: (state: typeof teamState) => unknown) => selector(teamState),
    );
    useAgentStoreMock.mockImplementation(
      (selector: (state: typeof agentState) => unknown) => selector(agentState),
    );
  });

  afterEach(() => {
    jest.useRealTimers();
  });

  it("loads team/agent details and shows aggregate runtime information", async () => {
    const user = userEvent.setup({ advanceTimers: jest.advanceTimersByTime });
    render(<TeamDetailView teamId="team-1" />);

    await waitFor(() => expect(fetchTeam).toHaveBeenCalledWith("team-1"));
    await waitFor(() => expect(fetchAgent).toHaveBeenCalledWith("planner-1"));
    expect(fetchAgent).toHaveBeenCalledWith("coder-1");
    expect(fetchAgent).toHaveBeenCalledWith("reviewer-1");

    expect(screen.getByTestId("team-pipeline")).toHaveTextContent("team-1");
    expect(screen.getByText("Resolved Runtime")).toBeInTheDocument();
    expect(screen.getByText("codex / openai / gpt-5.4")).toBeInTheDocument();
    expect(screen.getByText("11")).toBeInTheDocument();
    expect(screen.getByText("60m")).toBeInTheDocument();
    await user.click(screen.getByRole("tab", { name: "Planner" }));
    expect(screen.getByTestId("output-stream")).toHaveTextContent("planner ready");

    await user.click(screen.getByRole("button", { name: /Cancel/i }));
    await user.click(screen.getByRole("button", { name: "Cancel Team" }));
    expect(cancelTeam).toHaveBeenCalledWith("team-1");
  });

  it("shows a not-found state for unknown teams", () => {
    const emptyTeamState = { ...teamState, teams: [], loadingById: { "missing-team": false } };
    useTeamStoreMock.mockImplementationOnce(
      (selector: (state: typeof emptyTeamState) => unknown) =>
        selector(emptyTeamState),
    );

    render(<TeamDetailView teamId="missing-team" />);
    expect(screen.getByText("Team not found")).toBeInTheDocument();
  });

  it("shows a loading state while the team detail request is still in flight", () => {
    const loadingTeamState = {
      ...teamState,
      teams: [],
      loadingById: { "team-1": true },
    };
    useTeamStoreMock.mockImplementation(
      (selector: (state: typeof loadingTeamState) => unknown) =>
        selector(loadingTeamState),
    );

    const { container } = render(<TeamDetailView teamId="team-1" />);
    expect(container.querySelectorAll('[data-slot="skeleton"]').length).toBeGreaterThan(0);
    expect(screen.queryByText("Team not found")).not.toBeInTheDocument();
  });
});
