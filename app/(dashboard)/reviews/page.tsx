"use client";

import { use, useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { FileSearch } from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { ReviewWorkspace } from "@/components/review/review-workspace";
import { useReviewStore } from "@/lib/stores/review-store";

interface ReviewsPageProps {
  searchParams: Promise<{ id?: string | string[] | undefined }>;
}

export default function ReviewsPage({ searchParams }: ReviewsPageProps) {
  const t = useTranslations("reviews");
  const {
    allReviews,
    allReviewsLoading,
    error,
    fetchAllReviews,
    triggerReview,
    approveReview,
    requestChanges,
  } = useReviewStore();
  const [statusFilter, setStatusFilter] = useState("all");
  const [riskFilter, setRiskFilter] = useState("all");
  const resolvedSearchParams = use(searchParams);
  const selectedReviewId = Array.isArray(resolvedSearchParams.id)
    ? resolvedSearchParams.id[0] ?? null
    : resolvedSearchParams.id ?? null;

  useEffect(() => {
    fetchAllReviews({
      status: statusFilter === "all" ? undefined : statusFilter,
      riskLevel: riskFilter === "all" ? undefined : riskFilter,
    });
  }, [fetchAllReviews, statusFilter, riskFilter]);

  return (
    <div className="flex flex-col gap-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">{t("title")}</h1>
      </div>

      <div className="flex items-center gap-4">
        <div className="flex items-center gap-2">
          <span className="text-sm text-muted-foreground">{t("filterStatus")}</span>
          <Select value={statusFilter} onValueChange={setStatusFilter}>
            <SelectTrigger className="w-[160px]">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{t("all")}</SelectItem>
              <SelectItem value="pending">{t("statusPending")}</SelectItem>
              <SelectItem value="in_progress">{t("statusInProgress")}</SelectItem>
              <SelectItem value="completed">{t("statusCompleted")}</SelectItem>
              <SelectItem value="failed">{t("statusFailed")}</SelectItem>
              <SelectItem value="pending_human">{t("statusPendingHuman")}</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div className="flex items-center gap-2">
          <span className="text-sm text-muted-foreground">{t("filterRiskLevel")}</span>
          <Select value={riskFilter} onValueChange={setRiskFilter}>
            <SelectTrigger className="w-[140px]">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{t("all")}</SelectItem>
              <SelectItem value="critical">{t("riskCritical")}</SelectItem>
              <SelectItem value="high">{t("riskHigh")}</SelectItem>
              <SelectItem value="medium">{t("riskMedium")}</SelectItem>
              <SelectItem value="low">{t("riskLow")}</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </div>

      {allReviewsLoading && allReviews.length === 0 ? (
        <p className="text-muted-foreground">{t("loading")}</p>
      ) : allReviews.length === 0 ? (
        <Card>
          <CardContent className="py-12 text-center">
            <FileSearch className="mx-auto mb-4 size-12 text-muted-foreground" />
            <p className="text-muted-foreground">{t("emptyState")}</p>
          </CardContent>
        </Card>
      ) : (
        <ReviewWorkspace
          reviews={allReviews}
          loading={allReviewsLoading}
          error={error}
          showTitle={false}
          selectedReviewId={selectedReviewId}
          onTriggerReview={triggerReview}
          onApproveReview={approveReview}
          onRequestChangesReview={requestChanges}
        />
      )}
    </div>
  );
}
