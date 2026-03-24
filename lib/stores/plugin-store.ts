"use client";

import { create } from "zustand";
import { createApiClient } from "@/lib/api-client";
import { useAuthStore } from "./auth-store";

/* ── Types matching Go model/plugin.go ── */

export type PluginKind =
  | "RolePlugin"
  | "ToolPlugin"
  | "WorkflowPlugin"
  | "IntegrationPlugin"
  | "ReviewPlugin";

export type PluginRuntime = "declarative" | "mcp" | "go-plugin" | "wasm";

export type PluginSourceType = "builtin" | "local" | "git" | "npm" | "catalog";
export type PluginTrustState = "unknown" | "verified" | "untrusted";
export type PluginApprovalState = "not-required" | "pending" | "approved" | "rejected";

export type PluginLifecycleState =
  | "installed"
  | "enabled"
  | "activating"
  | "active"
  | "degraded"
  | "disabled";

export type PluginRuntimeHost = "go-orchestrator" | "ts-bridge";

export type PluginPanelSourceType = PluginSourceType | "marketplace" | "all";

export interface PluginPanelFilters {
  query: string;
  kind: PluginKind | "all";
  lifecycleState: PluginLifecycleState | "all";
  runtimeHost: PluginRuntimeHost | "all";
  sourceType: PluginPanelSourceType;
}

export interface PluginMetadata {
  id: string;
  name: string;
  version: string;
  description?: string;
  tags?: string[];
}

export interface PluginSpec {
  runtime: PluginRuntime;
  transport?: string;
  command?: string;
  args?: string[];
  url?: string;
  binary?: string;
  module?: string;
  abiVersion?: string;
  capabilities?: string[];
  config?: Record<string, unknown>;
  workflow?: {
    process: "sequential" | "hierarchical" | "event-driven";
    roles?: Array<{ id: string }>;
    steps: Array<{
      id: string;
      role: string;
      action: "agent" | "review" | "task";
      next?: string[];
    }>;
    triggers?: Array<{ event?: string }>;
    limits?: { maxRetries?: number };
  };
  review?: {
    entrypoint?: string;
    triggers: {
      events: string[];
      filePatterns?: string[];
    };
    output: {
      format: string;
    };
  };
  extra?: Record<string, unknown>;
}

export interface PluginPermissions {
  network?: { required: boolean; domains?: string[] };
  filesystem?: { required: boolean; allowed_paths?: string[] };
}

export interface PluginSource {
  type: PluginSourceType;
  path?: string;
  repository?: string;
  ref?: string;
  package?: string;
  version?: string;
  registry?: string;
  catalog?: string;
  entry?: string;
  digest?: string;
  signature?: string;
  trust?: {
    status: PluginTrustState;
    approvalState?: PluginApprovalState;
    source?: string;
    verifiedAt?: string;
    approvedBy?: string;
    approvedAt?: string;
    reason?: string;
  };
  release?: {
    version?: string;
    channel?: string;
    artifact?: string;
    notesUrl?: string;
    publishedAt?: string;
    availableVersion?: string;
  };
}

export interface PluginRuntimeMetadata {
  abi_version?: string;
  compatible: boolean;
}

export interface PluginRecord {
  apiVersion: string;
  kind: PluginKind;
  metadata: PluginMetadata;
  spec: PluginSpec;
  permissions: PluginPermissions;
  source: PluginSource;
  lifecycle_state: PluginLifecycleState;
  runtime_host?: PluginRuntimeHost;
  last_health_at?: string;
  last_error?: string;
  restart_count: number;
  resolved_source_path?: string;
  runtime_metadata?: PluginRuntimeMetadata;
}

export interface MarketplacePluginEntry {
  id: string;
  name: string;
  description: string;
  version: string;
  author: string;
  kind: string;
  installUrl?: string;
  sourceType?: PluginSourceType;
  runtime?: PluginRuntime;
  trustStatus?: PluginTrustState;
  approvalState?: PluginApprovalState;
  release?: PluginSource["release"];
}

export const DEFAULT_PLUGIN_PANEL_FILTERS: PluginPanelFilters = {
  query: "",
  kind: "all",
  lifecycleState: "all",
  runtimeHost: "all",
  sourceType: "all",
};

function normalizeSearchInput(value: string): string {
  return value.trim().toLowerCase();
}

export function filterPluginRecords(
  plugins: PluginRecord[],
  filters: PluginPanelFilters
): PluginRecord[] {
  const query = normalizeSearchInput(filters.query);

  return plugins.filter((plugin) => {
    if (filters.kind !== "all" && plugin.kind !== filters.kind) {
      return false;
    }
    if (
      filters.lifecycleState !== "all" &&
      plugin.lifecycle_state !== filters.lifecycleState
    ) {
      return false;
    }
    if (
      filters.runtimeHost !== "all" &&
      plugin.runtime_host !== filters.runtimeHost
    ) {
      return false;
    }
    if (
      filters.sourceType !== "all" &&
      plugin.source.type !== filters.sourceType
    ) {
      return false;
    }
    if (!query) {
      return true;
    }

    const haystack = [
      plugin.metadata.id,
      plugin.metadata.name,
      plugin.metadata.description,
      plugin.kind,
      plugin.runtime_host,
      plugin.source.type,
      ...(plugin.metadata.tags ?? []),
    ]
      .filter(Boolean)
      .join(" ")
      .toLowerCase();

    return haystack.includes(query);
  });
}

export function filterMarketplaceEntries(
  entries: MarketplacePluginEntry[],
  filters: PluginPanelFilters
): MarketplacePluginEntry[] {
  const query = normalizeSearchInput(filters.query);
  const normalizedKind =
    filters.kind === "all"
      ? "all"
      : filters.kind.replace("Plugin", "").toLowerCase().replace("integration", "integration");

  return entries.filter((entry) => {
    if (normalizedKind !== "all" && entry.kind.toLowerCase() !== normalizedKind) {
      return false;
    }
    if (filters.sourceType !== "all" && filters.sourceType !== "marketplace") {
      return false;
    }
    if (!query) {
      return true;
    }

    const haystack = [
      entry.id,
      entry.name,
      entry.description,
      entry.author,
      entry.kind,
    ]
      .filter(Boolean)
      .join(" ")
      .toLowerCase();

    return haystack.includes(query);
  });
}

/* ── Store ── */

interface PluginState {
  plugins: PluginRecord[];
  builtins: PluginRecord[];
  marketplace: MarketplacePluginEntry[];
  filters: PluginPanelFilters;
  selectedPluginId: string | null;
  loading: boolean;
  error: string | null;

  fetchPlugins: () => Promise<void>;
  discoverBuiltins: () => Promise<void>;
  fetchMarketplace: () => Promise<void>;
  installLocal: (path: string) => Promise<void>;
  enablePlugin: (id: string) => Promise<void>;
  disablePlugin: (id: string) => Promise<void>;
  activatePlugin: (id: string) => Promise<void>;
  uninstallPlugin: (id: string) => Promise<void>;
  updateConfig: (id: string, config: Record<string, unknown>) => Promise<void>;
  checkHealth: (id: string) => Promise<void>;
  restartPlugin: (id: string) => Promise<void>;
  setFilters: (next: Partial<PluginPanelFilters>) => void;
  resetFilters: () => void;
  selectPlugin: (id: string | null) => void;
}

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";

function getApi() {
  return createApiClient(API_URL);
}

function getToken() {
  return useAuthStore.getState().accessToken;
}

export const usePluginStore = create<PluginState>()((set, get) => ({
  plugins: [],
  builtins: [],
  marketplace: [],
  filters: DEFAULT_PLUGIN_PANEL_FILTERS,
  selectedPluginId: null,
  loading: false,
  error: null,

  fetchPlugins: async () => {
    const token = getToken();
    if (!token) return;

    set({ loading: true, error: null });
    try {
      const api = getApi();
      const { data } = await api.get<PluginRecord[]>("/api/v1/plugins", {
        token,
      });
      set({ plugins: data ?? [], error: null });
    } catch {
      set({ error: "Unable to load plugins" });
    } finally {
      set({ loading: false });
    }
  },

  discoverBuiltins: async () => {
    const token = getToken();
    if (!token) return;

    set({ loading: true, error: null });
    try {
      const api = getApi();
      const { data } = await api.get<PluginRecord[]>(
        "/api/v1/plugins/discover",
        { token }
      );
      set({ builtins: data ?? [], error: null });
    } catch {
      set({ error: "Unable to discover built-in plugins" });
    } finally {
      set({ loading: false });
    }
  },

  fetchMarketplace: async () => {
    const token = getToken();
    if (!token) return;

    set({ loading: true, error: null });
    try {
      const api = getApi();
      const { data } = await api.get<MarketplacePluginEntry[]>(
        "/api/v1/plugins/marketplace",
        { token }
      );
      set({ marketplace: data ?? [], error: null });
    } catch {
      set({ error: "Unable to load plugin marketplace" });
    } finally {
      set({ loading: false });
    }
  },

  installLocal: async (path) => {
    const token = getToken();
    if (!token) return;

    set({ loading: true, error: null });
    try {
      const api = getApi();
      await api.post("/api/v1/plugins/install", { path }, { token });
      await get().fetchPlugins();
    } catch {
      set({ error: "Failed to install plugin" });
    } finally {
      set({ loading: false });
    }
  },

  enablePlugin: async (id) => {
    const token = getToken();
    if (!token) return;

    set({ error: null });
    try {
      const api = getApi();
      await api.put(`/api/v1/plugins/${id}/enable`, {}, { token });
      await get().fetchPlugins();
    } catch {
      set({ error: "Failed to enable plugin" });
    }
  },

  disablePlugin: async (id) => {
    const token = getToken();
    if (!token) return;

    set({ error: null });
    try {
      const api = getApi();
      await api.put(`/api/v1/plugins/${id}/disable`, {}, { token });
      await get().fetchPlugins();
    } catch {
      set({ error: "Failed to disable plugin" });
    }
  },

  activatePlugin: async (id) => {
    const token = getToken();
    if (!token) return;

    set({ error: null });
    try {
      const api = getApi();
      await api.post(`/api/v1/plugins/${id}/activate`, {}, { token });
      await get().fetchPlugins();
    } catch {
      set({ error: "Failed to activate plugin" });
    }
  },

  uninstallPlugin: async (id) => {
    const token = getToken();
    if (!token) return;

    set({ error: null });
    try {
      const api = getApi();
      await api.delete(`/api/v1/plugins/${id}`, { token });
      await get().fetchPlugins();
    } catch {
      set({ error: "Failed to uninstall plugin" });
    }
  },

  updateConfig: async (id, config) => {
    const token = getToken();
    if (!token) return;

    set({ error: null });
    try {
      const api = getApi();
      await api.put(`/api/v1/plugins/${id}/config`, { config }, { token });
      await get().fetchPlugins();
    } catch {
      set({ error: "Failed to update plugin config" });
    }
  },

  checkHealth: async (id) => {
    const token = getToken();
    if (!token) return;

    set({ error: null });
    try {
      const api = getApi();
      await api.get(`/api/v1/plugins/${id}/health`, { token });
      await get().fetchPlugins();
    } catch {
      set({ error: "Health check failed" });
    }
  },

  restartPlugin: async (id) => {
    const token = getToken();
    if (!token) return;

    set({ error: null });
    try {
      const api = getApi();
      await api.post(`/api/v1/plugins/${id}/restart`, {}, { token });
      await get().fetchPlugins();
    } catch {
      set({ error: "Failed to restart plugin" });
    }
  },

  setFilters: (next) =>
    set((state) => ({
      filters: {
        ...state.filters,
        ...next,
      },
    })),

  resetFilters: () => set({ filters: DEFAULT_PLUGIN_PANEL_FILTERS }),

  selectPlugin: (id) => set({ selectedPluginId: id }),
}));
