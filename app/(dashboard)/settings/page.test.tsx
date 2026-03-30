import { Children, isValidElement, type ReactElement, type ReactNode } from "react";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import SettingsPage from "./page";
import settingsMessages from "@/messages/en/settings.json";
import type { Project } from "@/lib/stores/project-store";

const mockSetTheme = jest.fn();
jest.mock("next-themes", () => ({
  useTheme: () => ({ theme: "system", setTheme: mockSetTheme }),
}));

const mockSetLocale = jest.fn();
jest.mock("@/lib/stores/locale-store", () => ({
  useLocaleStore: (selector: (s: { locale: string; setLocale: typeof mockSetLocale }) => unknown) =>
    selector({ locale: "en", setLocale: mockSetLocale }),
  SUPPORTED_LOCALES: ["en", "zh-CN"],
  DEFAULT_LOCALE: "en",
}));

const fetchProjects = jest.fn();
const updateProject = jest.fn();

function createProjectFixture(): Project {
  return {
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
  };
}

jest.mock("next-intl", () => ({
  useTranslations: () => (key: string, values?: Record<string, string | number>) => {
    const resolved = key
      .split(".")
      .reduce<unknown>((current, segment) => {
        if (!current || typeof current !== "object") {
          return undefined;
        }
        return (current as Record<string, unknown>)[segment];
      }, settingsMessages);

    if (typeof resolved !== "string") {
      return key;
    }

    return resolved.replace(/\{(\w+)\}/g, (_match, token) =>
      String(values?.[token] ?? `{${token}}`)
    );
  },
}));

jest.mock("@/components/fields/field-definition-editor", () => ({
  FieldDefinitionEditor: () => <div>FieldDefinitionEditor</div>,
}));

jest.mock("@/components/forms/form-builder", () => ({
  FormBuilder: () => <div>FormBuilder</div>,
}));

jest.mock("@/components/automations/rule-editor", () => ({
  RuleEditor: () => <div>RuleEditor</div>,
}));

jest.mock("@/components/automations/rule-list", () => ({
  RuleList: () => <div>RuleList</div>,
}));

jest.mock("@/components/automations/automation-log-viewer", () => ({
  AutomationLogViewer: () => <div>AutomationLogViewer</div>,
}));

const dashboardState = {
  selectedProjectId: "project-1",
};

const projectState = {
  projects: [createProjectFixture()],
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

describe("SettingsPage — Appearance section", () => {
  beforeEach(() => {
    mockSetTheme.mockReset();
    mockSetLocale.mockReset();
    projectState.projects = [createProjectFixture()];
    fetchProjects.mockReset().mockResolvedValue(undefined);
    updateProject.mockReset();
  });

  it("renders Appearance heading and description even when no project is selected", async () => {
    dashboardState.selectedProjectId = null as unknown as string;

    await act(async () => {
      render(<SettingsPage />);
    });

    expect(screen.getByText(settingsMessages.appearance)).toBeInTheDocument();

    // restore
    dashboardState.selectedProjectId = "project-1";
  });

  it("renders Appearance heading when a project is selected", async () => {
    dashboardState.selectedProjectId = "project-1";

    await act(async () => {
      render(<SettingsPage />);
    });

    expect(screen.getByText(settingsMessages.appearance)).toBeInTheDocument();
  });

  it("renders Light, Dark, System theme buttons", async () => {
    await act(async () => {
      render(<SettingsPage />);
    });

    expect(screen.getByRole("button", { name: settingsMessages.themeLight })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: settingsMessages.themeDark })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: settingsMessages.themeSystem })).toBeInTheDocument();
  });

  it("calls setLocale when a different language is selected", async () => {
    const user = userEvent.setup();

    await act(async () => {
      render(<SettingsPage />);
    });

    const languageSelect = screen.getByRole("combobox", { name: settingsMessages.language });
    await user.selectOptions(languageSelect, "zh-CN");

    expect(mockSetLocale).toHaveBeenCalledWith("zh-CN");
  });
});

describe("SettingsPage", () => {
  function setControlValue(element: HTMLElement, value: string) {
    fireEvent.change(element, {
      target: { value },
    });
  }

  beforeEach(() => {
    projectState.projects = [createProjectFixture()];
    fetchProjects.mockReset().mockResolvedValue(undefined);
    updateProject.mockReset().mockImplementation(async (_id: string, input: unknown) => ({
      ...projectState.projects[0],
      ...(input as Record<string, unknown>),
      settings: {
        ...projectState.projects[0].settings,
        ...((input as { settings?: Record<string, unknown> }).settings ?? {}),
      },
    }));
  });

  it("renders legacy fallback diagnostics, tracks unsaved changes, and resets the draft", async () => {
    const user = userEvent.setup();

    await act(async () => {
      render(<SettingsPage />);
    });

    expect(fetchProjects).toHaveBeenCalled();
    expect(screen.getByText("Project Settings")).toBeInTheDocument();
    expect(screen.getByText("Fallback defaults are currently active for governance settings.")).toBeInTheDocument();

    const nameInput = screen.getByRole("textbox", { name: "Project Name" });
    setControlValue(nameInput, "AgentForge Next");

    expect(screen.getByText("Unsaved changes")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Discard Changes" })).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Discard Changes" }));

    expect(nameInput).toHaveValue("AgentForge");
    expect(screen.queryByText("Unsaved changes")).not.toBeInTheDocument();
  });

  it("blocks invalid threshold saves with validation feedback", async () => {
    const user = userEvent.setup();

    await act(async () => {
      render(<SettingsPage />);
    });

    const thresholdInput = screen.getByRole("spinbutton", {
      name: "Alert Threshold (%)",
    });
    setControlValue(thresholdInput, "101");
    await user.click(screen.getByRole("button", { name: "Save Settings" }));

    expect(updateProject).not.toHaveBeenCalled();
    expect(screen.getByText("Alert threshold must be between 0 and 100.")).toBeInTheDocument();
  });

  it("keeps the draft and shows save failure feedback when the request fails", async () => {
    const user = userEvent.setup();
    updateProject.mockRejectedValueOnce(new Error("Review policy rejected by server"));

    await act(async () => {
      render(<SettingsPage />);
    });

    const nameInput = screen.getByRole("textbox", { name: "Project Name" });
    setControlValue(nameInput, "AgentForge Failing Save");
    await user.click(screen.getByRole("button", { name: "Save Settings" }));

    await waitFor(() => {
      expect(screen.getByText("Review policy rejected by server")).toBeInTheDocument();
    });
    expect(nameInput).toHaveValue("AgentForge Failing Save");
    expect(screen.getByText("Unsaved changes")).toBeInTheDocument();
  });

  it("preserves all configured review layers when saving unrelated settings changes", async () => {
    const user = userEvent.setup();

    projectState.projects = [
      {
        ...createProjectFixture(),
        settings: {
          ...createProjectFixture().settings,
          reviewPolicy: {
            autoTriggerOnPR: true,
            requiredLayers: ["layer1", "layer2"],
            minRiskLevelForBlock: "high",
            requireManualApproval: true,
            enabledPluginDimensions: ["security"],
          },
        },
      },
    ];

    await act(async () => {
      render(<SettingsPage />);
    });

    const nameInput = screen.getByRole("textbox", { name: "Project Name" });
    setControlValue(nameInput, "AgentForge Policy");
    await user.click(screen.getByRole("button", { name: "Save Settings" }));

    await waitFor(() => {
      expect(updateProject).toHaveBeenCalledWith(
        "project-1",
        expect.objectContaining({
          settings: expect.objectContaining({
            reviewPolicy: expect.objectContaining({
              requiredLayers: ["layer1", "layer2"],
            }),
          }),
        })
      );
    });
  });

  it("keeps an empty review policy empty when saving unrelated settings changes", async () => {
    const user = userEvent.setup();

    await act(async () => {
      render(<SettingsPage />);
    });

    const nameInput = screen.getByRole("textbox", { name: "Project Name" });
    setControlValue(nameInput, "AgentForge Legacy Policy");
    await user.click(screen.getByRole("button", { name: "Save Settings" }));

    await waitFor(() => {
      expect(updateProject).toHaveBeenCalledWith(
        "project-1",
        expect.objectContaining({
          settings: expect.objectContaining({
            reviewPolicy: expect.objectContaining({
              requiredLayers: [],
              minRiskLevelForBlock: "",
            }),
          }),
        })
      );
    });
  });

  it("renders runtime diagnostics and clears draft state after a successful save", async () => {
    const user = userEvent.setup();

    await act(async () => {
      render(<SettingsPage />);
    });

    const selects = screen.getAllByLabelText("coding-agent-select");
    await user.selectOptions(selects[0], "opencode");

    expect(screen.getByText("Draft runtime selection differs from the last saved project settings.")).toBeInTheDocument();
    expect(screen.getByText("OpenCode CLI is not installed")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Save Settings" }));

    await waitFor(() => {
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

    expect(screen.getByText("Settings saved")).toBeInTheDocument();
    expect(screen.queryByText("Unsaved changes")).not.toBeInTheDocument();
  });

  it("preserves the webhook secret locally after a successful save even when the response redacts it", async () => {
    const user = userEvent.setup();

    projectState.projects = [
      {
        ...createProjectFixture(),
        settings: {
          ...createProjectFixture().settings,
          webhook: {
            url: "https://hooks.example.com/agentforge",
            secret: "signing-secret",
            events: ["push"],
            active: true,
          },
        },
      },
    ];

    updateProject.mockImplementation(async (_id: string, input: unknown) => ({
      ...projectState.projects[0],
      ...(input as Record<string, unknown>),
      settings: {
        ...projectState.projects[0].settings,
        ...((input as { settings?: Record<string, unknown> }).settings ?? {}),
        webhook: {
          ...projectState.projects[0].settings.webhook,
          ...((input as { settings?: { webhook?: Record<string, unknown> } }).settings?.webhook ?? {}),
          secret: "",
        },
      },
    }));

    await act(async () => {
      render(<SettingsPage />);
    });

    const secretInput = screen.getByLabelText("Webhook Secret") as HTMLInputElement;
    const nameInput = screen.getByRole("textbox", { name: "Project Name" });

    expect(secretInput).toHaveValue("signing-secret");

    setControlValue(nameInput, "AgentForge Retained Secret");
    await user.click(screen.getByRole("button", { name: "Save Settings" }));

    await waitFor(() => {
      expect(updateProject).toHaveBeenCalledTimes(1);
    });

    expect(secretInput).toHaveValue("signing-secret");

    setControlValue(nameInput, "AgentForge Second Save");
    await user.click(screen.getByRole("button", { name: "Save Settings" }));

    await waitFor(() => {
      expect(updateProject).toHaveBeenCalledTimes(2);
    });

    expect(updateProject).toHaveBeenNthCalledWith(
      2,
      "project-1",
      expect.objectContaining({
        settings: expect.objectContaining({
          webhook: expect.objectContaining({
            secret: "signing-secret",
          }),
        }),
      })
    );
  });
});
