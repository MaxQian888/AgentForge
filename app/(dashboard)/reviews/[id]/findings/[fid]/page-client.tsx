"use client";

import { useParams } from "next/navigation";
import { useMemo } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Check, X, Clock } from "lucide-react";
import { useReviewStore } from "@/lib/stores/review-store";
import ReactDiffViewer, { DiffMethod } from "react-diff-viewer-continued";

export default function FindingDetailPage() {
  const params = useParams<{ id: string; fid: string }>();
  const reviewId = params.id;
  const findingId = params.fid;
  const decideFinding = useReviewStore((s) => s.decideFinding);
  const allReviews = useReviewStore((s) => s.allReviews);

  const finding = useMemo(() => {
    for (const review of allReviews) {
      if (review.id === reviewId) {
        const f = review.findings.find((fi) => fi.id === findingId);
        if (f) return f;
      }
    }
    return null;
  }, [allReviews, reviewId, findingId]);

  if (!finding) {
    return (
      <div className="p-[var(--space-page-inline)] text-center text-sm text-muted-foreground">
        Finding not found.
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-[var(--space-section-gap)] p-[var(--space-page-inline)]">
      {/* Metadata header */}
      <div className="space-y-2">
        <h1 className="text-lg font-semibold">{finding.message}</h1>
        <div className="flex items-center gap-3">
          {finding.file && (
            <span className="font-mono text-xs text-muted-foreground">
              {finding.file}
              {finding.line ? `:${finding.line}` : ""}
            </span>
          )}
          <Badge variant="secondary" className="text-xs">
            {finding.severity}
          </Badge>
          <Badge variant="secondary" className="text-xs">
            {finding.decision ?? "pending"}
          </Badge>
          {finding.sources?.map((s) => (
            <Badge key={s} variant="outline" className="text-xs">
              {s}
            </Badge>
          ))}
        </div>
      </div>

      {/* Inline diff panel */}
      {finding.suggestedPatch && (
        <div className="rounded border p-2 text-xs" data-testid="diff-panel">
          <ReactDiffViewer
            oldValue=""
            newValue={finding.suggestedPatch}
            splitView={false}
            compareMethod={DiffMethod.LINES}
          />
        </div>
      )}

      {/* Actions */}
      <div className="flex gap-2">
        <Button
          size="sm"
          variant="outline"
          onClick={() => finding.id && decideFinding(finding.id, "approve")}
        >
          <Check className="mr-1 h-4 w-4" /> Approve
        </Button>
        <Button
          size="sm"
          variant="outline"
          onClick={() => finding.id && decideFinding(finding.id, "dismiss")}
        >
          <X className="mr-1 h-4 w-4" /> Dismiss
        </Button>
        <Button
          size="sm"
          variant="outline"
          onClick={() => finding.id && decideFinding(finding.id, "defer")}
        >
          <Clock className="mr-1 h-4 w-4" /> Defer
        </Button>
      </div>

      {/* Fix runs history (Plan 2E dependency) */}
      <div className="space-y-2">
        <h2 className="text-sm font-medium">Fix Run History</h2>
        {/* TODO: Depends on Plan 2E — GET /api/v1/findings/:fid/fix-runs */}
        <p className="text-sm text-muted-foreground">No fix runs yet</p>
      </div>
    </div>
  );
}
