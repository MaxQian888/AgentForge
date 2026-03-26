"use client";

import { cn } from "@/lib/utils";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import type { ReviewDTO } from "@/lib/stores/review-store";

const statusColors: Record<string, string> = {
  pending: "bg-zinc-500/15 text-zinc-600 dark:text-zinc-400",
  in_progress: "bg-blue-500/15 text-blue-700 dark:text-blue-400",
  completed: "bg-emerald-500/15 text-emerald-700 dark:text-emerald-400",
  failed: "bg-red-500/15 text-red-700 dark:text-red-400",
  pending_human: "bg-amber-500/15 text-amber-700 dark:text-amber-400",
};

const riskColors: Record<string, string> = {
  critical: "bg-red-500/15 text-red-700 dark:text-red-400",
  high: "bg-orange-500/15 text-orange-700 dark:text-orange-400",
  medium: "bg-yellow-500/15 text-yellow-700 dark:text-yellow-400",
  low: "bg-zinc-500/15 text-zinc-600 dark:text-zinc-400",
};

const recommendationLabels: Record<string, string> = {
  approve: "Approve",
  request_changes: "Request Changes",
  reject: "Reject",
};

interface ReviewListProps {
  reviews: ReviewDTO[];
  onSelect: (review: ReviewDTO) => void;
  onApprove?: (id: string) => void;
  onRequestChanges?: (id: string, comment?: string) => void;
}

export function ReviewList({
  reviews,
  onSelect,
  onApprove,
  onRequestChanges,
}: ReviewListProps) {
  if (reviews.length === 0) {
    return (
      <p className="py-6 text-center text-sm text-muted-foreground">
        No reviews yet.
      </p>
    );
  }

  return (
    <div className="flex flex-col gap-2">
      {reviews.map((review) => (
        <Card
          key={review.id}
          className="cursor-pointer transition-shadow hover:shadow-md"
          onClick={() => onSelect(review)}
        >
          <CardHeader className="p-3 pb-1">
            <div className="flex items-center justify-between">
              <CardTitle className="text-sm font-medium">
                Layer {review.layer} Review
              </CardTitle>
              <div className="flex items-center gap-1.5">
                <Badge
                  variant="secondary"
                  className={cn("text-[11px]", statusColors[review.status] ?? "")}
                >
                  {review.status.replace("_", " ")}
                </Badge>
                <Badge
                  variant="secondary"
                  className={cn("text-[11px]", riskColors[review.riskLevel] ?? "")}
                >
                  {review.riskLevel}
                </Badge>
              </div>
            </div>
          </CardHeader>
          <CardContent className="p-3 pt-1">
            <p className="mb-2 line-clamp-2 text-xs text-muted-foreground">
              {review.summary || "No summary available."}
            </p>
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2 text-xs text-muted-foreground">
                <span>
                  {recommendationLabels[review.recommendation] ??
                    review.recommendation}
                </span>
                <span>${review.costUsd.toFixed(2)}</span>
                <span>
                  {new Date(review.createdAt).toLocaleDateString()}
                </span>
              </div>
              {review.status === "pending_human" && (
                <div
                  className="flex items-center gap-1"
                  onClick={(e) => e.stopPropagation()}
                >
                  {onApprove && (
                    <Button
                      size="sm"
                      variant="outline"
                      className="h-6 text-xs"
                      onClick={() => onApprove(review.id)}
                    >
                      Approve
                    </Button>
                  )}
                  {onRequestChanges && (
                    <Button
                      size="sm"
                      variant="outline"
                      className="h-6 text-xs"
                      onClick={() => {
                        const comment = window.prompt("Request changes comment:") ?? "";
                        onRequestChanges(review.id, comment.trim() === "" ? undefined : comment.trim());
                      }}
                    >
                      Request Changes
                    </Button>
                  )}
                </div>
              )}
            </div>
          </CardContent>
        </Card>
      ))}
    </div>
  );
}
