import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import AgentsPage from "./page";

const fetchAgents = jest.fn();
const fetchPool = jest.fn();
const fetchRuntimeCatalog = jest.fn();
const fetchBridgeHealth = jest.fn();
const fetchDispatchStats = jest.fn();
const fetchDispatchHistory = jest.fn();
const resumeAgent = jest.fn();
const searchParamsState = {
  member: "member-2",
  action: null as string | null,
  project: "project-1",
  view: null as string | null,
  vizNode: null as string | null,
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
  fetchDispatchHistory,
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
  dispatchHistoryByTask: {},
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
  usePathname: () => "/agents",
  useSearchParams: () => ({
    get: (key: string) =>
      key === "member"
        ? searchParamsState.member
        : key === "action"
          ? searchParamsState.action
          : key === "project"
            ? searchParamsState.project
            : key === "view"
              ? searchParamsState.view
              : key === "vizNode"
                ? searchParamsState.vizNode
            : null,
    toString: () => {
      const params = new URLSearchParams();
      if (searchParamsState.member) {
        params.set("member", searchParamsState.member);
      }
      if (searchParamsState.action) {
        params.set("action", searchParamsState.action);
      }
      if (searchParamsState.project) {
        params.set("project", searchParamsState.project);
      }
      if (searchParamsState.view) {
        params.set("view", searchParamsState.view);
      }
      if (searchParamsState.vizNode) {
        params.set("vizNode", searchParamsState.vizNode);
      }
      return params.toString();
    },
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
    fetchDispatchHistory.mockReset();
    resumeAgent.mockReset();
    searchParamsState.member = "member-2";
    searchParamsState.action = null;
    searchParamsState.project = "project-1";
    searchParamsState.view = null;
    searchParamsState.vizNode = null;
  });

  it("keeps rendering the workspace when a legacy spawn action is present", () => {
    searchParamsState.action = "spawn";

    render(<AgentsPage />);

    expect(screen.getByRole("tablist")).toBeInTheDocument();
  });

  it("filters the visible agent list when a member query parameter is present", async () => {
    render(<AgentsPage />);

    await waitFor(() => expect(fetchAgents).toHaveBeenCalled());
    expect(fetchPool).toHaveBeenCalled();
    expect(fetchRuntimeCatalog).toHaveBeenCalled();
    expect(fetchBridgeHealth).toHaveBeenCalled();
    expect(fetchDispatchStats).toHaveBeenCalledWith("project-1");
    expect(screen.getAllByText("Review queue").length).toBeGreaterThan(0);
    expect(screen.queryByText("Timeline work")).not.toBeInTheDocument();
  });

  it("renders the visualization tab from URL-driven workspace state", async () => {
    searchParamsState.view = "visualization";
    render(<AgentsPage />);

    expect(screen.getByText("visualization.legend.title")).toBeInTheDocument();
    expect(screen.getByText("visualization.degraded.title")).toBeInTheDocument();
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
    expect(degradedAlert).toHaveTextContent("overview.bridgeHealth");
    expect(degradedAlert).toHaveTextContent("overview.bridgeStatus");
    expect(screen.getAllByText("Paused verification").length).toBeGreaterThan(0);

    const resumeButtons = screen.getAllByRole("button", {
      name: "workspace.quickResume",
    });
    expect(resumeButtons.length).toBeGreaterThan(0);
    resumeButtons.forEach((button) => expect(button).toBeDisabled());

    await user.click(resumeButtons[0]!);
    expect(resumeAgent).not.toHaveBeenCalled();
  });

  it("renders dispatch stats and queue priority details", () => {
    render(<AgentsPage />);

    expect(screen.getByText("stats.outcomes")).toBeInTheDocument();
    expect(screen.getByText("dispatchStatus.blocked: 1")).toBeInTheDocument();
    expect(screen.getByText("priority.high")).toBeInTheDocument();
  });
});
