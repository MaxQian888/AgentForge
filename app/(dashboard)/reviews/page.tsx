"use client";

import { use, useEffect, useMemo, useState } from "react";
import { useTranslations } from "next-intl";
import { FileSearch } from "lucide-react";
import { PageHeader } from "@/components/shared/page-header";
import { EmptyState } from "@/components/shared/empty-state";
import { FilterBar } from "@/components/shared/filter-bar";
import { ReviewWorkspace } from "@/components/review/review-workspace";
import { useReviewStore } from "@/lib/stores/review-store";
import { useTaskStore } from "@/lib/stores/task-store";
import { useBreadcrumbs } from "@/hooks/use-breadcrumbs";

interface ReviewsPageProps {
  searchParams: Promise<{ id?: string | string[] | undefined }>;
}

type AgeBucket = "all" | "day" | "week" | "month" | "older";

const AGE_MS: Record<Exclude<AgeBucket, "all">, number> = {
  day: 24 * 60 * 60 * 1000,
  week: 7 * 24 * 60 * 60 * 1000,
  month: 30 * 24 * 60 * 60 * 1000,
  older: Number.POSITIVE_INFINITY,
};

function matchesAge(createdAt: string, bucket: AgeBucket): boolean {
  if (bucket === "all") return true;
  const age = Date.now() - new Date(createdAt).getTime();
  if (bucket === "older") {
    return age > AGE_MS.month;
  }
  return age <= AGE_MS[bucket];
}

export default function ReviewsPage({ searchParams }: ReviewsPageProps) {
  const tc = useTranslations("common");
  useBreadcrumbs([{ label: tc("nav.group.project"), href: "/" }, { label: tc("nav.reviews") }]);
  const t = useTranslations("reviews");
  const {
    allReviews,
    allReviewsLoading,
    error,
    fetchAllReviews,
    triggerReview,
    approveReview,
    requestChanges,
    rejectReview,
  } = useReviewStore();
  const tasks = useTaskStore((state) => state.tasks);
  const [statusFilter, setStatusFilter] = useState("all");
  const [riskFilter, setRiskFilter] = useState("all");
  const [assigneeFilter, setAssigneeFilter] = useState("all");
  const [branchFilter, setBranchFilter] = useState("all");
  const [ageFilter, setAgeFilter] = useState<AgeBucket>("all");
  const [searchQuery, setSearchQuery] = useState("");
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

  const taskById = useMemo(
    () => new Map(tasks.map((task) => [task.id, task])),
    [tasks],
  );

  const assigneeOptions = useMemo(() => {
    const options = new Map<string, string>();
    options.set("__unassigned__", t("filterAssigneeUnassigned"));
    allReviews.forEach((review) => {
      const task = taskById.get(review.taskId);
      if (task?.assigneeId && task.assigneeName) {
        options.set(task.assigneeId, task.assigneeName);
      }
    });
    return Array.from(options.entries()).map(([value, label]) => ({ value, label }));
  }, [allReviews, taskById, t]);

  const branchOptions = useMemo(() => {
    const options = new Set<string>();
    allReviews.forEach((review) => {
      const task = taskById.get(review.taskId);
      if (task?.agentBranch) {
        options.add(task.agentBranch);
      }
    });
    return Array.from(options).map((branch) => ({ value: branch, label: branch }));
  }, [allReviews, taskById]);

  const filteredReviews = useMemo(() => {
    const trimmedQuery = searchQuery.trim().toLowerCase();
    return allReviews.filter((review) => {
      if (trimmedQuery) {
        const haystack = `${review.summary ?? ""} ${review.recommendation ?? ""} ${review.prUrl ?? ""}`.toLowerCase();
        const title = t("layerReview", { layer: review.layer }).toLowerCase();
        if (!haystack.includes(trimmedQuery) && !title.includes(trimmedQuery)) {
          return false;
        }
      }

      if (assigneeFilter !== "all") {
        const task = taskById.get(review.taskId);
        if (assigneeFilter === "__unassigned__") {
          if (task?.assigneeId) return false;
        } else if (task?.assigneeId !== assigneeFilter) {
          return false;
        }
      }

      if (branchFilter !== "all") {
        const task = taskById.get(review.taskId);
        if (task?.agentBranch !== branchFilter) {
          return false;
        }
      }

      if (!matchesAge(review.createdAt, ageFilter)) {
        return false;
      }

      return true;
    });
  }, [
    allReviews,
    searchQuery,
    assigneeFilter,
    branchFilter,
    ageFilter,
    taskById,
    t,
  ]);

  const handleResetFilters = () => {
    setStatusFilter("all");
    setRiskFilter("all");
    setAssigneeFilter("all");
    setBranchFilter("all");
    setAgeFilter("all");
    setSearchQuery("");
  };

  return (
    <div className="flex flex-col gap-[var(--space-section-gap)]">
      <PageHeader title={t("title")} />

      <FilterBar
        searchValue={searchQuery}
        searchPlaceholder={t("searchPlaceholder")}
        onSearch={setSearchQuery}
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
          {
            key: "assignee",
            label: t("filterAssignee"),
            value: assigneeFilter,
            onChange: setAssigneeFilter,
            options: assigneeOptions,
          },
          {
            key: "branch",
            label: t("filterBranch"),
            value: branchFilter,
            onChange: setBranchFilter,
            options: branchOptions,
          },
          {
            key: "age",
            label: t("filterAge"),
            value: ageFilter,
            onChange: (value) => setAgeFilter(value as AgeBucket),
            options: [
              { value: "day", label: t("ageOneDay") },
              { value: "week", label: t("ageOneWeek") },
              { value: "month", label: t("ageOneMonth") },
              { value: "older", label: t("ageOver") },
            ],
          },
        ]}
        onReset={handleResetFilters}
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
            key={selectedReviewId ?? "none"}
            reviews={filteredReviews}
            loading={allReviewsLoading}
            error={error}
          showTitle={false}
          selectedReviewId={selectedReviewId}
          enableBulkActions
          onTriggerReview={triggerReview}
          onApproveReview={approveReview}
          onRequestChangesReview={requestChanges}
          onRejectReview={rejectReview}
          onBlockReview={rejectReview}
        />
      )}
    </div>
  );
}
