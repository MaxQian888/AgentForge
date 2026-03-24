"use client";

import { create } from "zustand";
import { createApiClient } from "@/lib/api-client";
import { useAuthStore } from "./auth-store";

export interface SprintCostSummary {
  sprintId: string;
  sprintName: string;
  costUsd: number;
  budgetUsd: number;
  inputTokens: number;
  outputTokens: number;
}

export interface TaskCostDetail {
  taskId: string;
  taskTitle: string;
  agentRuns: number;
  costUsd: number;
  inputTokens: number;
  outputTokens: number;
  cacheReadTokens: number;
}

export interface ProjectCostSummary {
  totalCostUsd: number;
  totalInputTokens: number;
  totalOutputTokens: number;
  totalCacheReadTokens: number;
  totalTurns: number;
  activeAgents: number;
  sprintCosts: SprintCostSummary[];
  taskCosts: TaskCostDetail[];
  dailyCosts: Array<{ date: string; costUsd: number }>;
}

export interface VelocityPoint {
  period: string;
  tasksCompleted: number;
  costUsd: number;
}

export interface AgentPerformanceRecord {
  agentId: string;
  agentName: string;
  taskCount: number;
  successRate: number;
  avgCostUsd: number;
  avgDurationMinutes: number;
  totalCostUsd: number;
}

interface CostState {
  projectCost: ProjectCostSummary | null;
  loading: boolean;
  error: string | null;
  velocity: VelocityPoint[];
  velocityLoading: boolean;
  agentPerformance: AgentPerformanceRecord[];
  performanceLoading: boolean;
  fetchProjectCost: (projectId: string) => Promise<void>;
  fetchVelocity: (projectId: string) => Promise<void>;
  fetchAgentPerformance: (projectId: string) => Promise<void>;
}

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";

export const useCostStore = create<CostState>()((set) => ({
  projectCost: null,
  loading: false,
  error: null,
  velocity: [],
  velocityLoading: false,
  agentPerformance: [],
  performanceLoading: false,

  fetchProjectCost: async (projectId) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return;

    set({ loading: true, error: null });
    try {
      const api = createApiClient(API_URL);
      const { data } = await api.get<ProjectCostSummary>(
        `/api/v1/stats/cost?projectId=${projectId}`,
        { token }
      );
      set({ projectCost: data, error: null });
    } catch {
      set({ error: "Unable to load cost data" });
    } finally {
      set({ loading: false });
    }
  },

  fetchVelocity: async (projectId) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return;

    set({ velocityLoading: true });
    try {
      const api = createApiClient(API_URL);
      const { data } = await api.get<VelocityPoint[]>(
        `/api/v1/stats/velocity?projectId=${projectId}`,
        { token }
      );
      set({ velocity: data });
    } catch {
      set({ velocity: [] });
    } finally {
      set({ velocityLoading: false });
    }
  },

  fetchAgentPerformance: async (projectId) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return;

    set({ performanceLoading: true });
    try {
      const api = createApiClient(API_URL);
      const { data } = await api.get<AgentPerformanceRecord[]>(
        `/api/v1/stats/agent-performance?projectId=${projectId}`,
        { token }
      );
      set({ agentPerformance: data });
    } catch {
      set({ agentPerformance: [] });
    } finally {
      set({ performanceLoading: false });
    }
  },
}));
