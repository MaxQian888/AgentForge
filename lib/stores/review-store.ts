"use client";

import { create } from "zustand";
import { toast } from "sonner";
import { createApiClient } from "@/lib/api-client";
import { useAuthStore } from "./auth-store";

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";

export interface ReviewDecision {
  actor: string;
  action: string;
  comment: string;
  timestamp: string;
}

export interface ReviewExecutionResult {
  id: string;
  kind: string;
  status: string;
  displayName?: string;
  summary?: string;
  error?: string;
}

export interface ExecutionMetadata {
  triggerEvent?: string;
  projectId?: string;
  changedFiles?: string[];
  dimensions?: string[];
  results?: ReviewExecutionResult[];
  decisions?: ReviewDecision[];
}

export interface ReviewFinding {
  id?: string;
  category: string;
  subcategory?: string;
  severity: string;
  file?: string;
  line?: number;
  message: string;
  suggestion?: string;
  cwe?: string;
  sources?: string[];
  dismissed?: boolean;
}

export interface ReviewDTO {
  id: string;
  taskId: string;
  prUrl: string;
  prNumber: number;
  layer: number;
  status: string;
  riskLevel: string;
  findings: ReviewFinding[];
  executionMetadata?: ExecutionMetadata;
  summary: string;
  recommendation: string;
  costUsd: number;
  createdAt: string;
  updatedAt: string;
}

interface ReviewState {
  reviewsByTask: Record<string, ReviewDTO[]>;
  allReviews: ReviewDTO[];
  allReviewsLoading: boolean;
  loading: boolean;
  error: string | null;
  fetchAllReviews: (filters?: {
    status?: string;
    riskLevel?: string;
  }) => Promise<void>;
  fetchReviewsByTask: (taskId: string) => Promise<void>;
  triggerReview: (data: {
    taskId?: string;
    projectId?: string;
    prUrl: string;
    trigger: string;
    diff?: string;
  }) => Promise<void>;
  approveReview: (id: string, comment?: string) => Promise<void>;
  rejectReview: (id: string, reason: string, comment?: string) => Promise<void>;
  requestChanges: (id: string, comment?: string) => Promise<void>;
  markFalsePositive: (
    id: string,
    findingIds: string[],
    reason: string,
  ) => Promise<void>;
  updateReview: (review: ReviewDTO) => void;
}

function normalizeTaskID(taskId: string | undefined): string {
  const trimmed = (taskId ?? "").trim();
  if (!trimmed || trimmed === "00000000-0000-0000-0000-000000000000") {
    return "";
  }
  return trimmed;
}

function upsertReviewList(list: ReviewDTO[], review: ReviewDTO): ReviewDTO[] {
  const index = list.findIndex((item) => item.id === review.id);
  if (index === -1) {
    return [review, ...list];
  }
  const next = [...list];
  next[index] = review;
  return next;
}

export const useReviewStore = create<ReviewState>()((set, get) => ({
  reviewsByTask: {},
  allReviews: [],
  allReviewsLoading: false,
  loading: false,
  error: null,

  fetchAllReviews: async (filters) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return;

    set({ allReviewsLoading: true, error: null });
    try {
      const api = createApiClient(API_URL);
      const params = new URLSearchParams();
      if (filters?.status) params.set("status", filters.status);
      if (filters?.riskLevel) params.set("riskLevel", filters.riskLevel);
      const qs = params.toString();
      const path = `/api/v1/reviews${qs ? `?${qs}` : ""}`;
      const { data } = await api.get<ReviewDTO[]>(path, { token });
      set({ allReviews: data ?? [], error: null });
    } catch {
      set({ error: "Unable to load reviews" });
    } finally {
      set({ allReviewsLoading: false });
    }
  },

  fetchReviewsByTask: async (taskId) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return;

    set({ loading: true, error: null });
    try {
      const api = createApiClient(API_URL);
      const { data } = await api.get<ReviewDTO[]>(`/api/v1/tasks/${taskId}/reviews`, {
        token,
      });
      set({
        reviewsByTask: { ...get().reviewsByTask, [taskId]: data ?? [] },
        error: null,
      });
    } catch {
      set({ error: "Unable to load reviews" });
    } finally {
      set({ loading: false });
    }
  },

  triggerReview: async (body) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return;

    set({ loading: true, error: null });
    try {
      const api = createApiClient(API_URL);
      await api.post("/api/v1/reviews/trigger", body, { token });
    } catch {
      toast.error("Unable to trigger review");
      set({ error: "Unable to trigger review" });
    } finally {
      set({ loading: false });
    }
  },

  approveReview: async (id, comment) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return;

    set({ loading: true, error: null });
    try {
      const api = createApiClient(API_URL);
      const { data } = await api.post<ReviewDTO>(
        `/api/v1/reviews/${id}/approve`,
        { comment },
        { token },
      );
      if (data) {
        get().updateReview(data);
      }
    } catch {
      toast.error("Unable to approve review");
      set({ error: "Unable to approve review" });
    } finally {
      set({ loading: false });
    }
  },

  rejectReview: async (id, reason, comment) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return;

    set({ loading: true, error: null });
    try {
      const api = createApiClient(API_URL);
      const { data } = await api.post<ReviewDTO>(
        `/api/v1/reviews/${id}/reject`,
        { reason, comment },
        { token },
      );
      if (data) {
        get().updateReview(data);
      }
    } catch {
      toast.error("Unable to reject review");
      set({ error: "Unable to reject review" });
    } finally {
      set({ loading: false });
    }
  },

  requestChanges: async (id, comment) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return;

    set({ loading: true, error: null });
    try {
      const api = createApiClient(API_URL);
      const { data } = await api.post<ReviewDTO>(
        `/api/v1/reviews/${id}/request-changes`,
        { comment },
        { token },
      );
      if (data) {
        get().updateReview(data);
      }
    } catch {
      toast.error("Unable to request review changes");
      set({ error: "Unable to request review changes" });
    } finally {
      set({ loading: false });
    }
  },

  markFalsePositive: async (id, findingIds, reason) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return;

    set({ loading: true, error: null });
    try {
      const api = createApiClient(API_URL);
      const { data } = await api.post<ReviewDTO>(
        `/api/v1/reviews/${id}/false-positive`,
        { findingIds, reason },
        { token },
      );
      if (data) {
        get().updateReview(data);
      }
    } catch {
      toast.error("Unable to mark false positive finding");
      set({ error: "Unable to mark false positive finding" });
    } finally {
      set({ loading: false });
    }
  },

  updateReview: (review) => {
    set((state) => {
      const allReviews = upsertReviewList(state.allReviews, review);
      const reviewsByTask = { ...state.reviewsByTask };
      const taskID = normalizeTaskID(review.taskId);

      if (taskID) {
        reviewsByTask[taskID] = upsertReviewList(reviewsByTask[taskID] ?? [], review);
      } else {
        Object.keys(reviewsByTask).forEach((key) => {
          const reviews = reviewsByTask[key];
          if (!Array.isArray(reviews)) return;
          const index = reviews.findIndex((item) => item.id === review.id);
          if (index === -1) return;
          const next = [...reviews];
          next[index] = review;
          reviewsByTask[key] = next;
        });
      }

      return {
        ...state,
        allReviews,
        reviewsByTask,
      };
    });
  },
}));
