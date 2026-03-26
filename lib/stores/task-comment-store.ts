"use client";

import { create } from "zustand";
import { createApiClient } from "@/lib/api-client";
import { useAuthStore } from "./auth-store";

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";

export interface TaskComment {
  id: string;
  taskId: string;
  parentCommentId?: string | null;
  body: string;
  mentions: string[];
  resolvedAt?: string | null;
  createdBy: string;
  createdAt: string;
  updatedAt: string;
  deletedAt?: string | null;
}

interface TaskCommentState {
  commentsByTask: Record<string, TaskComment[]>;
  loading: boolean;
  error: string | null;
  fetchComments: (projectId: string, taskId: string) => Promise<void>;
  createComment: (input: {
    projectId: string;
    taskId: string;
    body: string;
    parentCommentId?: string | null;
  }) => Promise<void>;
  setResolved: (input: {
    projectId: string;
    taskId: string;
    commentId: string;
    resolved: boolean;
  }) => Promise<void>;
  deleteComment: (projectId: string, taskId: string, commentId: string) => Promise<void>;
  upsertComment: (comment: TaskComment) => void;
  removeComment: (taskId: string, commentId: string) => void;
}

function getApi() {
  const token = useAuthStore.getState().accessToken;
  if (!token) {
    throw new Error("missing access token");
  }
  return { token, api: createApiClient(API_URL) };
}

function normalizeComment(raw: Record<string, unknown>): TaskComment {
  return {
    id: String(raw.id ?? ""),
    taskId: String(raw.taskId ?? ""),
    parentCommentId: typeof raw.parentCommentId === "string" ? raw.parentCommentId : null,
    body: String(raw.body ?? ""),
    mentions: Array.isArray(raw.mentions) ? raw.mentions.map(String) : [],
    resolvedAt: typeof raw.resolvedAt === "string" ? raw.resolvedAt : null,
    createdBy: String(raw.createdBy ?? ""),
    createdAt: String(raw.createdAt ?? new Date().toISOString()),
    updatedAt: String(raw.updatedAt ?? new Date().toISOString()),
    deletedAt: typeof raw.deletedAt === "string" ? raw.deletedAt : null,
  };
}

export const useTaskCommentStore = create<TaskCommentState>()((set, get) => ({
  commentsByTask: {},
  loading: false,
  error: null,

  fetchComments: async (projectId, taskId) => {
    const { api, token } = getApi();
    set({ loading: true, error: null });
    try {
      const { data } = await api.get<Record<string, unknown>[]>(
        `/api/v1/projects/${projectId}/tasks/${taskId}/comments`,
        { token },
      );
      set((state) => ({
        commentsByTask: {
          ...state.commentsByTask,
          [taskId]: data.map(normalizeComment),
        },
      }));
    } catch (error) {
      set({ error: error instanceof Error ? error.message : "Failed to load task comments" });
    } finally {
      set({ loading: false });
    }
  },

  createComment: async (input) => {
    const { api, token } = getApi();
    const { data } = await api.post<Record<string, unknown>>(
      `/api/v1/projects/${input.projectId}/tasks/${input.taskId}/comments`,
      {
        body: input.body,
        parentCommentId: input.parentCommentId ?? undefined,
      },
      { token },
    );
    get().upsertComment(normalizeComment(data));
  },

  setResolved: async ({ projectId, taskId, commentId, resolved }) => {
    const { api, token } = getApi();
    const { data } = await api.patch<Record<string, unknown>>(
      `/api/v1/projects/${projectId}/tasks/${taskId}/comments/${commentId}`,
      { resolved },
      { token },
    );
    get().upsertComment(normalizeComment(data));
  },

  deleteComment: async (projectId, taskId, commentId) => {
    const { api, token } = getApi();
    await api.delete(`/api/v1/projects/${projectId}/tasks/${taskId}/comments/${commentId}`, {
      token,
    });
    await get().fetchComments(projectId, taskId);
  },

  upsertComment: (comment) => {
    set((state) => {
      const current = state.commentsByTask[comment.taskId] ?? [];
      const existingIndex = current.findIndex((item) => item.id === comment.id);
      const next =
        existingIndex === -1
          ? [...current, comment]
          : current.map((item) => (item.id === comment.id ? comment : item));
      return {
        commentsByTask: {
          ...state.commentsByTask,
          [comment.taskId]: next,
        },
      };
    });
  },

  removeComment: (taskId, commentId) => {
    set((state) => ({
      commentsByTask: {
        ...state.commentsByTask,
        [taskId]: (state.commentsByTask[taskId] ?? []).filter((comment) => comment.id !== commentId),
      },
    }));
  },
}));
