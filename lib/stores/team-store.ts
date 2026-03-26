"use client";

import { create } from "zustand";
import { createApiClient } from "@/lib/api-client";
import { useAuthStore } from "./auth-store";

export type TeamStatus =
  | "pending"
  | "planning"
  | "executing"
  | "reviewing"
  | "completed"
  | "failed"
  | "cancelled";

export interface AgentTeam {
  id: string;
  projectId: string;
  taskId: string;
  taskTitle: string;
  name: string;
  status: TeamStatus;
  strategy: string;
  runtime: string;
  provider: string;
  model: string;
  plannerRunId?: string;
  reviewerRunId?: string;
  coderRunIds: string[];
  totalBudget: number;
  totalSpent: number;
  errorMessage: string;
  createdAt: string;
  updatedAt: string;
}

export interface StartTeamOptions {
  strategy?: string;
  totalBudgetUsd?: number;
  runtime?: string;
  provider?: string;
  model?: string;
}

interface TeamState {
  teams: AgentTeam[];
  loading: boolean;
  error: string | null;
  loadingById: Record<string, boolean>;
  errorById: Record<string, string | null>;
  fetchTeams: (projectId?: string) => Promise<void>;
  fetchTeam: (id: string) => Promise<AgentTeam | null>;
  startTeam: (taskId: string, memberId: string, options?: StartTeamOptions) => Promise<void>;
  cancelTeam: (id: string) => Promise<void>;
  retryTeam: (id: string) => Promise<void>;
  upsertTeam: (team: AgentTeam) => void;
}

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";

export function normalizeTeamStrategy(strategy: unknown): string {
  switch (String(strategy ?? "").trim()) {
    case "planner_coder_reviewer":
    case "planner-coder-reviewer":
      return "plan-code-review";
    default:
      return String(strategy ?? "").trim();
  }
}

export function getTeamStrategyLabel(strategy: string): string {
  switch (normalizeTeamStrategy(strategy)) {
    case "plan-code-review":
      return "Planner → Coder → Reviewer";
    case "wave-based":
      return "Wave Based";
    case "pipeline":
      return "Pipeline";
    case "swarm":
      return "Swarm";
    default:
      return "Unknown strategy";
  }
}

export function normalizeTeam(raw: Record<string, unknown>): AgentTeam {
  // Handle coderRunIds which may come as coderRuns (array of objects with id) or coderRunIds (array of strings)
  let coderRunIds: string[] = [];
  if (Array.isArray(raw.coderRunIds)) {
    coderRunIds = raw.coderRunIds.map((id: unknown) => String(id));
  } else if (Array.isArray(raw.coderRuns)) {
    coderRunIds = (raw.coderRuns as Record<string, unknown>[]).map((run) =>
      typeof run === "string" ? run : String(run.id ?? "")
    );
  }

  return {
    id: String(raw.id ?? ""),
    projectId: String(raw.projectId ?? ""),
    taskId: String(raw.taskId ?? ""),
    taskTitle: String(raw.taskTitle ?? raw.taskId ?? ""),
    name: String(raw.name ?? ""),
    status: (typeof raw.status === "string" ? raw.status : "pending") as TeamStatus,
    strategy: normalizeTeamStrategy(raw.strategy),
    runtime: String(raw.runtime ?? ""),
    provider: String(raw.provider ?? ""),
    model: String(raw.model ?? ""),
    plannerRunId: typeof raw.plannerRunId === "string" ? raw.plannerRunId : undefined,
    reviewerRunId: typeof raw.reviewerRunId === "string" ? raw.reviewerRunId : undefined,
    coderRunIds,
    totalBudget: Number(raw.totalBudget ?? raw.totalBudgetUsd ?? 0),
    totalSpent: Number(raw.totalSpent ?? raw.totalSpentUsd ?? 0),
    errorMessage: String(raw.errorMessage ?? ""),
    createdAt: typeof raw.createdAt === "string" ? raw.createdAt : new Date().toISOString(),
    updatedAt: typeof raw.updatedAt === "string" ? raw.updatedAt : new Date().toISOString(),
  };
}

function upsertTeams(teams: AgentTeam[], next: AgentTeam): AgentTeam[] {
  const index = teams.findIndex((t) => t.id === next.id);
  if (index === -1) {
    return [...teams, next];
  }

  const copy = [...teams];
  const merged = { ...copy[index] };
  for (const [key, value] of Object.entries(next)) {
    if (value !== undefined) {
      (merged as Record<string, unknown>)[key] = value;
    }
  }
  copy[index] = merged;
  return copy;
}

export const useTeamStore = create<TeamState>()((set) => ({
  teams: [],
  loading: false,
  error: null,
  loadingById: {},
  errorById: {},

  fetchTeams: async (projectId) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return;
    set({ loading: true, error: null });
    try {
      const api = createApiClient(API_URL);
      const query = projectId ? `?projectId=${projectId}` : "";
      const { data } = await api.get<Record<string, unknown>[]>(`/api/v1/teams${query}`, { token });
      const teams = data.map(normalizeTeam);
      set({ teams, error: null });
    } catch (error) {
      set({
        error: error instanceof Error ? error.message : "Failed to load teams",
        teams: [],
      });
    } finally {
      set({ loading: false });
    }
  },

  fetchTeam: async (id) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return null;
    set((state) => ({
      loadingById: { ...state.loadingById, [id]: true },
      errorById: { ...state.errorById, [id]: null },
    }));
    const api = createApiClient(API_URL);
    try {
      const { data } = await api.get<Record<string, unknown>>(`/api/v1/teams/${id}`, { token });
      const team = normalizeTeam(data);
      set((state) => ({
        teams: upsertTeams(state.teams, team),
        errorById: { ...state.errorById, [id]: null },
      }));
      return team;
    } catch (error) {
      set((state) => ({
        errorById: {
          ...state.errorById,
          [id]: error instanceof Error ? error.message : "Failed to load team detail",
        },
      }));
      return null;
    } finally {
      set((state) => ({
        loadingById: { ...state.loadingById, [id]: false },
      }));
    }
  },

  startTeam: async (taskId, memberId, options = {}) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return;
    const api = createApiClient(API_URL);
    const { data } = await api.post<Record<string, unknown>>(
      "/api/v1/teams/start",
      {
        taskId,
        memberId,
        strategy: options.strategy,
        totalBudgetUsd: options.totalBudgetUsd,
        runtime: options.runtime,
        provider: options.provider,
        model: options.model,
      },
      { token }
    );
    const team = normalizeTeam(data);
    set((state) => ({ teams: upsertTeams(state.teams, team) }));
  },

  cancelTeam: async (id) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return;
    const api = createApiClient(API_URL);
    const { data } = await api.post<Record<string, unknown>>(`/api/v1/teams/${id}/cancel`, {}, { token });
    const team = normalizeTeam(data);
    set((state) => ({ teams: upsertTeams(state.teams, team) }));
  },

  retryTeam: async (id) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return;
    const api = createApiClient(API_URL);
    const { data } = await api.post<Record<string, unknown>>(`/api/v1/teams/${id}/retry`, {}, { token });
    const team = normalizeTeam(data);
    set((state) => ({ teams: upsertTeams(state.teams, team) }));
  },

  upsertTeam: (team) => {
    set((state) => ({ teams: upsertTeams(state.teams, team) }));
  },
}));
