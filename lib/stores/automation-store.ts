"use client";

import { create } from "zustand";
import { toast } from "sonner";
import { createApiClient } from "@/lib/api-client";
import { useAuthStore } from "./auth-store";

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";

export interface AutomationRule {
  id: string;
  projectId: string;
  name: string;
  enabled: boolean;
  eventType: string;
  conditions: unknown;
  actions: unknown;
  createdBy: string;
  createdAt: string;
  updatedAt: string;
}

export interface AutomationLog {
  id: string;
  ruleId: string;
  taskId?: string | null;
  eventType: string;
  triggeredAt: string;
  status: string;
  detail: unknown;
}

interface AutomationState {
  rulesByProject: Record<string, AutomationRule[]>;
  logsByProject: Record<string, AutomationLog[]>;
  fetchRules: (projectId: string) => Promise<void>;
  createRule: (projectId: string, input: Omit<AutomationRule, "id" | "projectId" | "createdBy" | "createdAt" | "updatedAt">) => Promise<void>;
  updateRule: (projectId: string, ruleId: string, input: Partial<Omit<AutomationRule, "id" | "projectId" | "createdBy" | "createdAt" | "updatedAt">>) => Promise<void>;
  deleteRule: (projectId: string, ruleId: string) => Promise<void>;
  fetchLogs: (projectId: string) => Promise<void>;
}

const getApi = () => createApiClient(API_URL);
const getToken = () => {
  const state = useAuthStore.getState() as { accessToken?: string | null; token?: string | null };
  return state.accessToken ?? state.token ?? null;
};

export const useAutomationStore = create<AutomationState>()((set) => ({
  rulesByProject: {},
  logsByProject: {},

  fetchRules: async (projectId) => {
    const token = getToken();
    if (!token) return;
    const { data } = await getApi().get<AutomationRule[]>(`/api/v1/projects/${projectId}/automations`, { token });
    set((state) => ({ rulesByProject: { ...state.rulesByProject, [projectId]: data ?? [] } }));
  },

  createRule: async (projectId, input) => {
    const token = getToken();
    if (!token) return;
    try {
      const { data } = await getApi().post<AutomationRule>(`/api/v1/projects/${projectId}/automations`, input, { token });
      set((state) => ({ rulesByProject: { ...state.rulesByProject, [projectId]: [...(state.rulesByProject[projectId] ?? []), data] } }));
    } catch (error) {
      toast.error("Failed to create automation rule", {
        description: error instanceof Error ? error.message : "Unknown error",
      });
    }
  },

  updateRule: async (projectId, ruleId, input) => {
    const token = getToken();
    if (!token) return;
    try {
      const { data } = await getApi().put<AutomationRule>(`/api/v1/projects/${projectId}/automations/${ruleId}`, input, { token });
      set((state) => ({
        rulesByProject: {
          ...state.rulesByProject,
          [projectId]: (state.rulesByProject[projectId] ?? []).map((item) => (item.id === ruleId ? data : item)),
        },
      }));
    } catch (error) {
      toast.error("Failed to update automation rule", {
        description: error instanceof Error ? error.message : "Unknown error",
      });
    }
  },

  deleteRule: async (projectId, ruleId) => {
    const token = getToken();
    if (!token) return;
    try {
      await getApi().delete(`/api/v1/projects/${projectId}/automations/${ruleId}`, { token });
      set((state) => ({
        rulesByProject: {
          ...state.rulesByProject,
          [projectId]: (state.rulesByProject[projectId] ?? []).filter((item) => item.id !== ruleId),
        },
      }));
    } catch (error) {
      toast.error("Failed to delete automation rule", {
        description: error instanceof Error ? error.message : "Unknown error",
      });
    }
  },

  fetchLogs: async (projectId) => {
    const token = getToken();
    if (!token) return;
    const { data } = await getApi().get<{ items: AutomationLog[] }>(`/api/v1/projects/${projectId}/automations/logs`, { token });
    set((state) => ({ logsByProject: { ...state.logsByProject, [projectId]: data.items ?? [] } }));
  },
}));
