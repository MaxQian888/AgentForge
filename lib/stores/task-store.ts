"use client";

import { create } from "zustand";
import { createApiClient } from "@/lib/api-client";
import { useAuthStore } from "./auth-store";

export type TaskStatus =
  | "inbox"
  | "triaged"
  | "assigned"
  | "in_progress"
  | "in_review"
  | "done"
  | "blocked"
  | "changes_requested"
  | "cancelled"
  | "budget_exceeded";

export type TaskPriority = "urgent" | "high" | "medium" | "low";
export type TaskExecutionMode = "human" | "agent";

export type TaskProgressHealth = "healthy" | "warning" | "stalled";

export interface TaskProgress {
  lastActivityAt: string;
  lastActivitySource: string;
  lastTransitionAt: string;
  healthStatus: TaskProgressHealth;
  riskReason: string;
  riskSinceAt: string | null;
  lastAlertState: string;
  lastAlertAt: string | null;
  lastRecoveredAt: string | null;
}

export interface Task {
  id: string;
  projectId: string;
  parentId?: string | null;
  sprintId?: string | null;
  executionMode?: TaskExecutionMode | null;
  title: string;
  description: string;
  status: TaskStatus;
  priority: TaskPriority;
  assigneeId: string | null;
  assigneeType: "human" | "agent" | null;
  assigneeName: string | null;
  cost: number | null;
  budgetUsd: number;
  spentUsd: number;
  agentBranch: string;
  agentWorktree: string;
  agentSessionId: string;
  blockedBy: string[];
  plannedStartAt: string | null;
  plannedEndAt: string | null;
  progress?: TaskProgress | null;
  createdAt: string;
  updatedAt: string;
}

interface TaskApiShape {
  id: string;
  projectId: string;
  parentId?: string | null;
  sprintId?: string | null;
  executionMode?: TaskExecutionMode | null;
  title: string;
  description: string;
  status: TaskStatus;
  priority: TaskPriority;
  assigneeId?: string | null;
  assigneeType?: "human" | "agent" | null;
  assigneeName?: string | null;
  cost?: number | null;
  budgetUsd?: number | null;
  spentUsd?: number | null;
  agentBranch?: string | null;
  agentWorktree?: string | null;
  agentSessionId?: string | null;
  blockedBy?: string[] | null;
  plannedStartAt?: string | null;
  plannedEndAt?: string | null;
  progress?: Partial<TaskProgress> | null;
  createdAt: string;
  updatedAt: string;
}

interface TaskListResponse {
  items: TaskApiShape[];
  total: number;
  page: number;
  limit: number;
}

interface TaskDispatchResponse {
  task: TaskApiShape;
  dispatch: {
    status: string;
    reason?: string;
  };
}

export interface TaskDecompositionResult {
  parentTask: Task;
  summary: string;
  subtasks: Task[];
}

interface TaskDecompositionApiResponse {
  parentTask: TaskApiShape;
  summary: string;
  subtasks: TaskApiShape[];
}

interface TaskState {
  tasks: Task[];
  loading: boolean;
  error: string | null;
  fetchTasks: (projectId: string) => Promise<void>;
  createTask: (data: Partial<Task>) => Promise<void>;
  updateTask: (id: string, data: Partial<Task>) => Promise<void>;
  transitionTask: (id: string, newStatus: TaskStatus) => Promise<void>;
  upsertTask: (task: TaskApiShape) => void;
  removeTask: (id: string) => void;
  assignTask: (
    id: string,
    assigneeId: string,
    type: "human" | "agent",
    assigneeName?: string
  ) => Promise<void>;
  decomposeTask: (id: string) => Promise<TaskDecompositionResult | null>;
}

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";

function normalizeTaskProgress(
  progress: TaskApiShape["progress"]
): TaskProgress | null {
  if (!progress) {
    return null;
  }

  return {
    lastActivityAt: progress.lastActivityAt ?? "",
    lastActivitySource: progress.lastActivitySource ?? "",
    lastTransitionAt: progress.lastTransitionAt ?? "",
    healthStatus: (progress.healthStatus ?? "healthy") as TaskProgressHealth,
    riskReason: progress.riskReason ?? "",
    riskSinceAt: progress.riskSinceAt ?? null,
    lastAlertState: progress.lastAlertState ?? "",
    lastAlertAt: progress.lastAlertAt ?? null,
    lastRecoveredAt: progress.lastRecoveredAt ?? null,
  };
}

function normalizeTask(task: TaskApiShape): Task {
  const spentUsd = task.spentUsd ?? task.cost ?? 0;
  return {
    id: task.id,
    projectId: task.projectId,
    parentId: task.parentId ?? null,
    sprintId: task.sprintId ?? null,
    executionMode: task.executionMode ?? null,
    title: task.title,
    description: task.description,
    status: task.status,
    priority: task.priority,
    assigneeId: task.assigneeId ?? null,
    assigneeType: task.assigneeType ?? null,
    assigneeName: task.assigneeName ?? null,
    cost: task.cost ?? spentUsd ?? null,
    budgetUsd: task.budgetUsd ?? 0,
    spentUsd,
    agentBranch: task.agentBranch ?? "",
    agentWorktree: task.agentWorktree ?? "",
    agentSessionId: task.agentSessionId ?? "",
    blockedBy: task.blockedBy ?? [],
    plannedStartAt: task.plannedStartAt ?? null,
    plannedEndAt: task.plannedEndAt ?? null,
    progress: normalizeTaskProgress(task.progress),
    createdAt: task.createdAt,
    updatedAt: task.updatedAt,
  };
}

function extractTaskPayload(payload: TaskApiShape | TaskDispatchResponse): TaskApiShape {
  if ("task" in payload) {
    return payload.task;
  }
  return payload;
}

function upsertNormalizedTask(tasks: Task[], task: Task): Task[] {
  const existingIndex = tasks.findIndex((item) => item.id === task.id);
  if (existingIndex === -1) {
    return [...tasks, task];
  }

  return tasks.map((item) => (item.id === task.id ? task : item));
}

export const useTaskStore = create<TaskState>()((set, get) => ({
  tasks: [],
  loading: false,
  error: null,

  fetchTasks: async (projectId) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return;

    set({ loading: true, error: null });
    try {
      const api = createApiClient(API_URL);
      const { data } = await api.get<TaskListResponse>(
        `/api/v1/projects/${projectId}/tasks`,
        { token }
      );
      set({ tasks: data.items.map(normalizeTask), error: null });
    } catch {
      set({ error: "Unable to load tasks" });
    } finally {
      set({ loading: false });
    }
  },

  createTask: async (data) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return;

    const api = createApiClient(API_URL);
    const { data: task } = await api.post<TaskApiShape>(
      `/api/v1/projects/${data.projectId}/tasks`,
      data,
      { token }
    );

    set((state) => ({
      tasks: upsertNormalizedTask(state.tasks, normalizeTask(task)),
    }));
  },

  updateTask: async (id, data) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return;

    const previousTask = get().tasks.find((task) => task.id === id);
    set((state) => ({
      tasks: state.tasks.map((task) =>
        task.id === id ? { ...task, ...data } : task
      ),
    }));

    try {
      const api = createApiClient(API_URL);
      const { data: task } = await api.put<TaskApiShape>(`/api/v1/tasks/${id}`, data, {
        token,
      });
      set((state) => ({
        tasks: state.tasks.map((item) =>
          item.id === id ? normalizeTask(task) : item
        ),
      }));
    } catch (error) {
      if (previousTask) {
        set((state) => ({
          tasks: state.tasks.map((task) =>
            task.id === id ? previousTask : task
          ),
        }));
      }
      throw error;
    }
  },

  transitionTask: async (id, newStatus) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return;

    const previousTask = get().tasks.find((task) => task.id === id);
    set((state) => ({
      tasks: state.tasks.map((task) =>
        task.id === id ? { ...task, status: newStatus } : task
      ),
    }));

    try {
      const api = createApiClient(API_URL);
      const { data: task } = await api.post<TaskApiShape>(
        `/api/v1/tasks/${id}/transition`,
        { status: newStatus },
        { token }
      );
      set((state) => ({
        tasks: state.tasks.map((item) =>
          item.id === id ? normalizeTask(task) : item
        ),
      }));
    } catch (error) {
      if (previousTask) {
        set((state) => ({
          tasks: state.tasks.map((task) =>
            task.id === id ? previousTask : task
          ),
        }));
      }
      throw error;
    }
  },

  assignTask: async (id, assigneeId, type, assigneeName) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return;

    const api = createApiClient(API_URL);
    const { data } = await api.post<TaskApiShape | TaskDispatchResponse>(
      `/api/v1/tasks/${id}/assign`,
      { assigneeId, assigneeType: type },
      { token }
    );
    const task = extractTaskPayload(data);
    const normalized = normalizeTask({
      ...task,
      assigneeName: task.assigneeName ?? assigneeName ?? null,
    });

    set((state) => ({
      tasks: state.tasks.map((item) =>
        item.id === id ? normalized : item
      ),
    }));
  },

  decomposeTask: async (id) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return null;

    const api = createApiClient(API_URL);
    const { data } = await api.post<TaskDecompositionApiResponse>(
      `/api/v1/tasks/${id}/decompose`,
      {},
      { token }
    );

    const parentTask = normalizeTask(data.parentTask);
    const subtasks = data.subtasks.map(normalizeTask);

    set((state) => {
      let tasks = upsertNormalizedTask(state.tasks, parentTask);
      for (const subtask of subtasks) {
        tasks = upsertNormalizedTask(tasks, subtask);
      }
      return { tasks };
    });

    return {
      parentTask,
      summary: data.summary,
      subtasks,
    };
  },

  upsertTask: (task) => {
    const normalized = normalizeTask(task);
    set((state) => ({
      tasks: upsertNormalizedTask(state.tasks, normalized),
    }));
  },

  removeTask: (id) => {
    set((state) => ({
      tasks: state.tasks.filter((task) => task.id !== id),
    }));
  },
}));
