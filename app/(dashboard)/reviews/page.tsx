"use client";

import { useEffect, useState } from "react";
import { FileSearch } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent } from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { cn } from "@/lib/utils";
import { useReviewStore, type ReviewDTO } from "@/lib/stores/review-store";

const statusColors: Record<string, string> = {
  pending: "bg-yellow-500/15 text-yellow-700 dark:text-yellow-400",
  in_progress: "bg-blue-500/15 text-blue-700 dark:text-blue-400",
  completed: "bg-green-500/15 text-green-700 dark:text-green-400",
  failed: "bg-red-500/15 text-red-700 dark:text-red-400",
  pending_human: "bg-purple-500/15 text-purple-700 dark:text-purple-400",
};

const riskColors: Record<string, string> = {
  critical: "bg-red-500/15 text-red-700 dark:text-red-400",
  high: "bg-orange-500/15 text-orange-700 dark:text-orange-400",
  medium: "bg-yellow-500/15 text-yellow-700 dark:text-yellow-400",
  low: "bg-green-500/15 text-green-700 dark:text-green-400",
};

const recommendationColors: Record<string, string> = {
  approve: "bg-green-500/15 text-green-700 dark:text-green-400",
  request_changes: "bg-yellow-500/15 text-yellow-700 dark:text-yellow-400",
  reject: "bg-red-500/15 text-red-700 dark:text-red-400",
};

export default function ReviewsPage() {
  const { allReviews, allReviewsLoading, fetchAllReviews } = useReviewStore();
  const [statusFilter, setStatusFilter] = useState("all");
  const [riskFilter, setRiskFilter] = useState("all");
  const [expandedId, setExpandedId] = useState<string | null>(null);

  useEffect(() => {
    fetchAllReviews({
      status: statusFilter === "all" ? undefined : statusFilter,
      riskLevel: riskFilter === "all" ? undefined : riskFilter,
    });
  }, [fetchAllReviews, statusFilter, riskFilter]);

  return (
    <div className="flex flex-col gap-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Reviews</h1>
      </div>

      <div className="flex items-center gap-4">
        <div className="flex items-center gap-2">
          <span className="text-sm text-muted-foreground">Status</span>
          <Select value={statusFilter} onValueChange={setStatusFilter}>
            <SelectTrigger className="w-[160px]">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All</SelectItem>
              <SelectItem value="pending">Pending</SelectItem>
              <SelectItem value="in_progress">In Progress</SelectItem>
              <SelectItem value="completed">Completed</SelectItem>
              <SelectItem value="failed">Failed</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div className="flex items-center gap-2">
          <span className="text-sm text-muted-foreground">Risk Level</span>
          <Select value={riskFilter} onValueChange={setRiskFilter}>
            <SelectTrigger className="w-[140px]">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All</SelectItem>
              <SelectItem value="critical">Critical</SelectItem>
              <SelectItem value="high">High</SelectItem>
              <SelectItem value="medium">Medium</SelectItem>
              <SelectItem value="low">Low</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </div>

      {allReviewsLoading ? (
        <p className="text-muted-foreground">Loading reviews...</p>
      ) : allReviews.length === 0 ? (
        <Card>
          <CardContent className="py-12 text-center">
            <FileSearch className="mx-auto mb-4 size-12 text-muted-foreground" />
            <p className="text-muted-foreground">
              No reviews found. Reviews are created when agents submit PRs for
              code review.
            </p>
          </CardContent>
        </Card>
      ) : (
        <div className="rounded-md border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Task</TableHead>
                <TableHead>PR URL</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Risk Level</TableHead>
                <TableHead>Recommendation</TableHead>
                <TableHead className="text-right">Cost</TableHead>
                <TableHead>Created</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {allReviews.map((review) => (
                <ReviewRow
                  key={review.id}
                  review={review}
                  expanded={expandedId === review.id}
                  onToggle={() =>
                    setExpandedId(expandedId === review.id ? null : review.id)
                  }
                />
              ))}
            </TableBody>
          </Table>
        </div>
      )}
    </div>
  );
}

function ReviewRow({
  review,
  expanded,
  onToggle,
}: {
  review: ReviewDTO;
  expanded: boolean;
  onToggle: () => void;
}) {
  return (
    <>
      <TableRow className="cursor-pointer" onClick={onToggle}>
        <TableCell className="font-medium max-w-[200px] truncate">
          {review.taskId.slice(0, 8)}...
        </TableCell>
        <TableCell className="max-w-[200px] truncate">
          {review.prUrl ? (
            <a
              href={review.prUrl}
              target="_blank"
              rel="noopener noreferrer"
              className="text-blue-600 hover:underline dark:text-blue-400"
              onClick={(e) => e.stopPropagation()}
            >
              {review.prUrl.replace(/^https?:\/\/github\.com\//, "")}
            </a>
          ) : (
            <span className="text-muted-foreground">-</span>
          )}
        </TableCell>
        <TableCell>
          <Badge
            variant="secondary"
            className={cn(statusColors[review.status])}
          >
            {review.status.replace(/_/g, " ")}
          </Badge>
        </TableCell>
        <TableCell>
          {review.riskLevel ? (
            <Badge
              variant="secondary"
              className={cn(riskColors[review.riskLevel])}
            >
              {review.riskLevel}
            </Badge>
          ) : (
            <span className="text-muted-foreground">-</span>
          )}
        </TableCell>
        <TableCell>
          {review.recommendation ? (
            <Badge
              variant="secondary"
              className={cn(recommendationColors[review.recommendation])}
            >
              {review.recommendation.replace(/_/g, " ")}
            </Badge>
          ) : (
            <span className="text-muted-foreground">-</span>
          )}
        </TableCell>
        <TableCell className="text-right text-xs text-muted-foreground">
          ${review.costUsd.toFixed(2)}
        </TableCell>
        <TableCell className="text-xs text-muted-foreground">
          {new Date(review.createdAt).toLocaleString()}
        </TableCell>
      </TableRow>
      {expanded && (
        <TableRow>
          <TableCell colSpan={7} className="bg-muted/50 p-4">
            <div className="space-y-3">
              {review.summary && (
                <div>
                  <p className="text-sm font-medium">Summary</p>
                  <p className="text-sm text-muted-foreground">
                    {review.summary}
                  </p>
                </div>
              )}
              {review.findings && review.findings.length > 0 ? (
                <div>
                  <p className="text-sm font-medium mb-2">
                    Findings ({review.findings.length})
                  </p>
                  <div className="space-y-2">
                    {review.findings.map((finding, idx) => (
                      <div
                        key={idx}
                        className="rounded border bg-background p-3 text-sm"
                      >
                        <div className="flex items-center gap-2 mb-1">
                          <Badge
                            variant="secondary"
                            className={cn(
                              riskColors[finding.severity] ??
                                "bg-zinc-500/15 text-zinc-600 dark:text-zinc-400"
                            )}
                          >
                            {finding.severity}
                          </Badge>
                          <span className="font-medium">
                            {finding.category}
                          </span>
                          {finding.file && (
                            <span className="text-muted-foreground">
                              {finding.file}
                              {finding.line ? `:${finding.line}` : ""}
                            </span>
                          )}
                        </div>
                        <p className="text-muted-foreground">
                          {finding.message}
                        </p>
                        {finding.suggestion && (
                          <p className="mt-1 text-xs text-muted-foreground italic">
                            Suggestion: {finding.suggestion}
                          </p>
                        )}
                      </div>
                    ))}
                  </div>
                </div>
              ) : (
                <p className="text-sm text-muted-foreground">
                  No findings recorded.
                </p>
              )}
            </div>
          </TableCell>
        </TableRow>
      )}
    </>
  );
}
