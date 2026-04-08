"use client";

import { create } from "zustand";
import { ApiError, createApiClient } from "@/lib/api-client";
import { useAuthStore } from "./auth-store";

/* ── Types matching Go model/plugin.go ── */

export type PluginKind =
  | "RolePlugin"
  | "ToolPlugin"
  | "WorkflowPlugin"
  | "IntegrationPlugin"
  | "ReviewPlugin";

export type PluginRuntime = "declarative" | "mcp" | "go-plugin" | "wasm";

export type PluginSourceType =
  | "builtin"
  | "local"
  | "git"
  | "npm"
  | "catalog"
  | "marketplace"
  | "registry";
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

export interface PluginBuiltInMetadata {
  official?: boolean;
  docsRef?: string;
  verificationProfile?: string;
  availabilityStatus?: string;
  availabilityMessage?: string;
  readinessStatus?: string;
  readinessMessage?: string;
  nextStep?: string;
  blockingReasons?: string[];
  missingPrerequisites?: string[];
  missingConfiguration?: string[];
  installable?: boolean;
  installBlockedReason?: string;
}

export interface PluginRoleDependency {
  roleId: string;
  roleName?: string;
  status: string;
  blocking: boolean;
  message?: string;
  references?: string[];
}

export interface PluginRoleConsumer {
  roleId: string;
  roleName?: string;
  referenceType: string;
  status: string;
  blocking: boolean;
  message?: string;
}

/* ── MCP types ── */

export interface MCPInteractionSummary {
  operation: string;
  status: string;
  at?: string;
  target?: string;
  summary?: string;
  error_code?: string;
  error_message?: string;
}

export interface PluginMCPRuntimeMetadata {
  transport: string;
  last_discovery_at?: string;
  tool_count: number;
  resource_count: number;
  prompt_count: number;
  latest_interaction?: MCPInteractionSummary;
}

export interface MCPCapabilityTool {
  name: string;
  description?: string;
}

export interface MCPCapabilityResource {
  uri: string;
  name?: string;
}

export interface MCPCapabilityPrompt {
  name: string;
  description?: string;
}

export interface PluginMCPCapabilitySnapshot {
  transport: string;
  last_discovery_at?: string;
  tool_count: number;
  resource_count: number;
  prompt_count: number;
  tools?: MCPCapabilityTool[];
  resources?: MCPCapabilityResource[];
  prompts?: MCPCapabilityPrompt[];
  latest_interaction?: MCPInteractionSummary;
}

export interface PluginMCPRefreshResult {
  plugin_id: string;
  lifecycle_state?: string;
  runtime_host?: string;
  runtime_metadata?: PluginRuntimeMetadata;
  snapshot: PluginMCPCapabilitySnapshot;
}

export interface MCPToolCallResult {
  content?: Array<{
    type?: string;
    text?: string;
    mimeType?: string;
    uri?: string;
  }>;
  isError: boolean;
  structuredContent?: Record<string, unknown>;
}

export interface MCPResourceReadResult {
  contents?: Array<{ uri?: string; mimeType?: string; text?: string }>;
}

export interface MCPPromptGetResult {
  description?: string;
  messages?: Array<{
    role?: string;
    content: { type?: string; text?: string };
  }>;
}

/* ── Event types ── */

export type PluginEventType =
  | "installed"
  | "enabled"
  | "disabled"
  | "deactivated"
  | "activating"
  | "activated"
  | "updated"
  | "mcp_discovery"
  | "mcp_interaction"
  | "runtime_sync"
  | "health"
  | "restarted"
  | "invoked"
  | "uninstalled"
  | "failed";

export type PluginEventSource =
  | "control-plane"
  | "ts-bridge"
  | "go-runtime"
  | "operator";

export interface PluginEventRecord {
  id: string;
  plugin_id: string;
  event_type: PluginEventType;
  event_source: PluginEventSource;
  lifecycle_state?: string;
  summary?: string;
  payload?: Record<string, unknown>;
  created_at?: string;
}

/* ── Workflow run types ── */

export type WorkflowRunStatus =
  | "pending"
  | "running"
  | "completed"
  | "failed"
  | "cancelled";

export type WorkflowStepRunStatus =
  | "pending"
  | "running"
  | "completed"
  | "failed"
  | "skipped";

export interface WorkflowStepAttempt {
  attempt: number;
  status: WorkflowStepRunStatus;
  output?: Record<string, unknown>;
  error?: string;
  started_at: string;
  completed_at?: string;
}

export interface WorkflowStepRun {
  step_id: string;
  role_id: string;
  action: string;
  status: WorkflowStepRunStatus;
  input?: Record<string, unknown>;
  output?: Record<string, unknown>;
  retry_count: number;
  error?: string;
  attempts?: WorkflowStepAttempt[];
  started_at?: string;
  completed_at?: string;
}

export interface WorkflowPluginRun {
  id: string;
  plugin_id: string;
  process: string;
  status: WorkflowRunStatus;
  trigger?: Record<string, unknown>;
  current_step_id?: string;
  steps?: WorkflowStepRun[];
  error?: string;
  started_at: string;
  completed_at?: string;
}

/* ── Runtime metadata ── */

export interface PluginRuntimeMetadata {
  abi_version?: string;
  compatible: boolean;
  mcp?: PluginMCPRuntimeMetadata;
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
  builtIn?: PluginBuiltInMetadata;
  roleDependencies?: PluginRoleDependency[];
  roleConsumers?: PluginRoleConsumer[];
}

export interface MarketplacePluginEntry {
  id: string;
  name: string;
  description: string;
  version: string;
  author: string;
  kind: string;
  installUrl?: string;
  installed?: boolean;
  sourceType?: PluginSourceType | "marketplace";
  registry?: string;
  runtime?: PluginRuntime;
  installable?: boolean;
  blockedReason?: string;
  trustStatus?: PluginTrustState;
  approvalState?: PluginApprovalState;
  release?: PluginSource["release"];
  builtIn?: PluginBuiltInMetadata;
}

export interface RemoteMarketplaceState {
  available: boolean;
  registry: string;
  error?: string;
  errorCode?: string;
  entries: MarketplacePluginEntry[];
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
    if (filters.sourceType !== "all") {
      if (filters.sourceType === "marketplace") {
        if (entry.sourceType !== "marketplace" && entry.sourceType !== "registry") {
          return false;
        }
      } else if (entry.sourceType !== filters.sourceType) {
        return false;
      }
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
  remoteMarketplace: RemoteMarketplaceState;
  catalogResults: MarketplacePluginEntry[];
  catalogQuery: string;
  events: Record<string, PluginEventRecord[]>;
  mcpSnapshots: Record<string, PluginMCPCapabilitySnapshot>;
  workflowRuns: Record<string, WorkflowPluginRun[]>;
  selectedWorkflowRunId: string | null;
  viewCategory: "installed" | "builtin" | "marketplace" | "remote";
  filters: PluginPanelFilters;
  selectedPluginId: string | null;
  selectedMarketplaceId: string | null;
  loading: boolean;
  error: string | null;

  setViewCategory: (category: "installed" | "builtin" | "marketplace" | "remote") => void;
  selectMarketplaceEntry: (id: string | null) => void;
  fetchPlugins: () => Promise<void>;
  discoverBuiltins: () => Promise<void>;
  fetchMarketplace: () => Promise<void>;
  fetchRemoteMarketplace: () => Promise<void>;
  installLocal: (path: string) => Promise<void>;
  enablePlugin: (id: string) => Promise<void>;
  disablePlugin: (id: string) => Promise<void>;
  activatePlugin: (id: string) => Promise<void>;
  deactivatePlugin: (id: string) => Promise<void>;
  uninstallPlugin: (id: string) => Promise<void>;
  updatePlugin: (plugin: PluginRecord) => Promise<void>;
  updateConfig: (id: string, config: Record<string, unknown>) => Promise<void>;
  checkHealth: (id: string) => Promise<void>;
  restartPlugin: (id: string) => Promise<void>;
  invokePlugin: (
    id: string,
    operation: string,
    payload?: Record<string, unknown>,
  ) => Promise<Record<string, unknown> | null>;
  searchCatalog: (query: string) => Promise<void>;
  installFromCatalog: (entryId: string) => Promise<void>;
  installFromRemote: (entryId: string, version?: string) => Promise<void>;
  refreshMCP: (id: string) => Promise<PluginMCPRefreshResult | null>;
  callMCPTool: (
    id: string,
    toolName: string,
    args?: Record<string, unknown>,
  ) => Promise<MCPToolCallResult | null>;
  readMCPResource: (
    id: string,
    uri: string,
  ) => Promise<MCPResourceReadResult | null>;
  getMCPPrompt: (
    id: string,
    name: string,
    args?: Record<string, string>,
  ) => Promise<MCPPromptGetResult | null>;
  fetchEvents: (id: string, limit?: number) => Promise<void>;
  startWorkflowRun: (
    id: string,
    trigger?: Record<string, unknown>,
  ) => Promise<void>;
  fetchWorkflowRuns: (id: string) => Promise<void>;
  fetchWorkflowRun: (runId: string) => Promise<void>;
  setCatalogQuery: (query: string) => void;
  setFilters: (next: Partial<PluginPanelFilters>) => void;
  resetFilters: () => void;
  selectPlugin: (id: string | null) => void;
  selectWorkflowRun: (id: string | null) => void;
}

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";

function getApi() {
  return createApiClient(API_URL);
}

function getToken() {
  return useAuthStore.getState().accessToken;
}

function getErrorMessage(error: unknown, fallback: string): string {
  if (error instanceof ApiError) {
    const body = error.body as { message?: string } | null;
    return body?.message ?? error.message;
  }
  if (error instanceof Error && error.message) {
    return error.message;
  }
  return fallback;
}

export const usePluginStore = create<PluginState>()((set, get) => ({
  plugins: [],
  builtins: [],
  marketplace: [],
  remoteMarketplace: {
    available: false,
    registry: "",
    entries: [],
  },
  catalogResults: [],
  catalogQuery: "",
  events: {},
  mcpSnapshots: {},
  workflowRuns: {},
  selectedWorkflowRunId: null,
  viewCategory: "installed" as const,
  filters: DEFAULT_PLUGIN_PANEL_FILTERS,
  selectedPluginId: null,
  selectedMarketplaceId: null,
  loading: false,
  error: null,

  setViewCategory: (category) => set({ viewCategory: category }),
  selectMarketplaceEntry: (id) => set({ selectedMarketplaceId: id }),
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

  fetchRemoteMarketplace: async () => {
    const token = getToken();
    if (!token) return;

    set({ loading: true, error: null });
    try {
      const api = getApi();
      const { data } = await api.get<RemoteMarketplaceState>(
        "/api/v1/plugins/marketplace/remote",
        { token },
      );
      set({
        remoteMarketplace: {
          available: data?.available ?? false,
          registry: data?.registry ?? "",
          error: data?.error,
          errorCode: data?.errorCode,
          entries: data?.entries ?? [],
        },
        error: null,
      });
    } catch (error) {
      set({
        remoteMarketplace: {
          available: false,
          registry: "",
          error: getErrorMessage(error, "Unable to load remote plugin registry"),
          entries: [],
        },
        error: "Unable to load remote plugin registry",
      });
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

  deactivatePlugin: async (id) => {
    const token = getToken();
    if (!token) return;

    set({ error: null });
    try {
      const api = getApi();
      await api.post(`/api/v1/plugins/${id}/deactivate`, {}, { token });
      await get().fetchPlugins();
    } catch {
      set({ error: "Failed to deactivate plugin" });
    }
  },

  updatePlugin: async (plugin) => {
    const token = getToken();
    if (!token) return;

    const path = plugin.source.path ?? plugin.resolved_source_path;
    if (!path) {
      set({ error: "No supported update source is available for this plugin" });
      return;
    }

    set({ error: null });
    try {
      const api = getApi();
      await api.post(
        `/api/v1/plugins/${plugin.metadata.id}/update`,
        { path, source: plugin.source },
        { token },
      );
      await get().fetchPlugins();
    } catch {
      set({ error: "Failed to update plugin" });
    }
  },

  invokePlugin: async (id, operation, payload) => {
    const token = getToken();
    if (!token) return null;

    set({ error: null });
    try {
      const api = getApi();
      const { data } = await api.post<Record<string, unknown>>(
        `/api/v1/plugins/${id}/invoke`,
        { operation, payload: payload ?? {} },
        { token },
      );
      return data ?? null;
    } catch {
      set({ error: "Failed to invoke plugin" });
      return null;
    }
  },

  searchCatalog: async (query) => {
    const token = getToken();
    if (!token) return;

    set({ catalogQuery: query, error: null });
    try {
      const api = getApi();
      const { data } = await api.get<MarketplacePluginEntry[]>(
        `/api/v1/plugins/catalog?q=${encodeURIComponent(query)}`,
        { token },
      );
      set({ catalogResults: data ?? [] });
    } catch {
      set({ error: "Failed to search catalog" });
    }
  },

  installFromCatalog: async (entryId) => {
    const token = getToken();
    if (!token) return;

    set({ loading: true, error: null });
    try {
      const api = getApi();
      await api.post(
        "/api/v1/plugins/catalog/install",
        { entry_id: entryId },
        { token },
      );
      await get().fetchPlugins();
    } catch {
      set({ error: "Failed to install from catalog" });
    } finally {
      set({ loading: false });
    }
  },

  installFromRemote: async (entryId, version = "latest") => {
    const token = getToken();
    if (!token) return;

    set({ loading: true, error: null });
    try {
      const api = getApi();
      await api.post(
        `/api/v1/plugins/marketplace/${entryId}/install-remote`,
        { version },
        { token },
      );
      await get().fetchPlugins();
      await get().fetchRemoteMarketplace();
    } catch (error) {
      set({ error: getErrorMessage(error, "Failed to install from remote registry") });
    } finally {
      set({ loading: false });
    }
  },

  refreshMCP: async (id) => {
    const token = getToken();
    if (!token) return null;

    set({ error: null });
    try {
      const api = getApi();
      const { data } = await api.post<PluginMCPRefreshResult>(
        `/api/v1/plugins/${id}/mcp/refresh`,
        {},
        { token },
      );
      if (data?.snapshot) {
        set((state) => ({
          mcpSnapshots: { ...state.mcpSnapshots, [id]: data.snapshot },
        }));
      }
      await get().fetchPlugins();
      return data ?? null;
    } catch {
      set({ error: "MCP refresh failed" });
      return null;
    }
  },

  callMCPTool: async (id, toolName, args) => {
    const token = getToken();
    if (!token) return null;

    set({ error: null });
    try {
      const api = getApi();
      const { data } = await api.post<{ result: MCPToolCallResult }>(
        `/api/v1/plugins/${id}/mcp/tools/call`,
        { tool_name: toolName, arguments: args ?? {} },
        { token },
      );
      return data?.result ?? null;
    } catch {
      set({ error: "MCP tool call failed" });
      return null;
    }
  },

  readMCPResource: async (id, uri) => {
    const token = getToken();
    if (!token) return null;

    set({ error: null });
    try {
      const api = getApi();
      const { data } = await api.post<{ result: MCPResourceReadResult }>(
        `/api/v1/plugins/${id}/mcp/resources/read`,
        { uri },
        { token },
      );
      return data?.result ?? null;
    } catch {
      set({ error: "MCP resource read failed" });
      return null;
    }
  },

  getMCPPrompt: async (id, name, args) => {
    const token = getToken();
    if (!token) return null;

    set({ error: null });
    try {
      const api = getApi();
      const { data } = await api.post<{ result: MCPPromptGetResult }>(
        `/api/v1/plugins/${id}/mcp/prompts/get`,
        { name, arguments: args ?? {} },
        { token },
      );
      return data?.result ?? null;
    } catch {
      set({ error: "MCP prompt get failed" });
      return null;
    }
  },

  fetchEvents: async (id, limit = 50) => {
    const token = getToken();
    if (!token) return;

    set({ error: null });
    try {
      const api = getApi();
      const { data } = await api.get<PluginEventRecord[]>(
        `/api/v1/plugins/${id}/events?limit=${limit}`,
        { token },
      );
      set((state) => ({
        events: { ...state.events, [id]: data ?? [] },
      }));
    } catch {
      set({ error: "Failed to load plugin events" });
    }
  },

  startWorkflowRun: async (id, trigger) => {
    const token = getToken();
    if (!token) return;

    set({ error: null });
    try {
      const api = getApi();
      await api.post(
        `/api/v1/plugins/${id}/workflow-runs`,
        { trigger: trigger ?? {} },
        { token },
      );
      await get().fetchWorkflowRuns(id);
    } catch {
      set({ error: "Failed to start workflow run" });
    }
  },

  fetchWorkflowRuns: async (id) => {
    const token = getToken();
    if (!token) return;

    set({ error: null });
    try {
      const api = getApi();
      const { data } = await api.get<WorkflowPluginRun[]>(
        `/api/v1/plugins/${id}/workflow-runs`,
        { token },
      );
      set((state) => ({
        workflowRuns: { ...state.workflowRuns, [id]: data ?? [] },
      }));
    } catch {
      set({ error: "Failed to load workflow runs" });
    }
  },

  fetchWorkflowRun: async (runId) => {
    const token = getToken();
    if (!token) return;

    set({ error: null });
    try {
      const api = getApi();
      const { data } = await api.get<WorkflowPluginRun>(
        `/api/v1/plugins/workflow-runs/${runId}`,
        { token },
      );
      if (data) {
        set((state) => {
          const existing = state.workflowRuns[data.plugin_id] ?? [];
          const updated = existing.map((r) => (r.id === runId ? data : r));
          if (!existing.some((r) => r.id === runId)) {
            updated.push(data);
          }
          return {
            workflowRuns: {
              ...state.workflowRuns,
              [data.plugin_id]: updated,
            },
          };
        });
      }
    } catch {
      set({ error: "Failed to load workflow run" });
    }
  },

  setCatalogQuery: (query) => set({ catalogQuery: query }),

  selectWorkflowRun: (id) => set({ selectedWorkflowRunId: id }),

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
