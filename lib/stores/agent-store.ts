"use client";

import { create } from "zustand";
import { createApiClient } from "@/lib/api-client";
import type { CodingAgentCatalog } from "./project-store";
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
    available?: boolean;
    diagnostics?: Array<{
      code?: string;
      message?: string;
      blocking?: boolean;
    }>;
  }>;
}

interface BridgeHealthApiShape {
  status?: string;
  last_check?: string;
  pool?: {
    active?: number;
    available?: number;
    warm?: number;
  };
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
  ) => Promise<DispatchPreflightSummary | null>;
  fetchDispatchHistory: (taskId: string) => Promise<DispatchAttemptRecord[]>;
  fetchDispatchStats: (projectId: string) => Promise<DispatchStatsSummary | null>;
  spawnAgent: (taskId: string, memberId: string, options?: SpawnAgentOptions) => Promise<void>;
  pauseAgent: (id: string) => Promise<void>;
  resumeAgent: (id: string) => Promise<void>;
  killAgent: (id: string) => Promise<void>;
  appendOutput: (id: string, line: string) => void;
  upsertAgent: (agent: AgentApiShape | Agent) => void;
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

  const runtimes = Array.isArray(raw.runtimes)
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
        available: Boolean(runtime?.available),
        diagnostics: Array.isArray(runtime?.diagnostics)
          ? runtime.diagnostics.map((diagnostic) => ({
              code: typeof diagnostic?.code === "string" ? diagnostic.code : "",
              message: typeof diagnostic?.message === "string" ? diagnostic.message : "",
              blocking: Boolean(diagnostic?.blocking),
            }))
          : [],
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

export const useAgentStore = create<AgentState>()((set, get) => ({
  agents: [],
  agentOutputs: new Map(),
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

  fetchDispatchPreflight: async (projectId, taskId, memberId) => {
    const token = useAuthStore.getState().accessToken;
    if (!token || !projectId || !taskId || !memberId) return null;
    const api = createApiClient(API_URL);
    const { data } = await api.get<DispatchPreflightSummary>(
      `/api/v1/projects/${projectId}/dispatch/preflight?taskId=${encodeURIComponent(taskId)}&memberId=${encodeURIComponent(memberId)}`,
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

  spawnAgent: async (taskId, memberId, options = {}) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return;
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
  },

  pauseAgent: async (id) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return;
    const api = createApiClient(API_URL);
    const { data } = await api.post<AgentApiShape>(`/api/v1/agents/${id}/pause`, {}, { token });
    const agent = normalizeAgent(data);
    set((state) => {
      const agents = upsertAgents(state.agents, agent);
      return { agents, pool: syncPoolWithAgents(state.pool, agents) };
    });
  },

  resumeAgent: async (id) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return;
    const api = createApiClient(API_URL);
    const { data } = await api.post<AgentApiShape>(`/api/v1/agents/${id}/resume`, {}, { token });
    const agent = normalizeAgent(data);
    set((state) => {
      const agents = upsertAgents(state.agents, agent);
      return { agents, pool: syncPoolWithAgents(state.pool, agents) };
    });
  },

  killAgent: async (id) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return;
    const api = createApiClient(API_URL);
    const { data } = await api.post<AgentApiShape>(`/api/v1/agents/${id}/kill`, {}, { token });
    const agent = normalizeAgent(data);
    set((state) => {
      const agents = upsertAgents(state.agents, agent);
      return { agents, pool: syncPoolWithAgents(state.pool, agents) };
    });
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
}));
