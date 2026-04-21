"use client";

import { create } from "zustand";
import { createApiClient, ApiError } from "@/lib/api-client";

export interface TimelineEntry {
  timestamp: string;
  source: "logs" | "automation" | "eventbus";
  level?: string;
  eventType?: string;
  summary?: string;
  detail?: Record<string, unknown>;
}

interface DebugState {
  entries: TimelineEntry[];
  loading: boolean;
  error: string | null;
  truncated: boolean;
  fetchTrace: (traceId: string, baseUrl: string, token?: string) => Promise<void>;
  clear: () => void;
}

export const useDebugStore = create<DebugState>((set) => ({
  entries: [],
  loading: false,
  error: null,
  truncated: false,
  fetchTrace: async (traceId, baseUrl, token) => {
    if (!traceId) return;
    set({ loading: true, error: null });
    try {
      const client = createApiClient(baseUrl);
      const resp = await client.get<{ entries: TimelineEntry[]; truncated: boolean }>(
        `/api/v1/debug/trace/${encodeURIComponent(traceId)}`,
        token ? { token } : undefined,
      );
      set({
        entries: resp.data?.entries ?? [],
        truncated: Boolean(resp.data?.truncated),
        loading: false,
      });
    } catch (err) {
      const message =
        err instanceof ApiError
          ? err.message
          : err instanceof Error
          ? err.message
          : String(err);
      set({ error: message, loading: false });
    }
  },
  clear: () => set({ entries: [], loading: false, error: null, truncated: false }),
}));
