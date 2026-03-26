"use client";

import { create } from "zustand";
import { createApiClient } from "@/lib/api-client";
import { useAuthStore } from "./auth-store";

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";

export interface Milestone {
  id: string;
  projectId: string;
  name: string;
  targetDate?: string | null;
  status: string;
  description: string;
  createdAt: string;
  updatedAt: string;
  metrics?: {
    totalTasks: number;
    completedTasks: number;
    totalSprints: number;
    completionRate: number;
  };
}

interface MilestoneState {
  milestonesByProject: Record<string, Milestone[]>;
  fetchMilestones: (projectId: string) => Promise<void>;
  createMilestone: (projectId: string, input: Omit<Milestone, "id" | "projectId" | "createdAt" | "updatedAt" | "metrics">) => Promise<void>;
  updateMilestone: (projectId: string, milestoneId: string, input: Partial<Omit<Milestone, "id" | "projectId" | "createdAt" | "updatedAt" | "metrics">>) => Promise<void>;
  deleteMilestone: (projectId: string, milestoneId: string) => Promise<void>;
}

const getApi = () => createApiClient(API_URL);
const getToken = () => {
  const state = useAuthStore.getState() as { accessToken?: string | null; token?: string | null };
  return state.accessToken ?? state.token ?? null;
};

export const useMilestoneStore = create<MilestoneState>()((set) => ({
  milestonesByProject: {},

  fetchMilestones: async (projectId) => {
    const token = getToken();
    if (!token) return;
    const { data } = await getApi().get<Milestone[]>(`/api/v1/projects/${projectId}/milestones`, { token });
    set((state) => ({ milestonesByProject: { ...state.milestonesByProject, [projectId]: data ?? [] } }));
  },

  createMilestone: async (projectId, input) => {
    const token = getToken();
    if (!token) return;
    const { data } = await getApi().post<Milestone>(`/api/v1/projects/${projectId}/milestones`, input, { token });
    set((state) => ({ milestonesByProject: { ...state.milestonesByProject, [projectId]: [...(state.milestonesByProject[projectId] ?? []), data] } }));
  },

  updateMilestone: async (projectId, milestoneId, input) => {
    const token = getToken();
    if (!token) return;
    const { data } = await getApi().put<Milestone>(`/api/v1/projects/${projectId}/milestones/${milestoneId}`, input, { token });
    set((state) => ({
      milestonesByProject: {
        ...state.milestonesByProject,
        [projectId]: (state.milestonesByProject[projectId] ?? []).map((item) => (item.id === milestoneId ? data : item)),
      },
    }));
  },

  deleteMilestone: async (projectId, milestoneId) => {
    const token = getToken();
    if (!token) return;
    await getApi().delete(`/api/v1/projects/${projectId}/milestones/${milestoneId}`, { token });
    set((state) => ({
      milestonesByProject: {
        ...state.milestonesByProject,
        [projectId]: (state.milestonesByProject[projectId] ?? []).filter((item) => item.id !== milestoneId),
      },
    }));
  },
}));
