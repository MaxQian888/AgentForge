import { messageBundles } from "./messages";
import { getPreferredLocale } from "@/lib/stores/locale-store";

function resolveValue(
  messages: Record<string, unknown> | undefined,
  key: string,
): string | undefined {
  if (!messages) return undefined;

  const parts = key.split(".");
  let current: unknown = messages;

  for (const part of parts) {
    if (!current || typeof current !== "object" || Array.isArray(current)) {
      return undefined;
    }
    current = (current as Record<string, unknown>)[part];
  }

  return typeof current === "string" ? current : undefined;
}

export function t(
  namespace: string,
  key: string,
  vars?: Record<string, string>,
  fallback?: string,
): string {
  const locale = getPreferredLocale();
  const bundle = messageBundles[locale]?.[namespace] as
    | Record<string, unknown>
    | undefined;

  let raw = resolveValue(bundle, key);
  if (raw === undefined) {
    const enBundle = messageBundles.en?.[namespace] as
      | Record<string, unknown>
      | undefined;
    raw = resolveValue(enBundle, key);
  }

  if (raw === undefined) {
    raw = fallback ?? key;
  }

  if (!vars) return raw;

  return raw.replace(/\{(\w+)\}/g, (_, name) => vars[name] ?? `{${name}}`);
}
