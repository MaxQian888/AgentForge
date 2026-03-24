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
  },
  capabilities: {
    allowedTools: ["Read", "Edit"],
    languages: ["TypeScript"],
    frameworks: ["Next.js"],
    maxTurns: 24,
    maxBudgetUsd: 6,
  },
  knowledge: {
    repositories: ["app", "components"],
    documents: ["docs/PRD.md"],
    patterns: ["responsive-layouts"],
  },
  security: {
    permissionMode: "default",
    allowedPaths: ["app/", "components/"],
    deniedPaths: ["secrets/"],
    maxBudgetUsd: 6,
    requireReview: true,
  },
  extends: "coding-agent",
};

describe("role management helpers", () => {
  it("builds a reusable draft from a role manifest", () => {
    expect(buildRoleDraft(role)).toMatchObject({
      roleId: "frontend-developer",
      version: "1.2.0",
      tagsInput: "frontend, ui",
      allowedTools: "Read, Edit",
      permissionMode: "default",
      extendsValue: "coding-agent",
    });
  });

  it("serializes a draft back into a normalized role payload", () => {
    const payload = serializeRoleDraft(
      {
        ...buildRoleDraft(role),
        roleId: "frontend-developer-custom",
        name: "Frontend Developer Custom",
        maxBudgetUsd: "7.5",
        requireReview: false,
      },
      role,
    );

    expect(payload).toMatchObject({
      metadata: expect.objectContaining({
        id: "frontend-developer-custom",
        name: "Frontend Developer Custom",
      }),
      capabilities: expect.objectContaining({
        allowedTools: ["Read", "Edit"],
        maxBudgetUsd: 7.5,
      }),
      security: expect.objectContaining({
        requireReview: false,
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
      safetyCues: [
        "Review required",
        "2 allowed paths",
        "1 denied path",
      ],
    });
  });
});
