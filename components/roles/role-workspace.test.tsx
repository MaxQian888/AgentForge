import { Children, isValidElement, type ReactNode, type ReactElement } from "react";
import rolesMessages from "../../messages/en/roles.json";

jest.mock("@/components/ui/select", () => {
  function flattenOptions(children: ReactNode): Array<{ value: string; label: string }> {
    const options: Array<{ value: string; label: string }> = [];
    function visit(node: ReactNode) {
      Children.forEach(node, (child) => {
        if (!isValidElement(child)) return;
        const element = child as ReactElement<{ children?: ReactNode; value?: string }>;
        if (element.props.value !== undefined) {
          options.push({
            value: element.props.value,
            label: typeof element.props.children === "string" ? element.props.children : String(element.props.value),
          });
          return;
        }
        visit(element.props.children);
      });
    }
    visit(children);
    return options;
  }

  return {
    Select: ({ value, onValueChange, children }: { value?: string; onValueChange?: (v: string) => void; children?: ReactNode }) => {
      const options = flattenOptions(children);
      let ariaLabel: string | undefined;
      Children.forEach(children, (child) => {
        if (!isValidElement(child)) return;
        const el = child as ReactElement<{ "aria-label"?: string }>;
        if (el.props["aria-label"]) ariaLabel = el.props["aria-label"];
      });
      return (
        <select aria-label={ariaLabel} value={value} onChange={(e: React.ChangeEvent<HTMLSelectElement>) => onValueChange?.(e.target.value)}>
          {options.map((o) => (
            <option key={o.value} value={o.value}>{o.label}</option>
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

jest.mock("next-intl", () => ({
  useTranslations: () => (key: string, values?: Record<string, string | number>) => {
    const resolved = key
      .split(".")
      .reduce<unknown>((current, part) => {
        if (current && typeof current === "object" && part in current) {
          return (current as Record<string, unknown>)[part];
        }
        return undefined;
      }, rolesMessages as unknown as Record<string, unknown>);

    if (typeof resolved !== "string") {
      return key;
    }
    return Object.entries(values ?? {}).reduce(
      (message, [name, value]) => message.replace(`{${name}}`, String(value)),
      resolved,
    );
  },
}));

let mockIsMobile = false;
jest.mock("@/hooks/use-mobile", () => ({
  useIsMobile: () => mockIsMobile,
}));

// matchMedia mock for auto-collapse logic
function mockMatchMedia(matches: boolean) {
  const listeners: Array<(e: { matches: boolean }) => void> = [];
  Object.defineProperty(window, "matchMedia", {
    configurable: true,
    writable: true,
    value: jest.fn().mockImplementation((query: string) => ({
      matches,
      media: query,
      addEventListener: (_: string, cb: (e: { matches: boolean }) => void) => listeners.push(cb),
      removeEventListener: (_: string, cb: (e: { matches: boolean }) => void) => {
        const idx = listeners.indexOf(cb);
        if (idx >= 0) listeners.splice(idx, 1);
      },
      addListener: jest.fn(),
      removeListener: jest.fn(),
      onchange: null,
      dispatchEvent: jest.fn(),
    })),
  });
  return listeners;
}

import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { RoleWorkspace } from "./role-workspace";
import type { RoleManifest, RoleSkillCatalogEntry } from "@/lib/stores/role-store";

const frontendRole: RoleManifest = {
  apiVersion: "agentforge/v1",
  kind: "Role",
  metadata: {
    id: "frontend-developer",
    name: "Frontend Developer",
    version: "1.2.0",
    description: "Builds polished product UI",
    author: "AgentForge",
    tags: ["frontend", "ui"],
  },
  identity: {
    role: "Senior Frontend Developer",
    goal: "Ship a great dashboard UX",
    backstory: "A patient frontend specialist",
    systemPrompt: "Focus on clarity and accessibility.",
    persona: "Helpful",
    goals: ["Improve workflows"],
    constraints: ["Keep tests green"],
    personality: "patient",
    language: "zh-CN",
    responseStyle: {
      tone: "professional",
      verbosity: "concise",
      formatPreference: "markdown",
    },
  },
  capabilities: {
    packages: ["design-system"],
    allowedTools: ["Read", "Edit"],
    toolConfig: {
      builtIn: ["Read", "Edit"],
      external: ["figma"],
      mcpServers: [{ name: "design-mcp", url: "http://localhost:3010/mcp" }],
    },
    customSettings: {
      approval_mode: "guided",
    },
    languages: ["TypeScript"],
    frameworks: ["Next.js"],
    skills: [
      { path: "skills/react", autoLoad: true },
      { path: "skills/testing", autoLoad: false },
    ],
    maxTurns: 24,
    maxBudgetUsd: 6,
  },
  knowledge: {
    repositories: ["app", "components"],
    documents: ["docs/PRD.md"],
    patterns: ["responsive-layouts"],
    shared: [
      {
        id: "design-guidelines",
        type: "vector",
        access: "read",
        description: "Shared UI guidance",
        sources: ["docs/PRD.md"],
      },
    ],
    memory: {
      shortTerm: { maxTokens: 64000 },
      episodic: { enabled: true, retentionDays: 45 },
    },
  },
  security: {
    profile: "standard",
    permissionMode: "default",
    allowedPaths: ["app/", "components/"],
    deniedPaths: ["secrets/"],
    maxBudgetUsd: 6,
    requireReview: true,
    outputFilters: ["no_pii"],
  },
  collaboration: {
    canDelegateTo: ["frontend-developer"],
    acceptsDelegationFrom: ["design-manager"],
    communication: {
      preferredChannel: "structured",
      reportFormat: "markdown",
      escalationPolicy: "auto",
    },
  },
  triggers: [{ event: "pr_created", action: "auto_review", condition: "labels.includes('ui')" }],
  extends: "coding-agent",
  overrides: {
    "identity.role": "Principal Frontend Developer",
  },
};

const skillCatalog: RoleSkillCatalogEntry[] = [
  {
    path: "skills/react",
    label: "React",
    description: "Build React interfaces.",
    source: "repo-local",
    sourceRoot: "skills",
  },
  {
    path: "skills/testing",
    label: "Testing",
    description: "Verify product behavior.",
    source: "repo-local",
    sourceRoot: "skills",
  },
];

function setViewport(width: number) {
  Object.defineProperty(window, "innerWidth", {
    configurable: true,
    writable: true,
    value: width,
  });
  window.dispatchEvent(new Event("resize"));
}

describe("RoleWorkspace", () => {
  function setFieldValue(label: string, value: string) {
    fireEvent.change(screen.getByLabelText(label), {
      target: { value },
    });
  }

  beforeEach(() => {
    mockIsMobile = false;
    mockMatchMedia(true); // >= 1280px by default
    setViewport(1440);
  });

  it("supports template-based creation with a live execution summary rail", async () => {
    const user = userEvent.setup();
    const onCreateRole = jest.fn().mockResolvedValue(undefined);

    render(
      <RoleWorkspace
        roles={[frontendRole]}
        skillCatalog={skillCatalog}
        loading={false}
        error={null}
        onCreateRole={onCreateRole}
        onUpdateRole={jest.fn().mockResolvedValue(undefined)}
        onDeleteRole={jest.fn().mockResolvedValue(undefined)}
        onPreviewRole={jest.fn().mockResolvedValue(undefined)}
        onSandboxRole={jest.fn().mockResolvedValue(undefined)}
      />,
    );

    await user.click(screen.getByRole("button", { name: "New Role" }));
    await user.selectOptions(screen.getByLabelText("Start from template"), "frontend-developer");
    setFieldValue("Role ID", "frontend-developer-copy");
    await user.click(screen.getByRole("button", { name: "Save Role" }));

    expect(onCreateRole).toHaveBeenCalledWith(
      expect.objectContaining({
        metadata: expect.objectContaining({
          id: "frontend-developer-copy",
          name: "Frontend Developer",
          version: "1.2.0",
        }),
        capabilities: expect.objectContaining({
          customSettings: { approval_mode: "guided" },
          toolConfig: expect.objectContaining({
            mcpServers: [{ name: "design-mcp", url: "http://localhost:3010/mcp" }],
          }),
        }),
        knowledge: expect.objectContaining({
          shared: [
            expect.objectContaining({
              id: "design-guidelines",
              description: "Shared UI guidance",
              sources: ["docs/PRD.md"],
            }),
          ],
          memory: expect.objectContaining({
            shortTerm: { maxTokens: 64000 },
            episodic: { enabled: true, retentionDays: 45 },
          }),
        }),
        overrides: { "identity.role": "Principal Frontend Developer" },
      }),
    );

    expect(screen.getByText("Execution Summary")).toBeInTheDocument();
    expect(screen.getByText("Ship a great dashboard UX")).toBeInTheDocument();
    expect(screen.getByText("Read, Edit")).toBeInTheDocument();
    expect(screen.getByText("1 auto-load / 1 on-demand")).toBeInTheDocument();
    expect(screen.getByText("Review required")).toBeInTheDocument();
    expect(screen.getByText("Authoring Guide")).toBeInTheDocument();
    expect(screen.getByText("YAML Preview")).toBeInTheDocument();
  });

  it("loads an existing role into structured workspace sections", async () => {
    const user = userEvent.setup();

    render(
      <RoleWorkspace
        roles={[frontendRole]}
        skillCatalog={skillCatalog}
        loading={false}
        error={null}
        onCreateRole={jest.fn().mockResolvedValue(undefined)}
        onUpdateRole={jest.fn().mockResolvedValue(undefined)}
        onDeleteRole={jest.fn().mockResolvedValue(undefined)}
        onPreviewRole={jest.fn().mockResolvedValue(undefined)}
        onSandboxRole={jest.fn().mockResolvedValue(undefined)}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Edit Frontend Developer" }));

    expect(screen.getByRole("button", { name: "Setup" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Identity" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Capabilities" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Knowledge" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Governance" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Review" })).toBeInTheDocument();
    expect(screen.getByText("Editing existing role")).toBeInTheDocument();
    expect(screen.getByText("Inherited from coding-agent")).toBeInTheDocument();
    expect(screen.getByLabelText("Role ID")).toHaveValue("frontend-developer");
    expect(screen.getByDisplayValue("1.2.0")).toBeInTheDocument();
    // Navigate to Capabilities to check skill fields
    await user.click(screen.getByRole("button", { name: "Capabilities" }));
    expect(screen.getAllByText("Skills").length).toBeGreaterThan(0);
    expect(screen.getByDisplayValue("skills/react")).toBeInTheDocument();
    expect(screen.getByDisplayValue("skills/testing")).toBeInTheDocument();
  });

  it("blocks save when duplicate skill paths are present", async () => {
    const user = userEvent.setup();
    const onCreateRole = jest.fn().mockResolvedValue(undefined);

    render(
      <RoleWorkspace
        roles={[frontendRole]}
        skillCatalog={skillCatalog}
        loading={false}
        error={null}
        onCreateRole={onCreateRole}
        onUpdateRole={jest.fn().mockResolvedValue(undefined)}
        onDeleteRole={jest.fn().mockResolvedValue(undefined)}
        onPreviewRole={jest.fn().mockResolvedValue(undefined)}
        onSandboxRole={jest.fn().mockResolvedValue(undefined)}
      />,
    );

    await user.click(screen.getByRole("button", { name: "New Role" }));
    await user.selectOptions(screen.getByLabelText("Start from template"), "frontend-developer");
    await user.click(screen.getByRole("button", { name: "Capabilities" }));
    const skillInputs = screen.getAllByLabelText("Skill Path");
    fireEvent.change(skillInputs[1]!, { target: { value: "skills/react" } });
    await user.click(screen.getByRole("button", { name: "Save Role" }));

    expect(onCreateRole).not.toHaveBeenCalled();
    expect(await screen.findByText("Skill paths must be unique.")).toBeInTheDocument();
  }, 10000);

  it("submits preview and sandbox requests for the current draft and renders their results", async () => {
    const user = userEvent.setup();
    const onPreviewRole = jest.fn().mockResolvedValue({
      normalizedManifest: frontendRole,
      effectiveManifest: frontendRole,
      executionProfile: {
        role_id: "frontend-developer",
        name: "Frontend Developer",
        role: "Senior Frontend Developer",
        goal: "Ship a great dashboard UX",
        backstory: "A patient frontend specialist",
        system_prompt: "Focus on clarity and accessibility.",
        allowed_tools: ["Read", "Edit"],
        loaded_skills: [
          {
            path: "skills/react",
            label: "React",
            description: "React UI implementation guidance",
            instructions: "Prefer server-safe React composition.",
          },
        ],
        available_skills: [
          {
            path: "skills/testing",
            label: "Testing",
            description: "Regression-oriented test guidance",
          },
        ],
        max_budget_usd: 6,
        max_turns: 24,
        permission_mode: "default",
      },
    });
    const onSandboxRole = jest.fn().mockResolvedValue({
      normalizedManifest: frontendRole,
      effectiveManifest: frontendRole,
      executionProfile: {
        role_id: "frontend-developer",
        name: "Frontend Developer",
        role: "Senior Frontend Developer",
        goal: "Ship a great dashboard UX",
        backstory: "A patient frontend specialist",
        system_prompt: "Focus on clarity and accessibility.",
        allowed_tools: ["Read", "Edit"],
        loaded_skills: [
          {
            path: "skills/react",
            label: "React",
            description: "React UI implementation guidance",
            instructions: "Prefer server-safe React composition.",
          },
        ],
        available_skills: [
          {
            path: "skills/testing",
            label: "Testing",
            description: "Regression-oriented test guidance",
          },
        ],
        max_budget_usd: 6,
        max_turns: 24,
        permission_mode: "default",
      },
      selection: {
        runtime: "claude_code",
        provider: "anthropic",
        model: "claude-sonnet-4-5",
      },
      probe: {
        text: "A calm frontend specialist for dashboard polish.",
        usage: { input_tokens: 12, output_tokens: 8 },
      },
    });

    render(
      <RoleWorkspace
        roles={[frontendRole]}
        skillCatalog={skillCatalog}
        loading={false}
        error={null}
        onCreateRole={jest.fn().mockResolvedValue(undefined)}
        onUpdateRole={jest.fn().mockResolvedValue(undefined)}
        onDeleteRole={jest.fn().mockResolvedValue(undefined)}
        onPreviewRole={onPreviewRole}
        onSandboxRole={onSandboxRole}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Preview Role Draft" }));
    expect(onPreviewRole).toHaveBeenCalledWith(
      expect.objectContaining({
        draft: expect.objectContaining({
          metadata: expect.objectContaining({ id: "" }),
        }),
      }),
    );

    setFieldValue("Sandbox Input", "Summarize this role.");
    await user.click(screen.getByRole("button", { name: "Run Sandbox Probe" }));
    expect(onSandboxRole).toHaveBeenCalledWith(
      expect.objectContaining({
        input: "Summarize this role.",
      }),
    );

    expect(await screen.findByText("A calm frontend specialist for dashboard polish.")).toBeInTheDocument();
    expect(screen.getByText("claude_code / anthropic / claude-sonnet-4-5")).toBeInTheDocument();
    expect(screen.getByText("Loaded skills")).toBeInTheDocument();
    expect(screen.getAllByText(/React \(skills\/react\)/).length).toBeGreaterThan(0);
    expect(screen.getByText("On-demand skills")).toBeInTheDocument();
    expect(screen.getAllByText(/Testing \(skills\/testing\)/).length).toBeGreaterThan(0);
  });

  it("updates an existing role with advanced workspace sections and can switch back to create mode", async () => {
    const user = userEvent.setup();
    const onUpdateRole = jest.fn().mockResolvedValue(undefined);

    render(
      <RoleWorkspace
        roles={[frontendRole]}
        skillCatalog={skillCatalog}
        loading={false}
        error={null}
        onCreateRole={jest.fn().mockResolvedValue(undefined)}
        onUpdateRole={onUpdateRole}
        onDeleteRole={jest.fn().mockResolvedValue(undefined)}
        onPreviewRole={jest.fn().mockResolvedValue(undefined)}
        onSandboxRole={jest.fn().mockResolvedValue(undefined)}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Edit Frontend Developer" }));
    setFieldValue("Name", "Frontend Captain");
    await user.click(screen.getByRole("button", { name: "Capabilities" }));
    setFieldValue("Packages", "design-system, ui-kit");
    setFieldValue("External Tools", "figma, linear");
    await user.click(screen.getByRole("button", { name: "Add Custom Setting" }));
    const customSettingKeys = screen.getAllByLabelText("Custom Setting Key");
    const customSettingValues = screen.getAllByLabelText("Custom Setting Value");
    fireEvent.change(customSettingKeys[1]!, {
      target: { value: "review_depth" },
    });
    fireEvent.change(customSettingValues[1]!, {
      target: { value: "strict" },
    });
    await user.click(screen.getByRole("button", { name: "Add MCP Server" }));
    const mcpServerNames = screen.getAllByLabelText("MCP Server Name");
    const mcpServerUrls = screen.getAllByLabelText("MCP Server URL");
    fireEvent.change(mcpServerNames[1]!, {
      target: { value: "figma-sync" },
    });
    fireEvent.change(mcpServerUrls[1]!, {
      target: { value: "https://mcp.example.com" },
    });
    await user.click(screen.getByRole("button", { name: "Identity" }));
    setFieldValue("Persona", "precise");
    await user.click(screen.getByRole("button", { name: "Knowledge" }));
    await user.click(screen.getByRole("button", { name: "Add Shared Knowledge" }));
    const knowledgeIds = screen.getAllByLabelText("Shared Knowledge ID");
    const knowledgeTypes = screen.getAllByLabelText("Shared Knowledge Type");
    fireEvent.change(knowledgeIds[1]!, {
      target: { value: "team-playbook" },
    });
    fireEvent.change(knowledgeTypes[1]!, { target: { value: "doc" } });
    await user.click(screen.getByRole("button", { name: "Add Private Knowledge" }));
    const privateKnowledgeIds = screen.getAllByLabelText("Private Knowledge ID");
    const privateKnowledgeTypes = screen.getAllByLabelText("Private Knowledge Type");
    fireEvent.change(privateKnowledgeIds[0]!, {
      target: { value: "operator-notes" },
    });
    fireEvent.change(privateKnowledgeTypes[0]!, { target: { value: "doc" } });
    setFieldValue("Short-term Memory Max Tokens", "48000");
    await user.click(screen.getByLabelText(/Enable procedural memory/i));
    await user.click(screen.getByLabelText(/Learn procedural memory from feedback/i));
    await user.click(screen.getByRole("button", { name: "Governance" }));
    await user.click(screen.getByRole("button", { name: "Add Trigger" }));
    const triggerEvents = screen.getAllByLabelText("Trigger Event");
    const triggerActions = screen.getAllByLabelText("Trigger Action");
    const triggerConditions = screen.getAllByLabelText("Trigger Condition");
    fireEvent.change(triggerEvents[1]!, {
      target: { value: "task_blocked" },
    });
    fireEvent.change(triggerActions[1]!, { target: { value: "escalate" } });
    fireEvent.change(triggerConditions[1]!, {
      target: { value: "severity === 'high'" },
    });
    await user.click(screen.getByRole("button", { name: "Review" }));
    fireEvent.change(screen.getByLabelText("Role Overrides"), {
      target: { value: '{\n  "identity.role": "Frontend Captain"\n}' },
    });
    await user.click(screen.getByRole("button", { name: "Save Role" }));

    expect(onUpdateRole).toHaveBeenCalledWith(
      "frontend-developer",
      expect.objectContaining({
        metadata: expect.objectContaining({ name: "Frontend Captain" }),
        capabilities: expect.objectContaining({
          packages: ["design-system", "ui-kit"],
          toolConfig: expect.objectContaining({
            external: ["figma", "linear"],
            mcpServers: expect.arrayContaining([
              expect.objectContaining({ name: "design-mcp" }),
              expect.objectContaining({
                name: "figma-sync",
                url: "https://mcp.example.com",
              }),
            ]),
          }),
          customSettings: {
            approval_mode: "guided",
            review_depth: "strict",
          },
        }),
        identity: expect.objectContaining({ persona: "precise" }),
        knowledge: expect.objectContaining({
          shared: expect.arrayContaining([
            expect.objectContaining({ id: "design-guidelines" }),
            expect.objectContaining({ id: "team-playbook", type: "doc" }),
          ]),
          private: [expect.objectContaining({ id: "operator-notes", type: "doc" })],
          memory: expect.objectContaining({
            shortTerm: { maxTokens: 48000 },
            episodic: { enabled: true, retentionDays: 45 },
            procedural: { enabled: true, learnFromFeedback: true },
          }),
        }),
        overrides: { "identity.role": "Frontend Captain" },
        triggers: expect.arrayContaining([
          expect.objectContaining({ event: "pr_created" }),
          expect.objectContaining({
            event: "task_blocked",
            action: "escalate",
            condition: "severity === 'high'",
          }),
        ]),
      }),
    );

    await user.click(screen.getByRole("button", { name: "Switch to Create" }));
    expect(screen.getByText("Create Role")).toBeInTheDocument();
  }, 15000);

  it("shows advanced authoring context for inherited and stored-only fields", async () => {
    const user = userEvent.setup();
    const onPreviewRole = jest.fn().mockResolvedValue({
      normalizedManifest: frontendRole,
      effectiveManifest: frontendRole,
      executionProfile: {
        role_id: "frontend-developer",
        name: "Frontend Developer",
        role: "Senior Frontend Developer",
        goal: "Ship a great dashboard UX",
        backstory: "A patient frontend specialist",
        system_prompt: "Focus on clarity and accessibility.",
        allowed_tools: ["Read", "Edit"],
        max_budget_usd: 6,
        max_turns: 24,
        permission_mode: "default",
      },
      inheritance: {
        parentRoleId: "coding-agent",
      },
      validationIssues: [
        {
          field: "overrides",
          message: "Use explicit override paths only.",
        },
      ],
    });
    const onSandboxRole = jest.fn().mockResolvedValue({
      normalizedManifest: frontendRole,
      effectiveManifest: frontendRole,
      executionProfile: {
        role_id: "frontend-developer",
        name: "Frontend Developer",
        role: "Senior Frontend Developer",
        goal: "Ship a great dashboard UX",
        backstory: "A patient frontend specialist",
        system_prompt: "Focus on clarity and accessibility.",
        allowed_tools: ["Read", "Edit"],
        max_budget_usd: 6,
        max_turns: 24,
        permission_mode: "default",
      },
      readinessDiagnostics: [
        {
          code: "missing_credentials",
          message: "Missing runtime credentials",
          blocking: true,
        },
      ],
    });

    render(
      <RoleWorkspace
        roles={[frontendRole]}
        skillCatalog={skillCatalog}
        loading={false}
        error={null}
        onCreateRole={jest.fn().mockResolvedValue(undefined)}
        onUpdateRole={jest.fn().mockResolvedValue(undefined)}
        onDeleteRole={jest.fn().mockResolvedValue(undefined)}
        onPreviewRole={onPreviewRole}
        onSandboxRole={onSandboxRole}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Preview Role Draft" }));
    setFieldValue("Sandbox Input", "Check advanced role context.");
    await user.click(screen.getByRole("button", { name: "Run Sandbox Probe" }));

    expect(await screen.findByText("Advanced Authoring")).toBeInTheDocument();
    expect(screen.getByText("Inherited from coding-agent")).toBeInTheDocument();
    expect(screen.getAllByText("knowledge.memory").length).toBeGreaterThan(0);
    expect(screen.getAllByText("overrides").length).toBeGreaterThan(0);
    expect(screen.getByText("Missing runtime credentials")).toBeInTheDocument();
    expect(screen.getByText("overrides: Use explicit override paths only.")).toBeInTheDocument();
  });

  it("keeps role library and review surfaces reachable on medium viewports", async () => {
    mockMatchMedia(false); // < 1280px — panels auto-collapsed
    setViewport(960);
    const user = userEvent.setup();

    render(
      <RoleWorkspace
        roles={[frontendRole]}
        skillCatalog={skillCatalog}
        loading={false}
        error={null}
        onCreateRole={jest.fn().mockResolvedValue(undefined)}
        onUpdateRole={jest.fn().mockResolvedValue(undefined)}
        onDeleteRole={jest.fn().mockResolvedValue(undefined)}
        onPreviewRole={jest.fn().mockResolvedValue(undefined)}
        onSandboxRole={jest.fn().mockResolvedValue(undefined)}
      />,
    );

    // Panel toggle buttons are always present in the toolbar
    expect(screen.getByRole("button", { name: "Toggle role catalog" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Toggle context rail" })).toBeInTheDocument();

    // Expand catalog via toggle
    await user.click(screen.getByRole("button", { name: "Toggle role catalog" }));
    expect(screen.getByText("Role Library")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Edit Frontend Developer" })).toBeInTheDocument();

    // Expand context rail via toggle
    await user.click(screen.getByRole("button", { name: "Toggle context rail" }));
    expect(screen.getByText("Execution Summary")).toBeInTheDocument();
    expect(screen.getByText("YAML Preview")).toBeInTheDocument();
    expect(screen.getByText("Preview And Sandbox")).toBeInTheDocument();
  });

  it("shows catalog-backed skill guidance and unresolved manual references", async () => {
    const user = userEvent.setup();

    render(
      <RoleWorkspace
        roles={[frontendRole]}
        skillCatalog={skillCatalog}
        loading={false}
        error={null}
        onCreateRole={jest.fn().mockResolvedValue(undefined)}
        onUpdateRole={jest.fn().mockResolvedValue(undefined)}
        onDeleteRole={jest.fn().mockResolvedValue(undefined)}
        onPreviewRole={jest.fn().mockResolvedValue(undefined)}
        onSandboxRole={jest.fn().mockResolvedValue(undefined)}
      />,
    );

    await user.click(screen.getByRole("button", { name: "New Role" }));
    await user.selectOptions(screen.getByLabelText("Start from template"), "frontend-developer");
    await user.click(screen.getByRole("button", { name: "Capabilities" }));

    expect(screen.getByText("Available repo-local skills")).toBeInTheDocument();
    expect(screen.getAllByText("React").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Testing").length).toBeGreaterThan(0);

    await user.click(screen.getByRole("button", { name: "Add Skill" }));
    const skillInputs = screen.getAllByLabelText("Skill Path");
    fireEvent.change(skillInputs[2]!, {
      target: { value: "skills/custom-flow" },
    });

    expect(screen.getByText(/Unresolved manual reference/)).toBeInTheDocument();
    expect(screen.getAllByText(/Template-derived/).length).toBeGreaterThan(0);
  });

  it("renders the Permissions section inside the Governance tab", async () => {
    const user = userEvent.setup();

    render(
      <RoleWorkspace
        roles={[frontendRole]}
        skillCatalog={skillCatalog}
        loading={false}
        error={null}
        onCreateRole={jest.fn().mockResolvedValue(undefined)}
        onUpdateRole={jest.fn().mockResolvedValue(undefined)}
        onDeleteRole={jest.fn().mockResolvedValue(undefined)}
        onPreviewRole={jest.fn().mockResolvedValue(undefined)}
        onSandboxRole={jest.fn().mockResolvedValue(undefined)}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Edit Frontend Developer" }));
    await user.click(screen.getByRole("button", { name: "Governance" }));

    expect(screen.getByText("Permissions")).toBeInTheDocument();
  });

  it("renders the Resource Limits section inside the Governance tab", async () => {
    const user = userEvent.setup();

    render(
      <RoleWorkspace
        roles={[frontendRole]}
        skillCatalog={skillCatalog}
        loading={false}
        error={null}
        onCreateRole={jest.fn().mockResolvedValue(undefined)}
        onUpdateRole={jest.fn().mockResolvedValue(undefined)}
        onDeleteRole={jest.fn().mockResolvedValue(undefined)}
        onPreviewRole={jest.fn().mockResolvedValue(undefined)}
        onSandboxRole={jest.fn().mockResolvedValue(undefined)}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Edit Frontend Developer" }));
    await user.click(screen.getByRole("button", { name: "Governance" }));

    expect(screen.getByText("Resource Limits")).toBeInTheDocument();
  });
});
