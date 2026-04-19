"use client";

import { create } from "zustand";
import { toast } from "sonner";
import { createApiClient } from "@/lib/api-client";
import { useAuthStore } from "./auth-store";

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";

export type TriggerSource = "im" | "schedule";

export interface WorkflowTrigger {
  id: string;
  workflowId: string;
  projectId: string;
  source: TriggerSource;
  config: Record<string, unknown>;
  inputMapping?: Record<string, unknown>;
  idempotencyKeyTemplate?: string;
  dedupeWindowSeconds: number;
  enabled: boolean;
  createdBy?: string | null;
  createdAt: string;
  updatedAt: string;
}

interface WorkflowTriggerStoreState {
  triggersByWorkflow: Record<string, WorkflowTrigger[]>;
  loading: Record<string, boolean>;
  fetchTriggers: (workflowId: string) => Promise<void>;
  setEnabled: (workflowId: string, triggerId: string, enabled: boolean) => Promise<void>;
}

const getApi = () => createApiClient(API_URL);
const getToken = () => {
  const state = useAuthStore.getState() as { accessToken?: string | null; token?: string | null };
  return state.accessToken ?? state.token ?? null;
};

export const useWorkflowTriggerStore = create<WorkflowTriggerStoreState>()((set) => ({
  triggersByWorkflow: {},
  loading: {},

  fetchTriggers: async (workflowId) => {
    const token = getToken();
    if (!token) return;
    set((s) => ({ loading: { ...s.loading, [workflowId]: true } }));
    try {
      const { data } = await getApi().get<WorkflowTrigger[]>(
        `/api/v1/workflows/${workflowId}/triggers`,
        { token }
      );
      set((s) => ({
        triggersByWorkflow: { ...s.triggersByWorkflow, [workflowId]: data ?? [] },
      }));
    } catch (err) {
      toast.error(`加载触发器失败: ${(err as Error).message}`);
    } finally {
      set((s) => ({ loading: { ...s.loading, [workflowId]: false } }));
    }
  },

  setEnabled: async (workflowId, triggerId, enabled) => {
    const token = getToken();
    if (!token) return;
    try {
      await getApi().post(
        `/api/v1/triggers/${triggerId}/enabled`,
        { enabled },
        { token }
      );
      set((s) => ({
        triggersByWorkflow: {
          ...s.triggersByWorkflow,
          [workflowId]: (s.triggersByWorkflow[workflowId] ?? []).map((t) =>
            t.id === triggerId ? { ...t, enabled } : t
          ),
        },
      }));
      toast.success(enabled ? "触发器已启用" : "触发器已停用");
    } catch (err) {
      toast.error(`切换失败: ${(err as Error).message}`);
    }
  },
}));
