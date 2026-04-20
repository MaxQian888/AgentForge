"use client";

import { create } from "zustand";
import { toast } from "sonner";
import { createApiClient } from "@/lib/api-client";
import { useAuthStore } from "./auth-store";

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";

export type QianchuanBindingStatus = "active" | "auth_expired" | "paused";

export interface QianchuanBinding {
  id: string;
  projectId: string;
  advertiserId: string;
  awemeId?: string;
  displayName?: string;
  status: QianchuanBindingStatus;
  actingEmployeeId?: string;
  accessTokenSecretRef: string;
  refreshTokenSecretRef: string;
  tokenExpiresAt?: string;
  lastSyncedAt?: string;
}

interface CreateInput {
  advertiserId: string;
  awemeId?: string;
  displayName?: string;
  actingEmployeeId?: string;
  accessTokenSecretRef: string;
  refreshTokenSecretRef: string;
}

interface State {
  byProject: Record<string, QianchuanBinding[]>;
  loading: Record<string, boolean>;
  fetchBindings: (projectId: string) => Promise<void>;
  createBinding: (
    projectId: string,
    input: CreateInput,
  ) => Promise<QianchuanBinding | null>;
  updateBinding: (
    id: string,
    patch: Partial<
      Pick<QianchuanBinding, "displayName" | "status" | "actingEmployeeId">
    >,
  ) => Promise<void>;
  deleteBinding: (projectId: string, id: string) => Promise<void>;
  syncBinding: (id: string) => Promise<void>;
  testBinding: (id: string) => Promise<{ ok: boolean; detail?: string }>;
}

const getApi = () => createApiClient(API_URL);
const getToken = () => {
  const state = useAuthStore.getState() as {
    accessToken?: string | null;
    token?: string | null;
  };
  return state.accessToken ?? state.token ?? null;
};

type Wire = {
  id: string;
  project_id: string;
  advertiser_id: string;
  aweme_id?: string;
  display_name?: string;
  status: QianchuanBindingStatus;
  acting_employee_id?: string;
  access_token_secret_ref: string;
  refresh_token_secret_ref: string;
  token_expires_at?: string;
  last_synced_at?: string;
};

const fromWire = (w: Wire): QianchuanBinding => ({
  id: w.id,
  projectId: w.project_id,
  advertiserId: w.advertiser_id,
  awemeId: w.aweme_id,
  displayName: w.display_name,
  status: w.status,
  actingEmployeeId: w.acting_employee_id,
  accessTokenSecretRef: w.access_token_secret_ref,
  refreshTokenSecretRef: w.refresh_token_secret_ref,
  tokenExpiresAt: w.token_expires_at,
  lastSyncedAt: w.last_synced_at,
});

export const useQianchuanBindingsStore = create<State>()((set, get) => ({
  byProject: {},
  loading: {},

  fetchBindings: async (projectId) => {
    const token = getToken();
    if (!token) return;
    set((s) => ({ loading: { ...s.loading, [projectId]: true } }));
    try {
      const { data } = await getApi().get<Wire[]>(
        `/api/v1/projects/${projectId}/qianchuan/bindings`,
        { token },
      );
      set((s) => ({
        byProject: {
          ...s.byProject,
          [projectId]: (data ?? []).map(fromWire),
        },
      }));
    } catch (e) {
      toast.error(`加载千川绑定失败：${(e as Error).message}`);
    } finally {
      set((s) => ({ loading: { ...s.loading, [projectId]: false } }));
    }
  },

  createBinding: async (projectId, input) => {
    const token = getToken();
    if (!token) return null;
    try {
      const { data } = await getApi().post<Wire>(
        `/api/v1/projects/${projectId}/qianchuan/bindings`,
        {
          advertiser_id: input.advertiserId,
          aweme_id: input.awemeId,
          display_name: input.displayName,
          acting_employee_id: input.actingEmployeeId,
          access_token_secret_ref: input.accessTokenSecretRef,
          refresh_token_secret_ref: input.refreshTokenSecretRef,
        },
        { token },
      );
      await get().fetchBindings(projectId);
      toast.success("绑定已创建");
      return data ? fromWire(data) : null;
    } catch (e) {
      toast.error(`创建失败：${(e as Error).message}`);
      return null;
    }
  },

  updateBinding: async (id, patch) => {
    const token = getToken();
    if (!token) return;
    try {
      await getApi().patch(
        `/api/v1/qianchuan/bindings/${id}`,
        {
          display_name: patch.displayName,
          status: patch.status,
          acting_employee_id: patch.actingEmployeeId,
        },
        { token },
      );
      // refetch each project that contains this binding
      for (const [pid, rows] of Object.entries(get().byProject)) {
        if (rows.some((b) => b.id === id)) {
          await get().fetchBindings(pid);
        }
      }
    } catch (e) {
      toast.error(`更新失败：${(e as Error).message}`);
    }
  },

  deleteBinding: async (projectId, id) => {
    const token = getToken();
    if (!token) return;
    try {
      await getApi().delete(`/api/v1/qianchuan/bindings/${id}`, { token });
      await get().fetchBindings(projectId);
      toast.success("绑定已删除");
    } catch (e) {
      toast.error(`删除失败：${(e as Error).message}`);
    }
  },

  syncBinding: async (id) => {
    const token = getToken();
    if (!token) return;
    try {
      await getApi().post(`/api/v1/qianchuan/bindings/${id}/sync`, {}, { token });
      toast.success("已触发同步");
    } catch (e) {
      toast.error(`同步失败：${(e as Error).message}`);
    }
  },

  testBinding: async (id) => {
    const token = getToken();
    if (!token) return { ok: false, detail: "未登录" };
    try {
      await getApi().post(`/api/v1/qianchuan/bindings/${id}/test`, {}, { token });
      return { ok: true };
    } catch (e) {
      return { ok: false, detail: (e as Error).message };
    }
  },
}));
