"use client";

import { useCallback, useEffect } from "react";
import { useReviewStore } from "@/lib/stores/review-store";
import { ReviewWorkspace } from "./review-workspace";

interface TaskReviewSectionProps {
  taskId: string;
}

export function TaskReviewSection({ taskId }: TaskReviewSectionProps) {
  const {
    reviewsByTask,
    loading,
    error,
    fetchReviewsByTask,
    triggerReview,
    approveReview,
    requestChanges,
  } = useReviewStore();

  const reviews = reviewsByTask[taskId] ?? [];

  useEffect(() => {
    fetchReviewsByTask(taskId);
  }, [taskId, fetchReviewsByTask]);

  const handleTrigger = useCallback(
    async (input: {
      taskId?: string;
      projectId?: string;
      prUrl: string;
      trigger: "manual";
    }) => {
      await triggerReview(input);
      fetchReviewsByTask(taskId);
    },
    [taskId, triggerReview, fetchReviewsByTask],
  );

  const handleApprove = useCallback(
    async (id: string, comment?: string) => {
      await approveReview(id, comment);
      fetchReviewsByTask(taskId);
    },
    [taskId, approveReview, fetchReviewsByTask],
  );

  const handleRequestChanges = useCallback(
    async (id: string, comment?: string) => {
      await requestChanges(id, comment);
      fetchReviewsByTask(taskId);
    },
    [taskId, requestChanges, fetchReviewsByTask],
  );

  return (
    <ReviewWorkspace
      reviews={reviews}
      loading={loading}
      error={error}
      onTriggerReview={handleTrigger}
      onApproveReview={handleApprove}
      onRequestChangesReview={handleRequestChanges}
      triggerTaskId={taskId}
    />
  );
}
