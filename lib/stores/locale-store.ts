"use client";
import { create } from "zustand";
import { persist } from "zustand/middleware";

export type Locale = "zh-CN" | "en";
export const SUPPORTED_LOCALES: Locale[] = ["zh-CN", "en"];
export const DEFAULT_LOCALE: Locale = "en";
export const LOCALE_STORAGE_KEY = "locale-storage";

interface LocaleState {
  locale: Locale;
  setLocale: (locale: Locale) => void;
}

function isLocale(value: unknown): value is Locale {
  return SUPPORTED_LOCALES.includes(value as Locale);
}

function readPersistedLocale(): Locale | null {
  if (typeof window === "undefined") {
    return null;
  }

  try {
    const raw = window.localStorage.getItem(LOCALE_STORAGE_KEY);
    if (!raw) {
      return null;
    }

    const parsed = JSON.parse(raw) as {
      state?: { locale?: unknown };
    };
    return isLocale(parsed.state?.locale) ? parsed.state.locale : null;
  } catch {
    return null;
  }
}

export const useLocaleStore = create<LocaleState>()(
  persist(
    (set) => ({
      locale: DEFAULT_LOCALE,
      setLocale: (locale) => set({ locale }),
    }),
    { name: LOCALE_STORAGE_KEY }
  )
);

export function getPreferredLocale(): Locale {
  const locale = useLocaleStore.getState().locale;
  if (useLocaleStore.persist.hasHydrated()) {
    return locale;
  }

  return readPersistedLocale() ?? locale;
}
