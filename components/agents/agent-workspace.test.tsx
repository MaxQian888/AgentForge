jest.mock("next-intl", () => ({
  useTranslations: () => (key: string) => {
    const map: Record<string, string> = {
      "monitor.title": "Monitor",
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
        onPause={jest.fn()}
        onResume={jest.fn()}
        onKill={jest.fn()}
      />,
    );

    expect(screen.getByTestId("workspace-detail")).toHaveTextContent("agent-1");

    await user.click(screen.getByRole("button", { name: "Back" }));
    expect(replaceMock).toHaveBeenCalledWith("/agents?", { scroll: false });
  });

  it("shows overview tabs, toggles the sidebar, and updates the selected agent", async () => {
    const user = userEvent.setup();
    useSearchParamsMock.mockReturnValue(new URLSearchParams(""));

    const { container } = render(
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
        onPause={jest.fn()}
        onResume={jest.fn()}
        onKill={jest.fn()}
      />,
    );

    expect(screen.getByTestId("workspace-overview-monitor")).toBeInTheDocument();
    await user.click(screen.getByRole("tab", { name: "Dispatch" }));
    expect(screen.getByTestId("workspace-overview-dispatch")).toBeInTheDocument();
    expect(screen.getByText("3")).toBeInTheDocument();

    const toggleButton = screen.getByRole("button", { name: "Toggle Sidebar" });
    await user.click(toggleButton);
    expect(container.querySelector(".w-0")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Select agent" }));
    expect(replaceMock).toHaveBeenCalledWith("/agents?agent=agent-2", {
      scroll: false,
    });
  });
});
