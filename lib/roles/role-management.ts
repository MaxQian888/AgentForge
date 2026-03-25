import type { RoleManifest } from "@/lib/stores/role-store";

export interface RoleSkillDraft {
  path: string;
  autoLoad: boolean;
}

export interface SerializedRoleDraft extends Partial<RoleManifest> {
  validationErrors?: string[];
}

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
  skillRows: RoleSkillDraft[];
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
  skillsLabel: string;
  keySkillPaths: string[];
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
    skillRows: (role?.capabilities.skills ?? []).map((skill) => ({
      path: skill.path,
      autoLoad: skill.autoLoad,
    })),
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
): SerializedRoleDraft {
  const validationErrors = validateSkillRows(draft.skillRows);

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
      skills: draft.skillRows.map((skill) => ({
        path: skill.path.trim(),
        autoLoad: skill.autoLoad,
      })),
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
    validationErrors: validationErrors.length > 0 ? validationErrors : undefined,
  };
}

export function buildRoleExecutionSummary(draft: RoleDraft): RoleExecutionSummary {
  const safetyCues: string[] = [];
  const allowedPaths = parseList(draft.allowedPaths);
  const deniedPaths = parseList(draft.deniedPaths);
  const normalizedSkills = draft.skillRows
    .map((skill) => ({ path: skill.path.trim(), autoLoad: skill.autoLoad }))
    .filter((skill) => skill.path.length > 0);
  const autoLoadCount = normalizedSkills.filter((skill) => skill.autoLoad).length;
  const onDemandCount = normalizedSkills.length - autoLoadCount;

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
    skillsLabel:
      normalizedSkills.length > 0
        ? `${autoLoadCount} auto-load / ${onDemandCount} on-demand`
        : "No skills configured",
    keySkillPaths: normalizedSkills.slice(0, 3).map((skill) => skill.path),
    safetyCues,
  };
}

function validateSkillRows(skillRows: RoleSkillDraft[]): string[] {
  const errors: string[] = [];
  const seen = new Set<string>();

  for (const skill of skillRows) {
    const path = skill.path.trim();
    if (!path) {
      errors.push("Skill path cannot be blank.");
      continue;
    }
    if (seen.has(path)) {
      errors.push("Skill paths must be unique.");
      continue;
    }
    seen.add(path);
  }

  return Array.from(new Set(errors));
}
