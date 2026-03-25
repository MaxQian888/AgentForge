import type {
  RoleCollaboration,
  RoleKnowledgeSource,
  RoleManifest,
  RoleTrigger,
} from "@/lib/stores/role-store";

export interface RoleSkillDraft {
  path: string;
  autoLoad: boolean;
}

export interface RoleKnowledgeSourceDraft {
  id: string;
  type: string;
  access: string;
  description: string;
  sourcesInput: string;
}

export interface RoleTriggerDraft {
  event: string;
  action: string;
  condition: string;
}

export interface SerializedRoleDraft extends Partial<RoleManifest> {
  validationErrors?: string[];
}

export interface RoleDraft {
  roleId: string;
  name: string;
  version: string;
  icon: string;
  description: string;
  tagsInput: string;
  extendsValue: string;
  identityRole: string;
  goal: string;
  backstory: string;
  systemPrompt: string;
  persona: string;
  goalsInput: string;
  constraintsInput: string;
  personality: string;
  language: string;
  responseTone: string;
  responseVerbosity: string;
  responseFormatPreference: string;
  packages: string;
  allowedTools: string;
  externalTools: string;
  skillRows: RoleSkillDraft[];
  languages: string;
  frameworks: string;
  maxTurns: string;
  maxBudgetUsd: string;
  repositories: string;
  documents: string;
  patterns: string;
  sharedKnowledgeRows: RoleKnowledgeSourceDraft[];
  securityProfile: string;
  permissionMode: string;
  allowedPaths: string;
  deniedPaths: string;
  outputFilters: string;
  requireReview: boolean;
  collaborationCanDelegateTo: string;
  collaborationAcceptsDelegationFrom: string;
  communicationPreferredChannel: string;
  communicationReportFormat: string;
  communicationEscalationPolicy: string;
  triggerRows: RoleTriggerDraft[];
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

function buildKnowledgeSourceDraft(
  source?: RoleKnowledgeSource,
): RoleKnowledgeSourceDraft {
  return {
    id: source?.id ?? "",
    type: source?.type ?? "",
    access: source?.access ?? "",
    description: source?.description ?? "",
    sourcesInput: stringifyList(source?.sources),
  };
}

function serializeKnowledgeSourceDraft(
  draft: RoleKnowledgeSourceDraft,
): RoleKnowledgeSource | null {
  const id = draft.id.trim();
  const type = draft.type.trim();
  const access = draft.access.trim();
  const description = draft.description.trim();
  const sources = parseList(draft.sourcesInput);

  if (!id && !type && !access && !description && sources.length === 0) {
    return null;
  }

  return {
    id: id || undefined,
    type: type || undefined,
    access: access || undefined,
    description: description || undefined,
    sources,
  };
}

function buildTriggerDraft(trigger?: RoleTrigger): RoleTriggerDraft {
  return {
    event: trigger?.event ?? "",
    action: trigger?.action ?? "",
    condition: trigger?.condition ?? "",
  };
}

function serializeTriggerDraft(draft: RoleTriggerDraft): RoleTrigger | null {
  const event = draft.event.trim();
  const action = draft.action.trim();
  const condition = draft.condition.trim();

  if (!event && !action && !condition) {
    return null;
  }

  return {
    event: event || undefined,
    action: action || undefined,
    condition: condition || undefined,
  };
}

export function buildRoleDraft(role?: RoleManifest): RoleDraft {
  return {
    roleId: role?.metadata.id ?? "",
    name: role?.metadata.name ?? "",
    version: role?.metadata.version ?? "1.0.0",
    icon: role?.metadata.icon ?? "",
    description: role?.metadata.description ?? "",
    tagsInput: stringifyList(role?.metadata.tags),
    extendsValue: role?.extends ?? "",
    identityRole: role?.identity.role ?? "",
    goal: role?.identity.goal ?? "",
    backstory: role?.identity.backstory ?? "",
    systemPrompt: role?.identity.systemPrompt ?? "",
    persona: role?.identity.persona ?? "",
    goalsInput: stringifyList(role?.identity.goals),
    constraintsInput: stringifyList(role?.identity.constraints),
    personality: role?.identity.personality ?? "",
    language: role?.identity.language ?? "",
    responseTone: role?.identity.responseStyle?.tone ?? "",
    responseVerbosity: role?.identity.responseStyle?.verbosity ?? "",
    responseFormatPreference: role?.identity.responseStyle?.formatPreference ?? "",
    packages: stringifyList(role?.capabilities.packages),
    allowedTools: stringifyList(role?.capabilities.allowedTools),
    externalTools: stringifyList(role?.capabilities.toolConfig?.external),
    skillRows: (role?.capabilities.skills ?? []).map((skill) => ({
      path: skill.path,
      autoLoad: skill.autoLoad,
    })),
    languages: stringifyList(role?.capabilities.languages),
    frameworks: stringifyList(role?.capabilities.frameworks),
    maxTurns:
      role?.capabilities.maxTurns != null ? String(role.capabilities.maxTurns) : "",
    maxBudgetUsd:
      role?.capabilities.maxBudgetUsd != null
        ? String(role.capabilities.maxBudgetUsd)
        : "",
    repositories: stringifyList(role?.knowledge.repositories),
    documents: stringifyList(role?.knowledge.documents),
    patterns: stringifyList(role?.knowledge.patterns),
    sharedKnowledgeRows:
      role?.knowledge.shared?.map((source) => buildKnowledgeSourceDraft(source)) ?? [],
    securityProfile: role?.security.profile ?? "",
    permissionMode: role?.security.permissionMode ?? "default",
    allowedPaths: stringifyList(role?.security.allowedPaths),
    deniedPaths: stringifyList(role?.security.deniedPaths),
    outputFilters: stringifyList(role?.security.outputFilters),
    requireReview: role?.security.requireReview ?? false,
    collaborationCanDelegateTo: stringifyList(role?.collaboration?.canDelegateTo),
    collaborationAcceptsDelegationFrom: stringifyList(
      role?.collaboration?.acceptsDelegationFrom,
    ),
    communicationPreferredChannel:
      role?.collaboration?.communication?.preferredChannel ?? "",
    communicationReportFormat:
      role?.collaboration?.communication?.reportFormat ?? "",
    communicationEscalationPolicy:
      role?.collaboration?.communication?.escalationPolicy ?? "",
    triggerRows: (role?.triggers ?? []).map((trigger) => buildTriggerDraft(trigger)),
  };
}

export function serializeRoleDraft(
  draft: RoleDraft,
  baseRole?: RoleManifest,
): SerializedRoleDraft {
  const validationErrors = validateRoleDraft(draft);

  const sharedKnowledge = draft.sharedKnowledgeRows
    .map((source) => serializeKnowledgeSourceDraft(source))
    .filter((source): source is RoleKnowledgeSource => source != null);
  const triggers = draft.triggerRows
    .map((trigger) => serializeTriggerDraft(trigger))
    .filter((trigger): trigger is RoleTrigger => trigger != null);
  const collaboration: RoleCollaboration = {
    canDelegateTo: parseList(draft.collaborationCanDelegateTo),
    acceptsDelegationFrom: parseList(draft.collaborationAcceptsDelegationFrom),
    communication: {
      preferredChannel: draft.communicationPreferredChannel || undefined,
      reportFormat: draft.communicationReportFormat || undefined,
      escalationPolicy: draft.communicationEscalationPolicy || undefined,
    },
  };

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
      icon: draft.icon || undefined,
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
      persona: draft.persona,
      goals: parseList(draft.goalsInput),
      constraints: parseList(draft.constraintsInput),
      personality: draft.personality || undefined,
      language: draft.language || undefined,
      responseStyle: {
        tone: draft.responseTone || undefined,
        verbosity: draft.responseVerbosity || undefined,
        formatPreference: draft.responseFormatPreference || undefined,
      },
    },
    capabilities: {
      ...(baseRole?.capabilities ?? {
        languages: [],
        frameworks: [],
      }),
      packages: parseList(draft.packages),
      allowedTools: parseList(draft.allowedTools),
      toolConfig: {
        builtIn:
          baseRole?.capabilities.toolConfig?.builtIn ??
          parseList(draft.allowedTools),
        external: parseList(draft.externalTools),
        mcpServers: baseRole?.capabilities.toolConfig?.mcpServers ?? [],
      },
      skills: draft.skillRows.map((skill) => ({
        path: skill.path.trim(),
        autoLoad: skill.autoLoad,
      })),
      languages: parseList(draft.languages),
      frameworks: parseList(draft.frameworks),
      maxTurns: draft.maxTurns ? Number(draft.maxTurns) : undefined,
      maxBudgetUsd: draft.maxBudgetUsd ? Number(draft.maxBudgetUsd) : undefined,
      maxConcurrency: baseRole?.capabilities.maxConcurrency,
      customSettings: baseRole?.capabilities.customSettings,
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
      shared: sharedKnowledge,
      private: baseRole?.knowledge.private ?? [],
      memory: baseRole?.knowledge.memory,
    },
    security: {
      ...(baseRole?.security ?? {
        allowedPaths: [],
        deniedPaths: [],
        maxBudgetUsd: 0,
      }),
      profile: draft.securityProfile || undefined,
      permissionMode: draft.permissionMode || "default",
      allowedPaths: parseList(draft.allowedPaths),
      deniedPaths: parseList(draft.deniedPaths),
      maxBudgetUsd: draft.maxBudgetUsd ? Number(draft.maxBudgetUsd) : 0,
      requireReview: draft.requireReview,
      permissions: baseRole?.security.permissions,
      outputFilters: parseList(draft.outputFilters),
      resourceLimits: baseRole?.security.resourceLimits,
    },
    collaboration,
    triggers,
    overrides: baseRole?.overrides,
    extends: draft.extendsValue || undefined,
    validationErrors: validationErrors.length > 0 ? validationErrors : undefined,
  };
}

export function buildRoleExecutionSummary(draft: RoleDraft): RoleExecutionSummary {
  const safetyCues: string[] = [];
  const allowedPaths = parseList(draft.allowedPaths);
  const deniedPaths = parseList(draft.deniedPaths);
  const outputFilters = parseList(draft.outputFilters);
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
  if (outputFilters.length > 0) {
    safetyCues.push(`${outputFilters.length} output filters`);
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

function validateRoleDraft(draft: RoleDraft): string[] {
  const errors = [...validateSkillRows(draft.skillRows)];
  const seenTriggerKeys = new Set<string>();

  for (const source of draft.sharedKnowledgeRows) {
    if (source.id.trim() === "" && source.type.trim() === "" && source.access.trim() === "") {
      errors.push("Shared knowledge rows must include at least an id, type, or access value.");
      break;
    }
  }

  for (const trigger of draft.triggerRows) {
    const event = trigger.event.trim();
    const action = trigger.action.trim();
    if (!event || !action) {
      errors.push("Trigger rows must include both event and action.");
      continue;
    }
    const key = `${event}:${action}:${trigger.condition.trim()}`;
    if (seenTriggerKeys.has(key)) {
      errors.push("Trigger rows must be unique.");
      continue;
    }
    seenTriggerKeys.add(key);
  }

  return Array.from(new Set(errors));
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

function renderYamlValue(value: unknown, indent = 0): string[] {
  const prefix = "  ".repeat(indent);

  if (Array.isArray(value)) {
    if (value.length === 0) {
      return [`${prefix}[]`];
    }
    return value.flatMap((item) => {
      if (item != null && typeof item === "object" && !Array.isArray(item)) {
        const objectLines = renderYamlValue(item, indent + 1);
        return [`${prefix}- ${objectLines[0]!.trimStart()}`, ...objectLines.slice(1)];
      }
      return [`${prefix}- ${String(item)}`];
    });
  }

  if (value != null && typeof value === "object") {
    const entries = Object.entries(value).filter(([, item]) => {
      if (item == null) return false;
      if (Array.isArray(item)) return item.length > 0;
      if (typeof item === "object") return Object.keys(item as Record<string, unknown>).length > 0;
      return item !== "";
    });
    if (entries.length === 0) {
      return [`${prefix}{}`];
    }

    return entries.flatMap(([key, item]) => {
      if (item != null && typeof item === "object") {
        return [`${prefix}${key}:`, ...renderYamlValue(item, indent + 1)];
      }
      return [`${prefix}${key}: ${String(item)}`];
    });
  }

  return [`${prefix}${String(value)}`];
}

export function renderRoleManifestYaml(role: Partial<RoleManifest>): string {
  return renderYamlValue(role).join("\n");
}
