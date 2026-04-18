"use client";

import { useEffect, useMemo } from "react";
import {
  useProjectPermissionsStore,
  type ProjectActionId,
} from "@/lib/stores/project-permissions-store";
import type { ProjectRole } from "@/lib/dashboard/summary";

export interface UseProjectRoleResult {
  projectRole: ProjectRole | null;
  loading: boolean;
  error: string | null;
  /**
   * Returns true when the caller's server-issued allowedActions set contains
   * the given action. Falls back to false when permissions are still loading
   * — never grants access optimistically.
   */
  can: (action: ProjectActionId) => boolean;
  refresh: () => Promise<void>;
}

// useProjectRole is the canonical hook for gating UI in the active project.
// It triggers a fetch of /auth/me/projects/:pid/permissions on mount (if not
// cached) and returns a `can(actionId)` predicate backed by the server-side
// matrix. Never duplicate the matrix client-side — the server is authoritative.
export function useProjectRole(projectId: string | null | undefined): UseProjectRoleResult {
  const fetchPermissions = useProjectPermissionsStore((s) => s.fetchPermissions);
  const permissions = useProjectPermissionsStore((s) =>
    projectId ? s.byProject[projectId] ?? null : null
  );
  const loading = useProjectPermissionsStore((s) =>
    projectId ? Boolean(s.loadingByProject[projectId]) : false
  );
  const error = useProjectPermissionsStore((s) =>
    projectId ? s.errorByProject[projectId] ?? null : null
  );

  useEffect(() => {
    if (!projectId) return;
    if (permissions || loading) return;
    void fetchPermissions(projectId);
  }, [projectId, permissions, loading, fetchPermissions]);

  const allowedSet = useMemo(() => {
    return new Set<ProjectActionId>(permissions?.allowedActions ?? []);
  }, [permissions?.allowedActions]);

  return {
    projectRole: permissions?.projectRole ?? null,
    loading,
    error,
    can: (action) => allowedSet.has(action),
    refresh: async () => {
      if (!projectId) return;
      useProjectPermissionsStore.getState().invalidate(projectId);
      await fetchPermissions(projectId);
    },
  };
}
