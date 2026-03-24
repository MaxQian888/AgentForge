"use client";

import { useState } from "react";
import { cn } from "@/lib/utils";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Separator } from "@/components/ui/separator";
import { ReviewFindingsTable } from "./review-findings-table";
import type { ReviewDTO } from "@/lib/stores/review-store";

const riskColors: Record<string, string> = {
  critical: "bg-red-500/15 text-red-700 dark:text-red-400",
  high: "bg-orange-500/15 text-orange-700 dark:text-orange-400",
  medium: "bg-yellow-500/15 text-yellow-700 dark:text-yellow-400",
  low: "bg-zinc-500/15 text-zinc-600 dark:text-zinc-400",
};

const recommendationColors: Record<string, string> = {
  approve: "bg-emerald-500/15 text-emerald-700 dark:text-emerald-400",
  request_changes: "bg-amber-500/15 text-amber-700 dark:text-amber-400",
  reject: "bg-red-500/15 text-red-700 dark:text-red-400",
};

const recommendationLabels: Record<string, string> = {
  approve: "Approve",
  request_changes: "Request Changes",
  reject: "Reject",
};

interface ReviewDetailPanelProps {
  review: ReviewDTO;
  onApprove?: (id: string, comment?: string) => void;
  onReject?: (id: string, reason: string) => void;
}

export function ReviewDetailPanel({
  review,
  onApprove,
  onReject,
}: ReviewDetailPanelProps) {
  const [approveComment, setApproveComment] = useState("");
  const [rejectReason, setRejectReason] = useState("");
  const [showApproveForm, setShowApproveForm] = useState(false);
  const [showRejectForm, setShowRejectForm] = useState(false);

  const handleApprove = () => {
    onApprove?.(review.id, approveComment || undefined);
    setApproveComment("");
    setShowApproveForm(false);
  };

  const handleReject = () => {
    if (!rejectReason.trim()) return;
    onReject?.(review.id, rejectReason);
    setRejectReason("");
    setShowRejectForm(false);
  };

  return (
    <div className="flex flex-col gap-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <h3 className="text-base font-semibold">
          Layer {review.layer} Review
        </h3>
        <div className="flex items-center gap-1.5">
          <Badge
            variant="secondary"
            className={cn("text-xs", riskColors[review.riskLevel] ?? "")}
          >
            {review.riskLevel} risk
          </Badge>
          <Badge
            variant="secondary"
            className={cn(
              "text-xs",
              recommendationColors[review.recommendation] ?? ""
            )}
          >
            {recommendationLabels[review.recommendation] ??
              review.recommendation}
          </Badge>
        </div>
      </div>

      {/* Summary */}
      <div>
        <Label className="text-xs font-medium text-muted-foreground">
          Summary
        </Label>
        <p className="mt-1 text-sm">{review.summary || "No summary."}</p>
      </div>

      {/* Meta */}
      <div className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground">
        <span>PR: {review.prUrl || `#${review.prNumber}`}</span>
        <span>Cost: ${review.costUsd.toFixed(2)}</span>
        <span>Status: {review.status.replace("_", " ")}</span>
        <span>
          Updated: {new Date(review.updatedAt).toLocaleString()}
        </span>
      </div>

      <Separator />

      {/* Findings */}
      <div>
        <Label className="text-xs font-medium text-muted-foreground">
          Findings ({review.findings?.length ?? 0})
        </Label>
        <div className="mt-2">
          <ReviewFindingsTable findings={review.findings ?? []} />
        </div>
      </div>

      {/* Actions */}
      {review.status === "completed" && (onApprove || onReject) && (
        <>
          <Separator />
          <div className="flex flex-col gap-2">
            {/* Approve */}
            {onApprove && !showApproveForm && !showRejectForm && (
              <Button
                variant="outline"
                size="sm"
                onClick={() => setShowApproveForm(true)}
              >
                Approve
              </Button>
            )}
            {showApproveForm && (
              <div className="flex flex-col gap-2 rounded-md border p-3">
                <Label className="text-xs">Comment (optional)</Label>
                <Input
                  value={approveComment}
                  onChange={(e) => setApproveComment(e.target.value)}
                  placeholder="Optional approval comment..."
                  className="h-8 text-sm"
                />
                <div className="flex items-center gap-2">
                  <Button size="sm" onClick={handleApprove}>
                    Confirm Approve
                  </Button>
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => setShowApproveForm(false)}
                  >
                    Cancel
                  </Button>
                </div>
              </div>
            )}

            {/* Reject */}
            {onReject && !showRejectForm && !showApproveForm && (
              <Button
                variant="outline"
                size="sm"
                className="text-red-600 dark:text-red-400"
                onClick={() => setShowRejectForm(true)}
              >
                Reject
              </Button>
            )}
            {showRejectForm && (
              <div className="flex flex-col gap-2 rounded-md border p-3">
                <Label className="text-xs">Reason (required)</Label>
                <Input
                  value={rejectReason}
                  onChange={(e) => setRejectReason(e.target.value)}
                  placeholder="Rejection reason..."
                  className="h-8 text-sm"
                />
                <div className="flex items-center gap-2">
                  <Button
                    size="sm"
                    variant="destructive"
                    onClick={handleReject}
                    disabled={!rejectReason.trim()}
                  >
                    Confirm Reject
                  </Button>
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => setShowRejectForm(false)}
                  >
                    Cancel
                  </Button>
                </div>
              </div>
            )}
          </div>
        </>
      )}
    </div>
  );
}
