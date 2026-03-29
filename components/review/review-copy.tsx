"use client";

import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";

type TranslationFn = (key: string, values?: Record<string, string | number>) => string;

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
  info: "bg-blue-500/15 text-blue-700 dark:text-blue-400",
};

const recommendationColors: Record<string, string> = {
  approve: "bg-emerald-500/15 text-emerald-700 dark:text-emerald-400",
  request_changes: "bg-amber-500/15 text-amber-700 dark:text-amber-400",
  reject: "bg-red-500/15 text-red-700 dark:text-red-400",
};

const statusLabelKeys: Record<string, string> = {
  pending: "statusPending",
  in_progress: "statusInProgress",
  completed: "statusCompleted",
  failed: "statusFailed",
  pending_human: "statusPendingHuman",
};

const riskLabelKeys: Record<string, string> = {
  critical: "riskCritical",
  high: "riskHigh",
  medium: "riskMedium",
  low: "riskLow",
  info: "riskInfo",
};

const recommendationLabelKeys: Record<string, string> = {
  approve: "recommendationApprove",
  request_changes: "recommendationRequestChanges",
  reject: "recommendationReject",
};

export function getReviewStatusLabel(t: TranslationFn, status: string): string {
  return t(statusLabelKeys[status] ?? "statusUnknown");
}

export function getReviewRiskLabel(t: TranslationFn, riskLevel: string): string {
  return t(riskLabelKeys[riskLevel] ?? "riskUnknown");
}

export function getReviewRecommendationLabel(
  t: TranslationFn,
  recommendation: string,
): string {
  return t(recommendationLabelKeys[recommendation] ?? "recommendationUnknown");
}

export function ReviewStatusBadge({
  status,
  t,
  className,
}: {
  status: string;
  t: TranslationFn;
  className?: string;
}) {
  return (
    <Badge
      variant="secondary"
      className={cn("text-[11px]", statusColors[status] ?? "", className)}
    >
      {getReviewStatusLabel(t, status)}
    </Badge>
  );
}

export function ReviewRiskBadge({
  riskLevel,
  t,
  className,
}: {
  riskLevel: string;
  t: TranslationFn;
  className?: string;
}) {
  return (
    <Badge
      variant="secondary"
      className={cn("text-[11px]", riskColors[riskLevel] ?? "", className)}
    >
      {getReviewRiskLabel(t, riskLevel)}
    </Badge>
  );
}

export function ReviewRecommendationBadge({
  recommendation,
  t,
  className,
}: {
  recommendation: string;
  t: TranslationFn;
  className?: string;
}) {
  return (
    <Badge
      variant="secondary"
      className={cn("text-[11px]", recommendationColors[recommendation] ?? "", className)}
    >
      {getReviewRecommendationLabel(t, recommendation)}
    </Badge>
  );
}
