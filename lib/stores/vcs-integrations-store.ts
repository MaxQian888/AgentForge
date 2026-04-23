"use client";

import { create } from "zustand";
import { toast } from "sonner";
import { createApiClient } from "@/lib/api-client";
import { useAuthStore } from "./auth-store";
import { getPreferredLocale } from "./locale-store";

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";

export type VCSProvider = "github" | "gitlab" | "gitea";

export type VCSIntegrationStatus = "active" | "auth_expired" | "paused";

export interface VCSIntegration {
  id: string;
  projectId: string;
  provider: VCSProvider;
  host: string;
  owner: string;
  repo: string;
  defaultBranch: string;
  webhookId?: string;
  webhookSecretRef: string;
  tokenSecretRef: string;
  status: VCSIntegrationStatus;
  actingEmployeeId?: string;
  lastSyncedAt?: string;
  createdAt: string;
  updatedAt: string;
}

export interface CreateIntegrationInput {
  provider: VCSProvider;
  host: string;
  owner: string;
  repo: string;
  defaultBranch?: string;
  tokenSecretRef: string;
  webhookSecretRef: string;
  actingEmployeeId?: string;
}

export interface PatchIntegrationInput {
  status?: VCSIntegrationStatus;
  tokenSecretRef?: string;
  actingEmployeeId?: string;
}

interface VCSIntegrationsStoreState {
  integrationsByProject: Record<string, VCSIntegration[]>;
  loadingByProject: Record<string, boolean>;
  fetchIntegrations: (projectId: string) => Promise<void>;
  createIntegration: (
    projectId: string,
    input: CreateIntegrationInput,
  ) => Promise<VCSIntegration | null>;
  patchIntegration: (
    id: string,
    input: PatchIntegrationInput,
  ) => Promise<VCSIntegration | null>;
  deleteIntegration: (projectId: string, id: string) => Promise<void>;
  syncIntegration: (id: string) => Promise<void>;
}

const getApi = () => createApiClient(API_URL);
const getToken = () => {
  const state = useAuthStore.getState() as {
    accessToken?: string | null;
    token?: string | null;
  };
  return state.accessToken ?? state.token ?? null;
};

export const useVCSIntegrationsStore = create<VCSIntegrationsStoreState>()(
  (set) => ({
    integrationsByProject: {},
    loadingByProject: {},

    fetchIntegrations: async (projectId) => {
      const token = getToken();
      if (!token) return;
      set((s) => ({
        loadingByProject: { ...s.loadingByProject, [projectId]: true },
      }));
      try {
        const { data } = await getApi().get<VCSIntegration[]>(
          `/api/v1/projects/${projectId}/vcs-integrations`,
          { token },
        );
        set((s) => ({
          integrationsByProject: {
            ...s.integrationsByProject,
            [projectId]: data ?? [],
          },
        }));
      } catch (err) {
        const locale = getPreferredLocale();
      toast.error(locale === "zh-CN" ? `加载 VCS 集成失败: ${(err as Error).message}` : `Failed to load VCS integrations: ${(err as Error).message}`);
      } finally {
        set((s) => ({
          loadingByProject: { ...s.loadingByProject, [projectId]: false },
        }));
      }
    },

    createIntegration: async (projectId, input) => {
      const token = getToken();
      if (!token) return null;
      try {
        const { data } = await getApi().post<VCSIntegration>(
          `/api/v1/projects/${projectId}/vcs-integrations`,
          input,
          { token },
        );
        set((s) => ({
          integrationsByProject: {
            ...s.integrationsByProject,
            [projectId]: [data, ...(s.integrationsByProject[projectId] ?? [])],
          },
        }));
        const locale = getPreferredLocale();
        toast.success(locale === "zh-CN" ? `已连接 ${input.owner}/${input.repo}` : `Connected ${input.owner}/${input.repo}`);
        return data;
      } catch (err) {
        const locale = getPreferredLocale();
        toast.error(locale === "zh-CN" ? `连接仓库失败: ${(err as Error).message}` : `Failed to connect repository: ${(err as Error).message}`);
        return null;
      }
    },

    patchIntegration: async (id, input) => {
      const token = getToken();
      if (!token) return null;
      try {
        const { data } = await getApi().patch<VCSIntegration>(
          `/api/v1/vcs-integrations/${id}`,
          input,
          { token },
        );
        set((s) => {
          const next: Record<string, VCSIntegration[]> = {
            ...s.integrationsByProject,
          };
          for (const [pid, list] of Object.entries(next)) {
            next[pid] = list.map((it) => (it.id === id ? data : it));
          }
          return { integrationsByProject: next };
        });
        return data;
      } catch (err) {
        const locale = getPreferredLocale();
        toast.error(locale === "zh-CN" ? `更新集成失败: ${(err as Error).message}` : `Failed to update integration: ${(err as Error).message}`);
        return null;
      }
    },

    deleteIntegration: async (projectId, id) => {
      const token = getToken();
      if (!token) return;
      try {
        await getApi().delete(`/api/v1/vcs-integrations/${id}`, { token });
        set((s) => ({
          integrationsByProject: {
            ...s.integrationsByProject,
            [projectId]: (s.integrationsByProject[projectId] ?? []).filter(
              (it) => it.id !== id,
            ),
          },
        }));
        const locale = getPreferredLocale();
        toast.success(locale === "zh-CN" ? "集成已删除" : "Integration deleted");
      } catch (err) {
        const locale = getPreferredLocale();
        toast.error(locale === "zh-CN" ? `删除集成失败: ${(err as Error).message}` : `Failed to delete integration: ${(err as Error).message}`);
      }
    },

    syncIntegration: async (id) => {
      const token = getToken();
      if (!token) return;
      try {
        await getApi().post(`/api/v1/vcs-integrations/${id}/sync`, {}, { token });
        const locale = getPreferredLocale();
        toast.message?.(locale === "zh-CN" ? "已排队后台同步" : "Queued background sync");
      } catch (err) {
        const locale = getPreferredLocale();
        toast.error(locale === "zh-CN" ? `触发同步失败: ${(err as Error).message}` : `Failed to trigger sync: ${(err as Error).message}`);
      }
    },
  }),
);
