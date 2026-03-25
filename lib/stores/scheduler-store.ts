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
  summary: string;
  errorMessage: string;
  metrics: string;
  createdAt: string;
  updatedAt: string;
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
  loading: boolean;
  actionJobKey: string | null;
  error: string | null;
  fetchJobs: () => Promise<void>;
  fetchRuns: (jobKey: string) => Promise<void>;
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

export const useSchedulerStore = create<SchedulerState>()((set, get) => ({
  jobs: [],
  runsByJobKey: {},
  draftSchedules: {},
  selectedJobKey: null,
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
      return {
        jobs,
        draftSchedules: {
          ...state.draftSchedules,
          [job.jobKey]: state.draftSchedules[job.jobKey] ?? job.schedule,
        },
      };
    }),

  recordRun: (run) =>
    set((state) => ({
      runsByJobKey: {
        ...state.runsByJobKey,
        [run.jobKey]: [run, ...(state.runsByJobKey[run.jobKey] ?? [])],
      },
    })),
}));
