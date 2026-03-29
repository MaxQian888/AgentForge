import type {
  RoleCollaboration,
  RoleKnowledgeMemory,
  RoleKnowledgeSource,
  RoleManifest,
  RoleMCPServer,
  RoleSkillCatalogEntry,
  RoleSkillReference,
  RoleTrigger,
} from "@/lib/stores/role-store";

export interface RoleSkillDraft {
  path: string;
  autoLoad: boolean;
}

export interface RoleMCPServerDraft {
  name: string;
  url: string;
}

export interface RoleKeyValueDraft {
  key: string;
  value: string;
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
  mcpServerRows: RoleMCPServerDraft[];
  customSettingRows: RoleKeyValueDraft[];
  skillRows: RoleSkillDraft[];
  languages: string;
  frameworks: string;
  maxTurns: string;
  maxBudgetUsd: string;
  repositories: string;
  documents: string;
  patterns: string;
  sharedKnowledgeRows: RoleKnowledgeSourceDraft[];
  privateKnowledgeRows: RoleKnowledgeSourceDraft[];
  memoryShortTermMaxTokens: string;
  memoryEpisodicEnabled: boolean;
  memoryEpisodicRetentionDays: string;
  memorySemanticEnabled: boolean;
  memorySemanticAutoExtract: boolean;
  memoryProceduralEnabled: boolean;
  memoryProceduralLearnFromFeedback: boolean;
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
  overridesInput: string;
  triggerRows: RoleTriggerDraft[];
  sourceManifest?: Partial<RoleManifest>;
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

export type RoleSkillResolutionStatus = "resolved" | "unresolved";
export type RoleSkillResolutionProvenance =
  | "explicit"
  | "template-derived"
  | "inherited";

export interface RoleSkillResolution {
  path: string;
  autoLoad: boolean;
  label: string;
  description: string;
  source: string;
  sourceRoot: string;
  status: RoleSkillResolutionStatus;
  provenance: RoleSkillResolutionProvenance;
}

export type RoleDraftValidationBySection = Record<
  "setup" | "identity" | "capabilities" | "knowledge" | "governance" | "review",
  string[]
>;

function parseList(input: string): string[] {
  return input
    .split(",")
    .map((item) => item.trim())
    .filter(Boolean);
}

function stringifyList(values?: string[]): string {
  return (values ?? []).join(", ");
}

function cloneRoleManifest<T>(value: T): T {
  if (value == null) {
    return value;
  }
  return JSON.parse(JSON.stringify(value)) as T;
}

function buildMCPServerDraft(server?: RoleMCPServer): RoleMCPServerDraft {
  return {
    name: server?.name ?? "",
    url: server?.url ?? "",
  };
}

function serializeMCPServerDraft(draft: RoleMCPServerDraft): RoleMCPServer | null {
  const name = draft.name.trim();
  const url = draft.url.trim();

  if (!name && !url) {
    return null;
  }

  return {
    name: name || undefined,
    url: url || undefined,
  };
}

function buildKeyValueDrafts(values?: Record<string, string>): RoleKeyValueDraft[] {
  return Object.entries(values ?? {}).map(([key, value]) => ({ key, value }));
}

function serializeKeyValueDrafts(drafts: RoleKeyValueDraft[]): Record<string, string> | undefined {
  const entries = drafts
    .map((draft) => ({ key: draft.key.trim(), value: draft.value.trim() }))
    .filter((draft) => draft.key || draft.value);

  if (entries.length === 0) {
    return undefined;
  }

  return Object.fromEntries(entries.map((draft) => [draft.key, draft.value]));
}

function buildMemoryDraft(memory?: RoleKnowledgeMemory) {
  return {
    memoryShortTermMaxTokens:
      memory?.shortTerm?.maxTokens != null ? String(memory.shortTerm.maxTokens) : "",
    memoryEpisodicEnabled: memory?.episodic?.enabled ?? false,
    memoryEpisodicRetentionDays:
      memory?.episodic?.retentionDays != null
        ? String(memory.episodic.retentionDays)
        : "",
    memorySemanticEnabled: memory?.semantic?.enabled ?? false,
    memorySemanticAutoExtract: memory?.semantic?.autoExtract ?? false,
    memoryProceduralEnabled: memory?.procedural?.enabled ?? false,
    memoryProceduralLearnFromFeedback:
      memory?.procedural?.learnFromFeedback ?? false,
  };
}

function serializeMemoryDraft(draft: RoleDraft): RoleKnowledgeMemory | undefined {
  const shortTermMaxTokens = draft.memoryShortTermMaxTokens
    ? Number(draft.memoryShortTermMaxTokens)
    : undefined;
  const episodicRetentionDays = draft.memoryEpisodicRetentionDays
    ? Number(draft.memoryEpisodicRetentionDays)
    : undefined;

  const hasShortTerm = shortTermMaxTokens != null;
  const hasEpisodic = draft.memoryEpisodicEnabled || episodicRetentionDays != null;
  const hasSemantic = draft.memorySemanticEnabled || draft.memorySemanticAutoExtract;
  const hasProcedural =
    draft.memoryProceduralEnabled || draft.memoryProceduralLearnFromFeedback;

  if (!hasShortTerm && !hasEpisodic && !hasSemantic && !hasProcedural) {
    return undefined;
  }

  return {
    shortTerm: hasShortTerm ? { maxTokens: shortTermMaxTokens } : undefined,
    episodic: hasEpisodic
      ? {
          enabled: draft.memoryEpisodicEnabled,
          retentionDays: episodicRetentionDays,
        }
      : undefined,
    semantic: hasSemantic
      ? {
          enabled: draft.memorySemanticEnabled,
          autoExtract: draft.memorySemanticAutoExtract,
        }
      : undefined,
    procedural: hasProcedural
      ? {
          enabled: draft.memoryProceduralEnabled,
          learnFromFeedback: draft.memoryProceduralLearnFromFeedback,
        }
      : undefined,
  };
}

function parseOverridesInput(input: string): Record<string, unknown> | undefined {
  const trimmed = input.trim();
  if (!trimmed) {
    return undefined;
  }
  return JSON.parse(trimmed) as Record<string, unknown>;
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
  const memoryDraft = buildMemoryDraft(role?.knowledge.memory);
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
    mcpServerRows:
      role?.capabilities.toolConfig?.mcpServers?.map((server) =>
        buildMCPServerDraft(server),
      ) ?? [],
    customSettingRows: buildKeyValueDrafts(role?.capabilities.customSettings),
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
    privateKnowledgeRows:
      role?.knowledge.private?.map((source) => buildKnowledgeSourceDraft(source)) ?? [],
    ...memoryDraft,
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
    overridesInput: role?.overrides ? JSON.stringify(role.overrides, null, 2) : "",
    triggerRows: (role?.triggers ?? []).map((trigger) => buildTriggerDraft(trigger)),
    sourceManifest: cloneRoleManifest(role),
  };
}

export function serializeRoleDraft(
  draft: RoleDraft,
  baseRole?: RoleManifest,
): SerializedRoleDraft {
  const validationErrors = validateRoleDraft(draft);
  const sourceManifest = cloneRoleManifest(
    baseRole ?? (draft.sourceManifest as RoleManifest | undefined),
  );

  const sharedKnowledge = draft.sharedKnowledgeRows
    .map((source) => serializeKnowledgeSourceDraft(source))
    .filter((source): source is RoleKnowledgeSource => source != null);
  const privateKnowledge = draft.privateKnowledgeRows
    .map((source) => serializeKnowledgeSourceDraft(source))
    .filter((source): source is RoleKnowledgeSource => source != null);
  const mcpServers = draft.mcpServerRows
    .map((server) => serializeMCPServerDraft(server))
    .filter((server): server is RoleMCPServer => server != null);
  const customSettings = serializeKeyValueDrafts(draft.customSettingRows);
  const memory = serializeMemoryDraft(draft);
  const triggers = draft.triggerRows
    .map((trigger) => serializeTriggerDraft(trigger))
    .filter((trigger): trigger is RoleTrigger => trigger != null);
  let overrides = sourceManifest?.overrides;
  try {
    overrides = parseOverridesInput(draft.overridesInput);
  } catch {
    if (validationErrors.length === 0) {
      throw new Error("Overrides input must be valid JSON.");
    }
  }
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
    ...(sourceManifest ?? {}),
    apiVersion: sourceManifest?.apiVersion ?? "agentforge/v1",
    kind: sourceManifest?.kind ?? "Role",
    metadata: {
      ...(sourceManifest?.metadata ?? {
        id: draft.roleId,
        version: draft.version || "1.0.0",
        author: "AgentForge",
      }),
      id: draft.roleId,
      name: draft.name,
      version: draft.version || baseRole?.metadata.version || "1.0.0",
      description: draft.description,
      author: sourceManifest?.metadata.author ?? "AgentForge",
      tags: parseList(draft.tagsInput),
      icon: draft.icon || undefined,
    },
    identity: {
      ...(sourceManifest?.identity ?? {
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
      ...(sourceManifest?.capabilities ?? {
        languages: [],
        frameworks: [],
      }),
      packages: parseList(draft.packages),
      allowedTools: parseList(draft.allowedTools),
      toolConfig: {
        builtIn:
          sourceManifest?.capabilities.toolConfig?.builtIn ??
          parseList(draft.allowedTools),
        external: parseList(draft.externalTools),
        mcpServers,
      },
      skills: draft.skillRows.map((skill) => ({
        path: skill.path.trim(),
        autoLoad: skill.autoLoad,
      })),
      languages: parseList(draft.languages),
      frameworks: parseList(draft.frameworks),
      maxTurns: draft.maxTurns ? Number(draft.maxTurns) : undefined,
      maxBudgetUsd: draft.maxBudgetUsd ? Number(draft.maxBudgetUsd) : undefined,
      maxConcurrency: sourceManifest?.capabilities.maxConcurrency,
      customSettings,
    },
    knowledge: {
      ...(sourceManifest?.knowledge ?? {
        repositories: [],
        documents: [],
        patterns: [],
      }),
      repositories: parseList(draft.repositories),
      documents: parseList(draft.documents),
      patterns: parseList(draft.patterns),
      shared: sharedKnowledge,
      private: privateKnowledge,
      memory,
    },
    security: {
      ...(sourceManifest?.security ?? {
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
      permissions: sourceManifest?.security.permissions,
      outputFilters: parseList(draft.outputFilters),
      resourceLimits: sourceManifest?.security.resourceLimits,
    },
    collaboration,
    triggers,
    overrides,
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

export function resolveRoleSkillReferences({
  skills,
  catalog,
  templateSkills = [],
  parentSkills = [],
}: {
  skills: Array<RoleSkillDraft | RoleSkillReference>;
  catalog: RoleSkillCatalogEntry[];
  templateSkills?: Array<RoleSkillDraft | RoleSkillReference>;
  parentSkills?: Array<RoleSkillDraft | RoleSkillReference>;
}): RoleSkillResolution[] {
  const catalogByPath = new Map(
    catalog.map((entry) => [entry.path.trim(), entry] satisfies [string, RoleSkillCatalogEntry]),
  );
  const templatePaths = new Set(
    templateSkills.map((skill) => skill.path.trim()).filter(Boolean),
  );
  const parentPaths = new Set(parentSkills.map((skill) => skill.path.trim()).filter(Boolean));

  return skills
    .map((skill) => ({
      path: skill.path.trim(),
      autoLoad: skill.autoLoad,
    }))
    .filter((skill) => skill.path.length > 0)
    .map((skill) => {
      const matchedCatalogEntry = catalogByPath.get(skill.path);
      const provenance: RoleSkillResolutionProvenance = parentPaths.has(skill.path)
        ? "inherited"
        : templatePaths.has(skill.path)
          ? "template-derived"
          : "explicit";

      if (matchedCatalogEntry) {
        return {
          path: skill.path,
          autoLoad: skill.autoLoad,
          label: matchedCatalogEntry.label,
          description: matchedCatalogEntry.description ?? "",
          source: matchedCatalogEntry.source,
          sourceRoot: matchedCatalogEntry.sourceRoot,
          status: "resolved" as const,
          provenance,
        };
      }

      return {
        path: skill.path,
        autoLoad: skill.autoLoad,
        label: skill.path,
        description: "",
        source: "manual",
        sourceRoot: "",
        status: "unresolved" as const,
        provenance,
      };
    });
}

export function groupRoleDraftValidationErrors(
  validationErrors: string[],
): RoleDraftValidationBySection {
  const grouped: RoleDraftValidationBySection = {
    setup: [],
    identity: [],
    capabilities: [],
    knowledge: [],
    governance: [],
    review: [],
  };

  for (const error of validationErrors) {
    if (
      error.includes("Skill") ||
      error.includes("Custom setting") ||
      error.includes("MCP server")
    ) {
      grouped.capabilities.push(error);
      continue;
    }
    if (error.includes("Overrides")) {
      grouped.review.push(error);
      continue;
    }
    if (error.includes("knowledge") || error.includes("memory")) {
      grouped.knowledge.push(error);
      continue;
    }
    if (error.includes("Trigger")) {
      grouped.governance.push(error);
      continue;
    }
    grouped.review.push(error);
  }

  return grouped;
}

function validateRoleDraft(draft: RoleDraft): string[] {
  const errors = [
    ...validateSkillRows(draft.skillRows),
    ...validateCustomSettingRows(draft.customSettingRows),
    ...validateMCPServerRows(draft.mcpServerRows),
    ...validateKnowledgeRows(
      draft.sharedKnowledgeRows,
      "Shared knowledge rows must include at least an id, type, or access value.",
    ),
    ...validateKnowledgeRows(
      draft.privateKnowledgeRows,
      "Private knowledge rows must include at least an id, type, or access value.",
    ),
    ...validateMemoryDraft(draft),
    ...validateOverridesInput(draft.overridesInput),
  ];
  const seenTriggerKeys = new Set<string>();

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

function validateCustomSettingRows(settingRows: RoleKeyValueDraft[]): string[] {
  const errors: string[] = [];
  const seen = new Set<string>();

  for (const row of settingRows) {
    const key = row.key.trim();
    const value = row.value.trim();
    if (!key && !value) {
      continue;
    }
    if (!key) {
      errors.push("Custom setting key cannot be blank.");
      continue;
    }
    if (seen.has(key)) {
      errors.push("Custom setting keys must be unique.");
      continue;
    }
    seen.add(key);
  }

  return Array.from(new Set(errors));
}

function validateMCPServerRows(serverRows: RoleMCPServerDraft[]): string[] {
  const errors: string[] = [];

  for (const row of serverRows) {
    const name = row.name.trim();
    const url = row.url.trim();
    if (!name && !url) {
      continue;
    }
    if (!name || !url) {
      errors.push("MCP server rows must include both name and url.");
    }
  }

  return Array.from(new Set(errors));
}

function validateKnowledgeRows(
  knowledgeRows: RoleKnowledgeSourceDraft[],
  message: string,
): string[] {
  for (const source of knowledgeRows) {
    if (source.id.trim() === "" && source.type.trim() === "" && source.access.trim() === "") {
      return [message];
    }
  }

  return [];
}

function validateMemoryDraft(draft: RoleDraft): string[] {
  const errors: string[] = [];

  if (draft.memoryShortTermMaxTokens.trim() !== "" && Number.isNaN(Number(draft.memoryShortTermMaxTokens))) {
    errors.push("Short-term memory max tokens must be a number.");
  }
  if (
    draft.memoryEpisodicRetentionDays.trim() !== "" &&
    Number.isNaN(Number(draft.memoryEpisodicRetentionDays))
  ) {
    errors.push("Episodic memory retention days must be a number.");
  }

  return errors;
}

function validateOverridesInput(input: string): string[] {
  const trimmed = input.trim();
  if (!trimmed) {
    return [];
  }

  try {
    parseOverridesInput(trimmed);
    return [];
  } catch {
    return ["Overrides input must be valid JSON."];
  }
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
