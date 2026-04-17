"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Separator } from "@/components/ui/separator";
import type { ReviewDTO } from "@/lib/stores/review-store";
import { ReviewBulkActions } from "./review-bulk-actions";
import { ReviewDetailPanel } from "./review-detail-panel";
import { ReviewList } from "./review-list";
import { ReviewTriggerForm } from "./review-trigger-form";
import { canTransition, invalidTransitionMessageKey } from "./review-transitions";

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
  onRejectReview?: (
    id: string,
    reason: string,
    comment?: string,
  ) => void | Promise<void>;
  onBlockReview?: (
    id: string,
    reason: string,
    comment?: string,
  ) => void | Promise<void>;
  enableBulkActions?: boolean;
  triggerTaskId?: string;
  triggerProjectId?: string;
}

type TransitionKind = "approve" | "reject" | "block" | "request_changes";

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
  onRejectReview,
  onBlockReview,
  enableBulkActions = false,
  triggerTaskId,
  triggerProjectId,
}: ReviewWorkspaceProps) {
  const t = useTranslations("reviews");
  const [selectedId, setSelectedId] = useState<string | null>(selectedReviewId);
  const [showTriggerForm, setShowTriggerForm] = useState(false);
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());
  const [transitionError, setTransitionError] = useState<string | null>(null);

  useEffect(() => {
    setSelectedId(selectedReviewId);
  }, [selectedReviewId]);

  useEffect(() => {
    const allIds = new Set(reviews.map((review) => review.id));
    setSelectedIds((previous) => {
      if (previous.size === 0) return previous;
      let mutated = false;
      const next = new Set<string>();
      previous.forEach((id) => {
        if (allIds.has(id)) {
          next.add(id);
        } else {
          mutated = true;
        }
      });
      return mutated ? next : previous;
    });
  }, [reviews]);

  const reviewById = useMemo(
    () => new Map(reviews.map((review) => [review.id, review])),
    [reviews],
  );

  const selectedReview = useMemo(
    () => (selectedId ? reviewById.get(selectedId) ?? null : null),
    [reviewById, selectedId],
  );

  const toggleSelect = useCallback((reviewId: string) => {
    setSelectedIds((previous) => {
      const next = new Set(previous);
      if (next.has(reviewId)) {
        next.delete(reviewId);
      } else {
        next.add(reviewId);
      }
      return next;
    });
  }, []);

  const clearSelection = useCallback(() => {
    setSelectedIds(new Set());
  }, []);

  const guardTransition = useCallback(
    (review: ReviewDTO | undefined, kind: TransitionKind): boolean => {
      if (!review) {
        setTransitionError(t(invalidTransitionMessageKey(kind)));
        return false;
      }
      if (!canTransition(review.status, kind)) {
        setTransitionError(t(invalidTransitionMessageKey(kind)));
        return false;
      }
      setTransitionError(null);
      return true;
    },
    [t],
  );

  const handleApprove = useCallback(
    async (id: string, comment?: string) => {
      if (!guardTransition(reviewById.get(id), "approve")) return;
      await onApproveReview?.(id, comment);
    },
    [guardTransition, onApproveReview, reviewById],
  );

  const handleRequestChanges = useCallback(
    async (id: string, comment?: string) => {
      if (!guardTransition(reviewById.get(id), "request_changes")) return;
      await onRequestChangesReview?.(id, comment);
    },
    [guardTransition, onRequestChangesReview, reviewById],
  );

  const handleReject = useCallback(
    async (id: string, reason: string, comment?: string) => {
      if (!guardTransition(reviewById.get(id), "reject")) return;
      await onRejectReview?.(id, reason, comment);
    },
    [guardTransition, onRejectReview, reviewById],
  );

  const handleBlock = useCallback(
    async (id: string, reason: string, comment?: string) => {
      if (!guardTransition(reviewById.get(id), "block")) return;
      // Block is a distinct UI intent but maps onto the reject endpoint.
      await onBlockReview?.(id, reason, comment);
    },
    [guardTransition, onBlockReview, reviewById],
  );

  const selectedReviews = useMemo(
    () =>
      Array.from(selectedIds)
        .map((id) => reviewById.get(id))
        .filter((value): value is ReviewDTO => Boolean(value)),
    [selectedIds, reviewById],
  );

  const eligibleReviews = useMemo(
    () => selectedReviews.filter((r) => canTransition(r.status, "approve")),
    [selectedReviews],
  );

  const runBulkApprove = useCallback(async () => {
    if (!onApproveReview) return;
    await Promise.all(eligibleReviews.map((r) => onApproveReview(r.id)));
    clearSelection();
  }, [onApproveReview, eligibleReviews, clearSelection]);

  const runBulkReject = useCallback(
    async (reason: string) => {
      if (!onRejectReview) return;
      await Promise.all(
        eligibleReviews.map((r) => onRejectReview(r.id, reason)),
      );
      clearSelection();
    },
    [onRejectReview, eligibleReviews, clearSelection],
  );

  const runBulkBlock = useCallback(
    async (reason: string) => {
      if (!onBlockReview) return;
      await Promise.all(
        eligibleReviews.map((r) => onBlockReview(r.id, reason)),
      );
      clearSelection();
    },
    [onBlockReview, eligibleReviews, clearSelection],
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

  const showBulkToolbar =
    enableBulkActions && Boolean(onApproveReview || onRejectReview || onBlockReview);

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

      {transitionError ? (
        <div
          data-testid="review-transition-error"
          className="rounded-md border border-red-300 bg-red-50 p-2 text-xs text-red-700 dark:border-red-900 dark:bg-red-950/30 dark:text-red-300"
        >
          <p className="font-medium">{t("transitionInvalidTitle")}</p>
          <p>{transitionError}</p>
        </div>
      ) : null}

      {showBulkToolbar ? (
        <ReviewBulkActions
          selectedCount={selectedIds.size}
          eligibleCount={eligibleReviews.length}
          onBulkApprove={runBulkApprove}
          onBulkReject={runBulkReject}
          onBulkBlock={runBulkBlock}
          onClearSelection={clearSelection}
        />
      ) : null}

      {loading && reviews.length === 0 ? (
        <p className="py-4 text-center text-xs text-muted-foreground">
          {t("loading")}
        </p>
      ) : !selectedReview ? (
        <ReviewList
          reviews={reviews}
          onSelect={(review) => setSelectedId(review.id)}
          onApprove={onApproveReview ? handleApprove : undefined}
          onRequestChanges={onRequestChangesReview ? handleRequestChanges : undefined}
          onReject={onRejectReview ? handleReject : undefined}
          onBlock={onBlockReview ? handleBlock : undefined}
          selectedIds={showBulkToolbar ? selectedIds : undefined}
          onToggleSelect={showBulkToolbar ? toggleSelect : undefined}
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
            onApprove={onApproveReview ? handleApprove : undefined}
            onRequestChanges={onRequestChangesReview ? handleRequestChanges : undefined}
            onReject={onRejectReview ? handleReject : undefined}
            onBlock={onBlockReview ? handleBlock : undefined}
          />
        </div>
      )}
    </div>
  );
}
