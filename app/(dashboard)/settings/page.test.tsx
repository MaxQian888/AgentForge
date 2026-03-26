import { Children, isValidElement, type ReactElement, type ReactNode } from "react";
import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import SettingsPage from "./page";

const fetchProjects = jest.fn();
const updateProject = jest.fn();

const dashboardState = {
  selectedProjectId: "project-1",
};

const projectState = {
  projects: [
    {
      id: "project-1",
      name: "AgentForge",
      description: "Provider-complete workspace",
      status: "active",
      taskCount: 0,
      agentCount: 0,
      createdAt: "2026-03-25T10:00:00.000Z",
      repoUrl: "https://github.com/acme/agentforge",
      defaultBranch: "main",
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
    },
  ],
  fetchProjects,
  updateProject,
};

jest.mock("@/lib/stores/dashboard-store", () => ({
  useDashboardStore: () => dashboardState,
}));

jest.mock("@/lib/stores/project-store", () => ({
  useProjectStore: () => projectState,
}));

type SelectMockProps = {
  value?: string;
  onValueChange?: (value: string) => void;
  disabled?: boolean;
  children?: ReactNode;
};

type SelectItemElement = ReactElement<{ value?: string; children?: ReactNode }>;

function readOptionLabel(node: ReactNode): string {
  if (typeof node === "string") {
    return node;
  }
  if (typeof node === "number") {
    return String(node);
  }
  return "";
}

jest.mock("@/components/ui/select", () => {
  return {
    Select: ({ value, onValueChange, disabled, children }: SelectMockProps) => {
      const options: Array<{ value: string; label: string }> = [];
      Children.forEach(children, (child) => {
        if (!isValidElement(child)) return;
        const contentChildren = (child as ReactElement<{ children?: ReactNode }>).props.children;
        Children.forEach(contentChildren, (grandChild) => {
          if (!isValidElement(grandChild)) return;
          const item = grandChild as SelectItemElement;
          if (item.props.value === undefined) return;
          options.push({
            value: item.props.value,
            label: readOptionLabel(item.props.children),
          });
        });
      });
      return (
        <select
          aria-label="coding-agent-select"
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
    SelectTrigger: ({ children }: { children?: ReactNode }) => <>{children}</>,
    SelectValue: () => null,
    SelectContent: ({ children }: { children?: ReactNode }) => <>{children}</>,
    SelectItem: ({ children }: { children?: ReactNode }) => <>{children}</>,
  };
});

describe("SettingsPage", () => {
  beforeEach(() => {
    fetchProjects.mockReset().mockResolvedValue(undefined);
    updateProject.mockReset().mockResolvedValue(undefined);
  });

  it("renders runtime catalog diagnostics and saves coding-agent defaults", async () => {
    const user = userEvent.setup();

    await act(async () => {
      render(<SettingsPage />);
    });

    expect(fetchProjects).toHaveBeenCalled();
    expect(screen.getByText("Project Settings")).toBeInTheDocument();
    expect(screen.getByText("OpenCode CLI is not installed")).toBeInTheDocument();

    const selects = screen.getAllByLabelText("coding-agent-select");
    await user.selectOptions(selects[0], "opencode");
    await user.click(screen.getByRole("button", { name: "Save Settings" }));

    expect(updateProject).toHaveBeenCalledWith(
      "project-1",
      expect.objectContaining({
        settings: expect.objectContaining({
          codingAgent: expect.objectContaining({
            runtime: "opencode",
          }),
        }),
      })
    );
  });
});
