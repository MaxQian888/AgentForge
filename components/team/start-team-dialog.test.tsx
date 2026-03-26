import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { StartTeamDialog } from "./start-team-dialog";

const startTeam = jest.fn();

const currentProject = {
  id: "project-1",
  name: "AgentForge",
  description: "Provider-complete workspace",
  status: "active",
  taskCount: 0,
  agentCount: 0,
  createdAt: "2026-03-25T10:00:00.000Z",
  settings: {
    codingAgent: {
      runtime: "codex",
      provider: "openai",
      model: "gpt-5-codex",
    },
  },
  codingAgentCatalog: {
    defaultRuntime: "claude_code",
    defaultSelection: {
      runtime: "codex",
      provider: "openai",
      model: "gpt-5-codex",
    },
    runtimes: [
      {
        runtime: "codex",
        label: "Codex",
        defaultProvider: "openai",
        compatibleProviders: ["openai", "codex"],
        defaultModel: "gpt-5-codex",
        available: true,
        diagnostics: [],
      },
      {
        runtime: "opencode",
        label: "OpenCode",
        defaultProvider: "opencode",
        compatibleProviders: ["opencode"],
        defaultModel: "opencode-default",
        available: false,
        diagnostics: [
          {
            code: "missing_cli",
            message: "OpenCode CLI is not installed",
            blocking: true,
          },
        ],
      },
    ],
  },
};
const projectStoreState = {
  currentProject,
  projects: [currentProject],
};

jest.mock("@/lib/stores/team-store", () => ({
  useTeamStore: (selector: (state: { startTeam: typeof startTeam }) => unknown) =>
    selector({ startTeam }),
}));

jest.mock("@/lib/stores/project-store", () => ({
  useProjectStore: (
    selector: (state: { currentProject: typeof currentProject; projects: typeof currentProject[] }) => unknown
  ) => selector(projectStoreState),
}));

jest.mock("@/lib/stores/dashboard-store", () => ({
  useDashboardStore: (selector: (state: { selectedProjectId: string }) => unknown) =>
    selector({ selectedProjectId: "project-1" }),
}));

jest.mock("@/components/ui/dialog", () => ({
  Dialog: ({ children }: any) => <div>{children}</div>,
  DialogContent: ({ children }: any) => <div>{children}</div>,
  DialogHeader: ({ children }: any) => <div>{children}</div>,
  DialogTitle: ({ children }: any) => <div>{children}</div>,
  DialogDescription: ({ children }: any) => <div>{children}</div>,
  DialogFooter: ({ children }: any) => <div>{children}</div>,
}));

jest.mock("@/components/ui/select", () => {
  const React = require("react");
  return {
    Select: ({ value, onValueChange, disabled, children }: any) => {
      const options: Array<{ value: string; label: string }> = [];
      React.Children.forEach(children, (child: any) => {
        if (!child) return;
        const contentChildren = child.props?.children;
        React.Children.forEach(contentChildren, (grandChild: any) => {
          if (!grandChild || grandChild.props?.value === undefined) return;
          options.push({
            value: grandChild.props.value,
            label: grandChild.props.children,
          });
        });
      });
      return (
        <select
          aria-label="team-runtime-select"
          value={value}
          disabled={disabled}
          onChange={(event) => onValueChange?.(event.target.value)}
        >
          {options.map((option) => (
            <option key={option.value} value={option.value}>
              {option.label}
            </option>
          ))}
        </select>
      );
    },
    SelectTrigger: ({ children }: any) => <>{children}</>,
    SelectValue: () => null,
    SelectContent: ({ children }: any) => <>{children}</>,
    SelectItem: ({ children }: any) => <>{children}</>,
  };
});

describe("StartTeamDialog", () => {
  beforeEach(() => {
    startTeam.mockReset().mockResolvedValue(undefined);
    projectStoreState.currentProject = currentProject;
    projectStoreState.projects = [currentProject];
  });

  it("uses project catalog defaults and blocks unavailable runtime selections", async () => {
    const user = userEvent.setup();
    const onOpenChange = jest.fn();

    await act(async () => {
      render(
        <StartTeamDialog
          taskId="task-1"
          taskTitle="Finish provider support"
          memberId="member-1"
          open
          onOpenChange={onOpenChange}
        />
      );
    });

    expect(
      screen.getByText((content) => content.includes("OpenCode CLI is not installed"))
    ).toBeInTheDocument();

    const selects = screen.getAllByLabelText("team-runtime-select");
    await user.selectOptions(selects[0], "codex");
    await user.click(screen.getByRole("button", { name: "Start Team" }));

    expect(startTeam).toHaveBeenCalledWith(
      "task-1",
      "member-1",
      expect.objectContaining({
        strategy: "plan-code-review",
        runtime: "codex",
        provider: "openai",
        model: "gpt-5-codex",
      })
    );
  });

  it("switches runtime/provider defaults and blocks unavailable runtime submission", async () => {
    const user = userEvent.setup();
    render(
      <StartTeamDialog
        taskId="task-2"
        taskTitle="Investigate runtime drift"
        memberId="member-2"
        open
        onOpenChange={jest.fn()}
      />
    );

    const selects = screen.getAllByLabelText("team-runtime-select");
    await user.selectOptions(selects[0], "opencode");

    expect(screen.getAllByText(/OpenCode CLI is not installed/)).toHaveLength(2);
    expect(screen.getByRole("button", { name: "Start Team" })).toBeDisabled();

    await user.selectOptions(selects[0], "codex");
    expect(screen.getByRole("button", { name: "Start Team" })).not.toBeDisabled();
  });
});
