"use client";

import { create } from "zustand";
import { persist } from "zustand/middleware";

export interface RuntimeEnvConfig {
  apiKey: string;
  commandPath: string;
  serverUrl: string;
  extraEnv: Record<string, string>;
}

const EMPTY_RUNTIME: RuntimeEnvConfig = {
  apiKey: "",
  commandPath: "",
  serverUrl: "",
  extraEnv: {},
};

export interface RuntimeConfigState {
  runtimes: Record<string, RuntimeEnvConfig>;
  getRuntime: (key: string) => RuntimeEnvConfig;
  setRuntimeField: (key: string, field: keyof Omit<RuntimeEnvConfig, "extraEnv">, value: string) => void;
  setRuntimeExtraEnv: (key: string, envKey: string, envValue: string) => void;
  removeRuntimeExtraEnv: (key: string, envKey: string) => void;
}

export const useRuntimeConfigStore = create<RuntimeConfigState>()(
  persist(
    (set, get) => ({
      runtimes: {},

      getRuntime: (key: string) => get().runtimes[key] ?? EMPTY_RUNTIME,

      setRuntimeField: (key, field, value) =>
        set((state) => ({
          runtimes: {
            ...state.runtimes,
            [key]: {
              ...(state.runtimes[key] ?? EMPTY_RUNTIME),
              [field]: value,
            },
          },
        })),

      setRuntimeExtraEnv: (key, envKey, envValue) =>
        set((state) => {
          const current = state.runtimes[key] ?? EMPTY_RUNTIME;
          return {
            runtimes: {
              ...state.runtimes,
              [key]: {
                ...current,
                extraEnv: { ...current.extraEnv, [envKey]: envValue },
              },
            },
          };
        }),

      removeRuntimeExtraEnv: (key, envKey) =>
        set((state) => {
          const current = state.runtimes[key] ?? EMPTY_RUNTIME;
          const next = { ...current.extraEnv };
          delete next[envKey];
          return {
            runtimes: {
              ...state.runtimes,
              [key]: { ...current, extraEnv: next },
            },
          };
        }),
    }),
    { name: "runtime-config-storage" },
  ),
);
