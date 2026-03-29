"use client";
import { NextIntlClientProvider } from "next-intl";
import enMessages from "@/messages/en";
import zhCNMessages from "@/messages/zh-CN";
import { DEFAULT_LOCALE, type Locale, useLocaleStore } from "@/lib/stores/locale-store";
import { useEffect, useSyncExternalStore, type ReactNode } from "react";

const messageBundles: Record<Locale, Record<string, unknown>> = {
  "zh-CN": zhCNMessages,
  en: enMessages,
};

function subscribeToLocaleHydration(onStoreChange: () => void) {
  return useLocaleStore.persist.onFinishHydration(() => {
    onStoreChange();
  });
}

export function I18nProvider({
  children,
  initialLocale = DEFAULT_LOCALE,
}: {
  children: ReactNode;
  initialLocale?: Locale;
}) {
  const persistedLocale = useLocaleStore((s) => s.locale);
  const hasHydrated = useSyncExternalStore(
    subscribeToLocaleHydration,
    () => useLocaleStore.persist.hasHydrated(),
    () => false
  );
  const locale = hasHydrated ? persistedLocale : initialLocale;
  const messages = messageBundles[locale] ?? messageBundles[DEFAULT_LOCALE];

  useEffect(() => {
    document.documentElement.lang = locale;
  }, [locale]);

  return (
    <NextIntlClientProvider locale={locale} messages={messages}>
      {children}
    </NextIntlClientProvider>
  );
}
