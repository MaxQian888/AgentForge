"use client";

/**
 * employee-trigger-store — Spec 1C frontend CRUD for workflow_triggers,
 * scoped to a single Digital Employee. Backs the
 * /employees/[id]/triggers page (TriggerListTable + TriggerEditDrawer +
 * TriggerTestModal). Mirrors employee-store.ts conventions: keyed by
 * employeeId, optimistic local-state updates after each mutation, toast
 * errors mapped from the backend's spec1 §10 error codes.
 */

import { create } from "zustand";
import { toast } from "sonner";
import { createApiClient } from "@/lib/api-client";
import { useAuthStore } from "./auth-store";
import type { WorkflowTrigger, TriggerSource } from "./workflow-trigger-store";

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";

export interface CreateTriggerInput {
  workflowId: string;
  source: TriggerSource;
  config: Record<string, unknown>;
  inputMapping?: Record<string, unknown>;
  actingEmployeeId?: string;
  displayName?: string;
  description?: string;
}

export interface PatchTriggerInput {
  config?: Record<string, unknown>;
  inputMapping?: Record<string, unknown>;
  // null clears the binding; undefined means "do not touch".
  actingEmployeeId?: string | null;
  displayName?: string;
  description?: string;
  enabled?: boolean;
}

export interface DryRunResult {
  matched: boolean;
  would_dispatch: boolean;
  rendered_input?: Record<string, unknown>;
  skip_reason?: string;
}

interface State {
  triggersByEmployee: Record<string, WorkflowTrigger[]>;
  loading: Record<string, boolean>;
  fetchByEmployee: (employeeId: string) => Promise<void>;
  createTrigger: (input: CreateTriggerInput) => Promise<WorkflowTrigger | null>;
  patchTrigger: (
    triggerId: string,
    input: PatchTriggerInput,
  ) => Promise<WorkflowTrigger | null>;
  deleteTrigger: (triggerId: string, employeeId: string) => Promise<void>;
  testTrigger: (
    triggerId: string,
    event: Record<string, unknown>,
  ) => Promise<DryRunResult | null>;
}

const getApi = () => createApiClient(API_URL);
const getToken = () => {
  const state = useAuthStore.getState() as {
    accessToken?: string | null;
    token?: string | null;
  };
  return state.accessToken ?? state.token ?? null;
};

// Map spec1 §10 backend error codes to user-facing toast strings. The
// backend includes `code` on the JSON error response; the api-client
// surfaces the raw error.message which already embeds the localized
// string from the server, so we look at the underlying response payload
// when available.
function toastTriggerError(err: unknown, fallback: string) {
  const e = err as { code?: string; message?: string };
  const code = e?.code;
  switch (code) {
    case "trigger:workflow_not_found":
      toast.error("目标工作流不存在");
      return;
    case "trigger:acting_employee_archived":
      toast.error("员工已归档或不属于该项目");
      return;
    case "trigger:cannot_delete_dag_managed":
      toast.error("此触发器由 DAG 节点维护，请到工作流编辑器中删除");
      return;
    case "trigger:immutable_field":
      toast.error("workflowId / source / createdVia 不可修改");
      return;
  }
  toast.error(`${fallback}: ${e?.message ?? "unknown error"}`);
}

export const useEmployeeTriggerStore = create<State>()((set) => ({
  triggersByEmployee: {},
  loading: {},

  fetchByEmployee: async (employeeId) => {
    const token = getToken();
    if (!token) return;
    set((s) => ({ loading: { ...s.loading, [employeeId]: true } }));
    try {
      const { data } = await getApi().get<WorkflowTrigger[]>(
        `/api/v1/employees/${employeeId}/triggers`,
        { token },
      );
      set((s) => ({
        triggersByEmployee: {
          ...s.triggersByEmployee,
          [employeeId]: data ?? [],
        },
      }));
    } catch (err) {
      toastTriggerError(err, "加载触发器失败");
    } finally {
      set((s) => ({ loading: { ...s.loading, [employeeId]: false } }));
    }
  },

  createTrigger: async (input) => {
    const token = getToken();
    if (!token) return null;
    try {
      const { data } = await getApi().post<WorkflowTrigger>(
        `/api/v1/triggers`,
        input,
        { token },
      );
      const empID = input.actingEmployeeId;
      if (empID) {
        set((s) => ({
          triggersByEmployee: {
            ...s.triggersByEmployee,
            [empID]: [data, ...(s.triggersByEmployee[empID] ?? [])],
          },
        }));
      }
      toast.success("触发器已创建");
      return data;
    } catch (err) {
      toastTriggerError(err, "创建触发器失败");
      return null;
    }
  },

  patchTrigger: async (triggerId, input) => {
    const token = getToken();
    if (!token) return null;
    try {
      const { data } = await getApi().patch<WorkflowTrigger>(
        `/api/v1/triggers/${triggerId}`,
        input,
        { token },
      );
      // In-place merge: scan all employee buckets so we update the row
      // wherever it lives (re-binding the actingEmployeeId would move
      // the row, but the store does not track those moves — callers
      // should refetchByEmployee() after a re-bind).
      set((s) => {
        const next: Record<string, WorkflowTrigger[]> = {};
        for (const [empID, rows] of Object.entries(s.triggersByEmployee)) {
          next[empID] = rows.map((r) => (r.id === triggerId ? data : r));
        }
        return { triggersByEmployee: next };
      });
      return data;
    } catch (err) {
      toastTriggerError(err, "更新触发器失败");
      return null;
    }
  },

  deleteTrigger: async (triggerId, employeeId) => {
    const token = getToken();
    if (!token) return;
    try {
      await getApi().delete(`/api/v1/triggers/${triggerId}`, { token });
      set((s) => ({
        triggersByEmployee: {
          ...s.triggersByEmployee,
          [employeeId]: (s.triggersByEmployee[employeeId] ?? []).filter(
            (r) => r.id !== triggerId,
          ),
        },
      }));
      toast.success("触发器已删除");
    } catch (err) {
      toastTriggerError(err, "删除触发器失败");
    }
  },

  testTrigger: async (triggerId, event) => {
    const token = getToken();
    if (!token) return null;
    try {
      const { data } = await getApi().post<DryRunResult>(
        `/api/v1/triggers/${triggerId}/test`,
        { event },
        { token },
      );
      return data;
    } catch (err) {
      toastTriggerError(err, "试运行失败");
      return null;
    }
  },
}));

// Export getter so non-react callers can pull the latest state without
// subscribing.
export function getEmployeeTriggers(employeeId: string): WorkflowTrigger[] {
  return useEmployeeTriggerStore.getState().triggersByEmployee[employeeId] ?? [];
}
