jest.mock("next-intl", () => ({
  useTranslations: () => (key: string) => {
    const map: Record<string, string> = {
      "monitor.title": "Monitor",
      "visualization.title": "Visualization",
      "stats.dispatch": "Dispatch",
      "workspace.toggleSidebar": "Toggle Sidebar",
      "monitor.teamsLink": "Teams",
      "monitor.loading": "Loading agents...",
    };
    return map[key] ?? key;
  },
}));

const replaceMock = jest.fn();
const useSearchParamsMock = jest.fn();
const useBreakpointMock = jest.fn();

jest.mock("next/navigation", () => ({
  useRouter: () => ({ replace: replaceMock }),
  useSearchParams: () => useSearchParamsMock(),
}));

jest.mock("@/hooks/use-breakpoint", () => ({
  useBreakpoint: () => useBreakpointMock(),
}));

jest.mock("@/components/ui/sheet", () => ({
  Sheet: ({
    open,
    onOpenChange,
    children,
  }: {
    open?: boolean;
    onOpenChange?: (open: boolean) => void;
    children?: React.ReactNode;
  }) => (
    <div data-testid={open ? "sheet-open" : "sheet-closed"}>
      {children}
      {open ? (
        <button type="button" onClick={() => onOpenChange?.(false)}>
          Close Detail Sheet
        </button>
      ) : null}
    </div>
  ),
  SheetContent: (props: React.HTMLAttributes<HTMLDivElement> & { showCloseButton?: boolean }) => {
    const { children, showCloseButton, ...divProps } = props;
    void showCloseButton;

    return (
      <div data-testid="detail-sheet-content" {...divProps}>
        {children}
      </div>
    );
  },
  SheetHeader: ({ children }: { children?: React.ReactNode }) => <div>{children}</div>,
  SheetTitle: ({ children }: { children?: React.ReactNode }) => <div>{children}</div>,
  SheetDescription: ({ children }: { children?: React.ReactNode }) => <div>{children}</div>,
}));

jest.mock("./agent-workspace-sidebar", () => ({
  AgentWorkspaceSidebar: () => <div data-testid="workspace-sidebar">sidebar</div>,
}));

jest.mock("./agent-workspace-overview", () => ({
  AgentWorkspaceOverview: ({ activeTab }: { activeTab: string }) => (
    <div data-testid={`workspace-overview-${activeTab}`}>{activeTab}</div>
  ),
}));

jest.mock("./agent-visualization-model", () => ({
  buildAgentVisualizationModel: () => ({
    nodes: [],
    edges: [],
    summary: {
      agentCount: 0,
      queueCount: 0,
      runtimeCount: 0,
      taskCount: 0,
      hasGraphData: false,
      isFiltered: false,
      isDegraded: false,
    },
    focusByNodeId: {},
  }),
}));

jest.mock("./agent-visualization-canvas", () => ({
  AgentVisualizationCanvas: () => <div data-testid="workspace-visualization">viz</div>,
}));

jest.mock("../tasks/spawn-agent-dialog", () => ({
  SpawnAgentDialog: () => null,
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

describe("AgentWorkspace detail panel", () => {
  beforeEach(() => {
    replaceMock.mockReset();
    useBreakpointMock.mockReturnValue({
      breakpoint: "desktop",
      isMobile: false,
      isTablet: false,
      isDesktop: true,
    });
  });

  it("keeps the overview visible while opening the selected agent in a slide-out panel", async () => {
    const user = userEvent.setup();
    useSearchParamsMock.mockReturnValue(new URLSearchParams("agent=agent-1"));

    render(
      <AgentWorkspace
        agents={[{ id: "agent-1", memberId: "member-1" } as never]}
        pool={null}
        runtimeCatalog={null}
        bridgeHealth={null}
        dispatchStats={null}
        loading={false}
        requestedMemberId={null}
        dispatchHistoryByTask={{}}
        fetchDispatchHistory={jest.fn()}
        onPause={jest.fn()}
        onResume={jest.fn()}
        onKill={jest.fn()}
      />,
    );

    expect(screen.getByTestId("workspace-overview-monitor")).toBeInTheDocument();
    expect(screen.getByTestId("sheet-open")).toBeInTheDocument();
    expect(screen.getByTestId("workspace-detail")).toHaveTextContent("agent-1");

    await user.click(screen.getByRole("button", { name: "Back" }));

    expect(replaceMock).toHaveBeenCalledWith("/agents?", { scroll: false });
  });

  it("clears the selected agent when the slide-out sheet is dismissed externally", async () => {
    const user = userEvent.setup();
    useSearchParamsMock.mockReturnValue(new URLSearchParams("agent=agent-1"));

    render(
      <AgentWorkspace
        agents={[{ id: "agent-1", memberId: "member-1" } as never]}
        pool={null}
        runtimeCatalog={null}
        bridgeHealth={null}
        dispatchStats={null}
        loading={false}
        requestedMemberId={null}
        dispatchHistoryByTask={{}}
        fetchDispatchHistory={jest.fn()}
        onPause={jest.fn()}
        onResume={jest.fn()}
        onKill={jest.fn()}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Close Detail Sheet" }));

    expect(replaceMock).toHaveBeenCalledWith("/agents?", { scroll: false });
  });
});
