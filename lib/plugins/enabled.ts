/**
 * Frontend plugin-enabled helpers.
 *
 * Mirrors the Go-side gating in src-go/internal/server/qianchuan_plugin.go.
 * First-party plugins are opt-out: if the env var is unset or matches any
 * truthy token, the plugin is considered enabled. Operators disable a
 * plugin by setting NEXT_PUBLIC_PLUGIN_<ID>=disabled (or false/0/off/no).
 *
 * NEXT_PUBLIC_* values are inlined at build time, so rebuild the frontend
 * after flipping the flag.
 */

const DISABLED_TOKENS = new Set(["0", "false", "no", "off", "disabled"]);

export function isPluginEnabled(flag: string | undefined): boolean {
  if (flag == null) return true;
  return !DISABLED_TOKENS.has(flag.trim().toLowerCase());
}

export function isQianchuanPluginEnabled(): boolean {
  return isPluginEnabled(process.env.NEXT_PUBLIC_PLUGIN_QIANCHUAN);
}
