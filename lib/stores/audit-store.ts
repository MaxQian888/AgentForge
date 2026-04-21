"use client";

import { create } from "zustand";
import { createApiClient } from "@/lib/api-client";
import { resolveBackendUrl } from "@/lib/backend-url";
import { useAuthStore } from "./auth-store";
import { withDevtools } from "./_devtools";

export type AuditResourceType =
  | "project" | "member" | "task" | "team_run" | "workflow"
  | "wiki" | "settings" | "automation" | "dashboard" | "auth";

export interface AuditEvent {
  id: string;
  projectId: string;
  occurredAt: string;
  actorUserId?: string;
  actorProjectRoleAtTime?: string;
  actionId: string;
  resourceType: AuditResourceType;
  resourceId?: string;
  payloadSnapshotJson: string;
  systemInitiated: boolean;
  configuredByUserId?: string;
  requestId?: string;
  ip?: string;
  userAgent?: string;
}

export interface AuditEventListResponse {
  events: AuditEvent[];
  nextCursor?: string;
}

export interface AuditQueryFilters {
  actionId?: string;
  actorUserId?: string;
  resourceType?: AuditResourceType | "";
  resourceId?: string;
  from?: string; // RFC3339
  to?: string;   // RFC3339
}

interface AuditPageState {
  events: AuditEvent[];
  nextCursor?: string;
  loading: boolean;
  loadingMore: boolean;
  error: string | null;
  filtersKey: string; // serialized filters for cache invalidation
}

interface AuditState {
  byProject: Record<string, AuditPageState>;
  detailById: Record<string, AuditEvent | undefined>;
  detailLoading: Record<string, boolean>;
  detailError: Record<string, string | null>;

  fetchEvents: (
    projectId: string,
    filters?: AuditQueryFilters,
    options?: { append?: boolean; cursor?: string },
  ) => Promise<void>;

  fetchEventDetail: (projectId: string, eventId: string) => Promise<void>;

  invalidate: (projectId?: string) => void;
}

function getToken(): string | undefined {
  const state = useAuthStore.getState();
  return state.accessToken ?? undefined;
}

function serializeFilters(filters?: AuditQueryFilters): string {
  if (!filters) return "";
  const sorted: Record<string, string> = {};
  for (const key of Object.keys(filters).sort()) {
    const v = (filters as Record<string, string | undefined>)[key];
    if (v !== undefined && v !== "") sorted[key] = v;
  }
  return JSON.stringify(sorted);
}

function buildQuery(filters?: AuditQueryFilters, cursor?: string, limit?: number): string {
  const params = new URLSearchParams();
  if (filters) {
    if (filters.actionId) params.set("actionId", filters.actionId);
    if (filters.actorUserId) params.set("actorUserId", filters.actorUserId);
    if (filters.resourceType) params.set("resourceType", filters.resourceType);
    if (filters.resourceId) params.set("resourceId", filters.resourceId);
    if (filters.from) params.set("from", filters.from);
    if (filters.to) params.set("to", filters.to);
  }
  if (cursor) params.set("cursor", cursor);
  if (limit) params.set("limit", String(limit));
  const qs = params.toString();
  return qs ? `?${qs}` : "";
}

const emptyPage = (filtersKey: string): AuditPageState => ({
  events: [],
  nextCursor: undefined,
  loading: false,
  loadingMore: false,
  error: null,
  filtersKey,
});

export const useAuditStore = create<AuditState>()(
  withDevtools((set, get) => ({
  byProject: {},
  detailById: {},
  detailLoading: {},
  detailError: {},

  async fetchEvents(projectId, filters, options) {
    if (!projectId) return;
    const filtersKey = serializeFilters(filters);
    const append = Boolean(options?.append);
    const cursor = options?.cursor;

    // Reset cached page when filters change and we're not appending.
    if (!append) {
      const current = get().byProject[projectId];
      if (!current || current.filtersKey !== filtersKey) {
        set((s) => ({
          byProject: { ...s.byProject, [projectId]: emptyPage(filtersKey) },
        }));
      }
    }

    set((s) => {
      const current = s.byProject[projectId] ?? emptyPage(filtersKey);
      return {
        byProject: {
          ...s.byProject,
          [projectId]: {
            ...current,
            loading: !append,
            loadingMore: append,
            error: null,
          },
        },
      };
    });

    try {
      const api = createApiClient(await resolveBackendUrl());
      const { data } = await api.get<AuditEventListResponse>(
        `/api/v1/projects/${projectId}/audit-events${buildQuery(filters, cursor)}`,
        { token: getToken() },
      );
      set((s) => {
        const current = s.byProject[projectId] ?? emptyPage(filtersKey);
        const next: AuditPageState = {
          filtersKey,
          loading: false,
          loadingMore: false,
          error: null,
          nextCursor: data.nextCursor,
          events: append
            ? [...current.events, ...(data.events ?? [])]
            : (data.events ?? []),
        };
        return { byProject: { ...s.byProject, [projectId]: next } };
      });
    } catch (error) {
      const message = error instanceof Error ? error.message : "fetchEvents failed";
      set((s) => {
        const current = s.byProject[projectId] ?? emptyPage(filtersKey);
        return {
          byProject: {
            ...s.byProject,
            [projectId]: { ...current, loading: false, loadingMore: false, error: message },
          },
        };
      });
    }
  },

  async fetchEventDetail(projectId, eventId) {
    if (!projectId || !eventId) return;
    set((s) => ({
      detailLoading: { ...s.detailLoading, [eventId]: true },
      detailError: { ...s.detailError, [eventId]: null },
    }));
    try {
      const api = createApiClient(await resolveBackendUrl());
      const { data } = await api.get<AuditEvent>(
        `/api/v1/projects/${projectId}/audit-events/${eventId}`,
        { token: getToken() },
      );
      set((s) => ({
        detailById: { ...s.detailById, [eventId]: data },
        detailLoading: { ...s.detailLoading, [eventId]: false },
      }));
    } catch (error) {
      const message = error instanceof Error ? error.message : "fetchEventDetail failed";
      set((s) => ({
        detailLoading: { ...s.detailLoading, [eventId]: false },
        detailError: { ...s.detailError, [eventId]: message },
      }));
    }
  },

  invalidate(projectId) {
    set((s) => {
      if (!projectId) return { byProject: {} };
      const { [projectId]: _drop, ...rest } = s.byProject;
      return { byProject: rest };
    });
  },
  }), { name: "audit-store" }),
);
