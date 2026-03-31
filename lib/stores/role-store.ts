"use client";

import { create } from "zustand";
import { createApiClient } from "@/lib/api-client";
import { useAuthStore } from "./auth-store";

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";

export interface RoleMetadata {
  id: string;
  name: string;
  version: string;
  description: string;
  author: string;
  tags: string[];
  icon?: string;
}

export interface RoleIdentity {
  role?: string;
  goal?: string;
  backstory?: string;
  systemPrompt: string;
  persona: string;
  goals: string[];
  constraints: string[];
  personality?: string;
  language?: string;
  responseStyle?: RoleResponseStyle;
}

export interface RoleCapabilities {
  packages?: string[];
  allowedTools?: string[];
  tools?: string[];
  toolConfig?: RoleToolConfig;
  skills?: RoleSkillReference[];
  languages: string[];
  frameworks: string[];
  maxTurns?: number;
  maxBudgetUsd?: number;
  maxConcurrency?: number;
  customSettings?: Record<string, string>;
}

export interface RoleSkillReference {
  path: string;
  autoLoad: boolean;
}

export interface RoleSkillCatalogEntry {
  path: string;
  label: string;
  description?: string;
  displayName?: string;
  shortDescription?: string;
  defaultPrompt?: string;
  availableParts?: string[];
  referenceCount?: number;
  scriptCount?: number;
  assetCount?: number;
  source: string;
  sourceRoot: string;
}

export interface RoleSecurity {
  profile?: string;
  permissionMode?: string;
  allowedPaths: string[];
  deniedPaths: string[];
  maxBudgetUsd: number;
  requireReview: boolean;
  permissions?: RolePermissions;
  outputFilters?: string[];
  resourceLimits?: RoleResourceLimits;
}

export interface RoleResponseStyle {
  tone?: string;
  verbosity?: string;
  formatPreference?: string;
}

export interface RoleToolConfig {
  builtIn?: string[];
  external?: string[];
  mcpServers?: RoleMCPServer[];
}

export interface RoleMCPServer {
  name?: string;
  url?: string;
}

export interface RoleKnowledgeSource {
  id?: string;
  type?: string;
  access?: string;
  description?: string;
  sources?: string[];
}

export interface RoleKnowledgeMemory {
  shortTerm?: { maxTokens?: number };
  episodic?: { enabled?: boolean; retentionDays?: number };
  semantic?: { enabled?: boolean; autoExtract?: boolean };
  procedural?: { enabled?: boolean; learnFromFeedback?: boolean };
}

export interface RolePermissions {
  fileAccess?: {
    allowedPaths?: string[];
    deniedPaths?: string[];
  };
  network?: {
    allowedDomains?: string[];
  };
  codeExecution?: {
    sandbox?: boolean;
    allowedLanguages?: string[];
  };
}

export interface RoleResourceLimits {
  tokenBudget?: { perTask?: number; perDay?: number; perMonth?: number };
  apiCalls?: { perMinute?: number; perHour?: number };
  executionTime?: { perTask?: string; perDay?: string };
  costLimit?: { perTask?: string; perDay?: string; alertThreshold?: number };
}

export interface RoleCollaboration {
  canDelegateTo?: string[];
  acceptsDelegationFrom?: string[];
  communication?: {
    preferredChannel?: string;
    reportFormat?: string;
    escalationPolicy?: string;
  };
}

export interface RoleTrigger {
  event?: string;
  action?: string;
  condition?: string;
  autoExecute?: boolean;
  requiresApproval?: boolean;
}

export interface RoleManifest {
  apiVersion: string;
  kind: string;
  metadata: RoleMetadata;
  identity: RoleIdentity;
  capabilities: RoleCapabilities;
  knowledge: {
    repositories: string[];
    documents: string[];
    patterns: string[];
    shared?: RoleKnowledgeSource[];
    private?: RoleKnowledgeSource[];
    memory?: RoleKnowledgeMemory;
  };
  security: RoleSecurity;
  extends?: string;
  collaboration?: RoleCollaboration;
  triggers?: RoleTrigger[];
  overrides?: Record<string, unknown>;
}

export interface RoleExecutionProfile {
  role_id: string;
  name: string;
  role: string;
  goal: string;
  backstory: string;
  system_prompt: string;
  allowed_tools: string[];
  max_budget_usd: number;
  max_turns: number;
  permission_mode: string;
  loaded_skills?: RoleExecutionSkill[];
  available_skills?: RoleExecutionSkill[];
  skill_diagnostics?: RoleExecutionSkillDiagnostic[];
}

export interface RoleExecutionSkill {
  path: string;
  label: string;
  description?: string;
  instructions?: string;
  display_name?: string;
  short_description?: string;
  default_prompt?: string;
  available_parts?: string[];
  reference_count?: number;
  script_count?: number;
  asset_count?: number;
  source?: string;
  source_root?: string;
  origin?: string;
  requires?: string[];
  tools?: string[];
}

export interface RoleExecutionSkillDiagnostic {
  code: string;
  path?: string;
  message: string;
  blocking: boolean;
  auto_load?: boolean;
}

export interface RolePreviewResponse {
  normalizedManifest?: RoleManifest;
  effectiveManifest?: RoleManifest;
  executionProfile?: RoleExecutionProfile;
  validationIssues?: Array<{ field: string; message: string }>;
  inheritance?: { parentRoleId?: string };
  readinessDiagnostics?: Array<{
    code: string;
    message: string;
    blocking: boolean;
  }>;
}

export interface RoleSandboxResponse extends RolePreviewResponse {
  readinessDiagnostics?: Array<{
    code: string;
    message: string;
    blocking: boolean;
  }>;
  selection?: {
    runtime: string;
    provider: string;
    model: string;
  };
  probe?: {
    text: string;
    usage: {
      input_tokens: number;
      output_tokens: number;
    };
  };
}

interface RoleState {
  roles: RoleManifest[];
  skillCatalog: RoleSkillCatalogEntry[];
  loading: boolean;
  skillCatalogLoading: boolean;
  error: string | null;
  fetchRoles: () => Promise<void>;
  fetchSkillCatalog: () => Promise<void>;
  createRole: (data: Partial<RoleManifest>) => Promise<RoleManifest>;
  updateRole: (id: string, data: Partial<RoleManifest>) => Promise<RoleManifest>;
  deleteRole: (id: string) => Promise<void>;
  previewRole: (payload: { roleId?: string; draft?: Partial<RoleManifest> }) => Promise<RolePreviewResponse>;
  sandboxRole: (payload: {
    roleId?: string;
    draft?: Partial<RoleManifest>;
    input: string;
    runtime?: string;
    provider?: string;
    model?: string;
  }) => Promise<RoleSandboxResponse>;
}

function getToken() {
  const authState = useAuthStore.getState() as {
    accessToken?: string | null;
    token?: string | null;
  };
  return authState.accessToken ?? authState.token ?? null;
}

export const useRoleStore = create<RoleState>()((set) => ({
  roles: [],
  skillCatalog: [],
  loading: false,
  skillCatalogLoading: false,
  error: null,

  fetchRoles: async () => {
    const token = getToken();
    if (!token) return;

    set({ loading: true, error: null });

    try {
      const api = createApiClient(API_URL);
      const { data } = await api.get<RoleManifest[]>("/api/v1/roles", {
        token,
      });

      set({ roles: data });
    } catch (error) {
      const message =
        error instanceof Error ? error.message : "Failed to load roles";
      set({ error: message });
    } finally {
      set({ loading: false });
    }
  },

  fetchSkillCatalog: async () => {
    const token = getToken();
    if (!token) return;

    set({ skillCatalogLoading: true, error: null });

    try {
      const api = createApiClient(API_URL);
      const { data } = await api.get<RoleSkillCatalogEntry[]>("/api/v1/roles/skills", {
        token,
      });

      set({ skillCatalog: data });
    } catch (error) {
      const message =
        error instanceof Error ? error.message : "Failed to load role skill catalog";
      set({ error: message });
    } finally {
      set({ skillCatalogLoading: false });
    }
  },

  createRole: async (data) => {
    const token = getToken();
    if (!token) throw new Error("Not authenticated");

    const api = createApiClient(API_URL);
    const { data: role } = await api.post<RoleManifest>(
      "/api/v1/roles",
      data,
      { token }
    );

    set((state) => ({ roles: [...state.roles, role] }));

    return role;
  },

  updateRole: async (id, data) => {
    const token = getToken();
    if (!token) throw new Error("Not authenticated");

    const api = createApiClient(API_URL);
    const { data: role } = await api.put<RoleManifest>(
      `/api/v1/roles/${id}`,
      data,
      { token }
    );

    set((state) => ({
      roles: state.roles.map((r) =>
        r.metadata.id === id ? role : r
      ),
    }));

    return role;
  },

  deleteRole: async (id) => {
    const token = getToken();
    if (!token) throw new Error("Not authenticated");

    const api = createApiClient(API_URL);
    await api.delete(`/api/v1/roles/${id}`, { token });

    set((state) => ({
      roles: state.roles.filter((r) => r.metadata.id !== id),
    }));
  },

  previewRole: async (payload) => {
    const token = getToken();
    if (!token) throw new Error("Not authenticated");

    const api = createApiClient(API_URL);
    const { data } = await api.post<RolePreviewResponse>(
      "/api/v1/roles/preview",
      payload,
      { token }
    );

    return data;
  },

  sandboxRole: async (payload) => {
    const token = getToken();
    if (!token) throw new Error("Not authenticated");

    const api = createApiClient(API_URL);
    const { data } = await api.post<RoleSandboxResponse>(
      "/api/v1/roles/sandbox",
      payload,
      { token }
    );

    return data;
  },
}));
