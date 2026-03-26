"use client";

import { create } from "zustand";
import { createApiClient } from "@/lib/api-client";
import { useAuthStore } from "./auth-store";

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";

export interface CustomFieldDefinition {
  id: string;
  projectId: string;
  name: string;
  fieldType: string;
  options: unknown;
  sortOrder: number;
  required: boolean;
  createdAt: string;
  updatedAt: string;
}

export interface CustomFieldValue {
  id: string;
  taskId: string;
  fieldDefId: string;
  value: unknown;
  createdAt: string;
  updatedAt: string;
}

interface CustomFieldState {
  definitionsByProject: Record<string, CustomFieldDefinition[]>;
  valuesByTask: Record<string, CustomFieldValue[]>;
  loadingByProject: Record<string, boolean>;
  errorByProject: Record<string, string | null>;
  fetchDefinitions: (projectId: string) => Promise<void>;
  createDefinition: (projectId: string, input: { name: string; fieldType: string; options?: unknown; required?: boolean }) => Promise<void>;
  updateDefinition: (projectId: string, fieldId: string, input: Partial<Pick<CustomFieldDefinition, "name" | "fieldType" | "options" | "required" | "sortOrder">>) => Promise<void>;
  deleteDefinition: (projectId: string, fieldId: string) => Promise<void>;
  reorderDefinitions: (projectId: string, fieldIds: string[]) => Promise<void>;
  fetchTaskValues: (projectId: string, taskId: string) => Promise<void>;
  setTaskValue: (projectId: string, taskId: string, fieldId: string, value: unknown) => Promise<void>;
  clearTaskValue: (projectId: string, taskId: string, fieldId: string) => Promise<void>;
}

function getToken() {
  const state = useAuthStore.getState() as { accessToken?: string | null; token?: string | null };
  return state.accessToken ?? state.token ?? null;
}

function getApi() {
  return createApiClient(API_URL);
}

export const useCustomFieldStore = create<CustomFieldState>()((set, get) => ({
  definitionsByProject: {},
  valuesByTask: {},
  loadingByProject: {},
  errorByProject: {},

  fetchDefinitions: async (projectId) => {
    const token = getToken();
    if (!token) return;
    set((state) => ({
      loadingByProject: { ...state.loadingByProject, [projectId]: true },
      errorByProject: { ...state.errorByProject, [projectId]: null },
    }));
    try {
      const { data } = await getApi().get<CustomFieldDefinition[]>(`/api/v1/projects/${projectId}/fields`, { token });
      set((state) => ({
        definitionsByProject: { ...state.definitionsByProject, [projectId]: data ?? [] },
      }));
    } catch {
      set((state) => ({
        errorByProject: { ...state.errorByProject, [projectId]: "Unable to load custom fields" },
      }));
    } finally {
      set((state) => ({
        loadingByProject: { ...state.loadingByProject, [projectId]: false },
      }));
    }
  },

  createDefinition: async (projectId, input) => {
    const token = getToken();
    if (!token) return;
    const { data } = await getApi().post<CustomFieldDefinition>(`/api/v1/projects/${projectId}/fields`, input, { token });
    set((state) => ({
      definitionsByProject: {
        ...state.definitionsByProject,
        [projectId]: [...(state.definitionsByProject[projectId] ?? []), data],
      },
    }));
  },

  updateDefinition: async (projectId, fieldId, input) => {
    const token = getToken();
    if (!token) return;
    const { data } = await getApi().put<CustomFieldDefinition>(`/api/v1/projects/${projectId}/fields/${fieldId}`, input, { token });
    set((state) => ({
      definitionsByProject: {
        ...state.definitionsByProject,
        [projectId]: (state.definitionsByProject[projectId] ?? []).map((item) => (item.id === fieldId ? data : item)),
      },
    }));
  },

  deleteDefinition: async (projectId, fieldId) => {
    const token = getToken();
    if (!token) return;
    await getApi().delete(`/api/v1/projects/${projectId}/fields/${fieldId}`, { token });
    set((state) => ({
      definitionsByProject: {
        ...state.definitionsByProject,
        [projectId]: (state.definitionsByProject[projectId] ?? []).filter((item) => item.id !== fieldId),
      },
    }));
  },

  reorderDefinitions: async (projectId, fieldIds) => {
    const token = getToken();
    if (!token) return;
    await getApi().put(`/api/v1/projects/${projectId}/fields/reorder`, { fieldIds }, { token });
    const definitions = get().definitionsByProject[projectId] ?? [];
    const byId = new Map(definitions.map((item) => [item.id, item]));
    set((state) => ({
      definitionsByProject: {
        ...state.definitionsByProject,
        [projectId]: fieldIds.map((id, index) => ({ ...byId.get(id)!, sortOrder: index + 1 })),
      },
    }));
  },

  fetchTaskValues: async (projectId, taskId) => {
    const token = getToken();
    if (!token) return;
    const { data } = await getApi().get<CustomFieldValue[]>(`/api/v1/projects/${projectId}/tasks/${taskId}/fields`, { token });
    set((state) => ({
      valuesByTask: { ...state.valuesByTask, [taskId]: data ?? [] },
    }));
  },

  setTaskValue: async (projectId, taskId, fieldId, value) => {
    const token = getToken();
    if (!token) return;
    const { data } = await getApi().put<CustomFieldValue>(`/api/v1/projects/${projectId}/tasks/${taskId}/fields/${fieldId}`, { value }, { token });
    set((state) => ({
      valuesByTask: {
        ...state.valuesByTask,
        [taskId]: [
          ...(state.valuesByTask[taskId] ?? []).filter((item) => item.fieldDefId !== fieldId),
          data,
        ],
      },
    }));
  },

  clearTaskValue: async (projectId, taskId, fieldId) => {
    const token = getToken();
    if (!token) return;
    await getApi().delete(`/api/v1/projects/${projectId}/tasks/${taskId}/fields/${fieldId}`, { token });
    set((state) => ({
      valuesByTask: {
        ...state.valuesByTask,
        [taskId]: (state.valuesByTask[taskId] ?? []).filter((item) => item.fieldDefId !== fieldId),
      },
    }));
  },
}));
