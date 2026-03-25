import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { RoleWorkspace } from "./role-workspace";
import type { RoleManifest } from "@/lib/stores/role-store";

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
    shared: [{ id: "design-guidelines", type: "vector", access: "read" }],
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
};

describe("RoleWorkspace", () => {
  it("supports template-based creation with a live execution summary rail", async () => {
    const user = userEvent.setup();
    const onCreateRole = jest.fn().mockResolvedValue(undefined);

    render(
      <RoleWorkspace
        roles={[frontendRole]}
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

    expect(screen.getByLabelText("Role ID")).toHaveValue("frontend-developer");
    expect(screen.getByDisplayValue("1.2.0")).toBeInTheDocument();
    expect(screen.getByText("Identity")).toBeInTheDocument();
    expect(screen.getByText("Capabilities")).toBeInTheDocument();
    expect(screen.getAllByText("Skills").length).toBeGreaterThan(0);
    expect(screen.getByDisplayValue("skills/react")).toBeInTheDocument();
    expect(screen.getByDisplayValue("skills/testing")).toBeInTheDocument();
    expect(screen.getByText("Knowledge")).toBeInTheDocument();
    expect(screen.getByText("Security")).toBeInTheDocument();
    expect(screen.getByText("Advanced Identity")).toBeInTheDocument();
    expect(screen.getByText("Collaboration")).toBeInTheDocument();
    expect(screen.getByText("Triggers")).toBeInTheDocument();
  });

  it("blocks save when duplicate skill paths are present", async () => {
    const user = userEvent.setup();
    const onCreateRole = jest.fn().mockResolvedValue(undefined);

    render(
      <RoleWorkspace
        roles={[frontendRole]}
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
  });

  it("updates an existing role with advanced workspace sections and can switch back to create mode", async () => {
    const user = userEvent.setup();
    const onUpdateRole = jest.fn().mockResolvedValue(undefined);

    render(
      <RoleWorkspace
        roles={[frontendRole]}
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
    await user.type(screen.getByLabelText("Packages"), ", ui-kit");
    await user.type(screen.getByLabelText("External Tools"), ", linear");
    await user.clear(screen.getByLabelText("Persona"));
    await user.type(screen.getByLabelText("Persona"), "precise");
    await user.click(screen.getByRole("button", { name: "Add Shared Knowledge" }));
    const knowledgeIds = screen.getAllByLabelText("Shared Knowledge ID");
    const knowledgeTypes = screen.getAllByLabelText("Shared Knowledge Type");
    await user.type(knowledgeIds[1]!, "team-playbook");
    await user.type(knowledgeTypes[1]!, "doc");
    await user.click(screen.getByRole("button", { name: "Add Trigger" }));
    const triggerEvents = screen.getAllByLabelText("Trigger Event");
    const triggerActions = screen.getAllByLabelText("Trigger Action");
    const triggerConditions = screen.getAllByLabelText("Trigger Condition");
    await user.type(triggerEvents[1]!, "task_blocked");
    await user.type(triggerActions[1]!, "escalate");
    await user.type(triggerConditions[1]!, "severity === 'high'");
    await user.click(screen.getByRole("button", { name: "Save Role" }));

    expect(onUpdateRole).toHaveBeenCalledWith(
      "frontend-developer",
      expect.objectContaining({
        metadata: expect.objectContaining({ name: "Frontend Captain" }),
        capabilities: expect.objectContaining({
          packages: ["design-system", "ui-kit"],
          toolConfig: expect.objectContaining({
            external: ["figma", "linear"],
          }),
        }),
        identity: expect.objectContaining({ persona: "precise" }),
        knowledge: expect.objectContaining({
          shared: expect.arrayContaining([
            expect.objectContaining({ id: "design-guidelines" }),
            expect.objectContaining({ id: "team-playbook", type: "doc" }),
          ]),
        }),
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
});
