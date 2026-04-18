"use client";
import { create } from "zustand";
import { toast } from "sonner";
import { createApiClient } from "@/lib/api-client";
import { useAuthStore } from "./auth-store";

export interface CodingAgentSelection {
  runtime: string;
  provider: string;
  model: string;
}

export interface CodingAgentDiagnostic {
  code: string;
  message: string;
  blocking: boolean;
}

export interface CodingAgentCapabilityDescriptor {
  state: string;
  reasonCode?: string;
  message?: string;
  requiresRequestFields?: string[];
}

export interface CodingAgentInteractionCapabilities {
  inputs: Record<string, CodingAgentCapabilityDescriptor>;
  lifecycle: Record<string, CodingAgentCapabilityDescriptor>;
  approval: Record<string, CodingAgentCapabilityDescriptor>;
  mcp: Record<string, CodingAgentCapabilityDescriptor>;
  diagnostics: Record<string, CodingAgentCapabilityDescriptor>;
}

export interface CodingAgentProvider {
  provider: string;
  connected: boolean;
  defaultModel?: string;
  modelOptions?: string[];
  authRequired?: boolean;
  authMethods?: string[];
}

export interface CodingAgentLaunchContract {
  promptTransport: "stdin" | "positional" | "prompt_flag";
  outputMode: "text" | "json" | "stream-json";
  supportedOutputModes: Array<"text" | "json" | "stream-json">;
  supportedApprovalModes: string[];
  additionalDirectories: boolean;
  envOverrides: boolean;
}

export interface CodingAgentLifecycle {
  stage: "active" | "sunsetting" | "sunset";
  sunsetAt?: string;
  replacementRuntime?: string;
  message?: string;
}

export interface CodingAgentRuntimeOption {
  runtime: string;
  label: string;
  defaultProvider: string;
  compatibleProviders: string[];
  defaultModel: string;
  modelOptions: string[];
  available: boolean;
  diagnostics: CodingAgentDiagnostic[];
  supportedFeatures: string[];
  interactionCapabilities?: CodingAgentInteractionCapabilities;
  providers?: CodingAgentProvider[];
  launchContract?: CodingAgentLaunchContract;
  lifecycle?: CodingAgentLifecycle;
}

export interface CodingAgentCatalog {
  defaultRuntime: string;
  defaultSelection: CodingAgentSelection;
  runtimes: CodingAgentRuntimeOption[];
}

export interface BudgetGovernance {
  maxTaskBudgetUsd: number;
  maxDailySpendUsd: number;
  alertThresholdPercent: number;
  autoStopOnExceed: boolean;
}

export interface ReviewPolicy {
  autoTriggerOnPR: boolean;
  requiredLayers: string[];
  minRiskLevelForBlock: string;
  requireManualApproval: boolean;
  enabledPluginDimensions: string[];
}

export interface WebhookConfig {
  url: string;
  secret: string;
  events: string[];
  active: boolean;
}

export interface ProjectSettings {
  codingAgent: CodingAgentSelection;
  budgetGovernance?: BudgetGovernance;
  reviewPolicy?: ReviewPolicy;
  webhook?: WebhookConfig;
}

export interface Project {
  id: string;
  name: string;
  description: string;
  status: string;
  archivedAt?: string;
  archivedByUserId?: string;
  taskCount: number;
  agentCount: number;
  createdAt: string;
  repoUrl?: string;
  defaultBranch?: string;
  slug?: string;
  settings: ProjectSettings;
  codingAgentCatalog?: CodingAgentCatalog;
}

export interface ProjectUpdateInput {
  name?: string;
  description?: string;
  repoUrl?: string;
  defaultBranch?: string;
  settings?: ProjectSettings;
}

interface ProjectState {
  projects: Project[];
  currentProject: Project | null;
  loading: boolean;
  includeArchived: boolean;
  fetchProjects: (opts?: { includeArchived?: boolean }) => Promise<void>;
  setIncludeArchived: (value: boolean) => void;
  setCurrentProject: (id: string) => void;
  createProject: (
    data: {
      name: string;
      description: string;
      templateSource?: "system" | "user" | "marketplace";
      templateId?: string;
    },
  ) => Promise<Project | undefined>;
  updateProject: (id: string, data: ProjectUpdateInput) => Promise<Project | undefined>;
  deleteProject: (id: string) => Promise<void>;
  archiveProject: (id: string) => Promise<Project | undefined>;
  unarchiveProject: (id: string) => Promise<Project | undefined>;
}

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";

function toProjectSlug(name: string) {
  const normalized = name
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");

  return normalized || "project";
}

function normalizeSelection(raw: unknown): CodingAgentSelection {
  if (!raw || typeof raw !== "object") {
    return { runtime: "", provider: "", model: "" };
  }

  return {
    runtime: typeof (raw as { runtime?: unknown }).runtime === "string" ? (raw as { runtime: string }).runtime : "",
    provider:
      typeof (raw as { provider?: unknown }).provider === "string"
        ? (raw as { provider: string }).provider
        : "",
    model: typeof (raw as { model?: unknown }).model === "string" ? (raw as { model: string }).model : "",
  };
}

function normalizeDiagnostics(raw: unknown): CodingAgentDiagnostic[] {
  if (!Array.isArray(raw)) {
    return [];
  }

  return raw.map((diagnostic) => ({
    code: typeof diagnostic?.code === "string" ? diagnostic.code : "",
    message: typeof diagnostic?.message === "string" ? diagnostic.message : "",
    blocking: Boolean(diagnostic?.blocking),
  }));
}

function normalizeCapabilityDescriptor(
  raw: unknown,
): CodingAgentCapabilityDescriptor {
  return {
    state: typeof (raw as { state?: unknown })?.state === "string"
      ? (raw as { state: string }).state
      : "unsupported",
    reasonCode:
      typeof (raw as { reasonCode?: unknown })?.reasonCode === "string"
        ? (raw as { reasonCode: string }).reasonCode
        : undefined,
    message:
      typeof (raw as { message?: unknown })?.message === "string"
        ? (raw as { message: string }).message
        : undefined,
    requiresRequestFields: Array.isArray((raw as { requiresRequestFields?: unknown })?.requiresRequestFields)
      ? ((raw as { requiresRequestFields: unknown[] }).requiresRequestFields).map((item) =>
          String(item),
        )
      : undefined,
  };
}

function normalizeCapabilityGroup(
  raw: unknown,
): Record<string, CodingAgentCapabilityDescriptor> {
  if (!raw || typeof raw !== "object") {
    return {};
  }

  return Object.fromEntries(
    Object.entries(raw).map(([key, value]) => [
      key,
      normalizeCapabilityDescriptor(value),
    ]),
  );
}

function normalizeInteractionCapabilities(
  raw: unknown,
): CodingAgentInteractionCapabilities | undefined {
  if (!raw || typeof raw !== "object") {
    return undefined;
  }

  return {
    inputs: normalizeCapabilityGroup((raw as { inputs?: unknown }).inputs),
    lifecycle: normalizeCapabilityGroup((raw as { lifecycle?: unknown }).lifecycle),
    approval: normalizeCapabilityGroup((raw as { approval?: unknown }).approval),
    mcp: normalizeCapabilityGroup((raw as { mcp?: unknown }).mcp),
    diagnostics: normalizeCapabilityGroup((raw as { diagnostics?: unknown }).diagnostics),
  };
}

function normalizeRuntimeProviders(
  raw: unknown,
): CodingAgentProvider[] | undefined {
  if (!Array.isArray(raw)) {
    return undefined;
  }

  return raw.map((provider) => ({
    provider:
      typeof provider?.provider === "string" ? provider.provider : "",
    connected: Boolean(provider?.connected),
    defaultModel:
      typeof provider?.defaultModel === "string"
        ? provider.defaultModel
        : undefined,
    modelOptions: Array.isArray(provider?.modelOptions)
      ? provider.modelOptions.map((item: unknown) => String(item))
      : undefined,
    authRequired:
      typeof provider?.authRequired === "boolean"
        ? provider.authRequired
        : undefined,
    authMethods: Array.isArray(provider?.authMethods)
      ? provider.authMethods.map((item: unknown) => String(item))
      : undefined,
  }));
}

function normalizeCatalog(raw: unknown): CodingAgentCatalog | undefined {
  if (!raw || typeof raw !== "object") {
    return undefined;
  }

  const catalog = raw as {
    defaultRuntime?: unknown;
    defaultSelection?: unknown;
    runtimes?: unknown;
  };

  return {
    defaultRuntime:
      typeof catalog.defaultRuntime === "string" ? catalog.defaultRuntime : "",
    defaultSelection: normalizeSelection(catalog.defaultSelection),
    runtimes: Array.isArray(catalog.runtimes)
      ? catalog.runtimes.map((runtime) => ({
          runtime: typeof runtime?.runtime === "string" ? runtime.runtime : "",
          label: typeof runtime?.label === "string" ? runtime.label : "",
          defaultProvider:
            typeof runtime?.defaultProvider === "string"
              ? runtime.defaultProvider
              : "",
          compatibleProviders: Array.isArray(runtime?.compatibleProviders)
            ? runtime.compatibleProviders.map((item: unknown) => String(item))
            : [],
          defaultModel:
            typeof runtime?.defaultModel === "string" ? runtime.defaultModel : "",
          modelOptions: Array.isArray(runtime?.modelOptions)
            ? runtime.modelOptions.map((item: unknown) => String(item))
            : [],
          available: Boolean(runtime?.available),
          diagnostics: normalizeDiagnostics(runtime?.diagnostics),
          supportedFeatures: Array.isArray(runtime?.supportedFeatures)
            ? runtime.supportedFeatures.map((item: unknown) => String(item))
            : [],
          interactionCapabilities: normalizeInteractionCapabilities(
            runtime?.interactionCapabilities,
          ),
          providers: normalizeRuntimeProviders(runtime?.providers),
          launchContract:
            runtime?.launchContract && typeof runtime.launchContract === "object"
              ? {
                  promptTransport:
                    runtime.launchContract.promptTransport === "stdin" ||
                    runtime.launchContract.promptTransport === "positional" ||
                    runtime.launchContract.promptTransport === "prompt_flag"
                      ? runtime.launchContract.promptTransport
                      : "stdin",
                  outputMode:
                    runtime.launchContract.outputMode === "text" ||
                    runtime.launchContract.outputMode === "json" ||
                    runtime.launchContract.outputMode === "stream-json"
                      ? runtime.launchContract.outputMode
                      : "text",
                  supportedOutputModes: Array.isArray(runtime.launchContract.supportedOutputModes)
                    ? runtime.launchContract.supportedOutputModes.map((item: unknown) => String(item)) as Array<"text" | "json" | "stream-json">
                    : [],
                  supportedApprovalModes: Array.isArray(runtime.launchContract.supportedApprovalModes)
                    ? runtime.launchContract.supportedApprovalModes.map((item: unknown) => String(item))
                    : [],
                  additionalDirectories: Boolean(runtime.launchContract.additionalDirectories),
                  envOverrides: Boolean(runtime.launchContract.envOverrides),
                }
              : undefined,
          lifecycle:
            runtime?.lifecycle && typeof runtime.lifecycle === "object"
              ? {
                  stage:
                    runtime.lifecycle.stage === "sunsetting" ||
                    runtime.lifecycle.stage === "sunset"
                      ? runtime.lifecycle.stage
                      : "active",
                  sunsetAt:
                    typeof runtime.lifecycle.sunsetAt === "string"
                      ? runtime.lifecycle.sunsetAt
                      : undefined,
                  replacementRuntime:
                    typeof runtime.lifecycle.replacementRuntime === "string"
                      ? runtime.lifecycle.replacementRuntime
                      : undefined,
                  message:
                    typeof runtime.lifecycle.message === "string"
                      ? runtime.lifecycle.message
                      : undefined,
                }
              : undefined,
        }))
      : [],
  };
}

function normalizeBudgetGovernance(raw: unknown): BudgetGovernance | undefined {
  if (!raw || typeof raw !== "object") return undefined;
  const bg = raw as Record<string, unknown>;
  return {
    maxTaskBudgetUsd: typeof bg.maxTaskBudgetUsd === "number" ? bg.maxTaskBudgetUsd : 0,
    maxDailySpendUsd: typeof bg.maxDailySpendUsd === "number" ? bg.maxDailySpendUsd : 0,
    alertThresholdPercent: typeof bg.alertThresholdPercent === "number" ? bg.alertThresholdPercent : 80,
    autoStopOnExceed: Boolean(bg.autoStopOnExceed),
  };
}

function normalizeReviewPolicy(raw: unknown): ReviewPolicy | undefined {
  if (!raw || typeof raw !== "object") return undefined;
  const rp = raw as Record<string, unknown>;
  return {
    autoTriggerOnPR: Boolean(rp.autoTriggerOnPR),
    requiredLayers: Array.isArray(rp.requiredLayers)
      ? rp.requiredLayers.map((item: unknown) => String(item))
      : [],
    minRiskLevelForBlock: typeof rp.minRiskLevelForBlock === "string" ? rp.minRiskLevelForBlock : "",
    requireManualApproval: Boolean(rp.requireManualApproval),
    enabledPluginDimensions: Array.isArray(rp.enabledPluginDimensions)
      ? rp.enabledPluginDimensions.map((item: unknown) => String(item))
      : [],
  };
}

function normalizeWebhookConfig(raw: unknown): WebhookConfig | undefined {
  if (!raw || typeof raw !== "object") return undefined;
  const wh = raw as Record<string, unknown>;
  return {
    url: typeof wh.url === "string" ? wh.url : "",
    secret: typeof wh.secret === "string" ? wh.secret : "",
    events: Array.isArray(wh.events) ? wh.events.map((item: unknown) => String(item)) : [],
    active: Boolean(wh.active),
  };
}

function normalizeSettings(raw: unknown): ProjectSettings {
  if (!raw || typeof raw !== "object") {
    return { codingAgent: { runtime: "", provider: "", model: "" } };
  }
  const s = raw as Record<string, unknown>;
  return {
    codingAgent: normalizeSelection(s.codingAgent),
    budgetGovernance: normalizeBudgetGovernance(s.budgetGovernance),
    reviewPolicy: normalizeReviewPolicy(s.reviewPolicy),
    webhook: normalizeWebhookConfig(s.webhook),
  };
}

function normalizeProject(raw: Record<string, unknown>): Project {
  return {
    id: String(raw.id ?? ""),
    name: String(raw.name ?? ""),
    description: String(raw.description ?? ""),
    status: String(raw.status ?? "active"),
    archivedAt: typeof raw.archivedAt === "string" ? raw.archivedAt : undefined,
    archivedByUserId:
      typeof raw.archivedByUserId === "string" ? raw.archivedByUserId : undefined,
    taskCount: Number(raw.taskCount ?? 0),
    agentCount: Number(raw.agentCount ?? 0),
    createdAt:
      typeof raw.createdAt === "string" ? raw.createdAt : new Date().toISOString(),
    repoUrl: typeof raw.repoUrl === "string" ? raw.repoUrl : "",
    defaultBranch: typeof raw.defaultBranch === "string" ? raw.defaultBranch : "main",
    slug: typeof raw.slug === "string" ? raw.slug : "",
    settings: normalizeSettings(raw.settings),
    codingAgentCatalog: normalizeCatalog(raw.codingAgentCatalog),
  };
}

export const useProjectStore = create<ProjectState>()((set, get) => ({
  projects: [],
  currentProject: null,
  loading: false,
  includeArchived: false,

  fetchProjects: async (opts) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return;
    const includeArchived = opts?.includeArchived ?? get().includeArchived;
    set({ loading: true, includeArchived });
    try {
      const api = createApiClient(API_URL);
      const path = includeArchived
        ? "/api/v1/projects?includeArchived=true"
        : "/api/v1/projects";
      const { data } = await api.get<Record<string, unknown>[]>(path, {
        token,
      });
      const projects = data.map(normalizeProject);
      set((state) => ({
        projects,
        currentProject:
          state.currentProject == null
            ? state.currentProject
            : projects.find((project) => project.id === state.currentProject?.id) ?? null,
      }));
    } finally {
      set({ loading: false });
    }
  },

  setIncludeArchived: (value) => {
    set({ includeArchived: value });
  },

  setCurrentProject: (id) => {
    const project = get().projects.find((p) => p.id === id) ?? null;
    set({ currentProject: project });
  },

  createProject: async (input) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return;
    try {
      const api = createApiClient(API_URL);
      const { data: createdProjectRaw } = await api.post<Record<string, unknown>>(
        "/api/v1/projects",
        {
          ...input,
          slug: toProjectSlug(input.name),
        },
        { token }
      );
      const project = normalizeProject(createdProjectRaw);
      set((s) => ({ projects: [...s.projects, project] }));
      return project;
    } catch (error) {
      toast.error("Failed to create project", {
        description: error instanceof Error ? error.message : "Unknown error",
      });
      throw error;
    }
  },

  updateProject: async (id, data) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return;
    try {
      const api = createApiClient(API_URL);
      const { data: updatedRaw } = await api.put<Record<string, unknown>>(
        `/api/v1/projects/${id}`,
        data,
        { token }
      );
      const updated = normalizeProject(updatedRaw);
      set((s) => ({
        projects: s.projects.map((p) => (p.id === id ? updated : p)),
        currentProject: s.currentProject?.id === id ? updated : s.currentProject,
      }));
      return updated;
    } catch (error) {
      toast.error("Failed to update project", {
        description: error instanceof Error ? error.message : "Unknown error",
      });
      throw error;
    }
  },

  deleteProject: async (id) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return;
    try {
      const api = createApiClient(API_URL);
      await api.delete(`/api/v1/projects/${id}`, { token });
      set((s) => ({
        projects: s.projects.filter((p) => p.id !== id),
        currentProject: s.currentProject?.id === id ? null : s.currentProject,
      }));
    } catch (error) {
      toast.error("Failed to delete project", {
        description: error instanceof Error ? error.message : "Unknown error",
      });
      throw error;
    }
  },

  archiveProject: async (id) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return;
    try {
      const api = createApiClient(API_URL);
      const { data } = await api.post<Record<string, unknown>>(
        `/api/v1/projects/${id}/archive`,
        {},
        { token },
      );
      const updated = normalizeProject(data);
      set((s) => ({
        projects: s.projects.map((p) => (p.id === id ? updated : p)),
        currentProject: s.currentProject?.id === id ? updated : s.currentProject,
      }));
      return updated;
    } catch (error) {
      toast.error("Failed to archive project", {
        description: error instanceof Error ? error.message : "Unknown error",
      });
      throw error;
    }
  },

  unarchiveProject: async (id) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return;
    try {
      const api = createApiClient(API_URL);
      const { data } = await api.post<Record<string, unknown>>(
        `/api/v1/projects/${id}/unarchive`,
        {},
        { token },
      );
      const updated = normalizeProject(data);
      set((s) => ({
        projects: s.projects.map((p) => (p.id === id ? updated : p)),
        currentProject: s.currentProject?.id === id ? updated : s.currentProject,
      }));
      return updated;
    } catch (error) {
      toast.error("Failed to unarchive project", {
        description: error instanceof Error ? error.message : "Unknown error",
      });
      throw error;
    }
  },
}));
