jest.mock("next-intl", () => ({
  useTranslations: () => (key: string) => {
    const map: Record<string, string> = {
      "workspace.quickPause": "Pause",
      "workspace.quickResume": "Resume",
      "workspace.quickKill": "Kill",
      "table.status": "Status",
      "table.runtime": "Runtime",
      "table.turns": "Turns",
      "table.costBudget": "Cost/Budget",
      "status.running": "Running",
      "status.paused": "Paused",
      "workspace.reasoning": "Reasoning",
      "workspace.toolCalls": "Tool Calls",
      "workspace.fileChanges": "File Changes",
      "workspace.todos": "Todos",
      "workspace.partialMessage": "Live Output",
      "workspace.permissionRequests": "Permission Requests",
      "workspace.permissionRequests.approve": "Approve",
      "workspace.permissionRequests.deny": "Deny",
      "workspace.logs": "Logs",
    };
    return map[key] ?? key;
  },
}));

jest.mock("@/lib/stores/auth-store", () => ({
  useAuthStore: { getState: () => ({ accessToken: "test-token" }) },
}));

jest.mock("@/components/agent/output-stream", () => ({
  OutputStream: ({ lines }: { lines: string[] }) => (
    <div data-testid="output-stream">{lines.join(" | ")}</div>
  ),
}));

jest.mock("@/components/tasks/dispatch-history-panel", () => ({
  DispatchHistoryPanel: ({ attempts }: { attempts: Array<{ id: string }> }) => (
    <div data-testid="dispatch-history">{attempts.length} attempts</div>
  ),
}));

import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AgentWorkspaceDetail } from "./agent-workspace-detail";

const fetchAgentMock = jest.fn();
const fetchDispatchHistoryMock = jest.fn();
const fetchAgentLogsMock = jest.fn().mockResolvedValue([]);
const pauseAgentMock = jest.fn();
const resumeAgentMock = jest.fn();
const killAgentMock = jest.fn();
const removePermissionRequestMock = jest.fn();
const storeState = {
  agents: [] as Array<Record<string, unknown>>,
  agentOutputs: new Map<string, string[]>(),
  agentToolCalls: new Map<string, unknown[]>(),
  agentToolResults: new Map<string, unknown[]>(),
  agentReasoning: new Map<string, string>(),
  agentFileChanges: new Map<string, unknown[]>(),
  agentTodos: new Map<string, unknown[]>(),
  agentPartialMessages: new Map<string, string>(),
  agentPermissionRequests: new Map<string, unknown[]>(),
  agentLogs: new Map<string, unknown[]>(),
  pool: null as Record<string, unknown> | null,
  dispatchHistoryByTask: {} as Record<string, Array<{ id: string }>>,
  fetchAgent: fetchAgentMock,
  fetchDispatchHistory: fetchDispatchHistoryMock,
  fetchAgentLogs: fetchAgentLogsMock,
  pauseAgent: pauseAgentMock,
  resumeAgent: resumeAgentMock,
  killAgent: killAgentMock,
  removePermissionRequest: removePermissionRequestMock,
};

const agentStoreRef: { current: Record<string, unknown> } = { current: {} };

jest.mock("@/lib/stores/agent-store", () => {
  const fn = (selector: (state: unknown) => unknown) => selector(agentStoreRef.current);
  fn.getState = () => agentStoreRef.current;
  return { useAgentStore: fn };
});

describe("AgentWorkspaceDetail", () => {
  beforeEach(() => {
    fetchAgentMock.mockReset();
    fetchAgentMock.mockResolvedValue(undefined);
    fetchDispatchHistoryMock.mockReset();
    fetchDispatchHistoryMock.mockResolvedValue(undefined);
    fetchAgentLogsMock.mockReset();
    fetchAgentLogsMock.mockResolvedValue([]);
    pauseAgentMock.mockReset();
    resumeAgentMock.mockReset();
    killAgentMock.mockReset();
    removePermissionRequestMock.mockReset();

    storeState.agents = [
      {
        id: "agent-1",
        taskId: "task-1",
        taskTitle: "Audit release",
        roleName: "Reviewer",
        status: "running",
        runtime: "codex",
        provider: "openai",
        model: "gpt-5.4",
        turns: 9,
        cost: 4.2,
        budget: 5,
        dispatchStatus: "blocked",
        guardrailType: "budget",
        lastActivity: "2026-03-30T11:00:00.000Z",
      },
    ];
    storeState.agentOutputs = new Map([["agent-1", ["line-1", "line-2"]]]);
    storeState.pool = { active: 2, available: 1, queued: 3, warm: 1 };
    storeState.dispatchHistoryByTask = {
      "task-1": [{ id: "attempt-1" }],
    };
    agentStoreRef.current = storeState;
  });

  it("renders a not-found state when the agent does not exist", () => {
    storeState.agents = [];

    render(<AgentWorkspaceDetail agentId="missing" onBack={jest.fn()} />);

    expect(screen.getByText("Agent not found")).toBeInTheDocument();
  });

  it("loads supporting data and routes quick actions for running agents", async () => {
    const user = userEvent.setup();
    const onBack = jest.fn();

    render(<AgentWorkspaceDetail agentId="agent-1" onBack={onBack} />);

    await waitFor(() => {
      expect(fetchAgentMock).toHaveBeenCalledWith("agent-1");
    });
    await waitFor(() => {
      expect(fetchDispatchHistoryMock).toHaveBeenCalledWith("task-1");
    });

    expect(screen.getByText("Reviewer")).toBeInTheDocument();
    expect(screen.getByText("Audit release")).toBeInTheDocument();
    expect(screen.getByText("Running")).toBeInTheDocument();
    expect(screen.getByText("codex")).toBeInTheDocument();
    expect(screen.getByText("$4.20")).toBeInTheDocument();
    expect(screen.getByText("/ $5.00")).toBeInTheDocument();
    expect(
      Number(
        screen
          .getByRole("progressbar", { name: "Budget usage" })
          .getAttribute("aria-valuenow"),
      ),
    ).toBeCloseTo(84, 5);
    expect(screen.getByTestId("output-stream")).toHaveTextContent("line-1 | line-2");
    expect(screen.getByTestId("dispatch-history")).toHaveTextContent("1 attempts");

    await user.click(screen.getByRole("button", { name: "Pause" }));
    await user.click(screen.getByRole("button", { name: "Kill" }));
    await user.click(screen.getAllByRole("button")[0]);

    expect(pauseAgentMock).toHaveBeenCalledWith("agent-1");
    expect(killAgentMock).toHaveBeenCalledWith("agent-1");
    expect(onBack).toHaveBeenCalled();
  });

  it("shows resume actions for paused agents", async () => {
    const user = userEvent.setup();
    storeState.agents = [
      {
        id: "agent-2",
        taskId: "task-2",
        taskTitle: "Resume verification",
        roleName: "Planner",
        status: "paused",
        runtime: "",
        provider: "",
        model: "",
        turns: 0,
        cost: 0,
        budget: 0,
      },
    ];
    storeState.agentOutputs = new Map();
    storeState.dispatchHistoryByTask = {};

    render(<AgentWorkspaceDetail agentId="agent-2" onBack={jest.fn()} />);

    await user.click(screen.getByRole("button", { name: "Resume" }));
    expect(resumeAgentMock).toHaveBeenCalledWith("agent-2");
  });
});
