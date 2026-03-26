"use client";

import { useCallback, useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Separator } from "@/components/ui/separator";
import { ReviewList } from "./review-list";
import { ReviewDetailPanel } from "./review-detail-panel";
import { useReviewStore, type ReviewDTO } from "@/lib/stores/review-store";

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
  const [selected, setSelected] = useState<ReviewDTO | null>(null);
  const [showTriggerForm, setShowTriggerForm] = useState(false);
  const [triggerPrUrl, setTriggerPrUrl] = useState("");

  useEffect(() => {
    fetchReviewsByTask(taskId);
  }, [taskId, fetchReviewsByTask]);

  const handleTrigger = useCallback(async () => {
    if (!triggerPrUrl.trim()) return;
    await triggerReview({ taskId, prUrl: triggerPrUrl, trigger: "manual" });
    setTriggerPrUrl("");
    setShowTriggerForm(false);
    fetchReviewsByTask(taskId);
  }, [taskId, triggerPrUrl, triggerReview, fetchReviewsByTask]);

  const handleApprove = useCallback(
    async (id: string, comment?: string) => {
      await approveReview(id, comment);
      fetchReviewsByTask(taskId);
    },
    [taskId, approveReview, fetchReviewsByTask]
  );

  const handleRequestChanges = useCallback(
    async (id: string, comment?: string) => {
      await requestChanges(id, comment);
      fetchReviewsByTask(taskId);
    },
    [taskId, requestChanges, fetchReviewsByTask]
  );

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-semibold">Reviews</h3>
        <Button
          size="sm"
          variant="outline"
          className="h-7 text-xs"
          onClick={() => setShowTriggerForm((v) => !v)}
          disabled={loading}
        >
          Trigger Review
        </Button>
      </div>

      {showTriggerForm && (
        <div className="flex flex-col gap-2 rounded-md border p-3">
          <Label className="text-xs">PR URL</Label>
          <Input
            value={triggerPrUrl}
            onChange={(e) => setTriggerPrUrl(e.target.value)}
            placeholder="https://github.com/org/repo/pull/123"
            className="h-8 text-sm"
          />
          <div className="flex items-center gap-2">
            <Button
              size="sm"
              onClick={handleTrigger}
              disabled={!triggerPrUrl.trim() || loading}
            >
              Submit
            </Button>
            <Button
              size="sm"
              variant="outline"
              onClick={() => setShowTriggerForm(false)}
            >
              Cancel
            </Button>
          </div>
        </div>
      )}

      {error && (
        <p className="text-xs text-red-600 dark:text-red-400">{error}</p>
      )}

      {loading && reviews.length === 0 && (
        <p className="py-4 text-center text-xs text-muted-foreground">
          Loading reviews...
        </p>
      )}

      {!selected ? (
        <ReviewList
          reviews={reviews}
          onSelect={setSelected}
          onApprove={(id) => handleApprove(id)}
          onRequestChanges={handleRequestChanges}
        />
      ) : (
        <div className="flex flex-col gap-2">
          <Button
            size="sm"
            variant="ghost"
            className="h-6 w-fit text-xs"
            onClick={() => setSelected(null)}
          >
            Back to list
          </Button>
          <Separator />
          <ReviewDetailPanel
            review={selected}
            onApprove={handleApprove}
            onRequestChanges={handleRequestChanges}
          />
        </div>
      )}
    </div>
  );
}
