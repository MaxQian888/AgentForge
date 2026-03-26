"use client";

import { create } from "zustand";
import { createApiClient } from "@/lib/api-client";
import { useAuthStore } from "./auth-store";

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";

export interface ReviewFinding {
  category: string;
  subcategory?: string;
  severity: string; // "critical" | "high" | "medium" | "low" | "info"
  file?: string;
  line?: number;
  message: string;
  suggestion?: string;
  cwe?: string;
  sources?: string[];
}

export interface ReviewDTO {
  id: string;
  taskId: string;
  prUrl: string;
  prNumber: number;
  layer: number;
  status: string; // "pending" | "in_progress" | "completed" | "failed"
  riskLevel: string; // "critical" | "high" | "medium" | "low"
  findings: ReviewFinding[];
  summary: string;
  recommendation: string; // "approve" | "request_changes" | "reject"
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
    taskId: string;
    prUrl: string;
    trigger: string;
  }) => Promise<void>;
  approveReview: (id: string, comment?: string) => Promise<void>;
  rejectReview: (
    id: string,
    reason: string,
    comment?: string
  ) => Promise<void>;
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
      const { data } = await api.get<ReviewDTO[]>(
        `/api/v1/tasks/${taskId}/reviews`,
        { token }
      );
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
      await api.post(`/api/v1/reviews/${id}/approve`, { comment }, { token });
    } catch {
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
      await api.post(
        `/api/v1/reviews/${id}/reject`,
        { reason, comment },
        { token }
      );
    } catch {
      set({ error: "Unable to reject review" });
    } finally {
      set({ loading: false });
    }
  },
}));
