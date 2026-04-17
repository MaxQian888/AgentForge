"use client";

/**
 * Lightweight feature-flag system for the enhance-frontend-panel rollout.
 *
 * Sources (evaluated in order, first match wins):
 *   1. Runtime overrides set via `setFeatureFlagOverride` (client-only,
 *      intended for dev tools and tests).
 *   2. Build-time environment variables named
 *      `NEXT_PUBLIC_FEATURE_<FLAG_NAME>` where the value is interpreted
 *      as boolean ("1", "true", "on", "yes" -> enabled).
 *   3. Hard-coded defaults declared in `DEFAULT_FEATURE_FLAGS`.
 *
 * All flags default to `true` because the features are shipped; the
 * indirection exists so we can dark-launch fixes and do gradual rollout
 * without code deletes.
 */

import { useCallback, useEffect, useSyncExternalStore } from "react";

export type FeatureFlagName =
  | "WORKFLOW_BUILDER"
  | "MEMORY_EXPLORER"
  | "IM_BRIDGE_PANEL"
  | "COST_DASHBOARD_CHARTS"
  | "SCHEDULER_CONTROL_PANEL"
  | "COMMAND_PALETTE"
  | "DASHBOARD_DRAGGABLE_WIDGETS"
  | "PLUGIN_MARKETPLACE_PANEL";

export type FeatureFlags = Record<FeatureFlagName, boolean>;

export const DEFAULT_FEATURE_FLAGS: FeatureFlags = {
  WORKFLOW_BUILDER: true,
  MEMORY_EXPLORER: true,
  IM_BRIDGE_PANEL: true,
  COST_DASHBOARD_CHARTS: true,
  SCHEDULER_CONTROL_PANEL: true,
  COMMAND_PALETTE: true,
  DASHBOARD_DRAGGABLE_WIDGETS: true,
  PLUGIN_MARKETPLACE_PANEL: true,
};

const TRUTHY = new Set(["1", "true", "on", "yes"]);
const FALSY = new Set(["0", "false", "off", "no"]);

function parseEnvFlag(value: string | undefined): boolean | null {
  if (typeof value !== "string") return null;
  const normalized = value.trim().toLowerCase();
  if (TRUTHY.has(normalized)) return true;
  if (FALSY.has(normalized)) return false;
  return null;
}

/**
 * Reads the env-derived override for a single flag.  Uses a static
 * lookup table so that Next.js / bundlers can inline the values at
 * build time.
 */
function readEnvOverride(name: FeatureFlagName): boolean | null {
  switch (name) {
    case "WORKFLOW_BUILDER":
      return parseEnvFlag(process.env.NEXT_PUBLIC_FEATURE_WORKFLOW_BUILDER);
    case "MEMORY_EXPLORER":
      return parseEnvFlag(process.env.NEXT_PUBLIC_FEATURE_MEMORY_EXPLORER);
    case "IM_BRIDGE_PANEL":
      return parseEnvFlag(process.env.NEXT_PUBLIC_FEATURE_IM_BRIDGE_PANEL);
    case "COST_DASHBOARD_CHARTS":
      return parseEnvFlag(
        process.env.NEXT_PUBLIC_FEATURE_COST_DASHBOARD_CHARTS,
      );
    case "SCHEDULER_CONTROL_PANEL":
      return parseEnvFlag(
        process.env.NEXT_PUBLIC_FEATURE_SCHEDULER_CONTROL_PANEL,
      );
    case "COMMAND_PALETTE":
      return parseEnvFlag(process.env.NEXT_PUBLIC_FEATURE_COMMAND_PALETTE);
    case "DASHBOARD_DRAGGABLE_WIDGETS":
      return parseEnvFlag(
        process.env.NEXT_PUBLIC_FEATURE_DASHBOARD_DRAGGABLE_WIDGETS,
      );
    case "PLUGIN_MARKETPLACE_PANEL":
      return parseEnvFlag(
        process.env.NEXT_PUBLIC_FEATURE_PLUGIN_MARKETPLACE_PANEL,
      );
    default:
      return null;
  }
}

/**
 * Pure resolver — used by both the hook and server-side callers.
 * Accepts an optional `flags` map to support dependency injection in
 * tests and future rollouts.
 */
export function isFeatureEnabled(
  name: FeatureFlagName,
  flags?: Partial<FeatureFlags>,
): boolean {
  if (flags && Object.prototype.hasOwnProperty.call(flags, name)) {
    const value = flags[name];
    if (typeof value === "boolean") return value;
  }
  const override = runtimeOverrides[name];
  if (typeof override === "boolean") return override;
  const fromEnv = readEnvOverride(name);
  if (typeof fromEnv === "boolean") return fromEnv;
  return DEFAULT_FEATURE_FLAGS[name];
}

/**
 * Returns a snapshot of every flag's resolved value.
 */
export function resolveFeatureFlags(
  overrides?: Partial<FeatureFlags>,
): FeatureFlags {
  const result = { ...DEFAULT_FEATURE_FLAGS };
  (Object.keys(result) as FeatureFlagName[]).forEach((name) => {
    result[name] = isFeatureEnabled(name, overrides);
  });
  return result;
}

// ---------------------------------------------------------------------------
// Runtime override store (client side only).  Small custom store so we
// don't pull another Zustand slice just for this.
// ---------------------------------------------------------------------------

const runtimeOverrides: Partial<FeatureFlags> = {};
const listeners = new Set<() => void>();

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

function notify(): void {
  listeners.forEach((l) => l());
}

/**
 * Override a flag at runtime.  Pass `null` to clear the override and
 * fall back to env/default.
 */
export function setFeatureFlagOverride(
  name: FeatureFlagName,
  value: boolean | null,
): void {
  if (value === null) {
    delete runtimeOverrides[name];
  } else {
    runtimeOverrides[name] = value;
  }
  notify();
}

/**
 * Clear all runtime overrides.  Exposed primarily for tests.
 */
export function clearFeatureFlagOverrides(): void {
  (Object.keys(runtimeOverrides) as FeatureFlagName[]).forEach((name) => {
    delete runtimeOverrides[name];
  });
  notify();
}

function getSnapshot(name: FeatureFlagName): boolean {
  return isFeatureEnabled(name);
}

/**
 * React hook reading a single flag.  Re-renders when an override is
 * applied via `setFeatureFlagOverride`.
 */
export function useFeatureFlag(name: FeatureFlagName): boolean {
  const subscribeFn = useCallback(
    (listener: () => void) => subscribe(listener),
    [],
  );
  const getSnap = useCallback(() => getSnapshot(name), [name]);
  // Server snapshot uses defaults + env; it is stable.
  const getServerSnap = useCallback(() => getSnapshot(name), [name]);
  return useSyncExternalStore(subscribeFn, getSnap, getServerSnap);
}

/**
 * Imperative helper for non-React callers (event handlers, services).
 */
export function getFeatureFlag(name: FeatureFlagName): boolean {
  return isFeatureEnabled(name);
}

/**
 * Effect hook that keeps an effect's dependency list in sync with a
 * feature flag's state.  Intended to be used inside dev-only panels.
 */
export function useFeatureFlagEffect(
  name: FeatureFlagName,
  effect: (enabled: boolean) => void | (() => void),
): void {
  const enabled = useFeatureFlag(name);
  useEffect(() => effect(enabled), [enabled, effect]);
}
