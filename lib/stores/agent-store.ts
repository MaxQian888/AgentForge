"use client";

import { create } from "zustand";
import { createApiClient } from "@/lib/api-client";
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
}

interface AgentPoolQueueEntry {
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
  budgetUsd?: number;
  agentRunId?: string;
  createdAt: string;
  updatedAt: string;
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

interface AgentState {
  agents: Agent[];
  agentOutputs: Map<string, string[]>;
  pool: AgentPoolSummary | null;
  loading: boolean;
  fetchAgents: () => Promise<void>;
  fetchAgent: (id: string) => Promise<Agent | null>;
  fetchPool: () => Promise<void>;
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

export const useAgentStore = create<AgentState>()((set) => ({
  agents: [],
  agentOutputs: new Map(),
  pool: null,
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
    if ("dispatch" in data && data.dispatch?.status === "queued" && data.dispatch.queue) {
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
        const nextQueue = [...(currentPool.queue ?? []), data.dispatch.queue];
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
