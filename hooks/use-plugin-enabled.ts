"use client";

import { useEffect, useState } from "react";
import { useAuthStore } from "@/lib/stores/auth-store";
import { isPluginEnabled } from "@/lib/plugins/enabled";

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";

export type PluginEnabledState = {
  loading: boolean;
  enabled: boolean;
  lifecycleState: string | null;
  error: string | null;
};

/**
 * usePluginEnabled probes GET /api/v1/plugins/:id/status to decide whether
 * a feature gated behind a plugin should render. Build-time NEXT_PUBLIC_*
 * overrides take precedence: if the env override resolves to disabled,
 * we short-circuit without hitting the backend. This keeps local dev
 * workflows (no backend) usable while still honoring runtime toggles.
 */
export function usePluginEnabled(
  pluginId: string,
  buildTimeOverride?: string,
): PluginEnabledState {
  const forcedDisabled =
    buildTimeOverride !== undefined && !isPluginEnabled(buildTimeOverride);

  const [state, setState] = useState<PluginEnabledState>(() =>
    forcedDisabled
      ? { loading: false, enabled: false, lifecycleState: null, error: null }
      : { loading: true, enabled: false, lifecycleState: null, error: null },
  );

  useEffect(() => {
    if (forcedDisabled) {
      return;
    }
    let cancelled = false;
    const token = useAuthStore.getState().accessToken;
    const headers: Record<string, string> = { Accept: "application/json" };
    if (token) headers.Authorization = `Bearer ${token}`;
    fetch(`${API_URL}/api/v1/plugins/${encodeURIComponent(pluginId)}/status`, {
      headers,
      credentials: "include",
    })
      .then(async (res) => {
        if (cancelled) return;
        if (res.status === 404) {
          setState({ loading: false, enabled: false, lifecycleState: null, error: null });
          return;
        }
        if (!res.ok) {
          setState({
            loading: false,
            enabled: false,
            lifecycleState: null,
            error: `status ${res.status}`,
          });
          return;
        }
        const body = (await res.json()) as {
          enabled?: boolean;
          lifecycle_state?: string;
        };
        setState({
          loading: false,
          enabled: Boolean(body.enabled),
          lifecycleState: body.lifecycle_state ?? null,
          error: null,
        });
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setState({
          loading: false,
          enabled: false,
          lifecycleState: null,
          error: err instanceof Error ? err.message : String(err),
        });
      });
    return () => {
      cancelled = true;
    };
  }, [pluginId, forcedDisabled]);

  return state;
}
