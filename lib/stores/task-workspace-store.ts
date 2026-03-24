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
  resetFilters: () => set({ filters: createDefaultTaskWorkspaceFilters() }),
  selectTask: (selectedTaskId) => set({ selectedTaskId }),
}));
