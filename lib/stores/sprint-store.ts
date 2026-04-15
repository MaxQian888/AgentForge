"use client";

import { create } from "zustand";
import { createApiClient } from "@/lib/api-client";
import { useAuthStore } from "./auth-store";

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";

export type SprintStatus = "planning" | "active" | "closed";

export interface Sprint {
  id: string;
  projectId: string;
  name: string;
  startDate: string;
  endDate: string;
  milestoneId?: string | null;
  status: SprintStatus;
  totalBudgetUsd: number;
  spentUsd: number;
  createdAt: string;
}

export interface SprintBurndownPoint {
  date: string;
  remainingTasks: number;
  completedTasks: number;
}

export interface SprintMetrics {
  sprint: Sprint;
  plannedTasks: number;
  completedTasks: number;
  remainingTasks: number;
  completionRate: number;
  velocityPerWeek: number;
  taskBudgetUsd: number;
  taskSpentUsd: number;
  burndown: SprintBurndownPoint[];
}

export type BudgetThresholdStatus =
  | "inactive"
  | "healthy"
  | "warning"
  | "exceeded";

export interface SprintBudgetTaskEntry {
  taskId: string;
  title: string;
  allocated: number;
  spent: number;
  remaining: number;
  thresholdStatus: BudgetThresholdStatus;
}

export interface SprintBudgetDetail {
  sprintId: string;
  projectId: string;
  sprintName: string;
  allocated: number;
  spent: number;
  remaining: number;
  thresholdStatus: BudgetThresholdStatus;
  warningThresholdPercent: number;
  tasksWithBudgetCount: number;
  tasks: SprintBudgetTaskEntry[];
}

export interface CreateSprintRequest {
  name: string;
  startDate: string;
  endDate: string;
  totalBudgetUsd: number;
  milestoneId?: string | null;
}

export interface UpdateSprintRequest {
  name?: string;
  startDate?: string;
  endDate?: string;
  milestoneId?: string | null;
  status?: SprintStatus;
  totalBudgetUsd?: number;
}

interface SprintState {
  sprintsByProject: Record<string, Sprint[]>;
  metricsBySprintId: Record<string, SprintMetrics>;
  budgetDetailBySprintId: Record<string, SprintBudgetDetail>;
  loadingByProject: Record<string, boolean>;
  metricsLoadingBySprintId: Record<string, boolean>;
  budgetLoadingBySprintId: Record<string, boolean>;
  errorByProject: Record<string, string | null>;
  metricsErrorBySprintId: Record<string, string | null>;
  budgetErrorBySprintId: Record<string, string | null>;
  fetchSprints: (projectId: string) => Promise<void>;
  fetchSprintMetrics: (projectId: string, sprintId: string) => Promise<void>;
  fetchSprintBudgetDetail: (sprintId: string) => Promise<void>;
  createSprint: (projectId: string, data: CreateSprintRequest) => Promise<Sprint>;
  updateSprint: (projectId: string, sprintId: string, data: UpdateSprintRequest) => Promise<Sprint>;
  upsertSprint: (sprint: Sprint) => void;
}

const DATE_ONLY_PATTERN = /^\d{4}-\d{2}-\d{2}$/;

export function normalizeSprintDateInput(value: string): string {
  const trimmed = value.trim();
  if (!trimmed) {
    return trimmed;
  }
  if (DATE_ONLY_PATTERN.test(trimmed)) {
    return new Date(`${trimmed}T00:00:00.000Z`).toISOString();
  }
  return trimmed;
}

function normalizeSprintPayload<T extends CreateSprintRequest | UpdateSprintRequest>(data: T): T {
  const next = { ...data };
  if (typeof next.startDate === "string") {
    next.startDate = normalizeSprintDateInput(next.startDate);
  }
  if (typeof next.endDate === "string") {
    next.endDate = normalizeSprintDateInput(next.endDate);
  }
  return next;
}

function getToken() {
  const authState = useAuthStore.getState() as {
    accessToken?: string | null;
    token?: string | null;
  };
  return authState.accessToken ?? authState.token ?? null;
}

export const useSprintStore = create<SprintState>()((set) => ({
  sprintsByProject: {},
  metricsBySprintId: {},
  budgetDetailBySprintId: {},
  loadingByProject: {},
  metricsLoadingBySprintId: {},
  budgetLoadingBySprintId: {},
  errorByProject: {},
  metricsErrorBySprintId: {},
  budgetErrorBySprintId: {},

  fetchSprints: async (projectId) => {
    const token = getToken();
    if (!token) return;

    set((state) => ({
      loadingByProject: { ...state.loadingByProject, [projectId]: true },
      errorByProject: { ...state.errorByProject, [projectId]: null },
    }));

    try {
      const api = createApiClient(API_URL);
      const { data } = await api.get<Sprint[]>(
        `/api/v1/projects/${projectId}/sprints`,
        { token }
      );

      set((state) => ({
        sprintsByProject: {
          ...state.sprintsByProject,
          [projectId]: data,
        },
      }));
    } catch (error) {
      const message =
        error instanceof Error ? error.message : "Failed to load sprints";
      set((state) => ({
        errorByProject: { ...state.errorByProject, [projectId]: message },
      }));
    } finally {
      set((state) => ({
        loadingByProject: { ...state.loadingByProject, [projectId]: false },
      }));
    }
  },

  createSprint: async (projectId, data) => {
    const token = getToken();
    if (!token) throw new Error("Not authenticated");

    const api = createApiClient(API_URL);
    const payload = normalizeSprintPayload(data);
    const { data: sprint } = await api.post<Sprint>(
      `/api/v1/projects/${projectId}/sprints`,
      payload,
      { token }
    );

    set((state) => ({
      sprintsByProject: {
        ...state.sprintsByProject,
        [projectId]: [...(state.sprintsByProject[projectId] ?? []), sprint],
      },
    }));

    return sprint;
  },

  updateSprint: async (projectId, sprintId, data) => {
    const token = getToken();
    if (!token) throw new Error("Not authenticated");

    const api = createApiClient(API_URL);
    const payload = normalizeSprintPayload(data);
    const { data: sprint } = await api.put<Sprint>(
      `/api/v1/projects/${projectId}/sprints/${sprintId}`,
      payload,
      { token }
    );

    set((state) => {
      const existing = state.sprintsByProject[projectId] ?? [];
      return {
        sprintsByProject: {
          ...state.sprintsByProject,
          [projectId]: existing.map((s) => (s.id === sprintId ? sprint : s)),
        },
      };
    });

    return sprint;
  },

  upsertSprint: (sprint) => {
    set((state) => {
      const existing = state.sprintsByProject[sprint.projectId] ?? [];
      const index = existing.findIndex((s) => s.id === sprint.id);
      const next = index >= 0
        ? existing.map((s) => (s.id === sprint.id ? sprint : s))
        : [...existing, sprint];
      return {
        sprintsByProject: {
          ...state.sprintsByProject,
          [sprint.projectId]: next,
        },
      };
    });
  },

  fetchSprintMetrics: async (projectId, sprintId) => {
    const token = getToken();
    if (!token) return;

    set((state) => ({
      metricsLoadingBySprintId: {
        ...state.metricsLoadingBySprintId,
        [sprintId]: true,
      },
      metricsErrorBySprintId: {
        ...state.metricsErrorBySprintId,
        [sprintId]: null,
      },
    }));

    try {
      const api = createApiClient(API_URL);
      const { data } = await api.get<SprintMetrics>(
        `/api/v1/projects/${projectId}/sprints/${sprintId}/metrics`,
        { token }
      );

      set((state) => ({
        metricsBySprintId: {
          ...state.metricsBySprintId,
          [sprintId]: data,
        },
      }));
    } catch (error) {
      const message =
        error instanceof Error ? error.message : "Failed to load sprint metrics";
      set((state) => ({
        metricsErrorBySprintId: {
          ...state.metricsErrorBySprintId,
          [sprintId]: message,
        },
      }));
    } finally {
      set((state) => ({
        metricsLoadingBySprintId: {
          ...state.metricsLoadingBySprintId,
          [sprintId]: false,
        },
      }));
    }
  },

  fetchSprintBudgetDetail: async (sprintId) => {
    const token = getToken();
    if (!token) return;

    set((state) => ({
      budgetLoadingBySprintId: {
        ...state.budgetLoadingBySprintId,
        [sprintId]: true,
      },
      budgetErrorBySprintId: {
        ...state.budgetErrorBySprintId,
        [sprintId]: null,
      },
    }));

    try {
      const api = createApiClient(API_URL);
      const { data } = await api.get<SprintBudgetDetail>(
        `/api/v1/sprints/${sprintId}/budget`,
        { token }
      );

      set((state) => ({
        budgetDetailBySprintId: {
          ...state.budgetDetailBySprintId,
          [sprintId]: data,
        },
      }));
    } catch (error) {
      const message =
        error instanceof Error ? error.message : "Failed to load sprint budget";
      set((state) => ({
        budgetErrorBySprintId: {
          ...state.budgetErrorBySprintId,
          [sprintId]: message,
        },
      }));
    } finally {
      set((state) => ({
        budgetLoadingBySprintId: {
          ...state.budgetLoadingBySprintId,
          [sprintId]: false,
        },
      }));
    }
  },
}));
