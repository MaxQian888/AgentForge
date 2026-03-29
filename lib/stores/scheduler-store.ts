"use client";

import { create } from "zustand";
import { createApiClient } from "@/lib/api-client";
import { useAuthStore } from "./auth-store";

export type SchedulerJobRunStatus =
  | "pending"
  | "running"
  | "succeeded"
  | "failed"
  | "skipped";

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
  failedJobs: number;
  activeRuns: number;
  totalRuns24h: number;
  failedRuns24h: number;
}

export interface UpdateSchedulerJobInput {
  enabled?: boolean;
  schedule?: string;
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
  fetchRuns: (jobKey: string) => Promise<void>;
  fetchStats: () => Promise<void>;
  updateJob: (jobKey: string, input: UpdateSchedulerJobInput) => Promise<void>;
  triggerJob: (jobKey: string) => Promise<void>;
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
    failedJobs: 0,
    activeRuns: 0,
    totalRuns24h: 0,
    failedRuns24h: 0,
  };
  return {
    ...base,
    totalJobs: jobs.length,
    enabledJobs: jobs.filter((j) => j.enabled).length,
    disabledJobs: jobs.filter((j) => !j.enabled).length,
    failedJobs: jobs.filter((j) => j.lastRunStatus === "failed").length,
  };
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

  fetchRuns: async (jobKey) => {
    const token = useAuthStore.getState().accessToken;
    if (!token || !jobKey) {
      return;
    }

    try {
      const api = createApiClient(API_URL);
      const { data } = await api.get<SchedulerJobRun[]>(
        `/api/v1/scheduler/jobs/${jobKey}/runs`,
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
    } catch {
      set({ error: "Unable to trigger scheduler job" });
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
