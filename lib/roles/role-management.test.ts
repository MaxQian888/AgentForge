import type { RoleManifest } from "@/lib/stores/role-store";
import {
  buildRoleDraft,
  buildRoleExecutionSummary,
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
});
