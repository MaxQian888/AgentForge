"use client";

import { create } from "zustand";
import { toast } from "sonner";
import { createApiClient } from "@/lib/api-client";
import { useAuthStore } from "./auth-store";
import { getPreferredLocale } from "./locale-store";

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";
const DEFAULT_PAGE_SIZE = 20;

export type EmployeeRunKind = "all" | "workflow" | "agent";

export interface EmployeeRunRow {
  kind: "workflow" | "agent";
  id: string;
  name: string;
  status: string;
  startedAt?: string;
  completedAt?: string;
  durationMs?: number;
  refUrl: string;
}

interface RunsResponse {
  items: EmployeeRunRow[];
  page: number;
  size: number;
  kind: EmployeeRunKind;
}

interface EmployeeRunsState {
  runsByEmployee: Record<string, EmployeeRunRow[]>;
  loadingByEmployee: Record<string, boolean>;
  pageByEmployee: Record<string, number>;
  hasMoreByEmployee: Record<string, boolean>;
  kindByEmployee: Record<string, EmployeeRunKind>;

  fetchRuns: (
    employeeId: string,
    page?: number,
    kind?: EmployeeRunKind,
  ) => Promise<void>;
  ingestWorkflowEvent: (employeeId: string, row: EmployeeRunRow) => void;
  reset: (employeeId: string) => void;
}

const getApi = () => createApiClient(API_URL);
const getToken = () => {
  const state = useAuthStore.getState() as {
    accessToken?: string | null;
    token?: string | null;
  };
  return state.accessToken ?? state.token ?? null;
};

export const useEmployeeRunsStore = create<EmployeeRunsState>()((set, get) => ({
  runsByEmployee: {},
  loadingByEmployee: {},
  pageByEmployee: {},
  hasMoreByEmployee: {},
  kindByEmployee: {},

  fetchRuns: async (employeeId, page = 1, kind) => {
    const token = getToken();
    if (!token) return;
    const effectiveKind = kind ?? get().kindByEmployee[employeeId] ?? "all";
    set((s) => ({
      loadingByEmployee: { ...s.loadingByEmployee, [employeeId]: true },
      kindByEmployee: { ...s.kindByEmployee, [employeeId]: effectiveKind },
    }));
    try {
      const qs = `?type=${encodeURIComponent(effectiveKind)}&page=${page}&size=${DEFAULT_PAGE_SIZE}`;
      const { data } = await getApi().get<RunsResponse>(
        `/api/v1/employees/${employeeId}/runs${qs}`,
        { token },
      );
      const items = data?.items ?? [];
      const merged =
        page <= 1
          ? items
          : [...(get().runsByEmployee[employeeId] ?? []), ...items];
      set((s) => ({
        runsByEmployee: { ...s.runsByEmployee, [employeeId]: merged },
        pageByEmployee: { ...s.pageByEmployee, [employeeId]: page },
        hasMoreByEmployee: {
          ...s.hasMoreByEmployee,
          [employeeId]: items.length >= DEFAULT_PAGE_SIZE,
        },
      }));
    } catch (err) {
      const locale = getPreferredLocale();
      toast.error(locale === "zh-CN" ? `加载员工执行历史失败: ${(err as Error).message}` : `Failed to load employee run history: ${(err as Error).message}`);
    } finally {
      set((s) => ({
        loadingByEmployee: { ...s.loadingByEmployee, [employeeId]: false },
      }));
    }
  },

  ingestWorkflowEvent: (employeeId, row) => {
    set((s) => {
      const existing = s.runsByEmployee[employeeId] ?? [];
      const idx = existing.findIndex(
        (r) => r.kind === row.kind && r.id === row.id,
      );
      let next: EmployeeRunRow[];
      if (idx >= 0) {
        next = existing.slice();
        next[idx] = { ...existing[idx], ...row };
      } else {
        next = [row, ...existing];
      }
      return {
        runsByEmployee: { ...s.runsByEmployee, [employeeId]: next },
      };
    });
  },

  reset: (employeeId) => {
    set((s) => {
      const {
        [employeeId]: _runs,
        ...restRuns
      } = s.runsByEmployee;
      const {
        [employeeId]: _loading,
        ...restLoading
      } = s.loadingByEmployee;
      const {
        [employeeId]: _page,
        ...restPage
      } = s.pageByEmployee;
      const {
        [employeeId]: _has,
        ...restHas
      } = s.hasMoreByEmployee;
      const {
        [employeeId]: _kind,
        ...restKind
      } = s.kindByEmployee;
      void _runs;
      void _loading;
      void _page;
      void _has;
      void _kind;
      return {
        runsByEmployee: restRuns,
        loadingByEmployee: restLoading,
        pageByEmployee: restPage,
        hasMoreByEmployee: restHas,
        kindByEmployee: restKind,
      };
    });
  },
}));
