"use client";

import { create } from "zustand";
import { persist } from "zustand/middleware";

export type Theme = "light" | "dark" | "system";
export const APPEARANCE_STORAGE_KEY = "appearance-storage";

interface AppearanceState {
  theme: Theme;
  setTheme: (theme: Theme) => void;
}

export const useAppearanceStore = create<AppearanceState>()(
  persist(
    (set) => ({
      theme: "system",
      setTheme: (theme) => set({ theme }),
    }),
    { name: APPEARANCE_STORAGE_KEY }
  )
);
