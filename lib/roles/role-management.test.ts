import type { RoleManifest } from "@/lib/stores/role-store";
import {
  buildRoleDraft,
  buildRoleExecutionSummary,
  renderRoleManifestYaml,
  serializeRoleDraft,
} from "./role-management";

const role: RoleManifest = {
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

describe("role management helpers", () => {
  it("builds sensible defaults for a new role draft", () => {
    expect(buildRoleDraft()).toEqual({
      roleId: "",
      name: "",
      version: "1.0.0",
      icon: "",
      description: "",
      tagsInput: "",
      extendsValue: "",
      identityRole: "",
      goal: "",
      backstory: "",
      systemPrompt: "",
      persona: "",
      goalsInput: "",
      constraintsInput: "",
      personality: "",
      language: "",
      responseTone: "",
      responseVerbosity: "",
      responseFormatPreference: "",
      packages: "",
      allowedTools: "",
      externalTools: "",
      skillRows: [],
      languages: "",
      frameworks: "",
      maxTurns: "",
      maxBudgetUsd: "",
      repositories: "",
      documents: "",
      patterns: "",
      sharedKnowledgeRows: [],
      securityProfile: "",
      permissionMode: "default",
      allowedPaths: "",
      deniedPaths: "",
      outputFilters: "",
      requireReview: false,
      collaborationCanDelegateTo: "",
      collaborationAcceptsDelegationFrom: "",
      communicationPreferredChannel: "",
      communicationReportFormat: "",
      communicationEscalationPolicy: "",
      triggerRows: [],
    });
  });

  it("builds a reusable draft from a role manifest", () => {
    expect(buildRoleDraft(role)).toMatchObject({
      roleId: "frontend-developer",
      version: "1.2.0",
      tagsInput: "frontend, ui",
      icon: "",
      allowedTools: "Read, Edit",
      packages: "design-system",
      externalTools: "figma",
      sharedKnowledgeRows: [{ id: "design-guidelines", type: "vector", access: "read", description: "", sourcesInput: "" }],
      skillRows: [
        { path: "skills/react", autoLoad: true },
        { path: "skills/testing", autoLoad: false },
      ],
      permissionMode: "default",
      securityProfile: "standard",
      outputFilters: "no_pii",
      collaborationCanDelegateTo: "frontend-developer",
      communicationPreferredChannel: "structured",
      triggerRows: [{ event: "pr_created", action: "auto_review", condition: "labels.includes('ui')" }],
      extendsValue: "coding-agent",
    });
  });

  it("serializes a fresh draft without a base role using repo-safe defaults", () => {
    const payload = serializeRoleDraft({
      ...buildRoleDraft(),
      roleId: "design-manager",
      name: "Design Manager",
      description: "Keeps UI delivery aligned",
      tagsInput: "design, review",
      identityRole: "Design Manager",
      systemPrompt: "Coordinate polished UI delivery.",
      allowedTools: " Read, Edit ",
      externalTools: " figma, slack ",
      skillRows: [{ path: " skills/design ", autoLoad: true }],
      languages: " TypeScript ",
      frameworks: " Next.js ",
      repositories: " app, components ",
      documents: " docs/PRD.md ",
      patterns: " review-workflows ",
      sharedKnowledgeRows: [
        { id: "", type: "", access: "", description: "", sourcesInput: "" },
        {
          id: "design-guidelines",
          type: "vector",
          access: "read",
          description: "",
          sourcesInput: " docs/PRD.md, docs/part/PLUGIN_SYSTEM_DESIGN.md ",
        },
      ],
      triggerRows: [
        { event: "", action: "", condition: "" },
        { event: "pr_created", action: "notify", condition: "labels.includes('design')" },
      ],
    });

    expect(payload).toEqual({
      metadata: {
        id: "design-manager",
        name: "Design Manager",
        version: "1.0.0",
        description: "Keeps UI delivery aligned",
        author: "AgentForge",
        tags: ["design", "review"],
        icon: undefined,
      },
      identity: {
        role: "Design Manager",
        goal: "",
        backstory: "",
        systemPrompt: "Coordinate polished UI delivery.",
        persona: "",
        goals: [],
        constraints: [],
        personality: undefined,
        language: undefined,
        responseStyle: {
          tone: undefined,
          verbosity: undefined,
          formatPreference: undefined,
        },
      },
      capabilities: {
        packages: [],
        allowedTools: ["Read", "Edit"],
        toolConfig: {
          builtIn: ["Read", "Edit"],
          external: ["figma", "slack"],
          mcpServers: [],
        },
        skills: [{ path: "skills/design", autoLoad: true }],
        languages: ["TypeScript"],
        frameworks: ["Next.js"],
        maxTurns: undefined,
        maxBudgetUsd: undefined,
        maxConcurrency: undefined,
        customSettings: undefined,
      },
      knowledge: {
        repositories: ["app", "components"],
        documents: ["docs/PRD.md"],
        patterns: ["review-workflows"],
        shared: [
          {
            id: "design-guidelines",
            type: "vector",
            access: "read",
            description: undefined,
            sources: ["docs/PRD.md", "docs/part/PLUGIN_SYSTEM_DESIGN.md"],
          },
        ],
        private: [],
        memory: undefined,
      },
      security: {
        profile: undefined,
        permissionMode: "default",
        allowedPaths: [],
        deniedPaths: [],
        maxBudgetUsd: 0,
        requireReview: false,
        permissions: undefined,
        outputFilters: [],
        resourceLimits: undefined,
      },
      collaboration: {
        canDelegateTo: [],
        acceptsDelegationFrom: [],
        communication: {
          preferredChannel: undefined,
          reportFormat: undefined,
          escalationPolicy: undefined,
        },
      },
      triggers: [
        {
          event: "pr_created",
          action: "notify",
          condition: "labels.includes('design')",
        },
      ],
      overrides: undefined,
      extends: undefined,
      validationErrors: [
        "Shared knowledge rows must include at least an id, type, or access value.",
        "Trigger rows must include both event and action.",
      ],
    });
  });

  it("serializes a draft back into a normalized role payload", () => {
    const payload = serializeRoleDraft(
      {
        ...buildRoleDraft(role),
        roleId: "frontend-developer-custom",
        name: "Frontend Developer Custom",
        packages: "design-system, review",
        externalTools: "figma, playwright",
        maxBudgetUsd: "7.5",
        requireReview: false,
        sharedKnowledgeRows: [
          { id: "design-guidelines", type: "vector", access: "read", description: "Shared", sourcesInput: "" },
        ],
        outputFilters: "no_pii, no_credentials",
        skillRows: [
          { path: "skills/react", autoLoad: false },
          { path: "skills/accessibility", autoLoad: true },
        ],
      },
      role,
    );

    expect(payload).toMatchObject({
      metadata: expect.objectContaining({
        id: "frontend-developer-custom",
        name: "Frontend Developer Custom",
      }),
      capabilities: expect.objectContaining({
        packages: ["design-system", "review"],
        allowedTools: ["Read", "Edit"],
        toolConfig: expect.objectContaining({
          external: ["figma", "playwright"],
        }),
        skills: [
          { path: "skills/react", autoLoad: false },
          { path: "skills/accessibility", autoLoad: true },
        ],
        maxBudgetUsd: 7.5,
      }),
      security: expect.objectContaining({
        requireReview: false,
        outputFilters: ["no_pii", "no_credentials"],
      }),
      knowledge: expect.objectContaining({
        shared: [{ id: "design-guidelines", type: "vector", access: "read", description: "Shared", sources: [] }],
      }),
    });
  });

  it("reports validation issues for invalid shared knowledge and trigger rows", () => {
    const payload = serializeRoleDraft(
      {
        ...buildRoleDraft(role),
        sharedKnowledgeRows: [
          { id: " ", type: " ", access: " ", description: "Needs context", sourcesInput: "" },
        ],
        triggerRows: [
          { event: "", action: "notify", condition: "" },
          { event: "pr_created", action: "notify", condition: "labels.includes('ui')" },
          { event: "pr_created", action: "notify", condition: "labels.includes('ui')" },
        ],
      },
      role,
    );

    expect(payload.validationErrors).toEqual(
      expect.arrayContaining([
        "Shared knowledge rows must include at least an id, type, or access value.",
        "Trigger rows must include both event and action.",
        "Trigger rows must be unique.",
      ]),
    );
  });

  it("builds an execution summary for role drafts", () => {
    const summary = buildRoleExecutionSummary(buildRoleDraft(role));

    expect(summary).toEqual({
      promptIntent: "Ship a great dashboard UX",
      toolsLabel: "Read, Edit",
      budgetLabel: "$6.00",
      turnsLabel: "24 turns",
      permissionMode: "default",
      skillsLabel: "1 auto-load / 1 on-demand",
      keySkillPaths: ["skills/react", "skills/testing"],
      safetyCues: [
        "Review required",
        "2 allowed paths",
        "1 denied path",
        "1 output filters",
      ],
    });
  });

  it("builds fallback execution labels when optional execution hints are omitted", () => {
    const summary = buildRoleExecutionSummary({
      ...buildRoleDraft(),
      identityRole: "",
      systemPrompt: "Review the role before execution.",
      allowedTools: " ",
      permissionMode: "",
      deniedPaths: "secrets/, private/",
      skillRows: [{ path: " ", autoLoad: true }],
    });

    expect(summary).toEqual({
      promptIntent: "Review the role before execution.",
      toolsLabel: "Inherits defaults",
      budgetLabel: "Unbounded",
      turnsLabel: "Default turns",
      permissionMode: "default",
      skillsLabel: "No skills configured",
      keySkillPaths: [],
      safetyCues: ["2 denied paths"],
    });
  });

  it("flags invalid skill rows before submit", () => {
    const payload = serializeRoleDraft(
      {
        ...buildRoleDraft(role),
        skillRows: [
          { path: "skills/react", autoLoad: true },
          { path: "skills/react", autoLoad: false },
          { path: " ", autoLoad: true },
        ],
      },
      role,
    );

    expect(payload.validationErrors).toEqual(
      expect.arrayContaining([
        expect.stringContaining("unique"),
        expect.stringContaining("blank"),
      ]),
    );
  });

  it("renders yaml for nested objects, arrays, and empty containers", () => {
    expect(renderRoleManifestYaml([] as unknown as Partial<RoleManifest>)).toBe("[]");
    expect(renderRoleManifestYaml({})).toBe("{}");
    expect(renderRoleManifestYaml("role: reviewer" as unknown as Partial<RoleManifest>)).toBe(
      "role: reviewer",
    );

    expect(
      renderRoleManifestYaml({
        metadata: {
          id: "frontend-developer",
          name: "Frontend Developer",
          version: "1.0.0",
          description: "Build polished UI",
          author: "AgentForge",
          tags: ["frontend"],
        },
        knowledge: {
          repositories: ["app"],
          documents: [],
          patterns: ["responsive-layouts"],
          shared: [
            {
              id: "design-guidelines",
              type: "vector",
              access: "read",
              sources: ["docs/PRD.md"],
            },
          ],
        },
        triggers: [{ event: "pr_created", action: "auto_review", condition: "" }],
      }),
    ).toBe(
      [
        "metadata:",
        "  id: frontend-developer",
        "  name: Frontend Developer",
        "  version: 1.0.0",
        "  description: Build polished UI",
        "  author: AgentForge",
        "  tags:",
        "    - frontend",
        "knowledge:",
        "  repositories:",
        "    - app",
        "  patterns:",
        "    - responsive-layouts",
        "  shared:",
        "    - id: design-guidelines",
        "      type: vector",
        "      access: read",
        "      sources:",
        "        - docs/PRD.md",
        "triggers:",
        "  - event: pr_created",
        "    action: auto_review",
      ].join("\n"),
    );
  });
});
