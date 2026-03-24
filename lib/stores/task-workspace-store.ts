"use client";

import { create } from "zustand";
import type {
  TaskPlanningFilter,
  TaskViewMode,
  TaskWorkspaceFilters,
} from "@/lib/tasks/task-workspace";
import { createDefaultTaskWorkspaceFilters } from "@/lib/tasks/task-workspace";
import type { TaskPriority, TaskStatus } from "./task-store";

export type ContextRailDisplay = "expanded" | "collapsed";

interface TaskWorkspaceState {
  viewMode: TaskViewMode;
  filters: TaskWorkspaceFilters;
  selectedTaskId: string | null;
  contextRailDisplay: ContextRailDisplay;
  setViewMode: (viewMode: TaskViewMode) => void;
  setSearch: (search: string) => void;
  setStatus: (status: "all" | TaskStatus) => void;
  setPriority: (priority: "all" | TaskPriority) => void;
  setAssigneeId: (assigneeId: string | "all") => void;
  setPlanning: (planning: TaskPlanningFilter) => void;
  setContextRailDisplay: (display: ContextRailDisplay) => void;
  resetFilters: () => void;
  selectTask: (taskId: string | null) => void;
}

export { createDefaultTaskWorkspaceFilters } from "@/lib/tasks/task-workspace";

export const useTaskWorkspaceStore = create<TaskWorkspaceState>()((set) => ({
  viewMode: "board",
  filters: createDefaultTaskWorkspaceFilters(),
  selectedTaskId: null,
  contextRailDisplay: "expanded",

  setViewMode: (viewMode) => set({ viewMode }),
  setSearch: (search) =>
    set((state) => ({ filters: { ...state.filters, search } })),
  setStatus: (status) =>
    set((state) => ({ filters: { ...state.filters, status } })),
  setPriority: (priority) =>
    set((state) => ({ filters: { ...state.filters, priority } })),
  setAssigneeId: (assigneeId) =>
    set((state) => ({ filters: { ...state.filters, assigneeId } })),
  setPlanning: (planning) =>
    set((state) => ({ filters: { ...state.filters, planning } })),
  setContextRailDisplay: (contextRailDisplay) => set({ contextRailDisplay }),
  resetFilters: () => set({ filters: createDefaultTaskWorkspaceFilters() }),
  selectTask: (selectedTaskId) => set({ selectedTaskId }),
}));
