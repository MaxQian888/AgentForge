"use client";

import { create } from "zustand";
import { createApiClient } from "@/lib/api-client";
import { useAuthStore } from "./auth-store";

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";

export interface SavedView {
  id: string;
  projectId: string;
  name: string;
  ownerId?: string | null;
  isDefault: boolean;
  sharedWith: unknown;
  config: unknown;
  createdAt: string;
  updatedAt: string;
}

interface SavedViewState {
  viewsByProject: Record<string, SavedView[]>;
  currentViewByProject: Record<string, string | null>;
  loadingByProject: Record<string, boolean>;
  fetchViews: (projectId: string) => Promise<void>;
  selectView: (projectId: string, viewId: string | null) => void;
  createView: (projectId: string, input: { name: string; ownerId?: string | null; isDefault?: boolean; sharedWith?: unknown; config: unknown }) => Promise<void>;
  updateView: (projectId: string, viewId: string, input: Partial<Omit<SavedView, "id" | "projectId" | "createdAt" | "updatedAt">>) => Promise<void>;
  deleteView: (projectId: string, viewId: string) => Promise<void>;
  setDefaultView: (projectId: string, viewId: string) => Promise<void>;
}

function getToken() {
  const state = useAuthStore.getState() as { accessToken?: string | null; token?: string | null };
  return state.accessToken ?? state.token ?? null;
}

const getApi = () => createApiClient(API_URL);

export const useSavedViewStore = create<SavedViewState>()((set) => ({
  viewsByProject: {},
  currentViewByProject: {},
  loadingByProject: {},

  fetchViews: async (projectId) => {
    const token = getToken();
    if (!token) return;
    set((state) => ({ loadingByProject: { ...state.loadingByProject, [projectId]: true } }));
    try {
      const { data } = await getApi().get<SavedView[]>(`/api/v1/projects/${projectId}/views`, { token });
      const views = data ?? [];
      set((state) => ({
        viewsByProject: { ...state.viewsByProject, [projectId]: views },
        currentViewByProject: {
          ...state.currentViewByProject,
          [projectId]: state.currentViewByProject[projectId] ?? views.find((item) => item.isDefault)?.id ?? views[0]?.id ?? null,
        },
      }));
    } finally {
      set((state) => ({ loadingByProject: { ...state.loadingByProject, [projectId]: false } }));
    }
  },

  selectView: (projectId, viewId) =>
    set((state) => ({
      currentViewByProject: { ...state.currentViewByProject, [projectId]: viewId },
    })),

  createView: async (projectId, input) => {
    const token = getToken();
    if (!token) return;
    const { data } = await getApi().post<SavedView>(`/api/v1/projects/${projectId}/views`, input, { token });
    set((state) => ({
      viewsByProject: { ...state.viewsByProject, [projectId]: [...(state.viewsByProject[projectId] ?? []), data] },
      currentViewByProject: { ...state.currentViewByProject, [projectId]: data.id },
    }));
  },

  updateView: async (projectId, viewId, input) => {
    const token = getToken();
    if (!token) return;
    const { data } = await getApi().put<SavedView>(`/api/v1/projects/${projectId}/views/${viewId}`, input, { token });
    set((state) => ({
      viewsByProject: {
        ...state.viewsByProject,
        [projectId]: (state.viewsByProject[projectId] ?? []).map((item) => (item.id === viewId ? data : item)),
      },
    }));
  },

  deleteView: async (projectId, viewId) => {
    const token = getToken();
    if (!token) return;
    await getApi().delete(`/api/v1/projects/${projectId}/views/${viewId}`, { token });
    set((state) => ({
      viewsByProject: {
        ...state.viewsByProject,
        [projectId]: (state.viewsByProject[projectId] ?? []).filter((item) => item.id !== viewId),
      },
      currentViewByProject: {
        ...state.currentViewByProject,
        [projectId]:
          state.currentViewByProject[projectId] === viewId
            ? (state.viewsByProject[projectId] ?? []).find((item) => item.id !== viewId)?.id ?? null
            : state.currentViewByProject[projectId] ?? null,
      },
    }));
  },

  setDefaultView: async (projectId, viewId) => {
    const token = getToken();
    if (!token) return;
    await getApi().post(`/api/v1/projects/${projectId}/views/${viewId}/default`, {}, { token });
    set((state) => ({
      viewsByProject: {
        ...state.viewsByProject,
        [projectId]: (state.viewsByProject[projectId] ?? []).map((item) => ({ ...item, isDefault: item.id === viewId })),
      },
      currentViewByProject: { ...state.currentViewByProject, [projectId]: viewId },
    }));
  },
}));
