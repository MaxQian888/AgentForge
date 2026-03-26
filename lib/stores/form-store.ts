"use client";

import { create } from "zustand";
import { createApiClient } from "@/lib/api-client";
import { useAuthStore } from "./auth-store";

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";

export interface FormDefinition {
  id: string;
  projectId: string;
  name: string;
  slug: string;
  fields: unknown;
  targetStatus: string;
  targetAssignee?: string | null;
  isPublic: boolean;
  createdAt: string;
  updatedAt: string;
}

interface FormState {
  formsByProject: Record<string, FormDefinition[]>;
  formsBySlug: Record<string, FormDefinition>;
  loadingByProject: Record<string, boolean>;
  fetchForms: (projectId: string) => Promise<void>;
  fetchFormBySlug: (slug: string) => Promise<FormDefinition>;
  createForm: (projectId: string, input: Omit<FormDefinition, "id" | "projectId" | "createdAt" | "updatedAt">) => Promise<void>;
  updateForm: (projectId: string, formId: string, input: Partial<Omit<FormDefinition, "id" | "projectId" | "createdAt" | "updatedAt">>) => Promise<void>;
  deleteForm: (projectId: string, formId: string) => Promise<void>;
  submitForm: (slug: string, input: { submittedBy?: string; values: Record<string, string> }) => Promise<{ id: string }>;
}

const getApi = () => createApiClient(API_URL);
const getToken = () => {
  const state = useAuthStore.getState() as { accessToken?: string | null; token?: string | null };
  return state.accessToken ?? state.token ?? null;
};

export const useFormStore = create<FormState>()((set) => ({
  formsByProject: {},
  formsBySlug: {},
  loadingByProject: {},

  fetchForms: async (projectId) => {
    const token = getToken();
    if (!token) return;
    set((state) => ({ loadingByProject: { ...state.loadingByProject, [projectId]: true } }));
    try {
      const { data } = await getApi().get<FormDefinition[]>(`/api/v1/projects/${projectId}/forms`, { token });
      set((state) => ({
        formsByProject: { ...state.formsByProject, [projectId]: data ?? [] },
        formsBySlug: {
          ...state.formsBySlug,
          ...(data ?? []).reduce<Record<string, FormDefinition>>((acc, form) => {
            acc[form.slug] = form;
            return acc;
          }, {}),
        },
      }));
    } finally {
      set((state) => ({ loadingByProject: { ...state.loadingByProject, [projectId]: false } }));
    }
  },

  fetchFormBySlug: async (slug) => {
    const token = getToken();
    const { data } = await getApi().get<FormDefinition>(
      `/api/v1/forms/${slug}`,
      token ? { token } : undefined
    );
    set((state) => ({
      formsBySlug: { ...state.formsBySlug, [slug]: data },
      formsByProject: {
        ...state.formsByProject,
        [data.projectId]: mergeForms(state.formsByProject[data.projectId] ?? [], data),
      },
    }));
    return data;
  },

  createForm: async (projectId, input) => {
    const token = getToken();
    if (!token) return;
    const { data } = await getApi().post<FormDefinition>(`/api/v1/projects/${projectId}/forms`, input, { token });
    set((state) => ({
      formsByProject: { ...state.formsByProject, [projectId]: [...(state.formsByProject[projectId] ?? []), data] },
      formsBySlug: { ...state.formsBySlug, [data.slug]: data },
    }));
  },

  updateForm: async (projectId, formId, input) => {
    const token = getToken();
    if (!token) return;
    const { data } = await getApi().put<FormDefinition>(`/api/v1/projects/${projectId}/forms/${formId}`, input, { token });
    set((state) => ({
      formsBySlug: {
        ...removeFormSlug(state.formsBySlug, formId),
        [data.slug]: data,
      },
      formsByProject: {
        ...state.formsByProject,
        [projectId]: (state.formsByProject[projectId] ?? []).map((item) => (item.id === formId ? data : item)),
      },
    }));
  },

  deleteForm: async (projectId, formId) => {
    const token = getToken();
    if (!token) return;
    await getApi().delete(`/api/v1/projects/${projectId}/forms/${formId}`, { token });
    set((state) => ({
      formsBySlug: removeFormSlug(state.formsBySlug, formId),
      formsByProject: {
        ...state.formsByProject,
        [projectId]: (state.formsByProject[projectId] ?? []).filter((item) => item.id !== formId),
      },
    }));
  },

  submitForm: async (slug, input) => {
    const { data } = await getApi().post<{ id: string }>(`/api/v1/forms/${slug}/submit`, input);
    return data;
  },
}));

function mergeForms(forms: FormDefinition[], nextForm: FormDefinition) {
  const existingIndex = forms.findIndex((item) => item.id === nextForm.id);
  if (existingIndex === -1) {
    return [...forms, nextForm];
  }
  return forms.map((item) => (item.id === nextForm.id ? nextForm : item));
}

function removeFormSlug(formsBySlug: Record<string, FormDefinition>, formId: string) {
  const next = { ...formsBySlug };
  for (const [slug, form] of Object.entries(next)) {
    if (form.id === formId) {
      delete next[slug];
    }
  }
  return next;
}
