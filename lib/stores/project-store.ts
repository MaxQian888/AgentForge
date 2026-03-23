"use client";
import { create } from "zustand";
import { createApiClient } from "@/lib/api-client";
import { useAuthStore } from "./auth-store";

export interface Project {
  id: string;
  name: string;
  description: string;
  status: string;
  taskCount: number;
  agentCount: number;
  createdAt: string;
}

interface ProjectState {
  projects: Project[];
  currentProject: Project | null;
  loading: boolean;
  fetchProjects: () => Promise<void>;
  setCurrentProject: (id: string) => void;
  createProject: (data: { name: string; description: string }) => Promise<void>;
}

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";

export const useProjectStore = create<ProjectState>()((set, get) => ({
  projects: [],
  currentProject: null,
  loading: false,

  fetchProjects: async () => {
    const token = useAuthStore.getState().token;
    if (!token) return;
    set({ loading: true });
    try {
      const api = createApiClient(API_URL);
      const { data } = await api.get<Project[]>("/api/v1/projects", { token });
      set({ projects: data });
    } finally {
      set({ loading: false });
    }
  },

  setCurrentProject: (id) => {
    const project = get().projects.find((p) => p.id === id) ?? null;
    set({ currentProject: project });
  },

  createProject: async (data) => {
    const token = useAuthStore.getState().token;
    if (!token) return;
    const api = createApiClient(API_URL);
    const { data: project } = await api.post<Project>(
      "/api/v1/projects",
      data,
      { token }
    );
    set((s) => ({ projects: [...s.projects, project] }));
  },
}));
