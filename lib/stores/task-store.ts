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
  | "done";

export type TaskPriority = "urgent" | "high" | "medium" | "low";

export interface Task {
  id: string;
  projectId: string;
  title: string;
  description: string;
  status: TaskStatus;
  priority: TaskPriority;
  assigneeId: string | null;
  assigneeType: "human" | "agent" | null;
  assigneeName: string | null;
  cost: number | null;
  createdAt: string;
  updatedAt: string;
}

interface TaskState {
  tasks: Task[];
  loading: boolean;
  fetchTasks: (projectId: string) => Promise<void>;
  createTask: (data: Partial<Task>) => Promise<void>;
  updateTask: (id: string, data: Partial<Task>) => Promise<void>;
  transitionTask: (id: string, newStatus: TaskStatus) => void;
  assignTask: (
    id: string,
    assigneeId: string,
    type: "human" | "agent"
  ) => Promise<void>;
}

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";

export const useTaskStore = create<TaskState>()((set, get) => ({
  tasks: [],
  loading: false,

  fetchTasks: async (projectId) => {
    const token = useAuthStore.getState().token;
    if (!token) return;
    set({ loading: true });
    try {
      const api = createApiClient(API_URL);
      const { data } = await api.get<Task[]>(
        `/api/v1/projects/${projectId}/tasks`,
        { token }
      );
      set({ tasks: data });
    } finally {
      set({ loading: false });
    }
  },

  createTask: async (data) => {
    const token = useAuthStore.getState().token;
    if (!token) return;
    const api = createApiClient(API_URL);
    const { data: task } = await api.post<Task>(
      `/api/v1/projects/${data.projectId}/tasks`,
      data,
      { token }
    );
    set((s) => ({ tasks: [...s.tasks, task] }));
  },

  updateTask: async (id, data) => {
    const token = useAuthStore.getState().token;
    if (!token) return;
    // Optimistic update
    set((s) => ({
      tasks: s.tasks.map((t) => (t.id === id ? { ...t, ...data } : t)),
    }));
    const api = createApiClient(API_URL);
    await api.put(`/api/v1/tasks/${id}`, data, { token });
  },

  transitionTask: (id, newStatus) => {
    // Optimistic update
    set((s) => ({
      tasks: s.tasks.map((t) =>
        t.id === id ? { ...t, status: newStatus } : t
      ),
    }));
    const token = useAuthStore.getState().token;
    if (!token) return;
    const api = createApiClient(API_URL);
    api.put(`/api/v1/tasks/${id}/transition`, { status: newStatus }, { token });
  },

  assignTask: async (id, assigneeId, type) => {
    const token = useAuthStore.getState().token;
    if (!token) return;
    const api = createApiClient(API_URL);
    const { data: task } = await api.put<Task>(
      `/api/v1/tasks/${id}/assign`,
      { assignee_id: assigneeId, type },
      { token }
    );
    set((s) => ({
      tasks: s.tasks.map((t) => (t.id === id ? task : t)),
    }));
  },
}));
