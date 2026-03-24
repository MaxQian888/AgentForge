import type { RoleManifest } from "@/lib/stores/role-store";

export interface RoleDraft {
  roleId: string;
  name: string;
  version: string;
  description: string;
  tagsInput: string;
  extendsValue: string;
  identityRole: string;
  goal: string;
  backstory: string;
  systemPrompt: string;
  allowedTools: string;
  languages: string;
  frameworks: string;
  maxTurns: string;
  maxBudgetUsd: string;
  repositories: string;
  documents: string;
  patterns: string;
  permissionMode: string;
  allowedPaths: string;
  deniedPaths: string;
  requireReview: boolean;
}

export interface RoleExecutionSummary {
  promptIntent: string;
  toolsLabel: string;
  budgetLabel: string;
  turnsLabel: string;
  permissionMode: string;
  safetyCues: string[];
}

function parseList(input: string): string[] {
  return input
    .split(",")
    .map((item) => item.trim())
    .filter(Boolean);
}

function stringifyList(values?: string[]): string {
  return (values ?? []).join(", ");
}

export function buildRoleDraft(role?: RoleManifest): RoleDraft {
  return {
    roleId: role?.metadata.id ?? "",
    name: role?.metadata.name ?? "",
    version: role?.metadata.version ?? "1.0.0",
    description: role?.metadata.description ?? "",
    tagsInput: stringifyList(role?.metadata.tags),
    extendsValue: role?.extends ?? "",
    identityRole: role?.identity.role ?? "",
    goal: role?.identity.goal ?? "",
    backstory: role?.identity.backstory ?? "",
    systemPrompt: role?.identity.systemPrompt ?? "",
    allowedTools: stringifyList(role?.capabilities.allowedTools),
    languages: stringifyList(role?.capabilities.languages),
    frameworks: stringifyList(role?.capabilities.frameworks),
    maxTurns:
      role?.capabilities.maxTurns != null ? String(role.capabilities.maxTurns) : "",
    maxBudgetUsd:
      role?.capabilities.maxBudgetUsd != null ? String(role.capabilities.maxBudgetUsd) : "",
    repositories: stringifyList(role?.knowledge.repositories),
    documents: stringifyList(role?.knowledge.documents),
    patterns: stringifyList(role?.knowledge.patterns),
    permissionMode: role?.security.permissionMode ?? "default",
    allowedPaths: stringifyList(role?.security.allowedPaths),
    deniedPaths: stringifyList(role?.security.deniedPaths),
    requireReview: role?.security.requireReview ?? false,
  };
}

export function serializeRoleDraft(
  draft: RoleDraft,
  baseRole?: RoleManifest,
): Partial<RoleManifest> {
  return {
    metadata: {
      ...(baseRole?.metadata ?? {
        id: draft.roleId,
        version: draft.version || "1.0.0",
        author: "AgentForge",
      }),
      id: draft.roleId,
      name: draft.name,
      version: draft.version || baseRole?.metadata.version || "1.0.0",
      description: draft.description,
      author: baseRole?.metadata.author ?? "AgentForge",
      tags: parseList(draft.tagsInput),
    },
    identity: {
      ...(baseRole?.identity ?? {
        persona: "",
        goals: [],
        constraints: [],
      }),
      role: draft.identityRole,
      goal: draft.goal,
      backstory: draft.backstory,
      systemPrompt: draft.systemPrompt,
    },
    capabilities: {
      ...(baseRole?.capabilities ?? {
        languages: [],
        frameworks: [],
      }),
      allowedTools: parseList(draft.allowedTools),
      languages: parseList(draft.languages),
      frameworks: parseList(draft.frameworks),
      maxTurns: draft.maxTurns ? Number(draft.maxTurns) : undefined,
      maxBudgetUsd: draft.maxBudgetUsd ? Number(draft.maxBudgetUsd) : undefined,
    },
    knowledge: {
      ...(baseRole?.knowledge ?? {
        repositories: [],
        documents: [],
        patterns: [],
      }),
      repositories: parseList(draft.repositories),
      documents: parseList(draft.documents),
      patterns: parseList(draft.patterns),
    },
    security: {
      ...(baseRole?.security ?? {
        allowedPaths: [],
        deniedPaths: [],
        maxBudgetUsd: 0,
      }),
      permissionMode: draft.permissionMode || "default",
      allowedPaths: parseList(draft.allowedPaths),
      deniedPaths: parseList(draft.deniedPaths),
      maxBudgetUsd: draft.maxBudgetUsd ? Number(draft.maxBudgetUsd) : 0,
      requireReview: draft.requireReview,
    },
    extends: draft.extendsValue || undefined,
  };
}

export function buildRoleExecutionSummary(draft: RoleDraft): RoleExecutionSummary {
  const safetyCues: string[] = [];
  const allowedPaths = parseList(draft.allowedPaths);
  const deniedPaths = parseList(draft.deniedPaths);

  if (draft.requireReview) {
    safetyCues.push("Review required");
  }
  if (allowedPaths.length > 0) {
    safetyCues.push(`${allowedPaths.length} allowed paths`);
  }
  if (deniedPaths.length > 0) {
    safetyCues.push(`${deniedPaths.length} denied path${deniedPaths.length === 1 ? "" : "s"}`);
  }

  return {
    promptIntent: draft.goal || draft.identityRole || draft.systemPrompt,
    toolsLabel: parseList(draft.allowedTools).join(", ") || "Inherits defaults",
    budgetLabel: draft.maxBudgetUsd ? `$${Number(draft.maxBudgetUsd).toFixed(2)}` : "Unbounded",
    turnsLabel: draft.maxTurns ? `${draft.maxTurns} turns` : "Default turns",
    permissionMode: draft.permissionMode || "default",
    safetyCues,
  };
}
