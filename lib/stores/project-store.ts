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
  repoUrl?: string;
  defaultBranch?: string;
  slug?: string;
}

interface ProjectState {
  projects: Project[];
  currentProject: Project | null;
  loading: boolean;
  fetchProjects: () => Promise<void>;
  setCurrentProject: (id: string) => void;
  createProject: (data: { name: string; description: string }) => Promise<void>;
  updateProject: (id: string, data: Partial<Pick<Project, "name" | "description" | "repoUrl" | "defaultBranch">>) => Promise<void>;
}

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";

function toProjectSlug(name: string) {
  const normalized = name
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");

  return normalized || "project";
}

export const useProjectStore = create<ProjectState>()((set, get) => ({
  projects: [],
  currentProject: null,
  loading: false,

  fetchProjects: async () => {
    const token = useAuthStore.getState().accessToken;
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
    const token = useAuthStore.getState().accessToken;
    if (!token) return;
    const api = createApiClient(API_URL);
    const { data: project } = await api.post<Project>(
      "/api/v1/projects",
      {
        ...data,
        slug: toProjectSlug(data.name),
      },
      { token }
    );
    set((s) => ({ projects: [...s.projects, project] }));
  },

  updateProject: async (id, data) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return;
    const api = createApiClient(API_URL);
    const { data: updated } = await api.put<Project>(
      `/api/v1/projects/${id}`,
      data,
      { token }
    );
    set((s) => ({
      projects: s.projects.map((p) => (p.id === id ? updated : p)),
      currentProject: s.currentProject?.id === id ? updated : s.currentProject,
    }));
  },
}));
