"use client";

import { useMemo } from "react";
import { useTranslations } from "next-intl";
import { Label } from "@/components/ui/label";
import { Separator } from "@/components/ui/separator";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { ReviewFindingsTable } from "./review-findings-table";
import { ReviewDecisionActions } from "./review-decision-actions";
import {
  ReviewRecommendationBadge,
  ReviewRiskBadge,
  ReviewStatusBadge,
  getReviewStatusLabel,
} from "./review-copy";
import type { ReviewDTO } from "@/lib/stores/review-store";

interface ReviewDetailPanelProps {
  review: ReviewDTO;
  onApprove?: (id: string, comment?: string) => void | Promise<void>;
  onRequestChanges?: (id: string, comment?: string) => void | Promise<void>;
  onReject?: (id: string, reason: string, comment?: string) => void | Promise<void>;
  onBlock?: (id: string, reason: string, comment?: string) => void | Promise<void>;
}

export function ReviewDetailPanel({
  review,
  onApprove,
  onRequestChanges,
  onReject,
  onBlock,
}: ReviewDetailPanelProps) {
  const t = useTranslations("reviews");
  const changedFileCount = review.executionMetadata?.changedFiles?.length ?? 0;
  const executionResults = useMemo(
    () => review.executionMetadata?.results ?? [],
    [review.executionMetadata?.results],
  );
  const decisions = useMemo(
    () => review.executionMetadata?.decisions ?? [],
    [review.executionMetadata?.decisions],
  );
  const comments = useMemo(
    () => decisions.filter((decision) => Boolean(decision.comment?.trim())),
    [decisions],
  );
  const hasExecutionMetadata = useMemo(() => {
    return (
      Boolean(review.executionMetadata?.triggerEvent) ||
      changedFileCount > 0 ||
      executionResults.length > 0
    );
  }, [review.executionMetadata?.triggerEvent, changedFileCount, executionResults.length]);
  const hasActions = Boolean(onApprove || onRequestChanges || onReject || onBlock);

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center justify-between">
        <h3 className="text-base font-semibold">{t("layerReview", { layer: review.layer })}</h3>
        <div className="flex items-center gap-1.5">
          <ReviewStatusBadge status={review.status} t={t} className="text-xs" />
          <ReviewRiskBadge riskLevel={review.riskLevel} t={t} className="text-xs" />
          <ReviewRecommendationBadge
            recommendation={review.recommendation}
            t={t}
            className="text-xs"
          />
        </div>
      </div>

      <div>
        <Label className="text-xs font-medium text-muted-foreground">{t("summary")}</Label>
        <p className="mt-1 text-sm">{review.summary || t("noSummary")}</p>
      </div>

      <div className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground">
        <span>{t("detailPrLabel", { value: review.prUrl || `#${review.prNumber}` })}</span>
        <span>{t("detailCostLabel", { value: review.costUsd.toFixed(2) })}</span>
        <span>{t("detailStatusLabel", { value: getReviewStatusLabel(t, review.status) })}</span>
        <span>{t("detailUpdatedLabel", { value: new Date(review.updatedAt).toLocaleString() })}</span>
      </div>

      <Tabs defaultValue="overview" className="w-full">
        <TabsList>
          <TabsTrigger value="overview">{t("detailsOverviewTab")}</TabsTrigger>
          <TabsTrigger value="history">{t("detailsHistoryTab")}</TabsTrigger>
          <TabsTrigger value="comments">{t("detailsCommentsTab")}</TabsTrigger>
        </TabsList>

        <TabsContent value="overview" className="flex flex-col gap-4 pt-3">
          {hasExecutionMetadata ? (
            <>
              <details className="rounded-md border p-3">
                <summary className="cursor-pointer text-sm font-medium">
                  {t("executionDetails")}
                </summary>
                <div className="mt-3 space-y-2 text-xs text-muted-foreground">
                  {review.executionMetadata?.triggerEvent ? (
                    <div>
                      <span className="font-medium text-foreground">{t("executionTrigger")}:</span>{" "}
                      {review.executionMetadata.triggerEvent}
                    </div>
                  ) : null}
                  <div>
                    <span className="font-medium text-foreground">{t("executionChangedFiles")}:</span>{" "}
                    {changedFileCount}
                  </div>
                  {executionResults.length > 0 ? (
                    <div className="space-y-1">
                      <div className="font-medium text-foreground">{t("executionResults")}</div>
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
                  ) : null}
                </div>
              </details>
              <Separator />
            </>
          ) : null}

          <div>
            <Label className="text-xs font-medium text-muted-foreground">
              {t("findingsCount", { count: review.findings?.length ?? 0 })}
            </Label>
            <div className="mt-2">
              <ReviewFindingsTable findings={review.findings ?? []} />
            </div>
          </div>
        </TabsContent>

        <TabsContent
          value="history"
          className="flex flex-col gap-2 pt-3"
          data-testid="review-history-tab"
        >
          {decisions.length === 0 ? (
            <p className="py-6 text-center text-xs text-muted-foreground">
              {t("historyEmpty")}
            </p>
          ) : (
            <div className="space-y-2">
              {decisions.map((decision, index) => (
                <div
                  key={`${decision.timestamp}-${index}`}
                  className="rounded border p-2"
                >
                  <div className="text-xs font-medium">
                    {decision.actor} - {decision.action}
                  </div>
                  <div className="text-xs text-muted-foreground">
                    {new Date(decision.timestamp).toLocaleString()}
                  </div>
                  {decision.comment ? (
                    <p className="mt-1 text-xs text-muted-foreground">
                      {decision.comment}
                    </p>
                  ) : null}
                </div>
              ))}
            </div>
          )}
        </TabsContent>

        <TabsContent
          value="comments"
          className="flex flex-col gap-2 pt-3"
          data-testid="review-comments-tab"
        >
          {comments.length === 0 ? (
            <p className="py-6 text-center text-xs text-muted-foreground">
              {t("commentsEmpty")}
            </p>
          ) : (
            <div className="space-y-2">
              {comments.map((decision, index) => (
                <div
                  key={`${decision.timestamp}-${index}`}
                  className="rounded border p-2"
                >
                  <div className="text-xs font-medium">
                    {decision.actor} ({decision.action})
                  </div>
                  <div className="text-xs text-muted-foreground">
                    {new Date(decision.timestamp).toLocaleString()}
                  </div>
                  <p className="mt-1 text-sm">{decision.comment}</p>
                </div>
              ))}
            </div>
          )}
        </TabsContent>
      </Tabs>

      {review.status === "pending_human" && hasActions ? (
        <>
          <Separator />
          <ReviewDecisionActions
            reviewId={review.id}
            onApprove={onApprove}
            onRequestChanges={onRequestChanges}
            onReject={onReject}
            onBlock={onBlock}
          />
        </>
      ) : null}
    </div>
  );
}
