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

export const DEFAULT_BOARD_COLUMN_ORDER: TaskStatus[] = [
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

const BOARD_COLUMN_ORDER_STORAGE_KEY = "task-workspace-board-columns";
const HIDDEN_BOARD_COLUMNS_STORAGE_KEY = "task-workspace-hidden-columns";

export interface TaskWorkspaceDisplayOptions {
  density: TaskWorkspaceDensity;
  showDescriptions: boolean;
  showLinkedDocs: boolean;
  boardColumnOrder?: TaskStatus[];
  hiddenBoardColumns?: TaskStatus[];
}

function readStoredTaskStatuses(storageKey: string): TaskStatus[] | undefined {
  if (typeof window === "undefined") {
    return undefined;
  }

  try {
    const raw = localStorage.getItem(storageKey);
    if (!raw) {
      return undefined;
    }

    const parsed = JSON.parse(raw);
    if (!Array.isArray(parsed)) {
      return undefined;
    }

    return parsed.filter((value): value is TaskStatus =>
      DEFAULT_BOARD_COLUMN_ORDER.includes(value as TaskStatus),
    );
  } catch {
    return undefined;
  }
}

function persistTaskStatuses(storageKey: string, statuses: TaskStatus[]) {
  if (typeof window === "undefined") {
    return;
  }

  try {
    localStorage.setItem(storageKey, JSON.stringify(statuses));
  } catch {}
}

function normalizeBoardColumnOrder(order?: TaskStatus[]): TaskStatus[] {
  const next = Array.isArray(order) ? [...order] : [];
  const seen = new Set<TaskStatus>();
  const normalized: TaskStatus[] = [];

  for (const status of next) {
    if (!DEFAULT_BOARD_COLUMN_ORDER.includes(status) || seen.has(status)) {
      continue;
    }
    normalized.push(status);
    seen.add(status);
  }

  for (const status of DEFAULT_BOARD_COLUMN_ORDER) {
    if (!seen.has(status)) {
      normalized.push(status);
    }
  }

  return normalized;
}

function normalizeHiddenBoardColumns(columns?: TaskStatus[]): TaskStatus[] {
  if (!Array.isArray(columns)) {
    return [];
  }

  const seen = new Set<TaskStatus>();
  return columns.filter((status): status is TaskStatus => {
    if (!DEFAULT_BOARD_COLUMN_ORDER.includes(status) || seen.has(status)) {
      return false;
    }
    seen.add(status);
    return true;
  });
}

interface TaskWorkspaceState {
  viewMode: TaskViewMode;
  filters: TaskWorkspaceFilters;
  selectedTaskId: string | null;
  selectedTaskIds: string[];
  contextRailDisplay: ContextRailDisplay;
  displayOptions: TaskWorkspaceDisplayOptions;
  setViewMode: (viewMode: TaskViewMode) => void;
  setSearch: (search: string) => void;
  setStatus: (status: "all" | TaskStatus) => void;
  setPriority: (priority: "all" | TaskPriority) => void;
  setAssigneeId: (assigneeId: string | "all") => void;
  setSprintId: (sprintId: string | "all") => void;
  setLabels: (labels: string[]) => void;
  setDueDateRange: (range: { start: string; end: string }) => void;
  setPlanning: (planning: TaskPlanningFilter) => void;
  setDependency: (dependency: TaskDependencyFilter) => void;
  setCustomFieldFilter: (fieldId: string, value: string | "all") => void;
  setContextRailDisplay: (display: ContextRailDisplay) => void;
  setDensity: (density: TaskWorkspaceDensity) => void;
  setShowDescriptions: (showDescriptions: boolean) => void;
  setShowLinkedDocs: (showLinkedDocs: boolean) => void;
  setBoardColumnOrder: (boardColumnOrder: TaskStatus[]) => void;
  setHiddenBoardColumns: (hiddenBoardColumns: TaskStatus[]) => void;
  applySavedViewConfig: (config: unknown) => void;
  resetFilters: () => void;
  selectTask: (taskId: string | null) => void;
  toggleTaskSelection: (taskId: string) => void;
  selectAllVisible: (taskIds: string[]) => void;
  clearSelection: () => void;
}

export { createDefaultTaskWorkspaceFilters } from "@/lib/tasks/task-workspace";

export const useTaskWorkspaceStore = create<TaskWorkspaceState>()((set) => ({
  viewMode: "board",
  filters: createDefaultTaskWorkspaceFilters(),
  selectedTaskId: null,
  selectedTaskIds: [],
  contextRailDisplay: "expanded",
  displayOptions: {
    density: "comfortable",
    showDescriptions: true,
    showLinkedDocs: false,
    boardColumnOrder: normalizeBoardColumnOrder(
      readStoredTaskStatuses(BOARD_COLUMN_ORDER_STORAGE_KEY),
    ),
    hiddenBoardColumns: normalizeHiddenBoardColumns(
      readStoredTaskStatuses(HIDDEN_BOARD_COLUMNS_STORAGE_KEY),
    ),
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
  setLabels: (labels) =>
    set((state) => ({ filters: { ...state.filters, labels } })),
  setDueDateRange: ({ start, end }) =>
    set((state) => ({
      filters: {
        ...state.filters,
        dueDateStart: start,
        dueDateEnd: end,
      },
    })),
  setPlanning: (planning) =>
    set((state) => ({ filters: { ...state.filters, planning } })),
  setDependency: (dependency) =>
    set((state) => ({ filters: { ...state.filters, dependency } })),
  setCustomFieldFilter: (fieldId, value) =>
    set((state) => {
      const nextFilters = { ...state.filters.customFieldFilters };
      if (!fieldId || value === "all" || value === "") {
        delete nextFilters[fieldId];
      } else {
        nextFilters[fieldId] = value;
      }
      return {
        filters: {
          ...state.filters,
          customFieldFilters: nextFilters,
        },
      };
    }),
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
  setBoardColumnOrder: (boardColumnOrder) => {
    const normalized = normalizeBoardColumnOrder(boardColumnOrder);
    persistTaskStatuses(BOARD_COLUMN_ORDER_STORAGE_KEY, normalized);
    set((state) => ({
      displayOptions: {
        ...state.displayOptions,
        boardColumnOrder: normalized,
      },
    }));
  },
  setHiddenBoardColumns: (hiddenBoardColumns) => {
    const normalized = normalizeHiddenBoardColumns(hiddenBoardColumns);
    persistTaskStatuses(HIDDEN_BOARD_COLUMNS_STORAGE_KEY, normalized);
    set((state) => ({
      displayOptions: {
        ...state.displayOptions,
        hiddenBoardColumns: normalized,
      },
    }));
  },
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
          case "dueDateStart":
          case "due_date_start":
            nextFilters.dueDateStart = typeof value === "string" ? value : nextFilters.dueDateStart;
            break;
          case "dueDateEnd":
          case "due_date_end":
            nextFilters.dueDateEnd = typeof value === "string" ? value : nextFilters.dueDateEnd;
            break;
          case "sprintId":
          case "sprint_id":
            nextFilters.sprintId = typeof value === "string" ? value : nextFilters.sprintId;
            break;
          case "search":
            nextFilters.search = typeof value === "string" ? value : nextFilters.search;
            break;
          default:
            if (field.startsWith("cf:") && typeof value === "string") {
              nextFilters.customFieldFilters[field.slice(3)] = value;
            }
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
  toggleTaskSelection: (taskId) =>
    set((state) => {
      const idx = state.selectedTaskIds.indexOf(taskId);
      if (idx === -1) {
        return { selectedTaskIds: [...state.selectedTaskIds, taskId] };
      }
      return { selectedTaskIds: state.selectedTaskIds.filter((id) => id !== taskId) };
    }),
  selectAllVisible: (taskIds) => set({ selectedTaskIds: taskIds }),
  clearSelection: () => set({ selectedTaskIds: [] }),
}));
