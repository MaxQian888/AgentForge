"use client";

import { create } from "zustand";
import { toast } from "sonner";
import { createApiClient } from "@/lib/api-client";
import { useAuthStore } from "./auth-store";

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";

export interface SecretMetadata {
  name: string;
  description?: string;
  lastUsedAt?: string;
  createdBy: string;
  createdAt: string;
  updatedAt: string;
}

export interface RevealedValue {
  projectId: string;
  name: string;
  value: string;
}

interface SecretsStoreState {
  secretsByProject: Record<string, SecretMetadata[]>;
  loadingByProject: Record<string, boolean>;
  // The last secret value the backend handed back on create or rotate.
  // Held in memory only; the FE clears it once the user dismisses the
  // reveal dialog.
  lastRevealedValue: RevealedValue | null;

  fetchSecrets: (projectId: string) => Promise<void>;
  createSecret: (
    projectId: string,
    name: string,
    value: string,
    description?: string,
  ) => Promise<SecretMetadata | null>;
  rotateSecret: (projectId: string, name: string, value: string) => Promise<void>;
  deleteSecret: (projectId: string, name: string) => Promise<void>;
  consumeRevealedValue: () => void;
}

const getApi = () => createApiClient(API_URL);
const getToken = () => {
  const state = useAuthStore.getState() as {
    accessToken?: string | null;
    token?: string | null;
  };
  return state.accessToken ?? state.token ?? null;
};

export const useSecretsStore = create<SecretsStoreState>()((set, get) => ({
  secretsByProject: {},
  loadingByProject: {},
  lastRevealedValue: null,

  fetchSecrets: async (projectId) => {
    const token = getToken();
    if (!token) return;
    set((s) => ({
      loadingByProject: { ...s.loadingByProject, [projectId]: true },
    }));
    try {
      const { data } = await getApi().get<SecretMetadata[]>(
        `/api/v1/projects/${projectId}/secrets`,
        { token },
      );
      set((s) => ({
        secretsByProject: { ...s.secretsByProject, [projectId]: data ?? [] },
      }));
    } catch (err) {
      toast.error(`加载密钥失败: ${(err as Error).message}`);
    } finally {
      set((s) => ({
        loadingByProject: { ...s.loadingByProject, [projectId]: false },
      }));
    }
  },

  createSecret: async (projectId, name, value, description) => {
    const token = getToken();
    if (!token) return null;
    try {
      const { data } = await getApi().post<SecretMetadata & { value: string }>(
        `/api/v1/projects/${projectId}/secrets`,
        { name, value, description: description ?? "" },
        { token },
      );
      const { value: revealed, ...metadata } = data;
      set((s) => ({
        secretsByProject: {
          ...s.secretsByProject,
          [projectId]: [metadata, ...(s.secretsByProject[projectId] ?? [])],
        },
        lastRevealedValue: { projectId, name, value: revealed },
      }));
      toast.success(`密钥 ${name} 已创建`);
      return metadata;
    } catch (err) {
      toast.error(`创建密钥失败: ${(err as Error).message}`);
      return null;
    }
  },

  rotateSecret: async (projectId, name, value) => {
    const token = getToken();
    if (!token) return;
    try {
      const { data } = await getApi().patch<{ name: string; value: string }>(
        `/api/v1/projects/${projectId}/secrets/${encodeURIComponent(name)}`,
        { value },
        { token },
      );
      set({ lastRevealedValue: { projectId, name, value: data.value } });
      toast.success(`密钥 ${name} 已轮换`);
      // Refresh the metadata list so updatedAt advances.
      await get().fetchSecrets(projectId);
    } catch (err) {
      toast.error(`轮换密钥失败: ${(err as Error).message}`);
    }
  },

  deleteSecret: async (projectId, name) => {
    const token = getToken();
    if (!token) return;
    try {
      await getApi().delete(
        `/api/v1/projects/${projectId}/secrets/${encodeURIComponent(name)}`,
        { token },
      );
      set((s) => ({
        secretsByProject: {
          ...s.secretsByProject,
          [projectId]: (s.secretsByProject[projectId] ?? []).filter(
            (r) => r.name !== name,
          ),
        },
      }));
      toast.success("密钥已删除");
    } catch (err) {
      toast.error(`删除密钥失败: ${(err as Error).message}`);
    }
  },

  consumeRevealedValue: () => set({ lastRevealedValue: null }),
}));
