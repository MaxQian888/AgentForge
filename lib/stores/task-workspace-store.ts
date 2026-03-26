"use client";

import { create } from "zustand";
import type {
  TaskDependencyFilter,
  TaskPlanningFilter,
  TaskViewMode,
  TaskWorkspaceFilters,
} from "@/lib/tasks/task-workspace";
import { createDefaultTaskWorkspaceFilters } from "@/lib/tasks/task-workspace";
import type { TaskPriority, TaskStatus } from "./task-store";

export type ContextRailDisplay = "expanded" | "collapsed";
export type TaskWorkspaceDensity = "comfortable" | "compact";

export interface TaskWorkspaceDisplayOptions {
  density: TaskWorkspaceDensity;
  showDescriptions: boolean;
  showLinkedDocs: boolean;
}

interface TaskWorkspaceState {
  viewMode: TaskViewMode;
  filters: TaskWorkspaceFilters;
  selectedTaskId: string | null;
  contextRailDisplay: ContextRailDisplay;
  displayOptions: TaskWorkspaceDisplayOptions;
  setViewMode: (viewMode: TaskViewMode) => void;
  setSearch: (search: string) => void;
  setStatus: (status: "all" | TaskStatus) => void;
  setPriority: (priority: "all" | TaskPriority) => void;
  setAssigneeId: (assigneeId: string | "all") => void;
  setSprintId: (sprintId: string | "all") => void;
  setPlanning: (planning: TaskPlanningFilter) => void;
  setDependency: (dependency: TaskDependencyFilter) => void;
  setContextRailDisplay: (display: ContextRailDisplay) => void;
  setDensity: (density: TaskWorkspaceDensity) => void;
  setShowDescriptions: (showDescriptions: boolean) => void;
  setShowLinkedDocs: (showLinkedDocs: boolean) => void;
  applySavedViewConfig: (config: unknown) => void;
  resetFilters: () => void;
  selectTask: (taskId: string | null) => void;
}

export { createDefaultTaskWorkspaceFilters } from "@/lib/tasks/task-workspace";

export const useTaskWorkspaceStore = create<TaskWorkspaceState>()((set) => ({
  viewMode: "board",
  filters: createDefaultTaskWorkspaceFilters(),
  selectedTaskId: null,
  contextRailDisplay: "expanded",
  displayOptions: {
    density: "comfortable",
    showDescriptions: true,
    showLinkedDocs: false,
  },

  setViewMode: (viewMode) => set({ viewMode }),
  setSearch: (search) =>
    set((state) => ({ filters: { ...state.filters, search } })),
  setStatus: (status) =>
    set((state) => ({ filters: { ...state.filters, status } })),
  setPriority: (priority) =>
    set((state) => ({ filters: { ...state.filters, priority } })),
  setAssigneeId: (assigneeId) =>
    set((state) => ({ filters: { ...state.filters, assigneeId } })),
  setSprintId: (sprintId) =>
    set((state) => ({ filters: { ...state.filters, sprintId } })),
  setPlanning: (planning) =>
    set((state) => ({ filters: { ...state.filters, planning } })),
  setDependency: (dependency) =>
    set((state) => ({ filters: { ...state.filters, dependency } })),
  setContextRailDisplay: (contextRailDisplay) => set({ contextRailDisplay }),
  setDensity: (density) =>
    set((state) => ({
      displayOptions: { ...state.displayOptions, density },
    })),
  setShowDescriptions: (showDescriptions) =>
    set((state) => ({
      displayOptions: { ...state.displayOptions, showDescriptions },
    })),
  setShowLinkedDocs: (showLinkedDocs) =>
    set((state) => ({
      displayOptions: { ...state.displayOptions, showLinkedDocs },
    })),
  applySavedViewConfig: (config) =>
    set((state) => {
      if (!config || typeof config !== "object") {
        return state;
      }
      const raw = config as Record<string, unknown>;
      const nextFilters = { ...state.filters };
      const layout = typeof raw.layout === "string" ? raw.layout : state.viewMode;
      const filters = Array.isArray(raw.filters) ? raw.filters : [];
      for (const entry of filters) {
        if (!entry || typeof entry !== "object") continue;
        const field = typeof (entry as { field?: unknown }).field === "string" ? (entry as { field: string }).field : "";
        const value = (entry as { value?: unknown }).value;
        switch (field) {
          case "status":
            nextFilters.status = typeof value === "string" ? (value as "all" | TaskStatus) : nextFilters.status;
            break;
          case "priority":
            nextFilters.priority = typeof value === "string" ? (value as "all" | TaskPriority) : nextFilters.priority;
            break;
          case "assigneeId":
          case "assignee_id":
            nextFilters.assigneeId = typeof value === "string" ? value : nextFilters.assigneeId;
            break;
          case "sprintId":
          case "sprint_id":
            nextFilters.sprintId = typeof value === "string" ? value : nextFilters.sprintId;
            break;
          case "search":
            nextFilters.search = typeof value === "string" ? value : nextFilters.search;
            break;
        }
      }
      return {
        viewMode: layout as TaskViewMode,
        filters: nextFilters,
      };
    }),
  resetFilters: () => set({ filters: createDefaultTaskWorkspaceFilters() }),
  selectTask: (selectedTaskId) => set({ selectedTaskId }),
}));
