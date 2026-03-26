"use client";

import { useMemo, useState } from "react";
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
  onRequestChanges?: (id: string, comment?: string) => void;
}

export function ReviewDetailPanel({
  review,
  onApprove,
  onRequestChanges,
}: ReviewDetailPanelProps) {
  const [approveComment, setApproveComment] = useState("");
  const [requestChangesComment, setRequestChangesComment] = useState("");
  const [showApproveForm, setShowApproveForm] = useState(false);
  const [showRequestChangesForm, setShowRequestChangesForm] = useState(false);

  const changedFileCount = review.executionMetadata?.changedFiles?.length ?? 0;
  const executionResults = review.executionMetadata?.results ?? [];
  const decisions = review.executionMetadata?.decisions ?? [];
  const hasExecutionMetadata = useMemo(() => {
    return (
      Boolean(review.executionMetadata?.triggerEvent) ||
      changedFileCount > 0 ||
      executionResults.length > 0
    );
  }, [review.executionMetadata?.triggerEvent, changedFileCount, executionResults.length]);

  const handleApprove = () => {
    onApprove?.(review.id, approveComment.trim() || undefined);
    setApproveComment("");
    setShowApproveForm(false);
  };

  const handleRequestChanges = () => {
    onRequestChanges?.(review.id, requestChangesComment.trim() || undefined);
    setRequestChangesComment("");
    setShowRequestChangesForm(false);
  };

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center justify-between">
        <h3 className="text-base font-semibold">Layer {review.layer} Review</h3>
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
              recommendationColors[review.recommendation] ?? "",
            )}
          >
            {recommendationLabels[review.recommendation] ?? review.recommendation}
          </Badge>
        </div>
      </div>

      <div>
        <Label className="text-xs font-medium text-muted-foreground">Summary</Label>
        <p className="mt-1 text-sm">{review.summary || "No summary."}</p>
      </div>

      <div className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground">
        <span>PR: {review.prUrl || `#${review.prNumber}`}</span>
        <span>Cost: ${review.costUsd.toFixed(2)}</span>
        <span>Status: {review.status.replace("_", " ")}</span>
        <span>Updated: {new Date(review.updatedAt).toLocaleString()}</span>
      </div>

      <Separator />

      {hasExecutionMetadata && (
        <>
          <details className="rounded-md border p-3">
            <summary className="cursor-pointer text-sm font-medium">
              Execution Details
            </summary>
            <div className="mt-3 space-y-2 text-xs text-muted-foreground">
              {review.executionMetadata?.triggerEvent && (
                <div>
                  <span className="font-medium text-foreground">Trigger:</span>{" "}
                  {review.executionMetadata.triggerEvent}
                </div>
              )}
              <div>
                <span className="font-medium text-foreground">Changed Files:</span>{" "}
                {changedFileCount}
              </div>
              {executionResults.length > 0 && (
                <div className="space-y-1">
                  <div className="font-medium text-foreground">Plugin / Dimension Results</div>
                  <div className="space-y-1">
                    {executionResults.map((result) => (
                      <div
                        key={`${result.kind}-${result.id}`}
                        className="rounded border bg-muted/40 px-2 py-1"
                      >
                        <span className="font-medium text-foreground">
                          {result.displayName || result.id}
                        </span>{" "}
                        <span>({result.kind})</span>{" "}
                        <span className="uppercase">{result.status}</span>
                        {result.summary ? <span> - {result.summary}</span> : null}
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </div>
          </details>
          <Separator />
        </>
      )}

      <div>
        <Label className="text-xs font-medium text-muted-foreground">
          Findings ({review.findings?.length ?? 0})
        </Label>
        <div className="mt-2">
          <ReviewFindingsTable findings={review.findings ?? []} />
        </div>
      </div>

      {decisions.length > 0 && (
        <>
          <Separator />
          <div className="space-y-2">
            <Label className="text-xs font-medium text-muted-foreground">Decisions</Label>
            <div className="space-y-2">
              {decisions.map((decision, index) => (
                <div key={`${decision.timestamp}-${index}`} className="rounded border p-2">
                  <div className="text-xs font-medium">
                    {decision.actor} - {decision.action}
                  </div>
                  <div className="text-xs text-muted-foreground">
                    {new Date(decision.timestamp).toLocaleString()}
                  </div>
                  {decision.comment ? (
                    <p className="mt-1 text-xs text-muted-foreground">{decision.comment}</p>
                  ) : null}
                </div>
              ))}
            </div>
          </div>
        </>
      )}

      {review.status === "pending_human" && (onApprove || onRequestChanges) && (
        <>
          <Separator />
          <div className="flex flex-col gap-2">
            {onApprove && !showApproveForm && !showRequestChangesForm && (
              <Button variant="outline" size="sm" onClick={() => setShowApproveForm(true)}>
                Approve
              </Button>
            )}
            {showApproveForm && (
              <div className="flex flex-col gap-2 rounded-md border p-3">
                <Label className="text-xs">Comment (optional)</Label>
                <Input
                  value={approveComment}
                  onChange={(event) => setApproveComment(event.target.value)}
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

            {onRequestChanges && !showRequestChangesForm && !showApproveForm && (
              <Button
                variant="outline"
                size="sm"
                onClick={() => setShowRequestChangesForm(true)}
              >
                Request Changes
              </Button>
            )}
            {showRequestChangesForm && (
              <div className="flex flex-col gap-2 rounded-md border p-3">
                <Label className="text-xs">Comment (optional)</Label>
                <Input
                  value={requestChangesComment}
                  onChange={(event) => setRequestChangesComment(event.target.value)}
                  placeholder="Describe what needs to change..."
                  className="h-8 text-sm"
                />
                <div className="flex items-center gap-2">
                  <Button size="sm" onClick={handleRequestChanges}>
                    Confirm Request Changes
                  </Button>
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => setShowRequestChangesForm(false)}
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
