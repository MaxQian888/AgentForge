"use client";

import { create } from "zustand";
import { toast } from "sonner";
import { createApiClient } from "@/lib/api-client";
import { useAuthStore } from "./auth-store";
import { getPreferredLocale } from "./locale-store";

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";

export type EmployeeState = "active" | "paused" | "archived";

export interface EmployeeSkill {
  employeeId?: string;
  skillPath: string;
  autoLoad: boolean;
  overrides?: Record<string, unknown>;
  addedAt?: string;
}

export interface Employee {
  id: string;
  projectId: string;
  name: string;
  displayName?: string;
  roleId: string;
  runtimePrefs?: Record<string, unknown>;
  config?: Record<string, unknown>;
  state: EmployeeState;
  createdBy?: string | null;
  createdAt: string;
  updatedAt: string;
  skills?: EmployeeSkill[];
}

export interface CreateEmployeeInput {
  name: string;
  displayName?: string;
  roleId: string;
  runtimePrefs?: Record<string, unknown>;
  config?: Record<string, unknown>;
  skills?: Array<{ skillPath: string; autoLoad: boolean }>;
}

export interface UpdateEmployeeInput {
  displayName?: string;
  roleId?: string;
  runtimePrefs?: Record<string, unknown>;
  config?: Record<string, unknown>;
}

interface EmployeeStoreState {
  employeesByProject: Record<string, Employee[]>;
  loadingByProject: Record<string, boolean>;
  fetchEmployees: (projectId: string, filter?: { state?: EmployeeState }) => Promise<void>;
  getEmployee: (projectId: string, id: string) => Promise<Employee | null>;
  createEmployee: (projectId: string, input: CreateEmployeeInput) => Promise<Employee | null>;
  updateEmployee: (projectId: string, id: string, input: UpdateEmployeeInput) => Promise<Employee | null>;
  setState: (projectId: string, id: string, state: EmployeeState) => Promise<void>;
  deleteEmployee: (projectId: string, id: string) => Promise<void>;
  addSkill: (projectId: string, employeeId: string, skill: { skillPath: string; autoLoad: boolean }) => Promise<void>;
  removeSkill: (projectId: string, employeeId: string, skillPath: string) => Promise<void>;
}

const getApi = () => createApiClient(API_URL);
const getToken = () => {
  const state = useAuthStore.getState() as { accessToken?: string | null; token?: string | null };
  return state.accessToken ?? state.token ?? null;
};

export const useEmployeeStore = create<EmployeeStoreState>()((set, get) => ({
  employeesByProject: {},
  loadingByProject: {},

  fetchEmployees: async (projectId, filter) => {
    const token = getToken();
    if (!token) return;
    set((s) => ({ loadingByProject: { ...s.loadingByProject, [projectId]: true } }));
    try {
      const qs = filter?.state ? `?state=${encodeURIComponent(filter.state)}` : "";
      const { data } = await getApi().get<Employee[]>(
        `/api/v1/projects/${projectId}/employees${qs}`,
        { token }
      );
      set((s) => ({
        employeesByProject: { ...s.employeesByProject, [projectId]: data ?? [] },
      }));
    } catch (err) {
      const locale = getPreferredLocale();
      toast.error(locale === "zh-CN" ? `加载员工失败: ${(err as Error).message}` : `Failed to load employees: ${(err as Error).message}`);
    } finally {
      set((s) => ({ loadingByProject: { ...s.loadingByProject, [projectId]: false } }));
    }
  },

  getEmployee: async (projectId, id) => {
    const token = getToken();
    if (!token) return null;
    try {
      const { data } = await getApi().get<Employee>(
        `/api/v1/projects/${projectId}/employees/${id}`,
        { token }
      );
      return data;
    } catch (err) {
      const locale = getPreferredLocale();
      toast.error(locale === "zh-CN" ? `加载员工详情失败: ${(err as Error).message}` : `Failed to load employee details: ${(err as Error).message}`);
      return null;
    }
  },

  createEmployee: async (projectId, input) => {
    const token = getToken();
    if (!token) return null;
    try {
      const { data } = await getApi().post<Employee>(
        `/api/v1/projects/${projectId}/employees`,
        input,
        { token }
      );
      set((s) => ({
        employeesByProject: {
          ...s.employeesByProject,
          [projectId]: [data, ...(s.employeesByProject[projectId] ?? [])],
        },
      }));
      const locale = getPreferredLocale();
      toast.success(locale === "zh-CN" ? `员工 ${data.name} 已创建` : `Employee ${data.name} created`);
      return data;
    } catch (err) {
      const locale = getPreferredLocale();
      toast.error(locale === "zh-CN" ? `创建员工失败: ${(err as Error).message}` : `Failed to create employee: ${(err as Error).message}`);
      return null;
    }
  },

  updateEmployee: async (projectId, id, input) => {
    const token = getToken();
    if (!token) return null;
    try {
      const { data } = await getApi().patch<Employee>(
        `/api/v1/projects/${projectId}/employees/${id}`,
        input,
        { token }
      );
      set((s) => ({
        employeesByProject: {
          ...s.employeesByProject,
          [projectId]: (s.employeesByProject[projectId] ?? []).map((e) =>
            e.id === id ? data : e
          ),
        },
      }));
      return data;
    } catch (err) {
      const locale = getPreferredLocale();
      toast.error(locale === "zh-CN" ? `更新员工失败: ${(err as Error).message}` : `Failed to update employee: ${(err as Error).message}`);
      return null;
    }
  },

  setState: async (projectId, id, state) => {
    const token = getToken();
    if (!token) return;
    try {
      await getApi().post(`/api/v1/projects/${projectId}/employees/${id}/state`, { state }, { token });
      set((s) => ({
        employeesByProject: {
          ...s.employeesByProject,
          [projectId]: (s.employeesByProject[projectId] ?? []).map((e) =>
            e.id === id ? { ...e, state } : e
          ),
        },
      }));
    } catch (err) {
      const locale = getPreferredLocale();
      toast.error(locale === "zh-CN" ? `状态更新失败: ${(err as Error).message}` : `Failed to update status: ${(err as Error).message}`);
    }
  },

  deleteEmployee: async (projectId, id) => {
    const token = getToken();
    if (!token) return;
    try {
      await getApi().delete(`/api/v1/projects/${projectId}/employees/${id}`, { token });
      set((s) => ({
        employeesByProject: {
          ...s.employeesByProject,
          [projectId]: (s.employeesByProject[projectId] ?? []).filter((e) => e.id !== id),
        },
      }));
      const locale = getPreferredLocale();
      toast.success(locale === "zh-CN" ? "员工已删除" : "Employee deleted");
    } catch (err) {
      const locale = getPreferredLocale();
      toast.error(locale === "zh-CN" ? `删除员工失败: ${(err as Error).message}` : `Failed to delete employee: ${(err as Error).message}`);
    }
  },

  addSkill: async (projectId, employeeId, skill) => {
    const token = getToken();
    if (!token) return;
    try {
      await getApi().post(
        `/api/v1/projects/${projectId}/employees/${employeeId}/skills`,
        skill,
        { token }
      );
      await get().fetchEmployees(projectId);
    } catch (err) {
      const locale = getPreferredLocale();
      toast.error(locale === "zh-CN" ? `添加技能失败: ${(err as Error).message}` : `Failed to add skill: ${(err as Error).message}`);
    }
  },

  removeSkill: async (projectId, employeeId, skillPath) => {
    const token = getToken();
    if (!token) return;
    try {
      await getApi().delete(
        `/api/v1/projects/${projectId}/employees/${employeeId}/skills/${encodeURIComponent(skillPath)}`,
        { token }
      );
      await get().fetchEmployees(projectId);
    } catch (err) {
      const locale = getPreferredLocale();
      toast.error(locale === "zh-CN" ? `移除技能失败: ${(err as Error).message}` : `Failed to remove skill: ${(err as Error).message}`);
    }
  },
}));
