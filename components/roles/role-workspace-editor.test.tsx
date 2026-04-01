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

import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  buildRoleDraft,
  type FieldProvenanceMap,
  type RoleDraftValidationBySection,
  type RoleSkillResolution,
} from "@/lib/roles/role-management";
import type { RoleManifest, RoleSkillCatalogEntry } from "@/lib/stores/role-store";
import { RoleWorkspaceEditor } from "./role-workspace-editor";
import type { PluginRecord } from "@/lib/stores/plugin-store";

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
    skills: [
      { path: "skills/react", autoLoad: true },
      { path: "skills/testing", autoLoad: false },
    ],
    languages: ["TypeScript"],
    frameworks: ["Next.js"],
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
    shortDescription: "Guide React work in the current repo.",
    requires: ["skills/typescript"],
    tools: ["code_editor", "browser_preview"],
    availableParts: ["agents", "references"],
    source: "repo-local",
    sourceRoot: "skills",
  },
  {
    path: "skills/testing",
    label: "Testing",
    description: "Verify product behavior.",
    availableParts: ["agents"],
    source: "repo-local",
    sourceRoot: "skills",
  },
];

const emptyValidation: RoleDraftValidationBySection = {
  setup: [],
  identity: [],
  capabilities: [],
  knowledge: [],
  governance: [],
  review: [],
};

const provenanceMap: FieldProvenanceMap = {
  customSettings: [{ key: "approval_mode", provenance: "template" }],
  mcpServers: [{ key: "design-mcp", provenance: "inherited" }],
  sharedKnowledge: [],
  privateKnowledge: [],
  triggers: [{ key: "pr_created:auto_review", provenance: "explicit" }],
  collaboration: [{ key: "canDelegateTo", provenance: "explicit" }],
};

const draftSkillResolution: RoleSkillResolution[] = [
  {
    path: "skills/react",
    autoLoad: true,
    label: "React",
    description: "Build React interfaces.",
    shortDescription: "Guide React work in the current repo.",
    requires: ["skills/typescript"],
    tools: ["code_editor", "browser_preview"],
    availableParts: ["agents", "references"],
    source: "repo-local",
    sourceRoot: "skills",
    status: "resolved",
    compatibilityStatus: "blocking",
    missingTools: ["browser_preview"],
    provenance: "explicit",
  },
  {
    path: "skills/testing",
    autoLoad: false,
    label: "skills/testing",
    description: "",
    tools: ["code_editor", "terminal"],
    source: "manual",
    sourceRoot: "",
    status: "unresolved",
    compatibilityStatus: "warning",
    provenance: "template-derived",
  },
];

const availablePlugins: PluginRecord[] = [
  {
    apiVersion: "agentforge/v1",
    kind: "ToolPlugin",
    metadata: {
      id: "repo-search",
      name: "Repo Search",
      version: "1.0.0",
      description: "Search the workspace repository.",
      tags: ["search"],
    },
    spec: {
      runtime: "mcp",
      capabilities: ["search_code", "open_file"],
    },
    permissions: {},
    source: { type: "local" },
    lifecycle_state: "active",
    restart_count: 0,
  },
];

function renderEditor(
  overrides: Partial<React.ComponentProps<typeof RoleWorkspaceEditor>> = {},
) {
  const onSubmit = jest.fn((event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
  });

  const props: React.ComponentProps<typeof RoleWorkspaceEditor> = {
    mode: "edit",
    draft: buildRoleDraft(frontendRole),
    templateId: "frontend-developer",
    selectedRole: frontendRole,
    skillCatalog,
    skillCatalogLoading: false,
    draftSkillResolution,
    selectedTemplateName: "Frontend Developer",
    selectedParentName: "coding-agent",
    saving: false,
    activeSection: "setup",
    onSelectSection: jest.fn(),
    onSubmit,
    onSwitchToCreate: jest.fn(),
    updateDraft: jest.fn(),
    updateSkillRow: jest.fn(),
    updateMCPServerRow: jest.fn(),
    updateCustomSettingRow: jest.fn(),
    updateKnowledgeRow: jest.fn(),
    updatePrivateKnowledgeRow: jest.fn(),
    updateTriggerRow: jest.fn(),
    onAddSkillRow: jest.fn(),
    onAddMCPServerRow: jest.fn(),
    onAddCustomSettingRow: jest.fn(),
    onAddKnowledgeRow: jest.fn(),
    onAddPrivateKnowledgeRow: jest.fn(),
    onAddTriggerRow: jest.fn(),
    availableRoles: [frontendRole],
    availablePlugins,
    onTemplateChange: jest.fn(),
    validationBySection: emptyValidation,
    provenanceMap,
    ...overrides,
  };

  return {
    ...render(<RoleWorkspaceEditor {...props} />),
    props,
    onSubmit,
  };
}

describe("RoleWorkspaceEditor", () => {
  it("renders setup mode controls and footer actions in edit mode", async () => {
    const user = userEvent.setup();
    const { props, onSubmit } = renderEditor();

    expect(screen.getByText("Role Workspace")).toBeInTheDocument();
    expect(screen.getByText("Editing existing role")).toBeInTheDocument();
    expect(screen.getByText("Template source: Frontend Developer")).toBeInTheDocument();
    expect(screen.getByText("Inherited from coding-agent")).toBeInTheDocument();
    expect(screen.getByDisplayValue("frontend-developer")).toBeDisabled();

    fireEvent.change(screen.getByLabelText("Name"), {
      target: { value: "Frontend Captain" },
    });
    await user.click(screen.getByRole("combobox", { name: "Inherits from" }));
    await user.click(screen.getByRole("option", { name: "Frontend Developer" }));
    fireEvent.change(screen.getByLabelText("Version"), {
      target: { value: "2.0.0" },
    });

    expect(props.updateDraft).toHaveBeenCalledWith("name", "Frontend Captain");
    expect(props.updateDraft).toHaveBeenCalledWith("extendsValue", "frontend-developer");
    expect(props.updateDraft).toHaveBeenCalledWith("version", "2.0.0");

    await user.click(screen.getByRole("button", { name: "Save Role" }));
    await user.click(screen.getByRole("button", { name: "Switch to Create" }));

    expect(onSubmit).toHaveBeenCalled();
    expect(props.onSwitchToCreate).toHaveBeenCalled();
  });

  it("renders capabilities tools, provenance, skill resolution, and add callbacks", async () => {
    const user = userEvent.setup();
    const { props } = renderEditor({
      activeSection: "capabilities",
      validationBySection: {
        ...emptyValidation,
        capabilities: ["Skill paths must be unique."],
      },
    });

    expect(screen.getByText("Advanced Capability Settings")).toBeInTheDocument();
    expect(screen.getByText("Available repo-local skills")).toBeInTheDocument();
    expect(screen.getAllByText("React").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Testing").length).toBeGreaterThan(0);
    expect(screen.getByText("Guide React work in the current repo.")).toBeInTheDocument();
    expect(screen.getAllByText("Agents").length).toBeGreaterThan(0);
    expect(screen.getAllByText("References").length).toBeGreaterThan(0);
    expect(screen.getByText("template")).toBeInTheDocument();
    expect(screen.getByText("inherited")).toBeInTheDocument();
    expect(screen.getByText(/React from skills/)).toBeInTheDocument();
    expect(screen.getByText("Parts:")).toBeInTheDocument();
    expect(screen.getByText("Dependencies: skills/typescript")).toBeInTheDocument();
    expect(screen.getByText("Declared tools: code_editor, browser_preview")).toBeInTheDocument();
    expect(screen.getByText("Blocking compatibility issue · Missing: browser_preview")).toBeInTheDocument();
    expect(screen.getByText(/Unresolved manual reference/)).toBeInTheDocument();
    expect(screen.getByText("Warning-only compatibility issue")).toBeInTheDocument();
    expect(screen.getByText(/Explicit/)).toBeInTheDocument();
    expect(screen.getByText(/Template-derived/)).toBeInTheDocument();
    expect(screen.getByText("Skill paths must be unique.")).toBeInTheDocument();
    expect(screen.getByText("Available plugins")).toBeInTheDocument();
    expect(screen.getAllByText("Repo Search").length).toBeGreaterThan(0);
    expect(screen.getByText("Functions: search_code, open_file")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Add Custom Setting" }));
    await user.click(screen.getByRole("button", { name: "Add MCP Server" }));
    await user.click(screen.getByRole("button", { name: "Add Skill" }));
    await user.click(screen.getByRole("button", { name: "Use plugin Repo Search" }));

    fireEvent.change(screen.getByLabelText("Custom Setting Key"), {
      target: { value: "review_depth" },
    });
    fireEvent.change(screen.getByLabelText("MCP Server Name"), {
      target: { value: "figma-sync" },
    });
    fireEvent.change(screen.getAllByLabelText("Skill Path")[0], {
      target: { value: "skills/testing" },
    });

    expect(props.onAddCustomSettingRow).toHaveBeenCalled();
    expect(props.onAddMCPServerRow).toHaveBeenCalled();
    expect(props.onAddSkillRow).toHaveBeenCalled();
    expect(props.updateDraft).toHaveBeenCalledWith("externalTools", "figma, repo-search");
    expect(props.updateDraft).toHaveBeenCalledWith("pluginBindingRows", [
      { pluginId: "repo-search", functionsInput: "search_code, open_file" },
    ]);
    expect(props.updateCustomSettingRow).toHaveBeenCalledWith(0, "key", "review_depth");
    expect(props.updateMCPServerRow).toHaveBeenCalledWith(0, "name", "figma-sync");
    expect(props.updateSkillRow).toHaveBeenCalledWith(0, "path", "skills/testing");

    cleanup();

    const updatedDraft = {
      ...buildRoleDraft(frontendRole),
      pluginBindingRows: [{ pluginId: "repo-search", functionsInput: "search_code, open_file" }],
    };
    const rerendered = renderEditor({
      activeSection: "capabilities",
      draft: updatedDraft,
    });
    fireEvent.change(rerendered.getByLabelText("Plugin Functions"), {
      target: { value: "search_code" },
    });
    expect(rerendered.props.updateDraft).toHaveBeenCalledWith("pluginBindingRows", [
      { pluginId: "repo-search", functionsInput: "search_code" },
    ]);
  });

  it("renders governance and review sections with trigger and override editing", async () => {
    const user = userEvent.setup();
    const { props, rerender } = renderEditor({
      activeSection: "governance",
      validationBySection: {
        ...emptyValidation,
        governance: ["Trigger rows must be unique."],
      },
    });

    expect(screen.getByText("Permissions")).toBeInTheDocument();
    expect(screen.getByText("Resource Limits")).toBeInTheDocument();
    expect(screen.getByText("Trigger rows must be unique.")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Add Trigger" }));
    fireEvent.change(screen.getByLabelText("Permission Mode"), {
      target: { value: "strict" },
    });
    fireEvent.change(screen.getByLabelText("Trigger Event"), {
      target: { value: "task_blocked" },
    });

    expect(props.onAddTriggerRow).toHaveBeenCalled();
    expect(props.updateDraft).toHaveBeenCalledWith("permissionMode", "strict");
    expect(props.updateTriggerRow).toHaveBeenCalledWith(0, "event", "task_blocked");

    rerender(
      <RoleWorkspaceEditor
        {...props}
        activeSection="review"
        validationBySection={{
          ...emptyValidation,
          review: ["Overrides input must be valid JSON."],
        }}
      />,
    );

    expect(screen.getByLabelText("Role Overrides")).toBeInTheDocument();
    expect(screen.getByText("Overrides input must be valid JSON.")).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText("Role Overrides"), {
      target: { value: '{ "identity.role": "Frontend Captain" }' },
    });
    expect(props.updateDraft).toHaveBeenCalledWith(
      "overridesInput",
      '{ "identity.role": "Frontend Captain" }',
    );
  });
});
