"use client";

import { useTranslations } from "next-intl";
import type { TaskDependencySummary } from "@/lib/tasks/task-dependencies";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import type {
  TaskCostSummary,
  TaskHealthCounts,
} from "@/lib/tasks/task-context-rail";

export interface TaskProgressSummaryProps {
  counts: TaskHealthCounts;
  dependencySummary: TaskDependencySummary;
  costSummary: TaskCostSummary;
  realtimeState: "live" | "degraded";
}

export function TaskProgressSummary({
  counts,
  dependencySummary,
  costSummary,
  realtimeState,
}: TaskProgressSummaryProps) {
  const t = useTranslations("tasks");
  return (
    <Card>
      <CardHeader className="pb-3">
        <div className="flex items-center justify-between gap-3">
          <CardTitle className="text-base">{t("progress.title")}</CardTitle>
          <Badge variant={realtimeState === "live" ? "secondary" : "outline"}>
            {realtimeState === "live" ? t("progress.realtimeLive") : t("progress.realtimeDegraded")}
          </Badge>
        </div>
      </CardHeader>
      <CardContent className="flex flex-col gap-2 text-sm">
        <div>{t("progress.healthy", { count: counts.healthy })}</div>
        <div>{t("progress.warning", { count: counts.warning })}</div>
        <div>{t("progress.stalled", { count: counts.stalled })}</div>
        <div>{t("progress.unscheduled", { count: counts.unscheduled })}</div>
        <div>{t("progress.blocked", { count: dependencySummary.blocked })}</div>
        <div>{t("progress.readyToUnblock", { count: dependencySummary.readyToUnblock })}</div>
        <div>
          {t("progress.taskSpend", { spent: costSummary.totalSpentUsd.toFixed(2), budget: costSummary.totalBudgetUsd.toFixed(2) })}
        </div>
        <div>
          {t("progress.activeRuns", { count: costSummary.activeRunCount, cost: costSummary.activeRunCostUsd.toFixed(2), budget: costSummary.activeRunBudgetUsd.toFixed(2) })}
        </div>
        <div>
          {t("progress.budgetedTasks", { budgeted: costSummary.budgetedTaskCount, over: costSummary.overBudgetTaskCount })}
        </div>
        {realtimeState === "degraded" ? (
          <div className="text-muted-foreground">{t("progress.realtimeUnavailable")}</div>
        ) : null}
      </CardContent>
    </Card>
  );
}
