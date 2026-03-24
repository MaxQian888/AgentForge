"use client";

import { create } from "zustand";
import { createApiClient } from "@/lib/api-client";
import { useAuthStore } from "./auth-store";

export interface AgentMemoryEntry {
  id: string;
  projectId: string;
  scope: "global" | "project" | "role";
  roleId: string;
  category: "episodic" | "semantic" | "procedural";
  key: string;
  content: string;
  metadata: string;
  relevanceScore: number;
  accessCount: number;
  createdAt: string;
}

interface MemoryState {
  entries: AgentMemoryEntry[];
  loading: boolean;
  searchMemory: (projectId: string, query?: string, scope?: string, category?: string) => Promise<void>;
  storeMemory: (projectId: string, input: { key: string; content: string; scope?: string; roleId?: string; category?: string }) => Promise<void>;
  deleteMemory: (projectId: string, memoryId: string) => Promise<void>;
}

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";

function normalizeMemoryEntry(raw: Record<string, unknown>): AgentMemoryEntry {
  return {
    id: String(raw.id ?? ""),
    projectId: String(raw.projectId ?? ""),
    scope: (typeof raw.scope === "string" ? raw.scope : "project") as AgentMemoryEntry["scope"],
    roleId: String(raw.roleId ?? ""),
    category: (typeof raw.category === "string" ? raw.category : "semantic") as AgentMemoryEntry["category"],
    key: String(raw.key ?? ""),
    content: String(raw.content ?? ""),
    metadata: typeof raw.metadata === "string" ? raw.metadata : JSON.stringify(raw.metadata ?? ""),
    relevanceScore: Number(raw.relevanceScore ?? 0),
    accessCount: Number(raw.accessCount ?? 0),
    createdAt: typeof raw.createdAt === "string" ? raw.createdAt : new Date().toISOString(),
  };
}

export const useMemoryStore = create<MemoryState>()((set) => ({
  entries: [],
  loading: false,

  searchMemory: async (projectId, query, scope, category) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return;
    set({ loading: true });
    try {
      const api = createApiClient(API_URL);
      const params = new URLSearchParams();
      if (query) params.set("query", query);
      if (scope) params.set("scope", scope);
      if (category) params.set("category", category);
      const qs = params.toString();
      const url = `/api/v1/projects/${projectId}/memory${qs ? `?${qs}` : ""}`;
      const { data } = await api.get<Record<string, unknown>[]>(url, { token });
      const entries = data.map(normalizeMemoryEntry);
      set({ entries });
    } finally {
      set({ loading: false });
    }
  },

  storeMemory: async (projectId, input) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return;
    const api = createApiClient(API_URL);
    const { data } = await api.post<Record<string, unknown>>(
      `/api/v1/projects/${projectId}/memory`,
      {
        key: input.key,
        content: input.content,
        scope: input.scope,
        roleId: input.roleId,
        category: input.category,
      },
      { token }
    );
    const entry = normalizeMemoryEntry(data);
    set((state) => ({ entries: [...state.entries, entry] }));
  },

  deleteMemory: async (projectId, memoryId) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return;
    const api = createApiClient(API_URL);
    await api.delete(`/api/v1/projects/${projectId}/memory/${memoryId}`, { token });
    set((state) => ({ entries: state.entries.filter((e) => e.id !== memoryId) }));
  },
}));
