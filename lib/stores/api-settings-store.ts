"use client";

import { create } from "zustand";
import { persist } from "zustand/middleware";

interface ApiSettingsState {
  timeoutMs: number;
  retryCount: number;
  setTimeoutMs: (ms: number) => void;
  setRetryCount: (count: number) => void;
}

export const useApiSettingsStore = create<ApiSettingsState>()(
  persist(
    (set) => ({
      timeoutMs: 30000,
      retryCount: 3,
      setTimeoutMs: (ms) => set({ timeoutMs: Math.max(0, ms) }),
      setRetryCount: (count) => set({ retryCount: Math.max(0, Math.min(10, count)) }),
    }),
    { name: "api-settings-storage" },
  ),
);
