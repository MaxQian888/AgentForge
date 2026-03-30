"use client";

import { use, useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { FileSearch } from "lucide-react";
import { PageHeader } from "@/components/shared/page-header";
import { EmptyState } from "@/components/shared/empty-state";
import { FilterBar } from "@/components/shared/filter-bar";
import { ReviewWorkspace } from "@/components/review/review-workspace";
import { useReviewStore } from "@/lib/stores/review-store";
import { useBreadcrumbs } from "@/hooks/use-breadcrumbs";

interface ReviewsPageProps {
  searchParams: Promise<{ id?: string | string[] | undefined }>;
}

export default function ReviewsPage({ searchParams }: ReviewsPageProps) {
  useBreadcrumbs([{ label: "Project", href: "/" }, { label: "Reviews" }]);
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
      <PageHeader title={t("title")} />

      <FilterBar
        filters={[
          {
            key: "status",
            label: t("filterStatus"),
            value: statusFilter,
            onChange: setStatusFilter,
            options: [
              { value: "pending", label: t("statusPending") },
              { value: "in_progress", label: t("statusInProgress") },
              { value: "completed", label: t("statusCompleted") },
              { value: "failed", label: t("statusFailed") },
              { value: "pending_human", label: t("statusPendingHuman") },
            ],
          },
          {
            key: "riskLevel",
            label: t("filterRiskLevel"),
            value: riskFilter,
            onChange: setRiskFilter,
            options: [
              { value: "critical", label: t("riskCritical") },
              { value: "high", label: t("riskHigh") },
              { value: "medium", label: t("riskMedium") },
              { value: "low", label: t("riskLow") },
            ],
          },
        ]}
        onReset={() => {
          setStatusFilter("all");
          setRiskFilter("all");
        }}
      />

      {allReviewsLoading && allReviews.length === 0 ? (
        <p className="text-muted-foreground">{t("loading")}</p>
      ) : allReviews.length === 0 ? (
        <EmptyState
          icon={FileSearch}
          title={t("emptyState")}
        />
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
