export type Locale = "zh-CN" | "en";

export const SUPPORTED_LOCALES: readonly Locale[] = ["zh-CN", "en"];
export const DEFAULT_LOCALE: Locale = "en";

export function isLocale(value: unknown): value is Locale {
  return SUPPORTED_LOCALES.includes(value as Locale);
}
