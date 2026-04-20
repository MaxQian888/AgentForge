"use client";

import { create } from "zustand";
import { toast } from "sonner";
import { ApiError, createApiClient } from "@/lib/api-client";
import { useAuthStore } from "./auth-store";

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";

export type StrategyStatus = "draft" | "published" | "archived";

export interface QianchuanStrategy {
  id: string;
  projectId: string | null;
  name: string;
  description: string;
  yamlSource: string;
  parsedSpec: string;
  version: number;
  status: StrategyStatus;
  createdBy: string;
  createdAt: string;
  updatedAt: string;
  isSystem: boolean;
}

export interface StrategyParseError {
  line: number;
  col: number;
  field: string;
  msg: string;
}

export interface TestRunResolvedAction {
  rule: string;
  type: string;
  ad_id?: string;
  params: Record<string, unknown> | null;
}

export interface TestRunResult {
  fired_rules: string[] | null;
  actions: TestRunResolvedAction[] | null;
}

interface QianchuanStrategiesState {
  strategies: QianchuanStrategy[];
  selected: QianchuanStrategy | null;
  loading: boolean;
  lastError: StrategyParseError | string | null;
  lastTestResult: TestRunResult | null;
  fetchList: (projectId: string) => Promise<void>;
  fetchOne: (id: string) => Promise<QianchuanStrategy | null>;
  create: (projectId: string, yamlSource: string) => Promise<QianchuanStrategy | null>;
  update: (id: string, yamlSource: string) => Promise<QianchuanStrategy | null>;
  publish: (id: string) => Promise<QianchuanStrategy | null>;
  archive: (id: string) => Promise<QianchuanStrategy | null>;
  remove: (id: string) => Promise<boolean>;
  testRun: (id: string, snapshot: Record<string, unknown>) => Promise<TestRunResult | null>;
  clearError: () => void;
}

const getApi = () => createApiClient(API_URL);
const getToken = () => {
  const state = useAuthStore.getState() as { accessToken?: string | null; token?: string | null };
  return state.accessToken ?? state.token ?? null;
};

// Promote ApiError 4xx bodies into StrategyParseError when the backend
// returned the structured shape, otherwise fall back to plain message text.
function extractError(err: unknown): StrategyParseError | string {
  if (err instanceof ApiError) {
    const body = err.body as { error?: unknown } | null;
    if (body && typeof body === "object" && "error" in body) {
      const inner = (body as { error: unknown }).error;
      if (
        inner &&
        typeof inner === "object" &&
        "msg" in (inner as Record<string, unknown>)
      ) {
        const cast = inner as Partial<StrategyParseError>;
        return {
          line: typeof cast.line === "number" ? cast.line : 0,
          col: typeof cast.col === "number" ? cast.col : 0,
          field: typeof cast.field === "string" ? cast.field : "",
          msg: typeof cast.msg === "string" ? cast.msg : err.message,
        };
      }
      if (typeof inner === "string") return inner;
    }
    return err.message;
  }
  return (err as Error)?.message ?? "unknown error";
}

export const useQianchuanStrategiesStore = create<QianchuanStrategiesState>()((set) => ({
  strategies: [],
  selected: null,
  loading: false,
  lastError: null,
  lastTestResult: null,

  fetchList: async (projectId) => {
    const token = getToken();
    if (!token) return;
    set({ loading: true, lastError: null });
    try {
      const { data } = await getApi().get<QianchuanStrategy[]>(
        `/api/v1/projects/${projectId}/qianchuan/strategies`,
        { token },
      );
      set({ strategies: data ?? [] });
    } catch (err) {
      const e = extractError(err);
      set({ lastError: e });
      toast.error(typeof e === "string" ? e : e.msg);
    } finally {
      set({ loading: false });
    }
  },

  fetchOne: async (id) => {
    const token = getToken();
    if (!token) return null;
    try {
      const { data } = await getApi().get<QianchuanStrategy>(
        `/api/v1/qianchuan/strategies/${id}`,
        { token },
      );
      set({ selected: data });
      return data;
    } catch (err) {
      const e = extractError(err);
      set({ lastError: e });
      return null;
    }
  },

  create: async (projectId, yamlSource) => {
    const token = getToken();
    if (!token) return null;
    set({ lastError: null });
    try {
      const { data } = await getApi().post<QianchuanStrategy>(
        `/api/v1/projects/${projectId}/qianchuan/strategies`,
        { yamlSource },
        { token },
      );
      set((s) => ({ strategies: [data, ...s.strategies] }));
      toast.success(`策略 ${data.name} 已创建（v${data.version}）`);
      return data;
    } catch (err) {
      const e = extractError(err);
      set({ lastError: e });
      toast.error(typeof e === "string" ? e : `${e.field}: ${e.msg}`);
      return null;
    }
  },

  update: async (id, yamlSource) => {
    const token = getToken();
    if (!token) return null;
    set({ lastError: null });
    try {
      const { data } = await getApi().patch<QianchuanStrategy>(
        `/api/v1/qianchuan/strategies/${id}`,
        { yamlSource },
        { token },
      );
      set((s) => ({
        strategies: s.strategies.map((row) => (row.id === id ? data : row)),
        selected: s.selected?.id === id ? data : s.selected,
      }));
      toast.success("策略已更新");
      return data;
    } catch (err) {
      const e = extractError(err);
      set({ lastError: e });
      toast.error(typeof e === "string" ? e : `${e.field}: ${e.msg}`);
      return null;
    }
  },

  publish: async (id) => {
    const token = getToken();
    if (!token) return null;
    try {
      const { data } = await getApi().post<QianchuanStrategy>(
        `/api/v1/qianchuan/strategies/${id}/publish`,
        {},
        { token },
      );
      set((s) => ({
        strategies: s.strategies.map((row) => (row.id === id ? data : row)),
        selected: s.selected?.id === id ? data : s.selected,
      }));
      toast.success("策略已发布");
      return data;
    } catch (err) {
      const e = extractError(err);
      set({ lastError: e });
      toast.error(typeof e === "string" ? e : e.msg);
      return null;
    }
  },

  archive: async (id) => {
    const token = getToken();
    if (!token) return null;
    try {
      const { data } = await getApi().post<QianchuanStrategy>(
        `/api/v1/qianchuan/strategies/${id}/archive`,
        {},
        { token },
      );
      set((s) => ({
        strategies: s.strategies.map((row) => (row.id === id ? data : row)),
        selected: s.selected?.id === id ? data : s.selected,
      }));
      toast.success("策略已归档");
      return data;
    } catch (err) {
      const e = extractError(err);
      set({ lastError: e });
      toast.error(typeof e === "string" ? e : e.msg);
      return null;
    }
  },

  remove: async (id) => {
    const token = getToken();
    if (!token) return false;
    try {
      await getApi().delete(`/api/v1/qianchuan/strategies/${id}`, { token });
      set((s) => ({ strategies: s.strategies.filter((row) => row.id !== id) }));
      toast.success("策略已删除");
      return true;
    } catch (err) {
      const e = extractError(err);
      set({ lastError: e });
      toast.error(typeof e === "string" ? e : e.msg);
      return false;
    }
  },

  testRun: async (id, snapshot) => {
    const token = getToken();
    if (!token) return null;
    try {
      const { data } = await getApi().post<TestRunResult>(
        `/api/v1/qianchuan/strategies/${id}/test`,
        { snapshot },
        { token },
      );
      set({ lastTestResult: data });
      return data;
    } catch (err) {
      const e = extractError(err);
      set({ lastError: e });
      toast.error(typeof e === "string" ? e : e.msg);
      return null;
    }
  },

  clearError: () => set({ lastError: null }),
}));
