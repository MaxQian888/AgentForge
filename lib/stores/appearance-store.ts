"use client";

import { create } from "zustand";
import { persist } from "zustand/middleware";

export type Theme = "light" | "dark" | "system";
export type Density = "compact" | "comfortable" | "spacious";
export type MotionPreference = "system" | "reduce" | "allow";
export const APPEARANCE_STORAGE_KEY = "appearance-storage";

interface AppearanceState {
  theme: Theme;
  density: Density;
  motionPreference: MotionPreference;
  highContrast: boolean;
  screenReaderMode: boolean;
  setTheme: (theme: Theme) => void;
  setDensity: (density: Density) => void;
  setMotionPreference: (preference: MotionPreference) => void;
  setHighContrast: (enabled: boolean) => void;
  setScreenReaderMode: (enabled: boolean) => void;
  resetAppearance: () => void;
}

export const DEFAULT_APPEARANCE = {
  theme: "system" as Theme,
  density: "comfortable" as Density,
  motionPreference: "system" as MotionPreference,
  highContrast: false,
  screenReaderMode: false,
};

export const useAppearanceStore = create<AppearanceState>()(
  persist(
    (set) => ({
      ...DEFAULT_APPEARANCE,
      setTheme: (theme) => set({ theme }),
      setDensity: (density) => set({ density }),
      setMotionPreference: (motionPreference) => set({ motionPreference }),
      setHighContrast: (highContrast) => set({ highContrast }),
      setScreenReaderMode: (screenReaderMode) => set({ screenReaderMode }),
      resetAppearance: () => set({ ...DEFAULT_APPEARANCE }),
    }),
    { name: APPEARANCE_STORAGE_KEY }
  )
);
