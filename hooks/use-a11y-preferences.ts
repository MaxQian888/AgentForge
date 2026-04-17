"use client";

import { useEffect, useState } from "react";
import {
  useAppearanceStore,
  type Density,
  type MotionPreference,
} from "@/lib/stores/appearance-store";

const REDUCED_MOTION_QUERY = "(prefers-reduced-motion: reduce)";
const HIGH_CONTRAST_QUERY = "(prefers-contrast: more)";

export interface A11yPreferences {
  density: Density;
  motionPreference: MotionPreference;
  highContrast: boolean;
  screenReaderMode: boolean;
  systemPrefersReducedMotion: boolean;
  systemPrefersHighContrast: boolean;
  /** Effective reduced-motion state after resolving system preference overrides. */
  reducedMotionActive: boolean;
}

function matches(query: string): boolean {
  if (typeof window === "undefined" || typeof window.matchMedia !== "function") {
    return false;
  }
  return window.matchMedia(query).matches;
}

function subscribe(query: string, listener: () => void): () => void {
  if (typeof window === "undefined" || typeof window.matchMedia !== "function") {
    return () => undefined;
  }
  const mql = window.matchMedia(query);
  if (typeof mql.addEventListener === "function") {
    mql.addEventListener("change", listener);
    return () => mql.removeEventListener("change", listener);
  }
  mql.addListener(listener);
  return () => mql.removeListener(listener);
}

/**
 * Subscribes to system accessibility preferences (prefers-reduced-motion,
 * prefers-contrast) and merges them with the user's explicit overrides from
 * the appearance store. Also mirrors the resolved values as data attributes
 * on `document.documentElement` so CSS selectors can react.
 */
export function useA11yPreferences(): A11yPreferences {
  const density = useAppearanceStore((s) => s.density);
  const motionPreference = useAppearanceStore((s) => s.motionPreference);
  const highContrast = useAppearanceStore((s) => s.highContrast);
  const screenReaderMode = useAppearanceStore((s) => s.screenReaderMode);

  const [systemPrefersReducedMotion, setSystemReducedMotion] = useState<boolean>(() =>
    matches(REDUCED_MOTION_QUERY),
  );
  const [systemPrefersHighContrast, setSystemHighContrast] = useState<boolean>(() =>
    matches(HIGH_CONTRAST_QUERY),
  );

  useEffect(() => {
    const unsubscribe = subscribe(REDUCED_MOTION_QUERY, () => {
      setSystemReducedMotion(matches(REDUCED_MOTION_QUERY));
    });
    return unsubscribe;
  }, []);

  useEffect(() => {
    const unsubscribe = subscribe(HIGH_CONTRAST_QUERY, () => {
      setSystemHighContrast(matches(HIGH_CONTRAST_QUERY));
    });
    return unsubscribe;
  }, []);

  const reducedMotionActive =
    motionPreference === "reduce" ||
    (motionPreference === "system" && systemPrefersReducedMotion);

  const highContrastActive = highContrast || systemPrefersHighContrast;

  useEffect(() => {
    if (typeof document === "undefined") return;
    const root = document.documentElement;
    root.setAttribute("data-density", density);
    root.setAttribute("data-reduced-motion", reducedMotionActive ? "true" : "false");
    root.setAttribute("data-contrast", highContrastActive ? "high" : "normal");
    root.setAttribute("data-screen-reader", screenReaderMode ? "true" : "false");
    return () => {
      // Intentionally leave attributes in place; subsequent renders will overwrite.
    };
  }, [density, reducedMotionActive, highContrastActive, screenReaderMode]);

  return {
    density,
    motionPreference,
    highContrast: highContrastActive,
    screenReaderMode,
    systemPrefersReducedMotion,
    systemPrefersHighContrast,
    reducedMotionActive,
  };
}
