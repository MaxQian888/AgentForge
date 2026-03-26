"use client";

import { create } from "zustand";
import { createApiClient } from "@/lib/api-client";
import { useAuthStore } from "./auth-store";

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";

export interface EntityLink {
  id: string;
  projectId: string;
  sourceType: string;
  sourceId: string;
  targetType: string;
  targetId: string;
  linkType: string;
  anchorBlockId?: string | null;
  createdBy: string;
  createdAt: string;
  deletedAt?: string | null;
}

interface EntityLinkState {
  linksByEntity: Record<string, EntityLink[]>;
  loading: boolean;
  error: string | null;
  fetchLinks: (projectId: string, entityType: string, entityId: string) => Promise<void>;
  createLink: (input: {
    projectId: string;
    sourceType: string;
    sourceId: string;
    targetType: string;
    targetId: string;
    linkType: string;
    anchorBlockId?: string | null;
  }) => Promise<void>;
  deleteLink: (projectId: string, entityType: string, entityId: string, linkId: string) => Promise<void>;
  upsertLink: (link: EntityLink) => void;
  removeLink: (linkId: string) => void;
}

function getApi() {
  const token = useAuthStore.getState().accessToken;
  if (!token) {
    throw new Error("missing access token");
  }
  return { token, api: createApiClient(API_URL) };
}

function entityKey(entityType: string, entityId: string) {
  return `${entityType}:${entityId}`;
}

function normalizeLink(raw: Record<string, unknown>): EntityLink {
  return {
    id: String(raw.id ?? ""),
    projectId: String(raw.projectId ?? ""),
    sourceType: String(raw.sourceType ?? ""),
    sourceId: String(raw.sourceId ?? ""),
    targetType: String(raw.targetType ?? ""),
    targetId: String(raw.targetId ?? ""),
    linkType: String(raw.linkType ?? ""),
    anchorBlockId: typeof raw.anchorBlockId === "string" ? raw.anchorBlockId : null,
    createdBy: String(raw.createdBy ?? ""),
    createdAt: String(raw.createdAt ?? new Date().toISOString()),
    deletedAt: typeof raw.deletedAt === "string" ? raw.deletedAt : null,
  };
}

export const useEntityLinkStore = create<EntityLinkState>()((set, get) => ({
  linksByEntity: {},
  loading: false,
  error: null,

  fetchLinks: async (projectId, entityType, entityId) => {
    const { api, token } = getApi();
    set({ loading: true, error: null });
    try {
      const { data } = await api.get<Record<string, unknown>[]>(
        `/api/v1/projects/${projectId}/links?source_type=${entityType}&source_id=${entityId}`,
        { token },
      );
      set((state) => ({
        linksByEntity: {
          ...state.linksByEntity,
          [entityKey(entityType, entityId)]: data.map(normalizeLink),
        },
      }));
    } catch (error) {
      set({ error: error instanceof Error ? error.message : "Failed to load links" });
    } finally {
      set({ loading: false });
    }
  },

  createLink: async (input) => {
    const { api, token } = getApi();
    const { data } = await api.post<Record<string, unknown>>(
      `/api/v1/projects/${input.projectId}/links`,
      input,
      { token },
    );
    get().upsertLink(normalizeLink(data));
  },

  deleteLink: async (projectId, entityType, entityId, linkId) => {
    const { api, token } = getApi();
    await api.delete(`/api/v1/projects/${projectId}/links/${linkId}`, { token });
    await get().fetchLinks(projectId, entityType, entityId);
  },

  upsertLink: (link) => {
    set((state) => {
      const key = entityKey(link.sourceType, link.sourceId);
      const current = state.linksByEntity[key] ?? [];
      const existingIndex = current.findIndex((item) => item.id === link.id);
      const next =
        existingIndex === -1
          ? [...current, link]
          : current.map((item) => (item.id === link.id ? link : item));
      return {
        linksByEntity: {
          ...state.linksByEntity,
          [key]: next,
        },
      };
    });
  },

  removeLink: (linkId) => {
    set((state) => {
      const nextEntries = Object.fromEntries(
        Object.entries(state.linksByEntity).map(([key, value]) => [
          key,
          value.filter((link) => link.id !== linkId),
        ]),
      );
      return { linksByEntity: nextEntries };
    });
  },
}));
