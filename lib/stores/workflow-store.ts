"use client";

import { create } from "zustand";
import { createApiClient } from "@/lib/api-client";
import { useAuthStore } from "./auth-store";
import type { TaskStatus } from "./task-store";

export interface WorkflowTrigger {
  fromStatus: string;
  toStatus: string;
  action: string;
  config?: Record<string, unknown>;
}

export interface WorkflowConfig {
  id?: string;
  projectId: string;
  transitions: Record<string, string[]>;
  triggers: WorkflowTrigger[];
  createdAt?: string;
  updatedAt?: string;
}

export interface WorkflowActivityEntry {
  taskId: string;
  action: string;
  from: string;
  to: string;
  timestamp: string;
  config?: Record<string, unknown>;
}

const MAX_WORKFLOW_ACTIVITY_ENTRIES = 10;

interface WorkflowState {
  config: WorkflowConfig | null;
  loading: boolean;
  saving: boolean;
  error: string | null;
  recentActivityByProject: Record<string, WorkflowActivityEntry[]>;
  fetchWorkflow: (projectId: string) => Promise<void>;
  updateWorkflow: (
    projectId: string,
    config: {
      transitions: Record<string, string[]>;
      triggers: WorkflowTrigger[];
    }
  ) => Promise<boolean>;
  appendActivity: (
    projectId: string,
    entry: Omit<WorkflowActivityEntry, "timestamp"> & { timestamp?: string }
  ) => void;
  clearActivity: (projectId: string) => void;
}

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";

export const ALL_TASK_STATUSES: TaskStatus[] = [
  "inbox",
  "triaged",
  "assigned",
  "in_progress",
  "blocked",
  "in_review",
  "changes_requested",
  "done",
  "cancelled",
  "budget_exceeded",
];

export const TRIGGER_ACTIONS = [
  { value: "auto_assign", label: "Auto-assign agent" },
  { value: "notify", label: "Send notification" },
  { value: "dispatch_agent", label: "Dispatch agent run" },
];

export const useWorkflowStore = create<WorkflowState>()((set) => ({
  config: null,
  loading: false,
  saving: false,
  error: null,
  recentActivityByProject: {},

  fetchWorkflow: async (projectId) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return;

    set({ loading: true, error: null });
    try {
      const api = createApiClient(API_URL);
      const { data } = await api.get<WorkflowConfig>(
        `/api/v1/projects/${projectId}/workflow`,
        { token }
      );
      set({ config: data, error: null });
    } catch {
      set({ error: "Unable to load workflow config" });
    } finally {
      set({ loading: false });
    }
  },

  updateWorkflow: async (projectId, payload) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return false;

    set({ saving: true, error: null });
    try {
      const api = createApiClient(API_URL);
      const { data } = await api.put<WorkflowConfig>(
        `/api/v1/projects/${projectId}/workflow`,
        payload,
        { token }
      );
      set({ config: data, error: null });
      return true;
    } catch {
      set({ error: "Unable to save workflow config" });
      return false;
    } finally {
      set({ saving: false });
    }
  },

  appendActivity: (projectId, entry) =>
    set((state) => {
      const existing = state.recentActivityByProject[projectId] ?? [];
      const nextEntry: WorkflowActivityEntry = {
        ...entry,
        timestamp: entry.timestamp ?? new Date().toISOString(),
      };

      return {
        recentActivityByProject: {
          ...state.recentActivityByProject,
          [projectId]: [nextEntry, ...existing].slice(
            0,
            MAX_WORKFLOW_ACTIVITY_ENTRIES
          ),
        },
      };
    }),

  clearActivity: (projectId) =>
    set((state) => ({
      recentActivityByProject: {
        ...state.recentActivityByProject,
        [projectId]: [],
      },
    })),
}));
