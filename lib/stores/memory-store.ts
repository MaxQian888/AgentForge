"use client";

import { create } from "zustand";
import { createApiClient } from "@/lib/api-client";
import { useAuthStore } from "./auth-store";

export interface AgentMemoryRelatedContext {
  type: string;
  id: string;
  label?: string;
}

export interface AgentMemoryEntry {
  id: string;
  projectId: string;
  scope: "global" | "project" | "role";
  roleId: string;
  category: "episodic" | "semantic" | "procedural" | "document";
  kind?: string;
  tags: string[];
  editable: boolean;
  key: string;
  content: string;
  metadata: string;
  metadataObject?: Record<string, unknown> | null;
  relatedContext?: AgentMemoryRelatedContext[];
  relevanceScore: number;
  accessCount: number;
  lastAccessedAt?: string | null;
  createdAt: string;
  updatedAt?: string;
}

export interface AgentMemoryDetail extends AgentMemoryEntry {
  metadataObject?: Record<string, unknown> | null;
  relatedContext?: AgentMemoryRelatedContext[];
}

export interface MemorySearchOptions {
  query?: string;
  scope?: string;
  category?: string;
  roleId?: string;
  tag?: string;
  startAt?: string;
  endAt?: string;
  limit?: number;
}

export interface MemoryExplorerFilters {
  query: string;
  scope: AgentMemoryEntry["scope"] | "all";
  category: AgentMemoryEntry["category"] | "all";
  roleId: string;
  tag: string;
  startAt: string;
  endAt: string;
  limit: number;
}

export interface MemoryPaginationState {
  page: number;
  pageSize: number;
}

export interface MemoryExplorerStats {
  totalCount: number;
  approxStorageBytes: number;
  byCategory: Record<string, number>;
  byScope: Record<string, number>;
  oldestCreatedAt?: string;
  newestCreatedAt?: string;
  lastAccessedAt?: string;
}

export interface MemoryDeleteResult {
  deletedCount: number;
}

export interface MemoryCleanupInput {
  scope?: string;
  roleId?: string;
  before?: string;
  retentionDays?: number;
}

export interface MemoryExportEntry {
  id: string;
  scope: string;
  roleId?: string;
  category: string;
  key: string;
  content: string;
  metadata: string;
  createdAt: string;
  updatedAt?: string;
}

export interface MemoryExportPayload {
  projectId: string;
  exportedAt: string;
  entries: MemoryExportEntry[];
}

export interface MemoryMutationResult {
  type:
    | "single-delete"
    | "bulk-delete"
    | "bulk-delete-criteria"
    | "cleanup"
    | "create-note"
    | "update-note"
    | "tag-added"
    | "tag-removed";
  deletedCount: number;
}

export type MemoryExportFormat = "json" | "csv";

export interface MemoryExportBlob {
  content: string;
  mimeType: string;
  extension: "json" | "csv";
}

interface MemoryState {
  currentProjectId: string | null;
  filters: MemoryExplorerFilters;
  pagination: MemoryPaginationState;
  entries: AgentMemoryEntry[];
  stats: MemoryExplorerStats | null;
  detail: AgentMemoryDetail | null;
  selectedMemoryId: string | null;
  selectedMemoryIds: string[];
  loading: boolean;
  statsLoading: boolean;
  detailLoading: boolean;
  actionLoading: boolean;
  error: string | null;
  statsError: string | null;
  detailError: string | null;
  actionError: string | null;
  lastMutation: MemoryMutationResult | null;
  setFilters: (partial: Partial<MemoryExplorerFilters>) => void;
  resetFilters: () => void;
  setPagination: (partial: Partial<MemoryPaginationState>) => void;
  loadWorkspace: (projectId: string, options?: MemorySearchOptions) => Promise<void>;
  searchMemory: (projectId: string, options?: MemorySearchOptions) => Promise<void>;
  fetchMemoryDetail: (projectId: string, memoryId: string, roleId?: string) => Promise<void>;
  selectMemory: (memoryId: string | null) => void;
  toggleMemorySelection: (memoryId: string) => void;
  setSelectedMemoryIds: (ids: string[]) => void;
  clearSelection: () => void;
  storeMemory: (
    projectId: string,
    input: {
      key: string;
      content: string;
      scope?: string;
      roleId?: string;
      category?: string;
      kind?: string;
      tags?: string[];
    },
  ) => Promise<void>;
  updateMemory: (
    projectId: string,
    memoryId: string,
    input: {
      key?: string;
      content?: string;
      tags?: string[];
      roleId?: string;
    },
  ) => Promise<AgentMemoryDetail | null>;
  deleteMemory: (projectId: string, memoryId: string) => Promise<void>;
  bulkDeleteMemories: (
    projectId: string,
    memoryIds: string[],
    roleId?: string,
  ) => Promise<MemoryDeleteResult>;
  bulkDeleteByCriteria: (
    projectId: string,
    criteria?: { ids?: string[]; roleId?: string },
  ) => Promise<MemoryDeleteResult>;
  addMemoryTag: (
    projectId: string,
    memoryId: string,
    tag: string,
  ) => Promise<AgentMemoryDetail | null>;
  removeMemoryTag: (
    projectId: string,
    memoryId: string,
    tag: string,
  ) => Promise<AgentMemoryDetail | null>;
  cleanupMemories: (
    projectId: string,
    input: MemoryCleanupInput,
  ) => Promise<MemoryDeleteResult>;
  exportMemories: (
    projectId: string,
    options?: MemorySearchOptions,
  ) => Promise<MemoryExportPayload | null>;
  exportMemoryEntry: (
    projectId: string,
    memoryId: string,
    roleId?: string,
  ) => Promise<AgentMemoryDetail | null>;
  buildExportBlob: (
    payload: MemoryExportPayload | AgentMemoryEntry | AgentMemoryDetail,
    format: MemoryExportFormat,
  ) => MemoryExportBlob;
  clearActionFeedback: () => void;
}

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";

const DEFAULT_FILTERS: MemoryExplorerFilters = {
  query: "",
  scope: "all",
  category: "all",
  roleId: "",
  tag: "",
  startAt: "",
  endAt: "",
  limit: 20,
};

const DEFAULT_PAGINATION: MemoryPaginationState = {
  page: 1,
  pageSize: 10,
};

function normalizeString(value: unknown): string {
  return typeof value === "string" ? value.trim() : "";
}

function extractErrorMessage(error: unknown): string {
  if (error instanceof Error && error.message) {
    return error.message;
  }
  return "Unknown memory request failure";
}

function normalizeMemoryEntry(raw: Record<string, unknown>): AgentMemoryEntry {
  return {
    id: String(raw.id ?? ""),
    projectId: String(raw.projectId ?? ""),
    scope: (typeof raw.scope === "string" ? raw.scope : "project") as AgentMemoryEntry["scope"],
    roleId: String(raw.roleId ?? ""),
    category: (typeof raw.category === "string"
      ? raw.category
      : "semantic") as AgentMemoryEntry["category"],
    kind: typeof raw.kind === "string" ? raw.kind : undefined,
    tags: Array.isArray(raw.tags)
      ? raw.tags
          .filter((tag): tag is string => typeof tag === "string")
          .map((tag) => tag.trim())
          .filter(Boolean)
      : [],
    editable: Boolean(raw.editable),
    key: String(raw.key ?? ""),
    content: String(raw.content ?? ""),
    metadata:
      typeof raw.metadata === "string"
        ? raw.metadata
        : JSON.stringify(raw.metadata ?? ""),
    metadataObject:
      raw.metadataObject && typeof raw.metadataObject === "object"
        ? (raw.metadataObject as Record<string, unknown>)
        : null,
    relatedContext: Array.isArray(raw.relatedContext)
      ? raw.relatedContext.map((item) => ({
          type: String((item as Record<string, unknown>).type ?? ""),
          id: String((item as Record<string, unknown>).id ?? ""),
          label:
            typeof (item as Record<string, unknown>).label === "string"
              ? String((item as Record<string, unknown>).label)
              : undefined,
        }))
      : undefined,
    relevanceScore: Number(raw.relevanceScore ?? 0),
    accessCount: Number(raw.accessCount ?? 0),
    lastAccessedAt:
      typeof raw.lastAccessedAt === "string" ? raw.lastAccessedAt : null,
    createdAt:
      typeof raw.createdAt === "string"
        ? raw.createdAt
        : new Date().toISOString(),
    updatedAt: typeof raw.updatedAt === "string" ? raw.updatedAt : undefined,
  };
}

function normalizeMemoryDetail(raw: Record<string, unknown>): AgentMemoryDetail {
  return normalizeMemoryEntry(raw);
}

function normalizeStats(raw: Record<string, unknown>): MemoryExplorerStats {
  return {
    totalCount: Number(raw.totalCount ?? 0),
    approxStorageBytes: Number(raw.approxStorageBytes ?? 0),
    byCategory:
      raw.byCategory && typeof raw.byCategory === "object"
        ? Object.fromEntries(
            Object.entries(raw.byCategory as Record<string, unknown>).map(
              ([key, value]) => [key, Number(value ?? 0)],
            ),
          )
        : {},
    byScope:
      raw.byScope && typeof raw.byScope === "object"
        ? Object.fromEntries(
            Object.entries(raw.byScope as Record<string, unknown>).map(
              ([key, value]) => [key, Number(value ?? 0)],
            ),
          )
        : {},
    oldestCreatedAt:
      typeof raw.oldestCreatedAt === "string" ? raw.oldestCreatedAt : undefined,
    newestCreatedAt:
      typeof raw.newestCreatedAt === "string" ? raw.newestCreatedAt : undefined,
    lastAccessedAt:
      typeof raw.lastAccessedAt === "string" ? raw.lastAccessedAt : undefined,
  };
}

function normalizeExportPayload(raw: Record<string, unknown>): MemoryExportPayload {
  return {
    projectId: String(raw.projectId ?? ""),
    exportedAt:
      typeof raw.exportedAt === "string"
        ? raw.exportedAt
        : new Date().toISOString(),
    entries: Array.isArray(raw.entries)
      ? raw.entries.map((entry) => ({
          id: String((entry as Record<string, unknown>).id ?? ""),
          scope: String((entry as Record<string, unknown>).scope ?? ""),
          roleId:
            typeof (entry as Record<string, unknown>).roleId === "string"
              ? String((entry as Record<string, unknown>).roleId)
              : undefined,
          category: String((entry as Record<string, unknown>).category ?? ""),
          key: String((entry as Record<string, unknown>).key ?? ""),
          content: String((entry as Record<string, unknown>).content ?? ""),
          metadata:
            typeof (entry as Record<string, unknown>).metadata === "string"
              ? String((entry as Record<string, unknown>).metadata)
              : JSON.stringify(
                  (entry as Record<string, unknown>).metadata ?? "",
                ),
          createdAt: String((entry as Record<string, unknown>).createdAt ?? ""),
          updatedAt:
            typeof (entry as Record<string, unknown>).updatedAt === "string"
              ? String((entry as Record<string, unknown>).updatedAt)
              : undefined,
        }))
      : [],
  };
}

function resolveFilters(
  current: MemoryExplorerFilters,
  options?: MemorySearchOptions,
): MemoryExplorerFilters {
  if (!options) {
    return current;
  }

  return {
    query: options.query ?? current.query,
    scope: (options.scope ?? current.scope) as MemoryExplorerFilters["scope"],
    category: (options.category ??
      current.category) as MemoryExplorerFilters["category"],
    roleId: options.roleId ?? current.roleId,
    tag: options.tag ?? current.tag,
    startAt: options.startAt ?? current.startAt,
    endAt: options.endAt ?? current.endAt,
    limit:
      typeof options.limit === "number" && options.limit > 0
        ? options.limit
        : current.limit,
  };
}

function buildSearchParams(filters: MemoryExplorerFilters): URLSearchParams {
  const params = new URLSearchParams();
  if (filters.query) params.set("query", filters.query);
  if (filters.scope && filters.scope !== "all") params.set("scope", filters.scope);
  if (filters.category && filters.category !== "all") {
    params.set("category", filters.category);
  }
  if (filters.roleId) params.set("roleId", filters.roleId);
  if (filters.tag) params.set("tag", filters.tag);
  if (filters.startAt) params.set("startAt", filters.startAt);
  if (filters.endAt) params.set("endAt", filters.endAt);
  if (filters.limit > 0) params.set("limit", String(filters.limit));
  return params;
}

function pruneSelection(
  entries: AgentMemoryEntry[],
  selectedMemoryId: string | null,
  selectedMemoryIds: string[],
) {
  const selectedIds = selectedMemoryIds.filter((id) =>
    entries.some((entry) => entry.id === id),
  );
  const currentSelection =
    selectedMemoryId && entries.some((entry) => entry.id === selectedMemoryId)
      ? selectedMemoryId
      : null;

  return {
    selectedMemoryId: currentSelection,
    selectedMemoryIds: selectedIds,
  };
}

function normalizeTags(tags?: string[]) {
  if (!tags?.length) return [];
  const seen = new Set<string>();
  const normalized: string[] = [];
  for (const tag of tags) {
    const value = tag.trim();
    if (!value) continue;
    const key = value.toLowerCase();
    if (seen.has(key)) continue;
    seen.add(key);
    normalized.push(value);
  }
  return normalized;
}

export const useMemoryStore = create<MemoryState>()((set, get) => ({
  currentProjectId: null,
  filters: DEFAULT_FILTERS,
  pagination: DEFAULT_PAGINATION,
  entries: [],
  stats: null,
  detail: null,
  selectedMemoryId: null,
  selectedMemoryIds: [],
  loading: false,
  statsLoading: false,
  detailLoading: false,
  actionLoading: false,
  error: null,
  statsError: null,
  detailError: null,
  actionError: null,
  lastMutation: null,

  setFilters: (partial) => {
    set((state) => ({
      filters: {
        ...state.filters,
        ...partial,
      },
      pagination: { ...state.pagination, page: 1 },
    }));
  },

  resetFilters: () => {
    set({ filters: DEFAULT_FILTERS, pagination: DEFAULT_PAGINATION });
  },

  setPagination: (partial) => {
    set((state) => {
      const nextSize =
        typeof partial.pageSize === "number" && partial.pageSize > 0
          ? partial.pageSize
          : state.pagination.pageSize;
      const nextPage =
        typeof partial.page === "number" && partial.page > 0
          ? partial.page
          : partial.pageSize
            ? 1
            : state.pagination.page;
      return {
        pagination: {
          page: nextPage,
          pageSize: nextSize,
        },
      };
    });
  },

  loadWorkspace: async (projectId, options) => {
    const nextFilters = resolveFilters(get().filters, options);
    const token = useAuthStore.getState().accessToken;

    set((state) => ({
      currentProjectId: projectId,
      filters: nextFilters,
      loading: !!token,
      statsLoading: !!token,
      error: null,
      statsError: null,
      ...(state.currentProjectId !== projectId
        ? {
            selectedMemoryId: null,
            selectedMemoryIds: [],
            detail: null,
            detailError: null,
          }
        : {}),
    }));

    if (!token) {
      return;
    }

    const api = createApiClient(API_URL);
    const qs = buildSearchParams(nextFilters).toString();
    const listUrl = `/api/v1/projects/${projectId}/memory${qs ? `?${qs}` : ""}`;
    const statsUrl = `/api/v1/projects/${projectId}/memory/stats${qs ? `?${qs}` : ""}`;

    const [listResult, statsResult] = await Promise.allSettled([
      api.get<Record<string, unknown>[]>(listUrl, { token }),
      api.get<Record<string, unknown>>(statsUrl, { token }),
    ]);

    set((state) => {
      const nextState: Partial<MemoryState> = {
        loading: false,
        statsLoading: false,
      };

      if (listResult.status === "fulfilled") {
        const entries = listResult.value.data.map(normalizeMemoryEntry);
        const pruned = pruneSelection(
          entries,
          state.selectedMemoryId,
          state.selectedMemoryIds,
        );
        nextState.entries = entries;
        nextState.selectedMemoryId = pruned.selectedMemoryId;
        nextState.selectedMemoryIds = pruned.selectedMemoryIds;
        if (!pruned.selectedMemoryId) {
          nextState.detail = null;
          nextState.detailError = null;
        }
      } else {
        nextState.error = extractErrorMessage(listResult.reason);
      }

      if (statsResult.status === "fulfilled") {
        nextState.stats = normalizeStats(statsResult.value.data);
      } else {
        nextState.statsError = extractErrorMessage(statsResult.reason);
      }

      return nextState as MemoryState;
    });
  },

  searchMemory: async (projectId, options) => {
    await get().loadWorkspace(projectId, options);
  },

  fetchMemoryDetail: async (projectId, memoryId, roleId) => {
    const token = useAuthStore.getState().accessToken;
    set({
      selectedMemoryId: memoryId,
      detailLoading: !!token,
      detailError: null,
    });
    if (!token) {
      return;
    }

    const api = createApiClient(API_URL);
    const params = new URLSearchParams();
    if (normalizeString(roleId)) {
      params.set("roleId", normalizeString(roleId));
    }
    const qs = params.toString();

    try {
      const { data } = await api.get<Record<string, unknown>>(
        `/api/v1/projects/${projectId}/memory/${memoryId}${qs ? `?${qs}` : ""}`,
        { token },
      );
      set({
        detail: normalizeMemoryDetail(data),
        detailLoading: false,
        detailError: null,
      });
    } catch (error) {
      const message = extractErrorMessage(error);
      set({
        detailLoading: false,
        detailError: message,
      });
      throw error;
    }
  },

  selectMemory: (memoryId) => {
    set({
      selectedMemoryId: memoryId,
      ...(memoryId ? {} : { detail: null, detailError: null }),
    });
  },

  toggleMemorySelection: (memoryId) => {
    set((state) => ({
      selectedMemoryIds: state.selectedMemoryIds.includes(memoryId)
        ? state.selectedMemoryIds.filter((id) => id !== memoryId)
        : [...state.selectedMemoryIds, memoryId],
    }));
  },

  setSelectedMemoryIds: (ids) => {
    set({ selectedMemoryIds: ids });
  },

  clearSelection: () => {
    set({ selectedMemoryIds: [] });
  },

  storeMemory: async (projectId, input) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return;

    const api = createApiClient(API_URL);
    const tags = normalizeTags(input.tags);
    const metadata =
      input.kind || tags.length > 0
        ? JSON.stringify({
            ...(input.kind ? { kind: input.kind } : {}),
            ...(input.kind === "operator_note" ? { editable: true } : {}),
            ...(tags.length > 0 ? { tags } : {}),
          })
        : undefined;
    const { data } = await api.post<Record<string, unknown>>(
      `/api/v1/projects/${projectId}/memory`,
      {
        key: input.key,
        content: input.content,
        scope: input.scope,
        roleId: input.roleId,
        category: input.category,
        tags: tags.length > 0 ? tags : undefined,
        metadata,
      },
      { token },
    );
    const entry = normalizeMemoryEntry(data);
    await get().loadWorkspace(projectId);
    set({
      selectedMemoryId: entry.id,
      detail: entry,
      actionError: null,
      lastMutation: { type: "create-note", deletedCount: 1 },
    });
  },

  updateMemory: async (projectId, memoryId, input) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) {
      return null;
    }

    const api = createApiClient(API_URL);
    set({
      actionLoading: true,
      actionError: null,
      lastMutation: null,
    });

    const params = new URLSearchParams();
    if (normalizeString(input.roleId)) {
      params.set("roleId", normalizeString(input.roleId));
    }
    const qs = params.toString();

    try {
      const { data } = await api.patch<Record<string, unknown>>(
        `/api/v1/projects/${projectId}/memory/${memoryId}${qs ? `?${qs}` : ""}`,
        {
          ...(typeof input.key === "string" ? { key: input.key } : {}),
          ...(typeof input.content === "string" ? { content: input.content } : {}),
          ...(input.tags ? { tags: normalizeTags(input.tags) } : {}),
        },
        { token },
      );
      const detail = normalizeMemoryDetail(data);
      await get().loadWorkspace(projectId);
      set({
        selectedMemoryId: memoryId,
        detail,
        actionLoading: false,
        actionError: null,
        lastMutation: { type: "update-note", deletedCount: 1 },
      });
      return detail;
    } catch (error) {
      const message = extractErrorMessage(error);
      set({
        actionLoading: false,
        actionError: message,
      });
      throw error;
    }
  },

  deleteMemory: async (projectId, memoryId) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return;

    const api = createApiClient(API_URL);
    set({
      actionLoading: true,
      actionError: null,
      lastMutation: null,
    });

    try {
      await api.delete(`/api/v1/projects/${projectId}/memory/${memoryId}`, { token });
      set((state) => ({
        entries: state.entries.filter((entry) => entry.id !== memoryId),
        selectedMemoryIds: state.selectedMemoryIds.filter((id) => id !== memoryId),
        ...(state.selectedMemoryId === memoryId
          ? {
              selectedMemoryId: null,
              detail: null,
              detailError: null,
            }
          : {}),
      }));
      await get().loadWorkspace(projectId);
      set({
        actionLoading: false,
        lastMutation: { type: "single-delete", deletedCount: 1 },
      });
    } catch (error) {
      const message = extractErrorMessage(error);
      set({
        actionLoading: false,
        actionError: message,
      });
      throw error;
    }
  },

  bulkDeleteMemories: async (projectId, memoryIds, roleId) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) {
      return { deletedCount: 0 };
    }

    const api = createApiClient(API_URL);
    set({
      actionLoading: true,
      actionError: null,
      lastMutation: null,
    });

    try {
      const { data } = await api.post<Record<string, unknown>>(
        `/api/v1/projects/${projectId}/memory/bulk-delete`,
        {
          ids: memoryIds,
          roleId: normalizeString(roleId) || undefined,
        },
        { token },
      );
      const result = {
        deletedCount: Number(data.deletedCount ?? 0),
      };

      set((state) => ({
        selectedMemoryIds: [],
        ...(memoryIds.includes(state.selectedMemoryId ?? "")
          ? {
              selectedMemoryId: null,
              detail: null,
              detailError: null,
            }
          : {}),
      }));
      await get().loadWorkspace(projectId);
      set({
        actionLoading: false,
        lastMutation: { type: "bulk-delete", deletedCount: result.deletedCount },
      });
      return result;
    } catch (error) {
      const message = extractErrorMessage(error);
      set({
        actionLoading: false,
        actionError: message,
      });
      throw error;
    }
  },

  bulkDeleteByCriteria: async (projectId, criteria) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) {
      return { deletedCount: 0 };
    }

    const state = get();
    const roleId =
      normalizeString(criteria?.roleId) || state.filters.roleId || undefined;
    let ids = criteria?.ids;
    if (!ids || ids.length === 0) {
      ids = state.entries.map((entry) => entry.id);
    }
    if (!ids || ids.length === 0) {
      return { deletedCount: 0 };
    }

    const api = createApiClient(API_URL);
    set({
      actionLoading: true,
      actionError: null,
      lastMutation: null,
    });

    try {
      const { data } = await api.post<Record<string, unknown>>(
        `/api/v1/projects/${projectId}/memory/bulk-delete`,
        {
          ids,
          roleId,
        },
        { token },
      );
      const result = {
        deletedCount: Number(data.deletedCount ?? 0),
      };
      await get().loadWorkspace(projectId);
      set({
        selectedMemoryIds: [],
        actionLoading: false,
        lastMutation: {
          type: "bulk-delete-criteria",
          deletedCount: result.deletedCount,
        },
      });
      return result;
    } catch (error) {
      const message = extractErrorMessage(error);
      set({
        actionLoading: false,
        actionError: message,
      });
      throw error;
    }
  },

  addMemoryTag: async (projectId, memoryId, tag) => {
    const value = tag.trim();
    if (!value) return null;

    const entry =
      get().entries.find((item) => item.id === memoryId) ??
      (get().detail?.id === memoryId ? get().detail : null);
    if (!entry) return null;

    const tags = normalizeTags([...(entry.tags ?? []), value]);
    const updated = await get().updateMemory(projectId, memoryId, {
      tags,
      roleId: entry.roleId || undefined,
    });
    set({ lastMutation: { type: "tag-added", deletedCount: 1 } });
    return updated;
  },

  removeMemoryTag: async (projectId, memoryId, tag) => {
    const target = tag.trim();
    if (!target) return null;

    const entry =
      get().entries.find((item) => item.id === memoryId) ??
      (get().detail?.id === memoryId ? get().detail : null);
    if (!entry) return null;

    const tags = (entry.tags ?? []).filter(
      (existing) => existing.trim().toLowerCase() !== target.toLowerCase(),
    );
    const updated = await get().updateMemory(projectId, memoryId, {
      tags,
      roleId: entry.roleId || undefined,
    });
    set({ lastMutation: { type: "tag-removed", deletedCount: 1 } });
    return updated;
  },

  cleanupMemories: async (projectId, input) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) {
      return { deletedCount: 0 };
    }

    const api = createApiClient(API_URL);
    set({
      actionLoading: true,
      actionError: null,
      lastMutation: null,
    });

    try {
      const { data } = await api.post<Record<string, unknown>>(
        `/api/v1/projects/${projectId}/memory/cleanup`,
        {
          scope: normalizeString(input.scope) || undefined,
          roleId: normalizeString(input.roleId) || undefined,
          before: normalizeString(input.before) || undefined,
          retentionDays: input.retentionDays,
        },
        { token },
      );
      const result = {
        deletedCount: Number(data.deletedCount ?? 0),
      };
      await get().loadWorkspace(projectId);
      set({
        actionLoading: false,
        lastMutation: { type: "cleanup", deletedCount: result.deletedCount },
      });
      return result;
    } catch (error) {
      const message = extractErrorMessage(error);
      set({
        actionLoading: false,
        actionError: message,
      });
      throw error;
    }
  },

  exportMemories: async (projectId, options) => {
    const token = useAuthStore.getState().accessToken;
    const filters = resolveFilters(get().filters, options);
    if (!token) {
      return null;
    }

    const api = createApiClient(API_URL);
    set({
      currentProjectId: projectId,
      filters,
      actionLoading: true,
      actionError: null,
    });

    try {
      const qs = buildSearchParams(filters).toString();
      const { data } = await api.get<Record<string, unknown>>(
        `/api/v1/projects/${projectId}/memory/export${qs ? `?${qs}` : ""}`,
        { token },
      );
      set({ actionLoading: false });
      return normalizeExportPayload(data);
    } catch (error) {
      const message = extractErrorMessage(error);
      set({
        actionLoading: false,
        actionError: message,
      });
      throw error;
    }
  },

  exportMemoryEntry: async (projectId, memoryId, roleId) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) {
      return null;
    }

    const currentDetail = get().detail;
    if (currentDetail && currentDetail.id === memoryId) {
      return currentDetail;
    }

    const api = createApiClient(API_URL);
    const params = new URLSearchParams();
    if (normalizeString(roleId)) {
      params.set("roleId", normalizeString(roleId));
    }
    const qs = params.toString();

    const { data } = await api.get<Record<string, unknown>>(
      `/api/v1/projects/${projectId}/memory/${memoryId}${qs ? `?${qs}` : ""}`,
      { token },
    );
    return normalizeMemoryDetail(data);
  },

  buildExportBlob: (payload, format) => {
    if (format === "csv") {
      const candidate = payload as { entries?: unknown };
      const entries: AgentMemoryEntry[] = Array.isArray(candidate?.entries)
        ? (candidate.entries as unknown as AgentMemoryEntry[])
        : [payload as AgentMemoryEntry];
      const header = [
        "id",
        "scope",
        "roleId",
        "category",
        "key",
        "content",
        "tags",
        "createdAt",
        "updatedAt",
      ];
      const escape = (value: unknown) => {
        const text = value === undefined || value === null ? "" : String(value);
        if (/[",\n]/.test(text)) {
          return `"${text.replace(/"/g, '""')}"`;
        }
        return text;
      };
      const rows = entries.map((entry) =>
        [
          entry.id,
          entry.scope,
          entry.roleId,
          entry.category,
          entry.key,
          entry.content,
          Array.isArray(entry.tags) ? entry.tags.join("|") : "",
          entry.createdAt,
          entry.updatedAt ?? "",
        ]
          .map(escape)
          .join(","),
      );
      return {
        content: [header.join(","), ...rows].join("\n"),
        mimeType: "text/csv",
        extension: "csv",
      };
    }
    return {
      content: JSON.stringify(payload, null, 2),
      mimeType: "application/json",
      extension: "json",
    };
  },

  clearActionFeedback: () => {
    set({
      actionError: null,
      lastMutation: null,
    });
  },
}));
