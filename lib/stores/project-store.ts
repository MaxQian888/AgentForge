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
  fetchProjects: () => Promise<void>;
  setCurrentProject: (id: string) => void;
  createProject: (data: { name: string; description: string }) => Promise<void>;
  updateProject: (id: string, data: ProjectUpdateInput) => Promise<Project | undefined>;
  deleteProject: (id: string) => Promise<void>;
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

  fetchProjects: async () => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return;
    set({ loading: true });
    try {
      const api = createApiClient(API_URL);
      const { data } = await api.get<Record<string, unknown>[]>("/api/v1/projects", {
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
}));
