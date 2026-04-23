import type { ReactNode } from "react";

jest.mock("next-intl", () => ({
  useTranslations: () => (key: string) => {
    const map: Record<string, string> = {
      "monitor.title": "Monitor",
      "visualization.title": "Visualization",
      "stats.dispatch": "Dispatch",
      "workspace.toggleSidebar": "Toggle Sidebar",
      "monitor.teamsLink": "Teams",
      "monitor.loading": "Loading agents...",
      "workspace.spawnAgent": "Spawn Agent",
    };
    return map[key] ?? key;
  },
}));

const buildVisualizationModelMock = jest.fn();
const replaceMock = jest.fn();
const useSearchParamsMock = jest.fn();
const useBreakpointMock = jest.fn();
const fetchDispatchHistoryMock = jest.fn();

jest.mock("next/navigation", () => ({
  useRouter: () => ({ replace: replaceMock }),
  useSearchParams: () => useSearchParamsMock(),
}));

jest.mock("next/link", () => ({
  __esModule: true,
  default: ({
    href,
    children,
    ...props
  }: React.AnchorHTMLAttributes<HTMLAnchorElement> & { href: string }) => (
    <a href={href} {...props}>
      {children}
    </a>
  ),
}));

jest.mock("@/hooks/use-breakpoint", () => ({
  useBreakpoint: () => useBreakpointMock(),
}));

jest.mock("@/components/ui/sheet", () => ({
  Sheet: ({ children }: { children?: ReactNode }) => <div>{children}</div>,
  SheetContent: ({ children }: { children?: ReactNode }) => (
    <div>{children}</div>
  ),
  SheetHeader: ({ children }: { children?: ReactNode }) => (
    <div>{children}</div>
  ),
  SheetTitle: ({ children }: { children?: ReactNode }) => (
    <div>{children}</div>
  ),
  SheetDescription: ({ children }: { children?: ReactNode }) => (
    <div>{children}</div>
  ),
}));

jest.mock("./agent-workspace-sidebar", () => ({
  AgentWorkspaceSidebar: ({
    onSelectAgent,
  }: {
    onSelectAgent: (id: string | null) => void;
  }) => (
    <div data-testid="workspace-sidebar">
      <button type="button" onClick={() => onSelectAgent("agent-2")}>
        Select agent
      </button>
    </div>
  ),
}));

jest.mock("./agent-workspace-overview", () => ({
  AgentWorkspaceOverview: ({ activeTab }: { activeTab: string }) => (
    <div data-testid={`workspace-overview-${activeTab}`}>{activeTab}</div>
  ),
}));

jest.mock("./agent-visualization-model", () => ({
  buildAgentVisualizationModel: (...args: unknown[]) =>
    buildVisualizationModelMock(...args),
}));

jest.mock("@/components/tasks/spawn-agent-dialog", () => ({
  SpawnAgentDialog: ({
    open,
  }: {
    open: boolean;
  }) => (open ? <div data-testid="workspace-spawn-dialog">spawn dialog</div> : null),
}));

jest.mock("./agent-visualization-canvas", () => ({
  AgentVisualizationCanvas: ({
    onSelectAgent,
    onSelectVisualizationNode,
    selectedVisualizationNodeId,
  }: {
    onSelectAgent: (id: string) => void;
    onSelectVisualizationNode: (id: string) => void;
    selectedVisualizationNodeId: string | null;
  }) => (
    <div data-testid="workspace-visualization">
      <div data-testid="workspace-selected-viz-node">
        {selectedVisualizationNodeId ?? "none"}
      </div>
      <button type="button" onClick={() => onSelectAgent("agent-3")}>
        Select graph agent
      </button>
      <button
        type="button"
        onClick={() => onSelectVisualizationNode("task:task-2")}
      >
        Focus task node
      </button>
    </div>
  ),
}));

jest.mock("./agent-visualization-focus-panel", () => ({
  AgentVisualizationFocusPanel: ({
    focus,
    onClearFocus,
  }: {
    focus: { kind: string; nodeId: string } | null;
    onClearFocus: () => void;
  }) =>
    focus ? (
      <div data-testid="workspace-visualization-focus">
        <span>{`${focus.kind}:${focus.nodeId}`}</span>
        <button type="button" onClick={onClearFocus}>
          Clear focus
        </button>
      </div>
    ) : null,
}));

jest.mock("./agent-workspace-detail", () => ({
  AgentWorkspaceDetail: ({
    agentId,
    onBack,
  }: {
    agentId: string;
    onBack: () => void;
  }) => (
    <div data-testid="workspace-detail">
      <span>{agentId}</span>
      <button type="button" onClick={onBack}>
        Back
      </button>
    </div>
  ),
}));

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AgentWorkspace } from "./agent-workspace";

describe("AgentWorkspace", () => {
  beforeEach(() => {
    replaceMock.mockReset();
    buildVisualizationModelMock.mockReset();
    fetchDispatchHistoryMock.mockReset();
    fetchDispatchHistoryMock.mockResolvedValue([]);
    buildVisualizationModelMock.mockReturnValue({
      nodes: [],
      edges: [],
      summary: {
        agentCount: 1,
        queueCount: 1,
        runtimeCount: 1,
        taskCount: 1,
        hasGraphData: true,
        isFiltered: false,
        isDegraded: false,
      },
      focusByNodeId: {
        "task:task-2": {
          kind: "task",
          nodeId: "task:task-2",
          taskId: "task-2",
          taskTitle: "Review runtime availability",
          agentCount: 1,
          queueCount: 1,
        },
      },
    });
    useBreakpointMock.mockReturnValue({
      breakpoint: "desktop",
      isMobile: false,
      isTablet: false,
      isDesktop: true,
    });
    useSearchParamsMock.mockReturnValue(
      new URLSearchParams("agent=agent-1"),
    );
    Object.defineProperty(window, "matchMedia", {
      writable: true,
      configurable: true,
      value: jest.fn().mockImplementation(() => ({
        matches: true,
        addEventListener: jest.fn(),
        removeEventListener: jest.fn(),
      })),
    });
  });

  it("shows loading when the workspace is waiting on agents", () => {
    useSearchParamsMock.mockReturnValue(new URLSearchParams(""));

    render(
      <AgentWorkspace
        agents={[]}
        pool={null}
        runtimeCatalog={null}
        bridgeHealth={null}
        dispatchStats={null}
        loading
        requestedMemberId={null}
        dispatchHistoryByTask={{}}
        fetchDispatchHistory={fetchDispatchHistoryMock}
        onPause={jest.fn()}
        onResume={jest.fn()}
        onKill={jest.fn()}
      />,
    );

    expect(screen.getByText("Loading agents...")).toBeInTheDocument();
  });

  it("renders the selected agent detail and can clear selection", async () => {
    const user = userEvent.setup();

    render(
      <AgentWorkspace
        agents={[{ id: "agent-1", memberId: "member-1" } as never]}
        pool={null}
        runtimeCatalog={null}
        bridgeHealth={null}
        dispatchStats={{ outcomes: { started: 2 } } as never}
        loading={false}
        requestedMemberId={null}
        dispatchHistoryByTask={{}}
        fetchDispatchHistory={fetchDispatchHistoryMock}
        onPause={jest.fn()}
        onResume={jest.fn()}
        onKill={jest.fn()}
      />,
    );

    expect(screen.getByTestId("workspace-detail")).toHaveTextContent("agent-1");

    await user.click(screen.getByRole("button", { name: "Back" }));
    expect(replaceMock).toHaveBeenCalledWith("/agents?", { scroll: false });
  });

  afterEach(() => {
    jest.useRealTimers();
  });

  it("shows overview tabs, visualization, toggles the sidebar, and updates the selected agent", async () => {
    const user = userEvent.setup();
    useSearchParamsMock.mockReturnValue(new URLSearchParams(""));

    const { container, rerender } = render(
      <AgentWorkspace
        agents={[
          { id: "agent-1", memberId: "member-1" } as never,
          { id: "agent-2", memberId: "member-2" } as never,
        ]}
        pool={null}
        runtimeCatalog={null}
        bridgeHealth={null}
        dispatchStats={{ outcomes: { started: 2, queued: 1 } } as never}
        loading={false}
        requestedMemberId={null}
        dispatchHistoryByTask={{}}
        fetchDispatchHistory={fetchDispatchHistoryMock}
        onPause={jest.fn()}
        onResume={jest.fn()}
        onKill={jest.fn()}
      />,
    );

    expect(screen.getByTestId("workspace-overview-monitor")).toBeInTheDocument();
    await user.click(screen.getByRole("tab", { name: "Visualization" }));
    useSearchParamsMock.mockReturnValue(new URLSearchParams("view=visualization"));
    rerender(
      <AgentWorkspace
        agents={[
          { id: "agent-1", memberId: "member-1" } as never,
          { id: "agent-2", memberId: "member-2" } as never,
        ]}
        pool={null}
        runtimeCatalog={null}
        bridgeHealth={null}
        dispatchStats={{ outcomes: { started: 2, queued: 1 } } as never}
        loading={false}
        requestedMemberId={null}
        dispatchHistoryByTask={{}}
        fetchDispatchHistory={fetchDispatchHistoryMock}
        onPause={jest.fn()}
        onResume={jest.fn()}
        onKill={jest.fn()}
      />,
    );
    expect(screen.getByTestId("workspace-visualization")).toBeInTheDocument();
    await user.click(screen.getByRole("tab", { name: "Dispatch" }));
    useSearchParamsMock.mockReturnValue(new URLSearchParams("view=dispatch"));
    rerender(
      <AgentWorkspace
        agents={[
          { id: "agent-1", memberId: "member-1" } as never,
          { id: "agent-2", memberId: "member-2" } as never,
        ]}
        pool={null}
        runtimeCatalog={null}
        bridgeHealth={null}
        dispatchStats={{ outcomes: { started: 2, queued: 1 } } as never}
        loading={false}
        requestedMemberId={null}
        dispatchHistoryByTask={{}}
        fetchDispatchHistory={fetchDispatchHistoryMock}
        onPause={jest.fn()}
        onResume={jest.fn()}
        onKill={jest.fn()}
      />,
    );
    expect(screen.getByTestId("workspace-overview-dispatch")).toBeInTheDocument();
    expect(screen.getByText("3")).toBeInTheDocument();

    const toggleButton = screen.getByRole("button", { name: "Toggle Sidebar" });
    await user.click(toggleButton);
    expect(container.querySelector(".w-0")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Select agent" }));
    expect(replaceMock).toHaveBeenLastCalledWith(
      "/agents?view=dispatch&agent=agent-2",
      {
        scroll: false,
      },
    );

    await user.click(screen.getByRole("tab", { name: "Visualization" }));
    useSearchParamsMock.mockReturnValue(new URLSearchParams("view=visualization"));
    rerender(
      <AgentWorkspace
        agents={[
          { id: "agent-1", memberId: "member-1" } as never,
          { id: "agent-2", memberId: "member-2" } as never,
        ]}
        pool={null}
        runtimeCatalog={null}
        bridgeHealth={null}
        dispatchStats={{ outcomes: { started: 2, queued: 1 } } as never}
        loading={false}
        requestedMemberId={null}
        dispatchHistoryByTask={{}}
        fetchDispatchHistory={fetchDispatchHistoryMock}
        onPause={jest.fn()}
        onResume={jest.fn()}
        onKill={jest.fn()}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Select graph agent" }));
    expect(replaceMock).toHaveBeenLastCalledWith(
      "/agents?view=visualization&agent=agent-3",
      {
        scroll: false,
      },
    );
  });

  it("opens a workspace spawn dialog when project-scoped tasks and members are available", async () => {
    const user = userEvent.setup();
    useSearchParamsMock.mockReturnValue(new URLSearchParams(""));

    render(
      <AgentWorkspace
        agents={[]}
        pool={null}
        runtimeCatalog={null}
        bridgeHealth={null}
        dispatchStats={null}
        loading={false}
        requestedMemberId={null}
        dispatchHistoryByTask={{}}
        fetchDispatchHistory={fetchDispatchHistoryMock}
        selectedProjectId="project-1"
        tasks={[
          {
            id: "task-1",
            projectId: "project-1",
            title: "Review launch plan",
            status: "assigned",
          } as never,
        ]}
        members={[
          {
            id: "member-1",
            name: "Reviewer",
            type: "agent",
            typeLabel: "Agent",
          } as never,
        ]}
        onSpawnAgent={jest.fn()}
        onPause={jest.fn()}
        onResume={jest.fn()}
        onKill={jest.fn()}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Spawn Agent" }));

    expect(screen.getByTestId("workspace-spawn-dialog")).toBeInTheDocument();
  });

  it("hydrates visualization view and focus from URL state and ignores missing focus ids", () => {
    useSearchParamsMock.mockReturnValue(
      new URLSearchParams("view=visualization&vizNode=task%3Atask-2"),
    );

    const { rerender } = render(
      <AgentWorkspace
        agents={[{ id: "agent-1", memberId: "member-1" } as never]}
        pool={null}
        runtimeCatalog={null}
        bridgeHealth={null}
        dispatchStats={null}
        loading={false}
        requestedMemberId={null}
        dispatchHistoryByTask={{}}
        fetchDispatchHistory={fetchDispatchHistoryMock}
        onPause={jest.fn()}
        onResume={jest.fn()}
        onKill={jest.fn()}
      />,
    );

    expect(
      screen.getByRole("tab", { name: "Visualization", selected: true }),
    ).toBeInTheDocument();
    expect(
      screen.getByTestId("workspace-visualization-focus"),
    ).toHaveTextContent("task:task:task-2");

    useSearchParamsMock.mockReturnValue(
      new URLSearchParams("view=visualization&vizNode=task%3Amissing"),
    );
    rerender(
      <AgentWorkspace
        agents={[{ id: "agent-1", memberId: "member-1" } as never]}
        pool={null}
        runtimeCatalog={null}
        bridgeHealth={null}
        dispatchStats={null}
        loading={false}
        requestedMemberId={null}
        dispatchHistoryByTask={{}}
        fetchDispatchHistory={fetchDispatchHistoryMock}
        onPause={jest.fn()}
        onResume={jest.fn()}
        onKill={jest.fn()}
      />,
    );

    expect(
      screen.getByRole("tab", { name: "Visualization", selected: true }),
    ).toBeInTheDocument();
    expect(
      screen.queryByTestId("workspace-visualization-focus"),
    ).not.toBeInTheDocument();
  });

  it("writes visualization view and focused node state back into the URL", async () => {
    const user = userEvent.setup();
    useSearchParamsMock.mockReturnValue(new URLSearchParams(""));

    const { rerender } = render(
      <AgentWorkspace
        agents={[{ id: "agent-1", memberId: "member-1" } as never]}
        pool={null}
        runtimeCatalog={null}
        bridgeHealth={null}
        dispatchStats={null}
        loading={false}
        requestedMemberId={null}
        dispatchHistoryByTask={{}}
        fetchDispatchHistory={fetchDispatchHistoryMock}
        onPause={jest.fn()}
        onResume={jest.fn()}
        onKill={jest.fn()}
      />,
    );

    await user.click(screen.getByRole("tab", { name: "Visualization" }));
    expect(replaceMock).toHaveBeenCalledWith("/agents?view=visualization", {
      scroll: false,
    });
    useSearchParamsMock.mockReturnValue(new URLSearchParams("view=visualization"));
    rerender(
      <AgentWorkspace
        agents={[{ id: "agent-1", memberId: "member-1" } as never]}
        pool={null}
        runtimeCatalog={null}
        bridgeHealth={null}
        dispatchStats={null}
        loading={false}
        requestedMemberId={null}
        dispatchHistoryByTask={{}}
        fetchDispatchHistory={fetchDispatchHistoryMock}
        onPause={jest.fn()}
        onResume={jest.fn()}
        onKill={jest.fn()}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Focus task node" }));
    expect(replaceMock).toHaveBeenCalledWith(
      "/agents?view=visualization&vizNode=task%3Atask-2",
      {
        scroll: false,
      },
    );
  });

  it("polls running agents every 5 seconds while the monitor view is active", async () => {
    jest.useFakeTimers();
    const fetchAgentMock = jest.fn().mockResolvedValue(null);
    useSearchParamsMock.mockReturnValue(new URLSearchParams("view=monitor"));

    render(
      <AgentWorkspace
        agents={[
          { id: "agent-1", memberId: "member-1", status: "running" } as never,
          { id: "agent-2", memberId: "member-2", status: "paused" } as never,
          { id: "agent-3", memberId: "member-3", status: "starting" } as never,
        ]}
        pool={null}
        runtimeCatalog={null}
        bridgeHealth={null}
        dispatchStats={null}
        loading={false}
        requestedMemberId={null}
        dispatchHistoryByTask={{}}
        fetchDispatchHistory={fetchDispatchHistoryMock}
        fetchAgent={fetchAgentMock}
        onPause={jest.fn()}
        onResume={jest.fn()}
        onKill={jest.fn()}
      />,
    );

    jest.advanceTimersByTime(5000);
    await Promise.resolve();

    expect(fetchAgentMock).toHaveBeenCalledTimes(1);
    expect(fetchAgentMock).toHaveBeenCalledWith("agent-1");
  });

  it("does not poll agent telemetry outside the monitor view", async () => {
    jest.useFakeTimers();
    const fetchAgentMock = jest.fn().mockResolvedValue(null);
    useSearchParamsMock.mockReturnValue(new URLSearchParams("view=dispatch"));

    render(
      <AgentWorkspace
        agents={[
          { id: "agent-1", memberId: "member-1", status: "running" } as never,
        ]}
        pool={null}
        runtimeCatalog={null}
        bridgeHealth={null}
        dispatchStats={null}
        loading={false}
        requestedMemberId={null}
        dispatchHistoryByTask={{}}
        fetchDispatchHistory={fetchDispatchHistoryMock}
        fetchAgent={fetchAgentMock}
        onPause={jest.fn()}
        onResume={jest.fn()}
        onKill={jest.fn()}
      />,
    );

    jest.advanceTimersByTime(5000);
    await Promise.resolve();

    expect(fetchAgentMock).not.toHaveBeenCalled();
  });
});
