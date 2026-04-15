"use client";

import { create } from "zustand";
import { createApiClient } from "@/lib/api-client";
import type { SkillPackagePreview } from "@/lib/stores/marketplace-store";
import { useAuthStore } from "./auth-store";

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";

export type SkillFamily =
  | "built-in-runtime"
  | "repo-assistant"
  | "workflow-mirror";

export type SkillHealthStatus =
  | "healthy"
  | "warning"
  | "blocked"
  | "drifted";

export interface SkillIssue {
  code: string;
  message: string;
  targetPath?: string;
  family?: SkillFamily;
  sourceType?: string;
}

export interface SkillHealthSummary {
  status: SkillHealthStatus;
  issues: SkillIssue[];
}

export interface SkillBundleInfo {
  member: boolean;
  category?: string;
  tags?: string[];
  docsRef?: string;
  featured?: boolean;
}

export interface SkillLockInfo {
  key: string;
  source?: string;
  sourceType?: string;
  computedHash?: string;
}

export interface SkillConsumerSurface {
  id: string;
  status: string;
  label: string;
  href?: string;
  message?: string;
}

export interface SkillBlockedAction {
  id: string;
  reason: string;
}

export interface GovernedSkillItem {
  id: string;
  family: SkillFamily;
  verificationProfile?: string;
  canonicalRoot: string;
  sourceType: string;
  docsRef?: string;
  lock?: SkillLockInfo | null;
  bundle: SkillBundleInfo;
  mirrorTargets?: string[];
  previewAvailable: boolean;
  previewError?: string;
  health: SkillHealthSummary;
  consumerSurfaces: SkillConsumerSurface[];
  supportedActions?: string[];
  blockedActions?: SkillBlockedAction[];
  preview?: SkillPackagePreview | null;
}

export interface SkillsVerifyResult {
  ok: boolean;
  results: Array<{
    skillId: string;
    family: SkillFamily;
    status: SkillHealthStatus;
    issues?: SkillIssue[];
  }>;
}

export interface SkillsSyncMirrorsResult {
  updatedTargets: string[];
  results: Array<{
    skillId: string;
    family: SkillFamily;
    status: SkillHealthStatus;
    issues?: SkillIssue[];
  }>;
}

export interface SkillsFilters {
  family: "all" | SkillFamily;
  status: "all" | SkillHealthStatus;
  query: string;
}

interface SkillsState {
  items: GovernedSkillItem[];
  selectedSkill: GovernedSkillItem | null;
  loading: boolean;
  detailLoading: boolean;
  actionLoading: boolean;
  error: string | null;
  filters: SkillsFilters;
  fetchSkills: () => Promise<void>;
  fetchSkillDetail: (id: string) => Promise<void>;
  verifySkills: (families?: SkillFamily[]) => Promise<SkillsVerifyResult>;
  syncMirrors: (skillIds?: string[]) => Promise<SkillsSyncMirrorsResult>;
  selectSkill: (id: string) => Promise<void>;
  setFilters: (next: Partial<SkillsFilters>) => void;
}

function getToken() {
  const authState = useAuthStore.getState() as {
    accessToken?: string | null;
    token?: string | null;
  };
  return authState.accessToken ?? authState.token ?? null;
}

export const useSkillsStore = create<SkillsState>()((set, get) => ({
  items: [],
  selectedSkill: null,
  loading: false,
  detailLoading: false,
  actionLoading: false,
  error: null,
  filters: {
    family: "all",
    status: "all",
    query: "",
  },

  fetchSkills: async () => {
    const token = getToken();
    if (!token) return;

    set({ loading: true, error: null });

    try {
      const api = createApiClient(API_URL);
      const { data } = await api.get<{ items: GovernedSkillItem[] }>("/api/v1/skills", {
        token,
      });

      const selectedId = get().selectedSkill?.id ?? null;
      const nextItems = data.items ?? [];
      set({ items: nextItems });

      if (selectedId) {
        const stillExists = nextItems.some((item) => item.id === selectedId);
        if (stillExists) {
          await get().fetchSkillDetail(selectedId);
        } else if (nextItems[0]) {
          await get().fetchSkillDetail(nextItems[0].id);
        } else {
          set({ selectedSkill: null });
        }
      } else if (nextItems[0]) {
        await get().fetchSkillDetail(nextItems[0].id);
      }
    } catch (error) {
      set({
        error: error instanceof Error ? error.message : "Failed to load skills",
      });
    } finally {
      set({ loading: false });
    }
  },

  fetchSkillDetail: async (id) => {
    const token = getToken();
    if (!token) return;

    set({ detailLoading: true, error: null });

    try {
      const api = createApiClient(API_URL);
      const { data } = await api.get<GovernedSkillItem>(`/api/v1/skills/${id}`, {
        token,
      });
      set({ selectedSkill: data });
    } catch (error) {
      set({
        error:
          error instanceof Error ? error.message : "Failed to load skill detail",
      });
    } finally {
      set({ detailLoading: false });
    }
  },

  verifySkills: async (families) => {
    const token = getToken();
    if (!token) {
      return { ok: false, results: [] };
    }

    set({ actionLoading: true, error: null });
    try {
      const api = createApiClient(API_URL);
      const { data } = await api.post<SkillsVerifyResult>(
        "/api/v1/skills/verify",
        families && families.length > 0 ? { families } : {},
        { token },
      );
      await get().fetchSkills();
      return data;
    } finally {
      set({ actionLoading: false });
    }
  },

  syncMirrors: async (skillIds) => {
    const token = getToken();
    if (!token) {
      return { updatedTargets: [], results: [] };
    }

    set({ actionLoading: true, error: null });
    try {
      const api = createApiClient(API_URL);
      const { data } = await api.post<SkillsSyncMirrorsResult>(
        "/api/v1/skills/sync-mirrors",
        skillIds && skillIds.length > 0 ? { skillIds } : {},
        { token },
      );
      await get().fetchSkills();
      return data;
    } finally {
      set({ actionLoading: false });
    }
  },

  selectSkill: async (id) => {
    await get().fetchSkillDetail(id);
  },

  setFilters: (next) =>
    set((state) => ({
      filters: {
        ...state.filters,
        ...next,
      },
    })),
}));
