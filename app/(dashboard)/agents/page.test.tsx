import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import AgentsPage from "./page";

const fetchAgents = jest.fn();
const fetchPool = jest.fn();
const fetchRuntimeCatalog = jest.fn();
const fetchBridgeHealth = jest.fn();
const fetchDispatchStats = jest.fn();
const resumeAgent = jest.fn();
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
      dispatchStatus: "started" as const,
      guardrailType: "",
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
      dispatchStatus: "queued" as const,
      guardrailType: "pool",
    },
    {
      id: "agent-3",
      taskId: "task-3",
      taskTitle: "Paused verification",
      memberId: "member-2",
      roleId: "role-3",
      roleName: "Coder",
      status: "paused" as const,
      runtime: "claude_code",
      provider: "anthropic",
      model: "claude-sonnet-4-5",
      turns: 7,
      cost: 1.5,
      budget: 5,
      worktreePath: "",
      branchName: "",
      sessionId: "",
      lastActivity: "2026-03-26T10:10:00.000Z",
      startedAt: "2026-03-26T10:00:00.000Z",
      createdAt: "2026-03-26T10:00:00.000Z",
      canResume: true,
      memoryStatus: "available" as const,
      dispatchStatus: "blocked" as const,
      guardrailType: "budget",
    },
  ],
  fetchAgents,
  fetchPool,
  fetchRuntimeCatalog,
  fetchBridgeHealth,
  fetchDispatchStats,
  resumeAgent,
  runtimeCatalog: {
    defaultRuntime: "codex",
    defaultSelection: {
      runtime: "codex",
      provider: "openai",
      model: "gpt-5.4",
    },
    runtimes: [
      {
        runtime: "codex",
        label: "Codex",
        defaultProvider: "openai",
        compatibleProviders: ["openai"],
        defaultModel: "gpt-5.4",
        available: true,
        diagnostics: [],
      },
    ],
  },
  bridgeHealth: {
    status: "degraded",
    lastCheck: "2026-03-26T10:10:00.000Z",
    pool: {
      active: 1,
      available: 1,
      warm: 0,
    },
  },
  pool: {
    active: 1,
    max: 2,
    available: 1,
    pausedResumable: 1,
    queued: 0,
    warm: 0,
    degraded: true,
    queue: [
      {
        entryId: "queue-1",
        projectId: "project-1",
        taskId: "task-2",
        memberId: "member-2",
        status: "queued",
        reason: "agent pool is at capacity",
        priority: 20,
        createdAt: "2026-03-26T10:00:00.000Z",
        updatedAt: "2026-03-26T10:00:00.000Z",
      },
    ],
  },
  dispatchStats: {
    outcomes: { started: 1, queued: 1, blocked: 1 },
    blockedReasons: { budget: 1 },
    queueDepth: 1,
    medianWaitSeconds: 20,
  },
  loading: false,
};

jest.mock("next-intl", () => ({
  useTranslations: () => (key: string) => key,
}));

jest.mock("next/navigation", () => ({
  useRouter: () => ({
    push: jest.fn(),
    replace: jest.fn(),
    prefetch: jest.fn(),
    back: jest.fn(),
  }),
  useSearchParams: () => ({
    get: (key: string) => (key === "member" ? searchParamsState.member : null),
  }),
}));

jest.mock("@/lib/stores/agent-store", () => ({
  useAgentStore: (selector?: (state: typeof agentState) => unknown) =>
    selector ? selector(agentState) : agentState,
}));

jest.mock("@/lib/stores/dashboard-store", () => ({
  useDashboardStore: (selector: (state: { selectedProjectId: string | null }) => unknown) =>
    selector({ selectedProjectId: "project-1" }),
}));

describe("AgentsPage", () => {
  beforeEach(() => {
    fetchAgents.mockReset();
    fetchPool.mockReset();
    fetchRuntimeCatalog.mockReset();
    fetchBridgeHealth.mockReset();
    fetchDispatchStats.mockReset();
    resumeAgent.mockReset();
    searchParamsState.member = "member-2";
  });

  it("filters the visible agent list when a member query parameter is present", async () => {
    render(<AgentsPage />);

    await waitFor(() => expect(fetchAgents).toHaveBeenCalled());
    expect(fetchPool).toHaveBeenCalled();
    expect(fetchRuntimeCatalog).toHaveBeenCalled();
    expect(fetchBridgeHealth).toHaveBeenCalled();
    expect(fetchDispatchStats).toHaveBeenCalledWith("project-1");
    expect(screen.getByText("Review queue")).toBeInTheDocument();
    expect(screen.queryByText("Timeline work")).not.toBeInTheDocument();
  });

  it("shows bridge degraded state and disables paused-agent resume", async () => {
    const user = userEvent.setup();
    render(<AgentsPage />);

    expect(screen.getByRole("tablist")).toBeInTheDocument();
    expect(
      screen.getByRole("tab", { name: "monitor.title", selected: true }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("tab", { name: "stats.dispatch", selected: false }),
    ).toBeInTheDocument();

    const degradedAlert = screen.getByRole("alert");
    expect(degradedAlert).toHaveTextContent("Bridge Health");
    expect(degradedAlert).toHaveTextContent("Status: degraded");
    expect(screen.getAllByText("Paused verification").length).toBeGreaterThan(0);

    const resumeButton = screen.getByRole("button", {
      name: "workspace.quickResume",
    });
    expect(resumeButton).toBeDisabled();

    await user.click(resumeButton);
    expect(resumeAgent).not.toHaveBeenCalled();
  });

  it("renders dispatch stats and queue priority details", () => {
    render(<AgentsPage />);

    expect(screen.getByText("stats.outcomes")).toBeInTheDocument();
    expect(screen.getByText("dispatchStatus.blocked: 1")).toBeInTheDocument();
    expect(screen.getByText("priority.high")).toBeInTheDocument();
  });
});
