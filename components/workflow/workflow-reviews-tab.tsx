"use client";

import { useEffect, useState } from "react";
import { ClipboardCheck, ChevronDown, ChevronUp } from "lucide-react";
import { toast } from "sonner";
import { useWorkflowStore, type WorkflowPendingReview } from "@/lib/stores/workflow-store";
import { EmptyState } from "@/components/shared/empty-state";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible";
import { cn } from "@/lib/utils";

interface WorkflowReviewsTabProps {
  projectId: string;
}

function decisionBadgeClass(decision: string): string {
  if (decision === "approved") {
    return "bg-green-500/15 text-green-700 dark:text-green-400";
  }
  if (decision === "rejected") {
    return "bg-red-500/15 text-red-700 dark:text-red-400";
  }
  // pending or anything else
  return "bg-yellow-500/15 text-yellow-700 dark:text-yellow-400";
}

interface ReviewCardProps {
  review: WorkflowPendingReview;
  expanded: boolean;
  onToggle: () => void;
  comment: string;
  onCommentChange: (value: string) => void;
  submitting: boolean;
  onResolve: (decision: string) => void;
}

function ReviewCard({
  review,
  expanded,
  onToggle,
  comment,
  onCommentChange,
  submitting,
  onResolve,
}: ReviewCardProps) {
  const [contextOpen, setContextOpen] = useState(false);

  return (
    <Card
      className={cn(
        "cursor-pointer transition-colors",
        expanded && "ring-1 ring-ring"
      )}
    >
      <CardContent className="p-4">
        {/* Collapsed header — always visible */}
        <div
          className="flex items-start gap-3"
          onClick={onToggle}
          role="button"
          tabIndex={0}
          onKeyDown={(e) => {
            if (e.key === "Enter" || e.key === " ") onToggle();
          }}
        >
          <Badge
            className={cn(
              "shrink-0 border-0 text-xs capitalize",
              decisionBadgeClass(review.decision)
            )}
          >
            {review.decision}
          </Badge>

          <div className="min-w-0 flex-1 space-y-1">
            <p className={cn("text-sm", !expanded && "truncate")}>
              {review.prompt}
            </p>
            <div className="flex items-center gap-3 text-xs text-muted-foreground">
              <span className="font-mono">
                {review.executionId.slice(0, 8)}
              </span>
              <span>{review.nodeId}</span>
              <span>{new Date(review.createdAt).toLocaleString()}</span>
            </div>
          </div>

          <span className="shrink-0 text-muted-foreground">
            {expanded ? (
              <ChevronUp className="size-4" />
            ) : (
              <ChevronDown className="size-4" />
            )}
          </span>
        </div>

        {/* Expanded body */}
        {expanded && (
          <div className="mt-3 space-y-3">
            {review.context && (
              <Collapsible open={contextOpen} onOpenChange={setContextOpen}>
                <CollapsibleTrigger asChild>
                  <Button variant="ghost" size="sm" className="h-7 px-2 text-xs">
                    {contextOpen ? "Hide context" : "Show context"}
                  </Button>
                </CollapsibleTrigger>
                <CollapsibleContent>
                  <pre className="text-xs bg-muted p-2 rounded overflow-auto max-h-40">
                    {JSON.stringify(review.context, null, 2)}
                  </pre>
                </CollapsibleContent>
              </Collapsible>
            )}

            <Textarea
              placeholder="Optional comment..."
              value={comment}
              onChange={(e) => onCommentChange(e.target.value)}
              className="mt-3"
            />

            <div className="flex gap-2 mt-3">
              <Button
                size="sm"
                className="bg-green-600 hover:bg-green-700 text-white"
                disabled={submitting}
                onClick={() => onResolve("approved")}
              >
                Approve
              </Button>
              <Button
                variant="destructive"
                size="sm"
                disabled={submitting}
                onClick={() => onResolve("rejected")}
              >
                Reject
              </Button>
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  );
}

export function WorkflowReviewsTab({ projectId }: WorkflowReviewsTabProps) {
  const pendingReviews = useWorkflowStore((s) => s.pendingReviews);
  const pendingReviewsLoading = useWorkflowStore((s) => s.pendingReviewsLoading);
  const fetchPendingReviews = useWorkflowStore((s) => s.fetchPendingReviews);
  const resolveReview = useWorkflowStore((s) => s.resolveReview);

  const [expandedId, setExpandedId] = useState<string | null>(null);
  const [comment, setComment] = useState("");
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    fetchPendingReviews(projectId);
  }, [projectId, fetchPendingReviews]);

  function toggleExpanded(id: string) {
    setExpandedId((prev) => {
      if (prev === id) {
        setComment("");
        return null;
      }
      setComment("");
      return id;
    });
  }

  async function handleResolve(review: WorkflowPendingReview, decision: string) {
    setSubmitting(true);
    const ok = await resolveReview(review.executionId, review.nodeId, decision, comment);
    setSubmitting(false);
    if (ok) {
      toast.success("Review " + decision);
      setComment("");
      setExpandedId(null);
      fetchPendingReviews(projectId);
    }
  }

  if (pendingReviewsLoading) {
    return (
      <div className="space-y-3">
        <Skeleton className="h-16 rounded-lg" />
        <Skeleton className="h-16 rounded-lg" />
        <Skeleton className="h-16 rounded-lg" />
      </div>
    );
  }

  if (pendingReviews.length === 0) {
    return (
      <EmptyState
        icon={ClipboardCheck}
        title="No pending reviews"
        description="All workflow reviews are resolved."
      />
    );
  }

  return (
    <div className="space-y-3">
      {pendingReviews.map((review) => (
        <ReviewCard
          key={review.id}
          review={review}
          expanded={expandedId === review.id}
          onToggle={() => toggleExpanded(review.id)}
          comment={expandedId === review.id ? comment : ""}
          onCommentChange={setComment}
          submitting={submitting}
          onResolve={(decision) => handleResolve(review, decision)}
        />
      ))}
    </div>
  );
}
