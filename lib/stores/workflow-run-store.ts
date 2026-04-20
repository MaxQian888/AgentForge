"use client";

import { create } from "zustand";
import { createApiClient } from "@/lib/api-client";
import { useAuthStore } from "./auth-store";

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";

export type UnifiedRunEngine = "dag" | "plugin";

export type UnifiedRunStatus =
  | "pending"
  | "running"
  | "paused"
  | "completed"
  | "failed"
  | "cancelled"
  | "unknown";

export type UnifiedRunTriggeredByKind =
  | "trigger"
  | "manual"
  | "sub_workflow"
  | "task";

export interface UnifiedRunWorkflowRef {
  id: string;
  name: string;
}

export interface UnifiedRunTriggeredBy {
  kind: UnifiedRunTriggeredByKind;
  ref?: string;
}

export interface UnifiedRunParentLink {
  parentExecutionId: string;
  parentNodeId: string;
}

export interface UnifiedRunRow {
  engine: UnifiedRunEngine;
  runId: string;
  workflowRef: UnifiedRunWorkflowRef;
  status: UnifiedRunStatus;
  startedAt?: string;
  completedAt?: string;
  actingEmployeeId?: string;
  triggeredBy: UnifiedRunTriggeredBy;
  parentLink?: UnifiedRunParentLink;
}

export interface UnifiedRunSummary {
  running: number;
  paused: number;
  failed: number;
}

export interface UnifiedRunListResult {
  rows: UnifiedRunRow[];
  nextCursor?: string;
  summary: UnifiedRunSummary;
}

export interface UnifiedRunFilter {
  engine?: UnifiedRunEngine | "all";
  status?: UnifiedRunStatus[];
  actingEmployeeId?: string;
  triggeredByKind?: UnifiedRunTriggeredByKind;
  triggerId?: string;
  startedAfter?: string;
  startedBefore?: string;
}

export interface UnifiedRunDetail {
  row: UnifiedRunRow;
  body: unknown;
}

export interface WorkflowRunStore {
  rows: UnifiedRunRow[];
  summary: UnifiedRunSummary;
  nextCursor: string | null;
  filter: UnifiedRunFilter;
  loading: boolean;
  error: string | null;
  selectedDetail: UnifiedRunDetail | null;
  detailLoading: boolean;

  setFilter: (filter: UnifiedRunFilter) => void;
  fetchUnifiedRuns: (
    projectId: string,
    opts?: { append?: boolean; cursor?: string | null }
  ) => Promise<void>;
  fetchRunDetail: (
    projectId: string,
    engine: UnifiedRunEngine,
    runId: string
  ) => Promise<UnifiedRunDetail | null>;
  applyRealtimeRow: (row: UnifiedRunRow, terminal: boolean) => void;
  clearDetail: () => void;
}

const EMPTY_SUMMARY: UnifiedRunSummary = { running: 0, paused: 0, failed: 0 };

function buildQueryString(
  filter: UnifiedRunFilter,
  cursor?: string | null,
  limit = 50
): string {
  const params = new URLSearchParams();
  if (filter.engine && filter.engine !== "all") {
    params.set("engine", filter.engine);
  }
  for (const s of filter.status ?? []) {
    params.append("status", s);
  }
  if (filter.actingEmployeeId) {
    params.set("actingEmployeeId", filter.actingEmployeeId);
  }
  if (filter.triggeredByKind) {
    params.set("triggeredByKind", filter.triggeredByKind);
  }
  if (filter.triggerId) {
    params.set("triggerId", filter.triggerId);
  }
  if (filter.startedAfter) {
    params.set("startedAfter", filter.startedAfter);
  }
  if (filter.startedBefore) {
    params.set("startedBefore", filter.startedBefore);
  }
  if (cursor) {
    params.set("cursor", cursor);
  }
  params.set("limit", String(limit));
  return params.toString();
}

function summarizeRows(rows: UnifiedRunRow[]): UnifiedRunSummary {
  const out: UnifiedRunSummary = { running: 0, paused: 0, failed: 0 };
  for (const r of rows) {
    if (r.status === "running") out.running++;
    else if (r.status === "paused") out.paused++;
    else if (r.status === "failed") out.failed++;
  }
  return out;
}

export const useWorkflowRunStore = create<WorkflowRunStore>()((set, get) => ({
  rows: [],
  summary: EMPTY_SUMMARY,
  nextCursor: null,
  filter: {},
  loading: false,
  error: null,
  selectedDetail: null,
  detailLoading: false,

  setFilter: (filter) => set({ filter }),

  fetchUnifiedRuns: async (projectId, opts) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return;
    const cursor = opts?.cursor ?? null;
    const append = opts?.append === true && cursor !== null;
    set({ loading: true, error: null });
    try {
      const api = createApiClient(API_URL);
      const query = buildQueryString(get().filter, cursor);
      const { data } = await api.get<UnifiedRunListResult>(
        `/api/v1/projects/${projectId}/workflow-runs?${query}`,
        { token }
      );
      const rows = data?.rows ?? [];
      const summary = data?.summary ?? EMPTY_SUMMARY;
      set((state) => ({
        rows: append ? [...state.rows, ...rows] : rows,
        summary,
        nextCursor: data?.nextCursor ?? null,
      }));
    } catch {
      set({ error: "Unable to load workflow runs" });
    } finally {
      set({ loading: false });
    }
  },

  fetchRunDetail: async (projectId, engine, runId) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return null;
    set({ detailLoading: true, error: null });
    try {
      const api = createApiClient(API_URL);
      const { data } = await api.get<UnifiedRunDetail>(
        `/api/v1/projects/${projectId}/workflow-runs/${engine}/${runId}`,
        { token }
      );
      set({ selectedDetail: data });
      return data;
    } catch {
      set({ error: "Unable to load run detail" });
      return null;
    } finally {
      set({ detailLoading: false });
    }
  },

  // applyRealtimeRow is invoked by the WS subscription when a
  // workflow.run.status_changed or workflow.run.terminal event arrives. It
  // upserts the row by (engine, runId) so the list stays consistent with
  // live transitions without a page refresh.
  applyRealtimeRow: (row, terminal) => {
    set((state) => {
      const idx = state.rows.findIndex(
        (r) => r.engine === row.engine && r.runId === row.runId
      );
      let nextRows: UnifiedRunRow[];
      if (idx >= 0) {
        nextRows = [...state.rows];
        nextRows[idx] = { ...nextRows[idx], ...row };
      } else if (!terminal) {
        nextRows = [row, ...state.rows];
      } else {
        nextRows = state.rows;
      }
      return { rows: nextRows, summary: summarizeRows(nextRows) };
    });
  },

  clearDetail: () => set({ selectedDetail: null }),
}));

// Utility helper exported for tests and potential callers that want to
// compute a summary independently (e.g., after deriving a filtered subset
// of rows on the client).
export function computeSummary(rows: UnifiedRunRow[]): UnifiedRunSummary {
  return summarizeRows(rows);
}
