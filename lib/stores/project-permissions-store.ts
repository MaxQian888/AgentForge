"use client";

import { create } from "zustand";
import { createApiClient } from "@/lib/api-client";
import { useAuthStore } from "./auth-store";
import { resolveBackendUrl } from "@/lib/backend-url";
import {
  normalizeProjectRole,
  type ProjectRole,
} from "@/lib/dashboard/summary";

// Canonical project ActionIDs. Mirror Go middleware/rbac.go ActionID
// constants. The frontend NEVER hard-codes role→action mappings; it only
// checks membership in the server-provided allowedActions array.
export type ProjectActionId =
  | "project.read" | "project.update" | "project.delete"
  | "member.read" | "member.create" | "member.update"
  | "member.role.change" | "member.delete" | "member.bulk.update"
  | "task.read" | "task.create" | "task.update" | "task.delete"
  | "task.assign" | "task.transition" | "task.dispatch"
  | "task.comment.write" | "task.decompose"
  | "team.run.start" | "team.run.retry" | "team.run.cancel"
  | "team.update" | "team.delete"
  | "workflow.read" | "workflow.write" | "workflow.execute"
  | "workflow.review.resolve" | "workflow.execution.cancel"
  | "automation.read" | "automation.write" | "automation.trigger"
  | "settings.read" | "settings.update"
  | "dashboard.read" | "dashboard.write"
  | "wiki.read" | "wiki.write" | "wiki.delete"
  | "memory.read" | "memory.write"
  | "log.read" | "log.write"
  | "custom_field.read" | "custom_field.write"
  | "saved_view.write" | "form.write"
  | "milestone.write" | "sprint.write"
  | "agent.spawn" | "agent.control"
  | "audit.read";

export interface ProjectPermissions {
  projectId: string;
  projectRole: ProjectRole;
  allowedActions: ProjectActionId[];
}

interface ProjectPermissionsState {
  byProject: Record<string, ProjectPermissions>;
  loadingByProject: Record<string, boolean>;
  errorByProject: Record<string, string | null>;
  fetchPermissions: (projectId: string) => Promise<ProjectPermissions | null>;
  invalidate: (projectId?: string) => void;
}

function getToken(): string | null {
  const state = useAuthStore.getState();
  return state.accessToken ?? null;
}

export const useProjectPermissionsStore = create<ProjectPermissionsState>(
  (set, get) => ({
    byProject: {},
    loadingByProject: {},
    errorByProject: {},

    async fetchPermissions(projectId) {
      if (!projectId) return null;
      // Avoid duplicate in-flight requests for the same project.
      if (get().loadingByProject[projectId]) {
        // Caller will see the cached value once the in-flight resolves.
        return get().byProject[projectId] ?? null;
      }
      set((s) => ({
        loadingByProject: { ...s.loadingByProject, [projectId]: true },
        errorByProject: { ...s.errorByProject, [projectId]: null },
      }));
      try {
        const api = createApiClient(await resolveBackendUrl());
        const { data } = await api.get<{
          projectId: string;
          projectRole: string;
          allowedActions: string[];
        }>(`/api/v1/auth/me/projects/${projectId}/permissions`, {
          token: getToken() ?? undefined,
        });
        const permissions: ProjectPermissions = {
          projectId: data.projectId,
          projectRole: normalizeProjectRole(data.projectRole),
          allowedActions: (data.allowedActions ?? []) as ProjectActionId[],
        };
        set((s) => ({
          byProject: { ...s.byProject, [projectId]: permissions },
          loadingByProject: { ...s.loadingByProject, [projectId]: false },
        }));
        return permissions;
      } catch (error) {
        const message =
          error instanceof Error ? error.message : "permissions fetch failed";
        set((s) => ({
          loadingByProject: { ...s.loadingByProject, [projectId]: false },
          errorByProject: { ...s.errorByProject, [projectId]: message },
        }));
        return null;
      }
    },

    invalidate(projectId) {
      set((s) => {
        if (!projectId) {
          return { byProject: {}, errorByProject: {} };
        }
        const { [projectId]: _drop, ...rest } = s.byProject;
        const { [projectId]: _err, ...errRest } = s.errorByProject;
        return { byProject: rest, errorByProject: errRest };
      });
    },
  })
);
