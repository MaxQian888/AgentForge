"use client";

import { create } from "zustand";
import { createApiClient } from "@/lib/api-client";
import { useAuthStore } from "./auth-store";

export type SchedulerJobRunStatus =
  | "pending"
  | "running"
  | "cancel_requested"
  | "cancelled"
  | "succeeded"
  | "failed"
  | "skipped";

export type SchedulerJobControlState = "active" | "paused";
export type SchedulerJobAction =
  | "pause"
  | "resume"
  | "trigger"
  | "cancel"
  | "cleanup"
  | "update";

export interface SchedulerJobRunSummary {
  runId: string;
  triggerSource: string;
  status: SchedulerJobRunStatus;
  startedAt: string;
  finishedAt?: string;
  durationMs?: number | null;
  summary: string;
  errorMessage: string;
}

export interface SchedulerJobActionSupport {
  action: SchedulerJobAction;
  enabled: boolean;
  reason?: string;
}

export interface SchedulerJobOccurrence {
  runAt: string;
}

export interface SchedulerJobConfigFieldOption {
  label: string;
  value: string;
}

export interface SchedulerJobConfigFieldDescriptor {
  key: string;
  label: string;
  type: string;
  required?: boolean;
  defaultValue?: unknown;
  helpText?: string;
  placeholder?: string;
  options?: SchedulerJobConfigFieldOption[];
}

export interface SchedulerJobConfigMetadata {
  schemaVersion?: string;
  editable: boolean;
  reason?: string;
  fields?: SchedulerJobConfigFieldDescriptor[];
}

export interface SchedulerJob {
  jobKey: string;
  name: string;
  scope: string;
  schedule: string;
  enabled: boolean;
  executionMode: string;
  overlapPolicy: string;
  lastRunStatus?: SchedulerJobRunStatus;
  lastRunAt?: string;
  nextRunAt?: string;
  lastRunSummary: string;
  lastError: string;
  config: string;
  controlState?: SchedulerJobControlState;
  activeRun?: SchedulerJobRunSummary | null;
  supportedActions?: SchedulerJobActionSupport[];
  configMetadata?: SchedulerJobConfigMetadata | null;
  upcomingRuns?: SchedulerJobOccurrence[];
  createdAt: string;
  updatedAt: string;
}

export interface SchedulerJobRun {
  runId: string;
  jobKey: string;
  triggerSource: string;
  status: SchedulerJobRunStatus;
  startedAt: string;
  finishedAt?: string;
  durationMs?: number | null;
  summary: string;
  errorMessage: string;
  metrics: string;
  createdAt: string;
  updatedAt: string;
}

export interface SchedulerStats {
  totalJobs: number;
  enabledJobs: number;
  disabledJobs: number;
  pausedJobs: number;
  failedJobs: number;
  activeRuns: number;
  queueDepth: number;
  totalRuns24h: number;
  successfulRuns24h: number;
  failedRuns24h: number;
  averageDurationMs: number;
  successRate24h: number;
}

export interface UpdateSchedulerJobInput {
  enabled?: boolean;
  schedule?: string;
}

export interface SchedulerRunHistoryFilters {
  status?: SchedulerJobRunStatus;
  triggerSource?: string;
  since?: string;
  before?: string;
  limit?: number;
}

export interface SchedulerRunCleanupPolicy {
  retainRecent?: number;
  startedBefore?: string;
}

interface SchedulerState {
  jobs: SchedulerJob[];
  runsByJobKey: Record<string, SchedulerJobRun[]>;
  draftSchedules: Record<string, string>;
  selectedJobKey: string | null;
  stats: SchedulerStats | null;
  loading: boolean;
  actionJobKey: string | null;
  error: string | null;
  fetchJobs: () => Promise<void>;
  fetchRuns: (jobKey: string, filters?: SchedulerRunHistoryFilters) => Promise<void>;
  fetchStats: () => Promise<void>;
  updateJob: (jobKey: string, input: UpdateSchedulerJobInput) => Promise<void>;
  triggerJob: (jobKey: string) => Promise<void>;
  pauseJob: (jobKey: string) => Promise<void>;
  resumeJob: (jobKey: string) => Promise<void>;
  cancelJob: (jobKey: string) => Promise<void>;
  cleanupRuns: (jobKey: string, policy: SchedulerRunCleanupPolicy) => Promise<void>;
  selectJob: (jobKey: string) => void;
  setDraftSchedule: (jobKey: string, schedule: string) => void;
  upsertJob: (job: SchedulerJob) => void;
  recordRun: (run: SchedulerJobRun) => void;
}

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";

function sortJobs(jobs: SchedulerJob[]): SchedulerJob[] {
  return [...jobs].sort((a, b) => a.jobKey.localeCompare(b.jobKey));
}

function deriveStats(jobs: SchedulerJob[], prev: SchedulerStats | null): SchedulerStats {
  const base: SchedulerStats = prev ?? {
    totalJobs: 0,
    enabledJobs: 0,
    disabledJobs: 0,
    pausedJobs: 0,
    failedJobs: 0,
    activeRuns: 0,
    queueDepth: 0,
    totalRuns24h: 0,
    successfulRuns24h: 0,
    failedRuns24h: 0,
    averageDurationMs: 0,
    successRate24h: 0,
  };
  return {
    ...base,
    totalJobs: jobs.length,
    enabledJobs: jobs.filter((j) => j.enabled).length,
    disabledJobs: jobs.filter((j) => !j.enabled).length,
    pausedJobs: jobs.filter((j) => j.controlState === "paused" || !j.enabled).length,
    failedJobs: jobs.filter((j) => j.lastRunStatus === "failed").length,
    activeRuns: jobs.filter((j) => Boolean(j.activeRun)).length,
    queueDepth: jobs.filter((j) => Boolean(j.activeRun)).length,
  };
}

function actionFor(
  job: SchedulerJob,
  action: SchedulerJobAction,
): SchedulerJobActionSupport | undefined {
  return job.supportedActions?.find((item) => item.action === action);
}

function buildRunsPath(jobKey: string, filters?: SchedulerRunHistoryFilters): string {
  const params = new URLSearchParams();
  if (filters?.status) {
    params.set("status", filters.status);
  }
  if (filters?.triggerSource) {
    params.set("triggerSource", filters.triggerSource);
  }
  if (filters?.since) {
    params.set("since", filters.since);
  }
  if (filters?.before) {
    params.set("before", filters.before);
  }
  if (filters?.limit) {
    params.set("limit", String(filters.limit));
  }
  const query = params.toString();
  return query
    ? `/api/v1/scheduler/jobs/${jobKey}/runs?${query}`
    : `/api/v1/scheduler/jobs/${jobKey}/runs`;
}

export const useSchedulerStore = create<SchedulerState>()((set, get) => ({
  jobs: [],
  runsByJobKey: {},
  draftSchedules: {},
  selectedJobKey: null,
  stats: null,
  loading: false,
  actionJobKey: null,
  error: null,

  fetchJobs: async () => {
    const token = useAuthStore.getState().accessToken;
    if (!token) {
      return;
    }

    set({ loading: true, error: null });
    try {
      const api = createApiClient(API_URL);
      const { data } = await api.get<SchedulerJob[]>("/api/v1/scheduler/jobs", {
        token,
      });
      const jobs = sortJobs(data ?? []);
      set((state) => {
        const nextDrafts = { ...state.draftSchedules };
        for (const job of jobs) {
          nextDrafts[job.jobKey] = nextDrafts[job.jobKey] ?? job.schedule;
        }
        return {
          jobs,
          draftSchedules: nextDrafts,
          selectedJobKey: state.selectedJobKey ?? jobs[0]?.jobKey ?? null,
          stats: deriveStats(jobs, state.stats),
          error: null,
        };
      });
    } catch {
      set({ error: "Unable to load scheduler jobs" });
    } finally {
      set({ loading: false });
    }
  },

  fetchRuns: async (jobKey, filters) => {
    const token = useAuthStore.getState().accessToken;
    if (!token || !jobKey) {
      return;
    }

    try {
      const api = createApiClient(API_URL);
      const { data } = await api.get<SchedulerJobRun[]>(
        buildRunsPath(jobKey, filters),
        { token }
      );
      set((state) => ({
        runsByJobKey: {
          ...state.runsByJobKey,
          [jobKey]: data ?? [],
        },
      }));
    } catch {
      set({ error: "Unable to load scheduler run history" });
    }
  },

  fetchStats: async () => {
    const token = useAuthStore.getState().accessToken;
    if (!token) {
      return;
    }

    try {
      const api = createApiClient(API_URL);
      const { data } = await api.get<SchedulerStats>(
        "/api/v1/scheduler/stats",
        { token }
      );
      set({ stats: data });
    } catch {
      // Stats are non-critical; silently ignore
    }
  },

  updateJob: async (jobKey, input) => {
    const token = useAuthStore.getState().accessToken;
    if (!token || !jobKey) {
      return;
    }

    set({ actionJobKey: jobKey, error: null });
    try {
      const api = createApiClient(API_URL);
      const { data } = await api.put<SchedulerJob>(
        `/api/v1/scheduler/jobs/${jobKey}`,
        input,
        { token }
      );
      get().upsertJob(data);
    } catch {
      set({ error: "Unable to update scheduler job" });
    } finally {
      set({ actionJobKey: null });
    }
  },

  triggerJob: async (jobKey) => {
    const token = useAuthStore.getState().accessToken;
    if (!token || !jobKey) {
      return;
    }

    set({ actionJobKey: jobKey, error: null });
    try {
      const api = createApiClient(API_URL);
      const { data } = await api.post<SchedulerJobRun>(
        `/api/v1/scheduler/jobs/${jobKey}/trigger`,
        {},
        { token }
      );
      get().recordRun(data);
      await get().fetchJobs();
    } catch {
      set({ error: "Unable to trigger scheduler job" });
    } finally {
      set({ actionJobKey: null });
    }
  },

  pauseJob: async (jobKey) => {
    const token = useAuthStore.getState().accessToken;
    if (!token || !jobKey) {
      return;
    }

    set({ actionJobKey: jobKey, error: null });
    try {
      const currentJob = get().jobs.find((job) => job.jobKey === jobKey);
      const support = currentJob ? actionFor(currentJob, "pause") : undefined;
      if (support && !support.enabled) {
        set({ error: support.reason ?? "Unable to pause scheduler job" });
        return;
      }
      const api = createApiClient(API_URL);
      const { data } = await api.post<SchedulerJob>(
        `/api/v1/scheduler/jobs/${jobKey}/pause`,
        {},
        { token }
      );
      get().upsertJob(data);
    } catch {
      set({ error: "Unable to pause scheduler job" });
    } finally {
      set({ actionJobKey: null });
    }
  },

  resumeJob: async (jobKey) => {
    const token = useAuthStore.getState().accessToken;
    if (!token || !jobKey) {
      return;
    }

    set({ actionJobKey: jobKey, error: null });
    try {
      const currentJob = get().jobs.find((job) => job.jobKey === jobKey);
      const support = currentJob ? actionFor(currentJob, "resume") : undefined;
      if (support && !support.enabled) {
        set({ error: support.reason ?? "Unable to resume scheduler job" });
        return;
      }
      const api = createApiClient(API_URL);
      const { data } = await api.post<SchedulerJob>(
        `/api/v1/scheduler/jobs/${jobKey}/resume`,
        {},
        { token }
      );
      get().upsertJob(data);
    } catch {
      set({ error: "Unable to resume scheduler job" });
    } finally {
      set({ actionJobKey: null });
    }
  },

  cancelJob: async (jobKey) => {
    const token = useAuthStore.getState().accessToken;
    if (!token || !jobKey) {
      return;
    }

    set({ actionJobKey: jobKey, error: null });
    try {
      const currentJob = get().jobs.find((job) => job.jobKey === jobKey);
      const support = currentJob ? actionFor(currentJob, "cancel") : undefined;
      if (support && !support.enabled) {
        set({ error: support.reason ?? "Unable to cancel scheduler job run" });
        return;
      }
      const api = createApiClient(API_URL);
      const { data } = await api.post<SchedulerJobRun>(
        `/api/v1/scheduler/jobs/${jobKey}/cancel`,
        {},
        { token }
      );
      get().recordRun(data);
      await get().fetchJobs();
    } catch {
      set({ error: "Unable to cancel scheduler job run" });
    } finally {
      set({ actionJobKey: null });
    }
  },

  cleanupRuns: async (jobKey, policy) => {
    const token = useAuthStore.getState().accessToken;
    if (!token || !jobKey) {
      return;
    }

    set({ actionJobKey: jobKey, error: null });
    try {
      const currentJob = get().jobs.find((job) => job.jobKey === jobKey);
      const support = currentJob ? actionFor(currentJob, "cleanup") : undefined;
      if (support && !support.enabled) {
        set({ error: support.reason ?? "Unable to cleanup scheduler history" });
        return;
      }
      const api = createApiClient(API_URL);
      await api.post(
        `/api/v1/scheduler/jobs/${jobKey}/runs/cleanup`,
        policy,
        { token }
      );
      await get().fetchRuns(jobKey);
    } catch {
      set({ error: "Unable to cleanup scheduler history" });
    } finally {
      set({ actionJobKey: null });
    }
  },

  selectJob: (jobKey) => set({ selectedJobKey: jobKey }),

  setDraftSchedule: (jobKey, schedule) =>
    set((state) => ({
      draftSchedules: {
        ...state.draftSchedules,
        [jobKey]: schedule,
      },
    })),

  upsertJob: (job) =>
    set((state) => {
      const jobs = sortJobs(
        state.jobs.some((item) => item.jobKey === job.jobKey)
          ? state.jobs.map((item) => (item.jobKey === job.jobKey ? job : item))
          : [...state.jobs, job]
      );
      // Sync draft schedule if the job's schedule changed externally
      const existingDraft = state.draftSchedules[job.jobKey];
      const existingJob = state.jobs.find((item) => item.jobKey === job.jobKey);
      const draftMatchedOldSchedule = existingDraft === existingJob?.schedule;
      return {
        jobs,
        stats: deriveStats(jobs, state.stats),
        draftSchedules: {
          ...state.draftSchedules,
          [job.jobKey]: draftMatchedOldSchedule ? job.schedule : (existingDraft ?? job.schedule),
        },
      };
    }),

  recordRun: (run) =>
    set((state) => {
      const existing = state.runsByJobKey[run.jobKey] ?? [];
      // Update in place if run already exists (status transition), otherwise prepend
      const existingIndex = existing.findIndex((r) => r.runId === run.runId);
      const updated =
        existingIndex >= 0
          ? existing.map((r, i) => (i === existingIndex ? run : r))
          : [run, ...existing];
      return {
        runsByJobKey: {
          ...state.runsByJobKey,
          [run.jobKey]: updated,
        },
      };
    }),
}));
