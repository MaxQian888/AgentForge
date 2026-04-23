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
import { getPreferredLocale } from "./locale-store";
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
  const locale = getPreferredLocale();
  const e = err as { code?: string; message?: string };
  const code = e?.code;
  switch (code) {
    case "trigger:workflow_not_found":
      toast.error(locale === "zh-CN" ? "目标工作流不存在" : "Target workflow does not exist");
      return;
    case "trigger:acting_employee_archived":
      toast.error(locale === "zh-CN" ? "员工已归档或不属于该项目" : "Employee is archived or does not belong to this project");
      return;
    case "trigger:cannot_delete_dag_managed":
      toast.error(locale === "zh-CN" ? "此触发器由 DAG 节点维护，请到工作流编辑器中删除" : "This trigger is maintained by a DAG node. Please delete it in the workflow editor.");
      return;
    case "trigger:immutable_field":
      toast.error(locale === "zh-CN" ? "workflowId / source / createdVia 不可修改" : "workflowId / source / createdVia cannot be modified");
      return;
  }
  toast.error(`${fallback}: ${e?.message ?? (locale === "zh-CN" ? "未知错误" : "unknown error")}`);
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
      const locale = getPreferredLocale();
      toastTriggerError(err, locale === "zh-CN" ? "加载触发器失败" : "Failed to load triggers");
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
      const locale = getPreferredLocale();
      toast.success(locale === "zh-CN" ? "触发器已创建" : "Trigger created");
      return data;
    } catch (err) {
      const locale = getPreferredLocale();
      toastTriggerError(err, locale === "zh-CN" ? "创建触发器失败" : "Failed to create trigger");
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
      const locale = getPreferredLocale();
      toastTriggerError(err, locale === "zh-CN" ? "更新触发器失败" : "Failed to update trigger");
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
      const locale = getPreferredLocale();
      toast.success(locale === "zh-CN" ? "触发器已删除" : "Trigger deleted");
    } catch (err) {
      const locale = getPreferredLocale();
      toastTriggerError(err, locale === "zh-CN" ? "删除触发器失败" : "Failed to delete trigger");
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
      const locale = getPreferredLocale();
      toastTriggerError(err, locale === "zh-CN" ? "试运行失败" : "Dry run failed");
      return null;
    }
  },
}));

// Export getter so non-react callers can pull the latest state without
// subscribing.
export function getEmployeeTriggers(employeeId: string): WorkflowTrigger[] {
  return useEmployeeTriggerStore.getState().triggersByEmployee[employeeId] ?? [];
}
