import rolesMessages from "../../messages/en/roles.json";

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
  beforeEach(() => {
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
    await user.clear(screen.getByLabelText("Role ID"));
    await user.type(screen.getByLabelText("Role ID"), "frontend-developer-copy");
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
    expect(screen.getByText("Current flow")).toBeInTheDocument();
    expect(screen.getByText("Editing existing role")).toBeInTheDocument();
    expect(screen.getByText("Inherited from coding-agent")).toBeInTheDocument();
    expect(screen.getByLabelText("Role ID")).toHaveValue("frontend-developer");
    expect(screen.getByDisplayValue("1.2.0")).toBeInTheDocument();
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
    const skillInputs = screen.getAllByLabelText("Skill Path");
    await user.clear(skillInputs[1]!);
    await user.type(skillInputs[1]!, "skills/react");
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

    await user.type(screen.getByLabelText("Sandbox Input"), "Summarize this role.");
    await user.click(screen.getByRole("button", { name: "Run Sandbox Probe" }));
    expect(onSandboxRole).toHaveBeenCalledWith(
      expect.objectContaining({
        input: "Summarize this role.",
      }),
    );

    expect(await screen.findByText("A calm frontend specialist for dashboard polish.")).toBeInTheDocument();
    expect(screen.getByText("claude_code / anthropic / claude-sonnet-4-5")).toBeInTheDocument();
    expect(screen.getByText("Loaded skills")).toBeInTheDocument();
    expect(screen.getByText("React (skills/react)")).toBeInTheDocument();
    expect(screen.getByText("On-demand skills")).toBeInTheDocument();
    expect(screen.getByText("Testing (skills/testing)")).toBeInTheDocument();
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
    await user.clear(screen.getByLabelText("Name"));
    await user.type(screen.getByLabelText("Name"), "Frontend Captain");
    await user.click(screen.getByRole("button", { name: "Capabilities" }));
    await user.type(screen.getByLabelText("Packages"), ", ui-kit");
    await user.type(screen.getByLabelText("External Tools"), ", linear");
    await user.click(screen.getByRole("button", { name: "Add Custom Setting" }));
    const customSettingKeys = screen.getAllByLabelText("Custom Setting Key");
    const customSettingValues = screen.getAllByLabelText("Custom Setting Value");
    await user.type(customSettingKeys[1]!, "review_depth");
    await user.type(customSettingValues[1]!, "strict");
    await user.click(screen.getByRole("button", { name: "Add MCP Server" }));
    const mcpServerNames = screen.getAllByLabelText("MCP Server Name");
    const mcpServerUrls = screen.getAllByLabelText("MCP Server URL");
    await user.type(mcpServerNames[1]!, "figma-sync");
    await user.type(mcpServerUrls[1]!, "https://mcp.example.com");
    await user.click(screen.getByRole("button", { name: "Identity" }));
    await user.clear(screen.getByLabelText("Persona"));
    await user.type(screen.getByLabelText("Persona"), "precise");
    await user.click(screen.getByRole("button", { name: "Knowledge" }));
    await user.click(screen.getByRole("button", { name: "Add Shared Knowledge" }));
    const knowledgeIds = screen.getAllByLabelText("Shared Knowledge ID");
    const knowledgeTypes = screen.getAllByLabelText("Shared Knowledge Type");
    await user.type(knowledgeIds[1]!, "team-playbook");
    await user.type(knowledgeTypes[1]!, "doc");
    await user.click(screen.getByRole("button", { name: "Add Private Knowledge" }));
    const privateKnowledgeIds = screen.getAllByLabelText("Private Knowledge ID");
    const privateKnowledgeTypes = screen.getAllByLabelText("Private Knowledge Type");
    await user.type(privateKnowledgeIds[0]!, "operator-notes");
    await user.type(privateKnowledgeTypes[0]!, "doc");
    await user.clear(screen.getByLabelText("Short-term Memory Max Tokens"));
    await user.type(screen.getByLabelText("Short-term Memory Max Tokens"), "48000");
    await user.click(screen.getByLabelText(/Enable procedural memory/i));
    await user.click(screen.getByLabelText(/Learn procedural memory from feedback/i));
    await user.click(screen.getByRole("button", { name: "Governance" }));
    await user.click(screen.getByRole("button", { name: "Add Trigger" }));
    const triggerEvents = screen.getAllByLabelText("Trigger Event");
    const triggerActions = screen.getAllByLabelText("Trigger Action");
    const triggerConditions = screen.getAllByLabelText("Trigger Condition");
    await user.type(triggerEvents[1]!, "task_blocked");
    await user.type(triggerActions[1]!, "escalate");
    await user.type(triggerConditions[1]!, "severity === 'high'");
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
    await user.type(screen.getByLabelText("Sandbox Input"), "Check advanced role context.");
    await user.click(screen.getByRole("button", { name: "Run Sandbox Probe" }));

    expect(await screen.findByText("Advanced Authoring")).toBeInTheDocument();
    expect(screen.getByText("Inherited from coding-agent")).toBeInTheDocument();
    expect(screen.getAllByText("knowledge.memory").length).toBeGreaterThan(0);
    expect(screen.getAllByText("overrides").length).toBeGreaterThan(0);
    expect(screen.getByText("Missing runtime credentials")).toBeInTheDocument();
    expect(screen.getByText("overrides: Use explicit override paths only.")).toBeInTheDocument();
  });

  it("keeps role library and review surfaces reachable on medium viewports", async () => {
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

    expect(screen.getByRole("button", { name: "Show Role Library" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Show Review Panel" })).toBeInTheDocument();
    expect(screen.queryByText("Role Library")).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Show Role Library" }));
    expect(screen.getByText("Role Library")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Edit Frontend Developer" })).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Show Review Panel" }));
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

    expect(screen.getByText("Available repo-local skills")).toBeInTheDocument();
    expect(screen.getAllByText("React").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Testing").length).toBeGreaterThan(0);

    await user.click(screen.getByRole("button", { name: "Add Skill" }));
    const skillInputs = screen.getAllByLabelText("Skill Path");
    await user.type(skillInputs[2]!, "skills/custom-flow");

    expect(screen.getByText(/Unresolved manual reference/)).toBeInTheDocument();
    expect(screen.getAllByText(/Template-derived/).length).toBeGreaterThan(0);
  });
});
