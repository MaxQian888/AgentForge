"use client";

import { useEffect, useMemo } from "react";
import { useTranslations } from "next-intl";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import type { ReviewDTO } from "@/lib/stores/review-store";
import { useTaskStore } from "@/lib/stores/task-store";
import { ReviewDecisionActions } from "./review-decision-actions";
import {
  getReviewRecommendationLabel,
  ReviewRiskBadge,
  getReviewStatusLabel,
  ReviewStatusBadge,
} from "./review-copy";

const reviewPipelineStatuses = [
  "pending",
  "in_progress",
  "pending_human",
  "completed",
  "failed",
] as const;

interface ReviewListProps {
  reviews: ReviewDTO[];
  onSelect: (review: ReviewDTO) => void;
  onApprove?: (id: string, comment?: string) => void | Promise<void>;
  onRequestChanges?: (id: string, comment?: string) => void | Promise<void>;
  onReject?: (id: string, reason: string, comment?: string) => void | Promise<void>;
  onBlock?: (id: string, reason: string, comment?: string) => void | Promise<void>;
  selectedIds?: Set<string>;
  onToggleSelect?: (reviewId: string) => void;
}

function formatReviewAge(createdAt: string, t: ReturnType<typeof useTranslations>): string {
  const diffMs = Date.now() - new Date(createdAt).getTime();
  const diffMinutes = Math.max(0, Math.floor(diffMs / (60 * 1000)));

  if (diffMinutes < 60) {
    return t("timeMinutesAgo", { count: Math.max(1, diffMinutes) });
  }

  const diffHours = Math.floor(diffMinutes / 60);
  if (diffHours < 24) {
    return t("timeHoursAgo", { count: diffHours });
  }

  return t("timeDaysAgo", { count: Math.floor(diffHours / 24) });
}

export function ReviewList({
  reviews,
  onSelect,
  onApprove,
  onRequestChanges,
  onReject,
  onBlock,
  selectedIds,
  onToggleSelect,
}: ReviewListProps) {
  const t = useTranslations("reviews");
  const tasks = useTaskStore((state) => state.tasks);
  const fetchTaskById = useTaskStore((state) => state.fetchTaskById);

  const taskById = useMemo(
    () => new Map(tasks.map((task) => [task.id, task])),
    [tasks]
  );

  useEffect(() => {
    const missingTaskIds = Array.from(
      new Set(
        reviews
          .map((review) => review.taskId)
          .filter((taskId) => taskId && !taskById.has(taskId))
      )
    );

    for (const taskId of missingTaskIds) {
      void fetchTaskById(taskId);
    }
  }, [fetchTaskById, reviews, taskById]);

  const reviewsByStatus = reviewPipelineStatuses.map((status) => ({
    status,
    reviews: reviews.filter((review) => review.status === status),
  }));

  if (reviews.length === 0) {
    return (
      <p className="py-6 text-center text-sm text-muted-foreground">
        {t("noReviewsYet")}
      </p>
    );
  }

  const selectionEnabled = Boolean(onToggleSelect);

  return (
    <div className="flex gap-3 overflow-x-auto pb-1">
      {reviewsByStatus.map(({ status, reviews: statusReviews }) => (
        <div
          key={status}
          data-testid={`review-column-${status}`}
          className="flex w-72 shrink-0 flex-col rounded-lg border bg-muted/40"
        >
          <div className="flex items-center justify-between border-b px-3 py-2">
            <h3 className="text-sm font-semibold">
              {getReviewStatusLabel(t, status)}
            </h3>
            <span className="rounded-full bg-background px-2 py-0.5 text-xs font-medium text-muted-foreground">
              {statusReviews.length}
            </span>
          </div>
          <div className="flex min-h-28 flex-col gap-2 p-2">
            {statusReviews.length === 0 ? (
              <p className="rounded-md border border-dashed px-3 py-4 text-center text-xs text-muted-foreground">
                {t("noReviewsYet")}
              </p>
            ) : null}
            {statusReviews.map((review) => {
              const isSelected = selectedIds?.has(review.id) ?? false;
              return (
                <Card
                  key={review.id}
                  data-testid={`review-card-${review.id}`}
                  className={`cursor-pointer transition-shadow hover:shadow-md${
                    isSelected ? " ring-2 ring-primary" : ""
                  }`}
                  onClick={() => onSelect(review)}
                >
                  <CardHeader className="p-3 pb-1">
                    <div className="flex items-center justify-between gap-2">
                      <div className="flex items-center gap-2">
                        {selectionEnabled ? (
                          <input
                            type="checkbox"
                            aria-label={t("selectReview")}
                            data-testid={`review-select-${review.id}`}
                            checked={isSelected}
                            onClick={(event) => event.stopPropagation()}
                            onChange={() => onToggleSelect?.(review.id)}
                            className="size-3.5 cursor-pointer"
                          />
                        ) : null}
                        <CardTitle className="text-sm font-medium">
                          {t("layerReview", { layer: review.layer })}
                        </CardTitle>
                      </div>
                      <div className="flex items-center gap-1.5">
                        <ReviewStatusBadge status={review.status} t={t} />
                        <ReviewRiskBadge riskLevel={review.riskLevel} t={t} />
                      </div>
                    </div>
                  </CardHeader>
                  <CardContent className="p-3 pt-1">
                    <p className="mb-2 line-clamp-2 text-xs text-muted-foreground">
                      {review.summary || t("noSummary")}
                    </p>
                    {(() => {
                      const task = taskById.get(review.taskId);
                      const assigneeLabel = task?.assigneeName ?? t("unassigned");
                      const branchLabel = task?.agentBranch || t("noBranch");
                      const isUnassigned = !task?.assigneeName;

                      return (
                        <div
                          className={`mb-2 flex flex-wrap items-center gap-2 text-xs ${
                            isUnassigned
                              ? "text-amber-700 dark:text-amber-400"
                              : "text-muted-foreground"
                          }`}
                        >
                          <span className="inline-flex items-center gap-1.5">
                            <Avatar className="size-5">
                              <AvatarFallback className="text-[10px]">
                                {task?.assigneeName?.[0]?.toUpperCase() ?? "U"}
                              </AvatarFallback>
                            </Avatar>
                            <span>{assigneeLabel}</span>
                          </span>
                          <span className="font-mono">{branchLabel}</span>
                          <span>{formatReviewAge(review.createdAt, t)}</span>
                        </div>
                      );
                    })()}
                    <div className="flex items-center justify-between gap-3">
                      <div className="flex items-center gap-2 text-xs text-muted-foreground">
                        <span>
                          {getReviewRecommendationLabel(t, review.recommendation)}
                        </span>
                        <span>${review.costUsd.toFixed(2)}</span>
                      </div>
                      {review.status === "pending_human" ? (
                        <div
                          className="min-w-[220px]"
                          onClick={(event) => event.stopPropagation()}
                        >
                          <ReviewDecisionActions
                            reviewId={review.id}
                            onApprove={onApprove}
                            onRequestChanges={onRequestChanges}
                            onReject={onReject}
                            onBlock={onBlock}
                            compact
                          />
                        </div>
                      ) : null}
                    </div>
                  </CardContent>
                </Card>
              );
            })}
          </div>
        </div>
      ))}
    </div>
  );
}
