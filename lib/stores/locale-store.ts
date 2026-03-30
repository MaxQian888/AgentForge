"use client";
import { create } from "zustand";
import { persist } from "zustand/middleware";
import {
  DEFAULT_LOCALE,
  isLocale,
  type Locale,
} from "@/lib/i18n/config";

export { DEFAULT_LOCALE, SUPPORTED_LOCALES, type Locale } from "@/lib/i18n/config";
export const LOCALE_STORAGE_KEY = "locale-storage";

interface LocaleState {
  locale: Locale;
  setLocale: (locale: Locale) => void;
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
