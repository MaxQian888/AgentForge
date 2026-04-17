"use client";

import {
  createContext,
  useContext,
  useMemo,
  type ReactNode,
} from "react";
import type { ProjectionResult } from "./types";

/**
 * A block reference passed to action callbacks. The chrome/renderers only
 * need the stable `id` and the parsed kind to decide what to do — real
 * provider implementations will use the wider info for freeze/remove.
 */
export interface LiveArtifactActionBlock {
  id: string;
  live_kind: string;
  target_ref: unknown;
  view_opts: unknown;
}

export interface LiveArtifactActions {
  openSource: (block: LiveArtifactActionBlock) => void;
  freeze: (block: LiveArtifactActionBlock) => void;
  remove: (block: LiveArtifactActionBlock) => void;
  /** Optional post-freeze hook (e.g. toast / refresh). */
  onFreezeComplete?: (block: LiveArtifactActionBlock) => void;
}

export interface LiveArtifactContextValue {
  /** Map of block id to the projection result the hook has computed. */
  projections: Record<string, ProjectionResult | undefined>;
  /** Block-level actions invoked by the chrome dropdown. */
  actions: LiveArtifactActions;
  /** Auth context — the provider (set up in §12) will supply real values. */
  assetId: string;
  projectId: string;
  token: string;
  apiUrl: string;
}

let _warnedOnce = false;
function warnOnce(message: string) {
  if (_warnedOnce) return;
  _warnedOnce = true;
  console.warn(`[live-artifact] ${message}`);
}

const defaultValue: LiveArtifactContextValue = {
  projections: {},
  actions: {
    openSource: () => {
      warnOnce(
        "LiveArtifactProvider not found — openSource fell back to no-op"
      );
    },
    freeze: () => {
      warnOnce("LiveArtifactProvider not found — freeze is a no-op");
    },
    remove: () => {
      warnOnce("LiveArtifactProvider not found — remove is a no-op");
    },
  },
  assetId: "",
  projectId: "",
  token: "",
  apiUrl: "",
};

const LiveArtifactContext =
  createContext<LiveArtifactContextValue>(defaultValue);

export function useLiveArtifactContext(): LiveArtifactContextValue {
  return useContext(LiveArtifactContext);
}

export interface LiveArtifactProviderProps {
  value?: Partial<LiveArtifactContextValue>;
  children: ReactNode;
}

/**
 * Provider wrapper used in tests or when §12's real provider is not mounted.
 * The actual projection orchestration lives in §12 (`use-live-artifact-projections`).
 */
export function LiveArtifactProvider({
  value,
  children,
}: LiveArtifactProviderProps) {
  const merged = useMemo<LiveArtifactContextValue>(
    () => ({
      projections: value?.projections ?? defaultValue.projections,
      actions: {
        openSource:
          value?.actions?.openSource ?? defaultValue.actions.openSource,
        freeze: value?.actions?.freeze ?? defaultValue.actions.freeze,
        remove: value?.actions?.remove ?? defaultValue.actions.remove,
        onFreezeComplete: value?.actions?.onFreezeComplete,
      },
      assetId: value?.assetId ?? defaultValue.assetId,
      projectId: value?.projectId ?? defaultValue.projectId,
      token: value?.token ?? defaultValue.token,
      apiUrl: value?.apiUrl ?? defaultValue.apiUrl,
    }),
    [value]
  );

  return (
    <LiveArtifactContext.Provider value={merged}>
      {children}
    </LiveArtifactContext.Provider>
  );
}

// Exported for tests that need to reference the raw context (rare).
export { LiveArtifactContext };
