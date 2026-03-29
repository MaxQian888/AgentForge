"use client";

import { useTranslations } from "next-intl";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import type { ReviewDTO } from "@/lib/stores/review-store";
import { ReviewDecisionActions } from "./review-decision-actions";
import {
  getReviewRecommendationLabel,
  ReviewRiskBadge,
  ReviewStatusBadge,
} from "./review-copy";

interface ReviewListProps {
  reviews: ReviewDTO[];
  onSelect: (review: ReviewDTO) => void;
  onApprove?: (id: string, comment?: string) => void | Promise<void>;
  onRequestChanges?: (id: string, comment?: string) => void | Promise<void>;
}

export function ReviewList({
  reviews,
  onSelect,
  onApprove,
  onRequestChanges,
}: ReviewListProps) {
  const t = useTranslations("reviews");

  if (reviews.length === 0) {
    return (
      <p className="py-6 text-center text-sm text-muted-foreground">
        {t("noReviewsYet")}
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
                {t("layerReview", { layer: review.layer })}
              </CardTitle>
              <div className="flex items-center gap-1.5">
                <ReviewStatusBadge status={review.status} t={t} />
                <ReviewRiskBadge riskLevel={review.riskLevel} t={t} />
              </div>
            </div>
          </CardHeader>
          <CardContent className="p-3 pt-1">
            <p className="mb-2 line-clamp-2 text-xs text-muted-foreground">
              {review.summary || t("noSummary")}
            </p>
            <div className="flex items-center justify-between gap-3">
              <div className="flex items-center gap-2 text-xs text-muted-foreground">
                <span>{getReviewRecommendationLabel(t, review.recommendation)}</span>
                <span>${review.costUsd.toFixed(2)}</span>
                <span>{new Date(review.createdAt).toLocaleDateString()}</span>
              </div>
              {review.status === "pending_human" ? (
                <div className="min-w-[220px]" onClick={(event) => event.stopPropagation()}>
                  <ReviewDecisionActions
                    reviewId={review.id}
                    onApprove={onApprove}
                    onRequestChanges={onRequestChanges}
                    compact
                  />
                </div>
              ) : null}
            </div>
          </CardContent>
        </Card>
      ))}
    </div>
  );
}
