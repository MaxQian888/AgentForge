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

export interface CreateSprintRequest {
  name: string;
  startDate: string;
  endDate: string;
  totalBudgetUsd: number;
}

export interface UpdateSprintRequest {
  name?: string;
  startDate?: string;
  endDate?: string;
  status?: SprintStatus;
  totalBudgetUsd?: number;
}

interface SprintState {
  sprintsByProject: Record<string, Sprint[]>;
  metricsBySprintId: Record<string, SprintMetrics>;
  loadingByProject: Record<string, boolean>;
  metricsLoadingBySprintId: Record<string, boolean>;
  errorByProject: Record<string, string | null>;
  metricsErrorBySprintId: Record<string, string | null>;
  fetchSprints: (projectId: string) => Promise<void>;
  fetchSprintMetrics: (projectId: string, sprintId: string) => Promise<void>;
  createSprint: (projectId: string, data: CreateSprintRequest) => Promise<Sprint>;
  updateSprint: (projectId: string, sprintId: string, data: UpdateSprintRequest) => Promise<Sprint>;
  upsertSprint: (sprint: Sprint) => void;
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
  loadingByProject: {},
  metricsLoadingBySprintId: {},
  errorByProject: {},
  metricsErrorBySprintId: {},

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
    const { data: sprint } = await api.post<Sprint>(
      `/api/v1/projects/${projectId}/sprints`,
      data,
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
    const { data: sprint } = await api.put<Sprint>(
      `/api/v1/projects/${projectId}/sprints/${sprintId}`,
      data,
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
}));
