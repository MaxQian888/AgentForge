"use client";

import { create } from "zustand";
import { toast } from "sonner";
import { createApiClient } from "@/lib/api-client";
import { withDevtools } from "./_devtools";
import { getPreferredLocale } from "./locale-store";
import type {
  CodingAgentCatalog,
  CodingAgentInteractionCapabilities,
  CodingAgentProvider,
} from "./project-store";
import { useAuthStore } from "./auth-store";

export type AgentStatus =
  | "starting"
  | "running"
  | "paused"
  | "completed"
  | "failed"
  | "cancelled"
  | "budget_exceeded";

export type MemoryStatus = "none" | "available" | "warming";
export type DispatchStatus = "started" | "queued" | "blocked" | "skipped";

export interface AgentResourceUtilization {
  cpuPercent: number;
  memoryPercent: number;
  cpuHistory: number[];
  memoryHistory: number[];
  updatedAt?: string;
}

interface AgentApiShape {
  id: string;
  taskId: string;
  taskTitle?: string;
  memberId: string;
  roleId?: string;
  roleName?: string;
  status: AgentStatus;
  runtime?: string;
  provider?: string;
  model?: string;
  inputTokens?: number;
  outputTokens?: number;
  cacheReadTokens?: number;
  costUsd?: number;
  budgetUsd?: number;
  turnCount?: number;
  worktreePath?: string;
  branchName?: string;
  sessionId?: string;
  lastActivityAt?: string;
  startedAt?: string;
  createdAt: string;
  completedAt?: string | null;
  canResume?: boolean;
  memoryStatus?: MemoryStatus;
  resourceUtilization?: AgentResourceUtilization;
  teamId?: string;
  teamRole?: string;
  dispatchStatus?: DispatchStatus;
  guardrailType?: string;
}

export interface AgentPoolQueueEntry {
  entryId: string;
  projectId: string;
  taskId: string;
  memberId: string;
  status: string;
  reason: string;
  runtime?: string;
  provider?: string;
  model?: string;
  roleId?: string;
  priority?: number;
  budgetUsd?: number;
  agentRunId?: string;
  createdAt: string;
  updatedAt: string;
}

export interface DispatchBudgetState {
  scope: string;
  message: string;
}

export interface DispatchPreflightSummary {
  admissionLikely: boolean;
  budgetWarning?: DispatchBudgetState;
  budgetBlocked?: DispatchBudgetState;
  poolActive?: number;
  poolAvailable?: number;
  poolQueued?: number;
  dispatchOutcomeHint: DispatchStatus;
}

export interface DispatchAttemptRecord {
  id: string;
  projectId: string;
  taskId: string;
  memberId?: string | null;
  outcome: DispatchStatus;
  triggerSource: string;
  reason?: string;
  guardrailType?: string;
  guardrailScope?: string;
  createdAt: string;
}

export interface DispatchStatsSummary {
  outcomes: Record<string, number>;
  blockedReasons: Record<string, number>;
  queueDepth: number;
  medianWaitSeconds?: number;
}

interface AgentDispatchResponse {
  task?: { id: string };
  dispatch?: {
    status?: string;
    reason?: string;
    run?: AgentApiShape;
    queue?: AgentPoolQueueEntry;
  };
}

export interface Agent {
  id: string;
  taskId: string;
  taskTitle: string;
  memberId: string;
  roleId: string;
  roleName: string;
  status: AgentStatus;
  runtime: string;
  provider: string;
  model: string;
  turns: number;
  cost: number;
  budget: number;
  worktreePath: string;
  branchName: string;
  sessionId: string;
  lastActivity: string;
  startedAt: string;
  createdAt: string;
  completedAt?: string | null;
  canResume: boolean;
  memoryStatus: MemoryStatus;
  resourceUtilization?: AgentResourceUtilization;
  teamId?: string;
  teamRole?: string;
  dispatchStatus?: DispatchStatus;
  guardrailType?: string;
}

export interface AgentPoolSummary {
  active: number;
  max: number;
  available: number;
  pausedResumable: number;
  queued?: number;
  warm?: number;
  degraded?: boolean;
  queue?: AgentPoolQueueEntry[];
}

export interface SpawnAgentOptions {
  runtime?: string;
  provider?: string;
  model?: string;
  roleId?: string;
  maxBudgetUsd?: number;
}

interface RuntimeCatalogApiShape {
  default_runtime?: string;
  runtimes?: Array<{
    key?: string;
    label?: string;
    display_name?: string;
    default_provider?: string;
    compatible_providers?: string[];
    default_model?: string;
    model_options?: string[];
    available?: boolean;
    diagnostics?: Array<{
      code?: string;
      message?: string;
      blocking?: boolean;
    }>;
    supported_features?: string[];
    interaction_capabilities?: {
      inputs?: Record<string, {
        state?: string;
        reason_code?: string;
        message?: string;
        requires_request_fields?: string[];
      }>;
      lifecycle?: Record<string, {
        state?: string;
        reason_code?: string;
        message?: string;
        requires_request_fields?: string[];
      }>;
      approval?: Record<string, {
        state?: string;
        reason_code?: string;
        message?: string;
        requires_request_fields?: string[];
      }>;
      mcp?: Record<string, {
        state?: string;
        reason_code?: string;
        message?: string;
        requires_request_fields?: string[];
      }>;
      diagnostics?: Record<string, {
        state?: string;
        reason_code?: string;
        message?: string;
        requires_request_fields?: string[];
      }>;
    };
    providers?: Array<{
      provider?: string;
      connected?: boolean;
      default_model?: string;
      model_options?: string[];
      auth_required?: boolean;
      auth_methods?: string[];
    }>;
    launch_contract?: {
      prompt_transport?: string;
      output_mode?: string;
      supported_output_modes?: string[];
      supported_approval_modes?: string[];
      additional_directories?: boolean;
      env_overrides?: boolean;
    };
    lifecycle?: {
      stage?: string;
      sunset_at?: string;
      replacement_runtime?: string;
      message?: string;
    };
  }>;
}

type RuntimeCatalogApiRuntime = NonNullable<RuntimeCatalogApiShape["runtimes"]>[number];
type RuntimeCatalogApiCapabilityGroup =
  | Record<
      string,
      {
        state?: string;
        reason_code?: string;
        message?: string;
        requires_request_fields?: string[];
      }
    >
  | undefined;
type RuntimeCatalogApiProviders = RuntimeCatalogApiRuntime["providers"];

interface BridgeHealthApiShape {
  status?: string;
  last_check?: string;
  pool?: {
    active?: number;
    available?: number;
    warm?: number;
  };
}

/* ── Agent streaming data types (consumed from WS events) ── */

export interface AgentToolCallEntry {
  toolName: string;
  toolCallId?: string;
  input?: unknown;
  turnNumber?: number;
}

export interface AgentToolResultEntry {
  toolName: string;
  toolCallId?: string;
  output?: unknown;
  isError?: boolean;
  turnNumber?: number;
}

export interface AgentFileChangeEntry {
  path: string;
  changeType?: string;
}

export interface AgentTodoEntry {
  id?: string;
  content?: string;
  status?: string;
}

export interface AgentPermissionRequestEntry {
  requestId: string;
  toolName?: string;
  context?: unknown;
  elicitationType?: string;
  fields?: unknown[];
  mcpServerId?: string;
}

export interface AgentLogEntry {
  timestamp: string;
  content: string;
  type: string;
}

export interface BridgeHealthSummary {
  status: string;
  lastCheck: string;
  pool: {
    active: number;
    available: number;
    warm: number;
  };
}

interface AgentState {
  agents: Agent[];
  agentOutputs: Map<string, string[]>;
  agentToolCalls: Map<string, AgentToolCallEntry[]>;
  agentToolResults: Map<string, AgentToolResultEntry[]>;
  agentReasoning: Map<string, string>;
  agentFileChanges: Map<string, AgentFileChangeEntry[]>;
  agentTodos: Map<string, AgentTodoEntry[]>;
  agentPartialMessages: Map<string, string>;
  agentPermissionRequests: Map<string, AgentPermissionRequestEntry[]>;
  agentLogs: Map<string, AgentLogEntry[]>;
  pool: AgentPoolSummary | null;
  runtimeCatalog: CodingAgentCatalog | null;
  runtimeCatalogFetchedAt: number;
  bridgeHealth: BridgeHealthSummary | null;
  dispatchStats: DispatchStatsSummary | null;
  dispatchHistoryByTask: Record<string, DispatchAttemptRecord[]>;
  loading: boolean;
  fetchAgents: () => Promise<void>;
  fetchAgent: (id: string) => Promise<Agent | null>;
  fetchPool: () => Promise<void>;
  fetchRuntimeCatalog: (force?: boolean) => Promise<CodingAgentCatalog | null>;
  fetchBridgeHealth: () => Promise<BridgeHealthSummary | null>;
  fetchDispatchPreflight: (
    projectId: string,
    taskId: string,
    memberId: string,
    options?: {
      runtime?: string;
      provider?: string;
      model?: string;
      roleId?: string;
      budgetUsd?: number;
    },
  ) => Promise<DispatchPreflightSummary | null>;
  fetchDispatchHistory: (taskId: string) => Promise<DispatchAttemptRecord[]>;
  fetchDispatchStats: (projectId: string) => Promise<DispatchStatsSummary | null>;
  fetchAgentLogs: (id: string) => Promise<AgentLogEntry[]>;
  spawnAgent: (taskId: string, memberId: string, options?: SpawnAgentOptions) => Promise<void>;
  pauseAgent: (id: string) => Promise<void>;
  resumeAgent: (id: string) => Promise<void>;
  killAgent: (id: string) => Promise<void>;
  appendOutput: (id: string, line: string) => void;
  upsertAgent: (agent: AgentApiShape | Agent) => void;
  appendToolCall: (id: string, entry: AgentToolCallEntry) => void;
  appendToolResult: (id: string, entry: AgentToolResultEntry) => void;
  setReasoning: (id: string, content: string) => void;
  appendFileChanges: (id: string, files: AgentFileChangeEntry[]) => void;
  setTodos: (id: string, todos: AgentTodoEntry[]) => void;
  setPartialMessage: (id: string, content: string) => void;
  appendPermissionRequest: (id: string, entry: AgentPermissionRequestEntry) => void;
  removePermissionRequest: (agentId: string, requestId: string) => void;
  clearAgentStreamData: (id: string) => void;
}

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";

function normalizeAgent(agent: AgentApiShape | Agent): Agent {
  if ("turns" in agent && "cost" in agent && "budget" in agent) {
    return agent;
  }

  return {
    id: agent.id,
    taskId: agent.taskId,
    taskTitle: agent.taskTitle ?? agent.taskId,
    memberId: agent.memberId,
    roleId: agent.roleId ?? "",
    roleName: agent.roleName ?? agent.roleId ?? "Agent",
    status: agent.status,
    runtime: agent.runtime ?? "",
    provider: agent.provider ?? "",
    model: agent.model ?? "",
    turns: agent.turnCount ?? 0,
    cost: agent.costUsd ?? 0,
    budget: agent.budgetUsd ?? 0,
    worktreePath: agent.worktreePath ?? "",
    branchName: agent.branchName ?? "",
    sessionId: agent.sessionId ?? "",
    lastActivity: agent.lastActivityAt ?? agent.createdAt,
    startedAt: agent.startedAt ?? agent.createdAt,
    createdAt: agent.createdAt,
    completedAt: agent.completedAt ?? undefined,
    canResume: Boolean(agent.canResume),
    memoryStatus: agent.memoryStatus ?? (agent.sessionId ? "available" : "none"),
    resourceUtilization: agent.resourceUtilization,
    teamId: agent.teamId,
    teamRole: agent.teamRole,
    dispatchStatus: agent.dispatchStatus ?? "started",
    guardrailType: agent.guardrailType ?? "",
  };
}

function upsertAgents(agents: Agent[], next: Agent): Agent[] {
  const index = agents.findIndex((item) => item.id === next.id);
  if (index === -1) {
    return [...agents, next];
  }

  const copy = [...agents];
  const merged = { ...copy[index] };
  for (const [key, value] of Object.entries(next)) {
    if (value !== undefined) {
      (merged as Record<string, unknown>)[key] = value;
    }
  }
  copy[index] = merged;
  return copy;
}

function syncPoolWithAgents(
  pool: AgentPoolSummary | null,
  agents: Agent[],
): AgentPoolSummary | null {
  if (!pool) {
    return null;
  }

  const uniqueAgents = Array.from(new Map(agents.map((agent) => [agent.id, agent])).values());

  const active = uniqueAgents.filter(
    (agent) => agent.status === "starting" || agent.status === "running"
  ).length;
  const pausedResumable = uniqueAgents.filter((agent) => agent.status === "paused").length;

  return {
    active,
    max: pool.max,
    available: Math.max(pool.max - active, 0),
    pausedResumable,
    queued: pool.queued ?? 0,
    warm: pool.warm ?? 0,
    degraded: pool.degraded ?? false,
    queue: pool.queue ?? [],
  };
}

function normalizeRuntimeCatalog(raw: RuntimeCatalogApiShape | null | undefined): CodingAgentCatalog | null {
  if (!raw) {
    return null;
  }

  const normalizeCapabilityGroup = (
    group: RuntimeCatalogApiCapabilityGroup,
  ): Record<
    string,
    {
      state: string;
      reasonCode?: string;
      message?: string;
      requiresRequestFields?: string[];
    }
  > => {
    if (!group) {
      return {};
    }

    return Object.fromEntries(
      Object.entries(group).map(([key, descriptor]) => [
        key,
        {
          state: typeof descriptor?.state === "string" ? descriptor.state : "unsupported",
          reasonCode:
            typeof descriptor?.reason_code === "string"
              ? descriptor.reason_code
              : undefined,
          message:
            typeof descriptor?.message === "string" ? descriptor.message : undefined,
          requiresRequestFields: Array.isArray(descriptor?.requires_request_fields)
            ? descriptor.requires_request_fields.map((item) => String(item))
            : undefined,
        },
      ]),
    );
  };

  const normalizeInteractionCapabilities = (
    capabilities: RuntimeCatalogApiRuntime["interaction_capabilities"] | undefined,
  ): CodingAgentInteractionCapabilities | undefined => {
    if (!capabilities) {
      return undefined;
    }

    return {
      inputs: normalizeCapabilityGroup(capabilities.inputs),
      lifecycle: normalizeCapabilityGroup(capabilities.lifecycle),
      approval: normalizeCapabilityGroup(capabilities.approval),
      mcp: normalizeCapabilityGroup(capabilities.mcp),
      diagnostics: normalizeCapabilityGroup(capabilities.diagnostics),
    };
  };

  const normalizeProviders = (
    providers: RuntimeCatalogApiProviders,
  ): CodingAgentProvider[] | undefined => {
    if (!Array.isArray(providers)) {
      return undefined;
    }

    return providers.map((provider) => ({
      provider: typeof provider?.provider === "string" ? provider.provider : "",
      connected: Boolean(provider?.connected),
      defaultModel:
        typeof provider?.default_model === "string" ? provider.default_model : undefined,
      modelOptions: Array.isArray(provider?.model_options)
        ? provider.model_options.map((item) => String(item))
        : undefined,
      authRequired:
        typeof provider?.auth_required === "boolean"
          ? provider.auth_required
          : undefined,
      authMethods: Array.isArray(provider?.auth_methods)
        ? provider.auth_methods.map((item) => String(item))
        : undefined,
    }));
  };

  const normalizeOutputMode = (
    value: unknown,
  ): "text" | "json" | "stream-json" => {
    if (value === "json" || value === "stream-json") {
      return value;
    }
    return "text";
  };

  const normalizePromptTransport = (
    value: unknown,
  ): "stdin" | "positional" | "prompt_flag" => {
    if (value === "positional" || value === "prompt_flag") {
      return value;
    }
    return "stdin";
  };

  const normalizeOutputModes = (values: unknown): Array<"text" | "json" | "stream-json"> => {
    if (!Array.isArray(values)) {
      return [];
    }
    return values
      .map((item) => normalizeOutputMode(item))
      .filter((item, index, array) => array.indexOf(item) === index);
  };

  const runtimes: CodingAgentCatalog["runtimes"] = Array.isArray(raw.runtimes)
    ? raw.runtimes.map((runtime) => ({
        runtime: typeof runtime?.key === "string" ? runtime.key : "",
        label:
          typeof runtime?.label === "string"
            ? runtime.label
            : typeof runtime?.display_name === "string"
              ? runtime.display_name
              : typeof runtime?.key === "string"
                ? runtime.key
                : "",
        defaultProvider:
          typeof runtime?.default_provider === "string" ? runtime.default_provider : "",
        compatibleProviders: Array.isArray(runtime?.compatible_providers)
          ? runtime.compatible_providers.map((item) => String(item))
          : [],
        defaultModel: typeof runtime?.default_model === "string" ? runtime.default_model : "",
        modelOptions: Array.isArray(runtime?.model_options)
          ? runtime.model_options.map((item) => String(item))
          : [],
        available: Boolean(runtime?.available),
        diagnostics: Array.isArray(runtime?.diagnostics)
          ? runtime.diagnostics.map((diagnostic) => ({
              code: typeof diagnostic?.code === "string" ? diagnostic.code : "",
              message: typeof diagnostic?.message === "string" ? diagnostic.message : "",
              blocking: Boolean(diagnostic?.blocking),
            }))
          : [],
        supportedFeatures: Array.isArray(runtime?.supported_features)
          ? runtime.supported_features.map((item) => String(item))
          : [],
        interactionCapabilities: normalizeInteractionCapabilities(
          runtime?.interaction_capabilities,
        ),
        providers: normalizeProviders(runtime?.providers),
        launchContract:
          runtime?.launch_contract
            ? {
                promptTransport: normalizePromptTransport(
                  runtime.launch_contract.prompt_transport,
                ),
                outputMode: normalizeOutputMode(runtime.launch_contract.output_mode),
                supportedOutputModes: normalizeOutputModes(
                  runtime.launch_contract.supported_output_modes,
                ),
                supportedApprovalModes: Array.isArray(runtime.launch_contract.supported_approval_modes)
                  ? runtime.launch_contract.supported_approval_modes.map((item) => String(item))
                  : [],
                additionalDirectories: Boolean(runtime.launch_contract.additional_directories),
                envOverrides: Boolean(runtime.launch_contract.env_overrides),
              }
            : undefined,
        lifecycle:
          runtime?.lifecycle
            ? {
                stage:
                  runtime.lifecycle.stage === "sunsetting" ||
                  runtime.lifecycle.stage === "sunset"
                    ? runtime.lifecycle.stage
                    : "active",
                sunsetAt:
                  typeof runtime.lifecycle.sunset_at === "string"
                    ? runtime.lifecycle.sunset_at
                    : undefined,
                replacementRuntime:
                  typeof runtime.lifecycle.replacement_runtime === "string"
                    ? runtime.lifecycle.replacement_runtime
                    : undefined,
                message:
                  typeof runtime.lifecycle.message === "string"
                    ? runtime.lifecycle.message
                    : undefined,
              }
            : undefined,
      }))
    : [];

  const defaultRuntime =
    typeof raw.default_runtime === "string" ? raw.default_runtime : runtimes[0]?.runtime ?? "";
  const defaultSelectionRuntime =
    runtimes.find((runtime) => runtime.runtime === defaultRuntime) ?? runtimes[0];

  return {
    defaultRuntime,
    defaultSelection: {
      runtime: defaultSelectionRuntime?.runtime ?? "",
      provider: defaultSelectionRuntime?.defaultProvider ?? "",
      model: defaultSelectionRuntime?.defaultModel ?? "",
    },
    runtimes,
  };
}

function normalizeBridgeHealth(raw: BridgeHealthApiShape | null | undefined): BridgeHealthSummary | null {
  if (!raw) {
    return null;
  }

  return {
    status: typeof raw.status === "string" ? raw.status : "degraded",
    lastCheck: typeof raw.last_check === "string" ? raw.last_check : "",
    pool: {
      active: typeof raw.pool?.active === "number" ? raw.pool.active : 0,
      available: typeof raw.pool?.available === "number" ? raw.pool.available : 0,
      warm: typeof raw.pool?.warm === "number" ? raw.pool.warm : 0,
    },
  };
}

export const useAgentStore = create<AgentState>()(
  withDevtools((set, get) => ({
  agents: [],
  agentOutputs: new Map(),
  agentToolCalls: new Map(),
  agentToolResults: new Map(),
  agentReasoning: new Map(),
  agentFileChanges: new Map(),
  agentTodos: new Map(),
  agentPartialMessages: new Map(),
  agentPermissionRequests: new Map(),
  agentLogs: new Map(),
  pool: null,
  runtimeCatalog: null,
  runtimeCatalogFetchedAt: 0,
  bridgeHealth: null,
  dispatchStats: null,
  dispatchHistoryByTask: {},
  loading: false,

  fetchAgents: async () => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return;
    set({ loading: true });
    try {
      const api = createApiClient(API_URL);
      const { data } = await api.get<AgentApiShape[]>("/api/v1/agents", { token });
      set((state) => {
        const agents = data.map(normalizeAgent);
        return { agents, pool: syncPoolWithAgents(state.pool, agents) };
      });
    } finally {
      set({ loading: false });
    }
  },

  fetchAgent: async (id) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return null;
    const api = createApiClient(API_URL);
    const { data } = await api.get<AgentApiShape>(`/api/v1/agents/${id}`, { token });
    const agent = normalizeAgent(data);
    set((state) => {
      const agents = upsertAgents(state.agents, agent);
      return { agents, pool: syncPoolWithAgents(state.pool, agents) };
    });
    return agent;
  },

  fetchPool: async () => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return;
    const api = createApiClient(API_URL);
    const { data } = await api.get<AgentPoolSummary>("/api/v1/agents/pool", { token });
    set({ pool: data });
  },

  fetchRuntimeCatalog: async (force = false) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return null;
    const { runtimeCatalog, runtimeCatalogFetchedAt } = get();
    if (!force && runtimeCatalog && Date.now() - runtimeCatalogFetchedAt < 60_000) {
      return runtimeCatalog;
    }
    const api = createApiClient(API_URL);
    const { data } = await api.get<RuntimeCatalogApiShape>("/api/v1/bridge/runtimes", { token });
    const runtimeCatalogNext = normalizeRuntimeCatalog(data);
    set({
      runtimeCatalog: runtimeCatalogNext,
      runtimeCatalogFetchedAt: Date.now(),
    });
    return runtimeCatalogNext;
  },

  fetchBridgeHealth: async () => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return null;
    const api = createApiClient(API_URL);
    const { data } = await api.get<BridgeHealthApiShape>("/api/v1/bridge/health", { token });
    const bridgeHealth = normalizeBridgeHealth(data);
    set({ bridgeHealth });
    return bridgeHealth;
  },

  fetchDispatchPreflight: async (projectId, taskId, memberId, options) => {
    const token = useAuthStore.getState().accessToken;
    if (!token || !projectId || !taskId || !memberId) return null;
    const api = createApiClient(API_URL);
    const params = new URLSearchParams({
      taskId,
      memberId,
    });
    if (options?.runtime) params.set("runtime", options.runtime);
    if (options?.provider) params.set("provider", options.provider);
    if (options?.model) params.set("model", options.model);
    if (options?.roleId) params.set("roleId", options.roleId);
    if (options?.budgetUsd != null && !Number.isNaN(options.budgetUsd)) {
      params.set("budgetUsd", String(options.budgetUsd));
    }
    const { data } = await api.get<DispatchPreflightSummary>(
      `/api/v1/projects/${projectId}/dispatch/preflight?${params.toString()}`,
      { token },
    );
    return data;
  },

  fetchDispatchHistory: async (taskId) => {
    const token = useAuthStore.getState().accessToken;
    if (!token || !taskId) return [];
    const api = createApiClient(API_URL);
    const { data } = await api.get<DispatchAttemptRecord[]>(
      `/api/v1/tasks/${taskId}/dispatch/history`,
      { token },
    );
    set((state) => ({
      dispatchHistoryByTask: {
        ...state.dispatchHistoryByTask,
        [taskId]: data,
      },
    }));
    return data;
  },

  fetchDispatchStats: async (projectId) => {
    const token = useAuthStore.getState().accessToken;
    if (!token || !projectId) return null;
    const api = createApiClient(API_URL);
    const { data } = await api.get<DispatchStatsSummary>(
      `/api/v1/projects/${projectId}/dispatch/stats`,
      { token },
    );
    set({ dispatchStats: data });
    return data;
  },

  fetchAgentLogs: async (id) => {
    const token = useAuthStore.getState().accessToken;
    if (!token || !id) return [];
    try {
      const api = createApiClient(API_URL);
      const { data } = await api.get<AgentLogEntry[]>(
        `/api/v1/agents/${id}/logs`,
        { token },
      );
      const logs = Array.isArray(data) ? data : [];
      set((state) => {
        const map = new Map(state.agentLogs);
        map.set(id, logs);
        return { agentLogs: map };
      });
      return logs;
    } catch {
      return [];
    }
  },

  spawnAgent: async (taskId, memberId, options = {}) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return;
    try {
      const api = createApiClient(API_URL);
      const { data } = await api.post<AgentApiShape | AgentDispatchResponse>(
        "/api/v1/agents/spawn",
        {
          taskId,
          memberId,
          runtime: options.runtime,
          provider: options.provider,
          model: options.model,
          roleId: options.roleId,
          maxBudgetUsd: options.maxBudgetUsd,
        },
        { token }
      );
      const queuedDispatch =
        "dispatch" in data && data.dispatch?.status === "queued"
          ? data.dispatch.queue
          : undefined;
      if (queuedDispatch) {
        set((state) => {
          const currentPool = state.pool ?? {
            active: 0,
            max: 0,
            available: 0,
            pausedResumable: 0,
            queued: 0,
            warm: 0,
            degraded: false,
            queue: [],
          };
          const nextQueue = [...(currentPool.queue ?? []), queuedDispatch];
          return {
            pool: {
              ...currentPool,
              queued: nextQueue.length,
              queue: nextQueue,
            },
          };
        });
        return;
      }

      const agentSource =
        "dispatch" in data && data.dispatch?.run ? data.dispatch.run : (data as AgentApiShape);
      const agent = normalizeAgent(agentSource);
      set((state) => {
        const agents = upsertAgents(state.agents, agent);
        return { agents, pool: syncPoolWithAgents(state.pool, agents) };
      });
    } catch (error) {
      const locale = getPreferredLocale();
      toast.error(locale === "zh-CN" ? "启动 Agent 失败" : "Failed to spawn agent", {
        description: error instanceof Error ? error.message : (locale === "zh-CN" ? "未知错误" : "Unknown error"),
      });
    }
  },

  pauseAgent: async (id) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return;
    try {
      const api = createApiClient(API_URL);
      const { data } = await api.post<AgentApiShape>(`/api/v1/agents/${id}/pause`, {}, { token });
      const agent = normalizeAgent(data);
      set((state) => {
        const agents = upsertAgents(state.agents, agent);
        return { agents, pool: syncPoolWithAgents(state.pool, agents) };
      });
    } catch (error) {
      const locale = getPreferredLocale();
      toast.error(locale === "zh-CN" ? "暂停 Agent 失败" : "Failed to pause agent", {
        description: error instanceof Error ? error.message : (locale === "zh-CN" ? "未知错误" : "Unknown error"),
      });
    }
  },

  resumeAgent: async (id) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return;
    try {
      const api = createApiClient(API_URL);
      const { data } = await api.post<AgentApiShape>(`/api/v1/agents/${id}/resume`, {}, { token });
      const agent = normalizeAgent(data);
      set((state) => {
        const agents = upsertAgents(state.agents, agent);
        return { agents, pool: syncPoolWithAgents(state.pool, agents) };
      });
    } catch (error) {
      const locale = getPreferredLocale();
      toast.error(locale === "zh-CN" ? "恢复 Agent 失败" : "Failed to resume agent", {
        description: error instanceof Error ? error.message : (locale === "zh-CN" ? "未知错误" : "Unknown error"),
      });
    }
  },

  killAgent: async (id) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return;
    try {
      const api = createApiClient(API_URL);
      const { data } = await api.post<AgentApiShape>(`/api/v1/agents/${id}/kill`, {}, { token });
      const agent = normalizeAgent(data);
      set((state) => {
        const agents = upsertAgents(state.agents, agent);
        return { agents, pool: syncPoolWithAgents(state.pool, agents) };
      });
    } catch (error) {
      const locale = getPreferredLocale();
      toast.error(locale === "zh-CN" ? "终止 Agent 失败" : "Failed to terminate agent", {
        description: error instanceof Error ? error.message : (locale === "zh-CN" ? "未知错误" : "Unknown error"),
      });
    }
  },

  appendOutput: (id, line) => {
    set((state) => {
      const outputs = new Map(state.agentOutputs);
      const lines = outputs.get(id) ?? [];
      outputs.set(id, [...lines, line]);
      return { agentOutputs: outputs };
    });
  },

  upsertAgent: (agent) => {
    const normalized = normalizeAgent(agent);
    set((state) => {
      const agents = upsertAgents(state.agents, normalized);
      return { agents, pool: syncPoolWithAgents(state.pool, agents) };
    });
  },

  appendToolCall: (id, entry) => {
    set((state) => {
      const map = new Map(state.agentToolCalls);
      map.set(id, [...(map.get(id) ?? []), entry]);
      return { agentToolCalls: map };
    });
  },

  appendToolResult: (id, entry) => {
    set((state) => {
      const map = new Map(state.agentToolResults);
      map.set(id, [...(map.get(id) ?? []), entry]);
      return { agentToolResults: map };
    });
  },

  setReasoning: (id, content) => {
    set((state) => {
      const map = new Map(state.agentReasoning);
      map.set(id, content);
      return { agentReasoning: map };
    });
  },

  appendFileChanges: (id, files) => {
    set((state) => {
      const map = new Map(state.agentFileChanges);
      map.set(id, [...(map.get(id) ?? []), ...files]);
      return { agentFileChanges: map };
    });
  },

  setTodos: (id, todos) => {
    set((state) => {
      const map = new Map(state.agentTodos);
      map.set(id, todos);
      return { agentTodos: map };
    });
  },

  setPartialMessage: (id, content) => {
    set((state) => {
      const map = new Map(state.agentPartialMessages);
      map.set(id, content);
      return { agentPartialMessages: map };
    });
  },

  appendPermissionRequest: (id, entry) => {
    set((state) => {
      const map = new Map(state.agentPermissionRequests);
      map.set(id, [...(map.get(id) ?? []), entry]);
      return { agentPermissionRequests: map };
    });
  },

  removePermissionRequest: (agentId, requestId) => {
    set((state) => {
      const map = new Map(state.agentPermissionRequests);
      const current = map.get(agentId);
      if (!current) return state;
      const filtered = current.filter((entry) => entry.requestId !== requestId);
      if (filtered.length === 0) {
        map.delete(agentId);
      } else {
        map.set(agentId, filtered);
      }
      return { agentPermissionRequests: map };
    });
  },

  clearAgentStreamData: (id) => {
    set((state) => {
      const toolCalls = new Map(state.agentToolCalls);
      const toolResults = new Map(state.agentToolResults);
      const reasoning = new Map(state.agentReasoning);
      const fileChanges = new Map(state.agentFileChanges);
      const todos = new Map(state.agentTodos);
      const partialMessages = new Map(state.agentPartialMessages);
      const permissionRequests = new Map(state.agentPermissionRequests);
      toolCalls.delete(id);
      toolResults.delete(id);
      reasoning.delete(id);
      fileChanges.delete(id);
      todos.delete(id);
      partialMessages.delete(id);
      permissionRequests.delete(id);
      return {
        agentToolCalls: toolCalls,
        agentToolResults: toolResults,
        agentReasoning: reasoning,
        agentFileChanges: fileChanges,
        agentTodos: todos,
        agentPartialMessages: partialMessages,
        agentPermissionRequests: permissionRequests,
      };
    });
  },
  }), { name: "agent-store" }),
);
