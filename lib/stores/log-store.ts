"use client";

import { create } from "zustand";
import { toast } from "sonner";
import { createApiClient } from "@/lib/api-client";
import { useAuthStore } from "./auth-store";

export type LogTab = "agent" | "system";
export type LogLevel = "debug" | "info" | "warn" | "error";

export interface LogEntry {
  id: string;
  projectId: string;
  tab: LogTab;
  level: LogLevel;
  actorType: string;
  actorId: string;
  agentId?: string;
  sessionId?: string;
  eventType: string;
  action: string;
  resourceType: string;
  resourceId: string;
  summary: string;
  detail: Record<string, unknown>;
  createdAt: string;
}

export interface LogListResponse {
  items: LogEntry[];
  total: number;
  page: number;
  pageSize: number;
}

interface LogFilters {
  tab: LogTab;
  level: string;
  search: string;
  from: string;
  to: string;
}

interface LogState {
  logs: Record<string, LogEntry[]>;
  total: Record<string, number>;
  page: number;
  pageSize: number;
  filters: LogFilters;
  loading: boolean;
  error: string | null;
  live: boolean;

  fetchLogs: (projectId: string) => Promise<void>;
  setFilters: (filters: Partial<LogFilters>) => void;
  setPage: (page: number) => void;
  prependLog: (projectId: string, log: LogEntry) => void;
  setLive: (live: boolean) => void;
}

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";

export const useLogStore = create<LogState>()((set, get) => ({
  logs: {},
  total: {},
  page: 1,
  pageSize: 50,
  filters: {
    tab: "agent",
    level: "",
    search: "",
    from: "",
    to: "",
  },
  loading: false,
  error: null,
  live: false,

  fetchLogs: async (projectId: string) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return;
    set({ loading: true, error: null });
    try {
      const { page, pageSize, filters } = get();
      const params = new URLSearchParams();
      params.set("tab", filters.tab);
      params.set("page", String(page));
      params.set("pageSize", String(pageSize));
      if (filters.level) params.set("level", filters.level);
      if (filters.search) params.set("search", filters.search);
      if (filters.from) params.set("from", filters.from);
      if (filters.to) params.set("to", filters.to);

      const api = createApiClient(API_URL);
      const { data } = await api.get<LogListResponse>(
        `/api/v1/projects/${projectId}/logs?${params.toString()}`,
        { token }
      );
      set((state) => ({
        logs: { ...state.logs, [projectId]: data?.items ?? [] },
        total: { ...state.total, [projectId]: data?.total ?? 0 },
        page: data?.page ?? state.page,
        pageSize: data?.pageSize ?? state.pageSize,
        loading: false,
      }));
    } catch {
      set({ loading: false, error: "Failed to fetch logs" });
      toast.error("Failed to fetch logs");
    }
  },

  setFilters: (filters) => {
    set((state) => ({
      filters: { ...state.filters, ...filters },
      page: 1,
    }));
  },

  setPage: (page) => set({ page }),

  prependLog: (projectId, log) => {
    set((state) => ({
      logs: {
        ...state.logs,
        [projectId]: [log, ...(state.logs[projectId] ?? [])].slice(0, state.pageSize),
      },
      total: {
        ...state.total,
        [projectId]: (state.total[projectId] ?? 0) + 1,
      },
    }));
  },

  setLive: (live) => set({ live }),
}));
