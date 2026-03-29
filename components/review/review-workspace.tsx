"use client";

import { useEffect, useMemo, useState } from "react";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Separator } from "@/components/ui/separator";
import type { ReviewDTO } from "@/lib/stores/review-store";
import { ReviewDetailPanel } from "./review-detail-panel";
import { ReviewList } from "./review-list";
import { ReviewTriggerForm } from "./review-trigger-form";

interface ReviewWorkspaceProps {
  reviews: ReviewDTO[];
  loading?: boolean;
  error?: string | null;
  title?: string;
  showTitle?: boolean;
  selectedReviewId?: string | null;
  onTriggerReview?: (input: {
    taskId?: string;
    projectId?: string;
    prUrl: string;
    trigger: "manual";
  }) => void | Promise<void>;
  onApproveReview?: (id: string, comment?: string) => void | Promise<void>;
  onRequestChangesReview?: (id: string, comment?: string) => void | Promise<void>;
  triggerTaskId?: string;
  triggerProjectId?: string;
}

export function ReviewWorkspace({
  reviews,
  loading = false,
  error = null,
  title,
  showTitle = true,
  selectedReviewId = null,
  onTriggerReview,
  onApproveReview,
  onRequestChangesReview,
  triggerTaskId,
  triggerProjectId,
}: ReviewWorkspaceProps) {
  const t = useTranslations("reviews");
  const [selectedId, setSelectedId] = useState<string | null>(selectedReviewId);
  const [showTriggerForm, setShowTriggerForm] = useState(false);

  useEffect(() => {
    setSelectedId(selectedReviewId);
  }, [selectedReviewId]);

  const selectedReview = useMemo(
    () => reviews.find((review) => review.id === selectedId) ?? null,
    [reviews, selectedId],
  );

  const handleTrigger = async (prUrl: string) => {
    await onTriggerReview?.({
      taskId: triggerTaskId,
      projectId: triggerProjectId,
      prUrl,
      trigger: "manual",
    });
    setShowTriggerForm(false);
  };

  return (
    <div className="flex flex-col gap-4">
      {showTitle || onTriggerReview ? (
        <div className="flex items-center justify-between">
          <h3 className="text-sm font-semibold">
            {showTitle ? title ?? t("sectionTitle") : ""}
          </h3>
          {onTriggerReview ? (
            <Button
              size="sm"
              variant="outline"
              className="h-7 text-xs"
              onClick={() => setShowTriggerForm((value) => !value)}
              disabled={loading}
            >
              {t("triggerReview")}
            </Button>
          ) : null}
        </div>
      ) : null}

      {onTriggerReview ? (
        <ReviewTriggerForm
          open={showTriggerForm}
          loading={loading}
          onOpenChange={setShowTriggerForm}
          onSubmit={handleTrigger}
        />
      ) : null}

      {error ? (
        <p className="text-xs text-red-600 dark:text-red-400">{error}</p>
      ) : null}

      {loading && reviews.length === 0 ? (
        <p className="py-4 text-center text-xs text-muted-foreground">
          {t("loading")}
        </p>
      ) : !selectedReview ? (
        <ReviewList
          reviews={reviews}
          onSelect={(review) => setSelectedId(review.id)}
          onApprove={onApproveReview}
          onRequestChanges={onRequestChangesReview}
        />
      ) : (
        <div className="flex flex-col gap-2">
          <Button
            size="sm"
            variant="ghost"
            className="h-6 w-fit text-xs"
            onClick={() => setSelectedId(null)}
          >
            {t("backToList")}
          </Button>
          <Separator />
          <ReviewDetailPanel
            review={selectedReview}
            onApprove={onApproveReview}
            onRequestChanges={onRequestChangesReview}
          />
        </div>
      )}
    </div>
  );
}
