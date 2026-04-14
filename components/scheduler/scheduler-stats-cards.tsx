"use client";

import { useTranslations } from "next-intl";
import {
  CalendarClock,
  CheckCircle2,
  XCircle,
  Activity,
  Power,
  PauseCircle,
  Timer,
  Gauge,
} from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import type { SchedulerStats } from "@/lib/stores/scheduler-store";
import { Skeleton } from "@/components/ui/skeleton";

interface SchedulerStatsCardsProps {
  stats: SchedulerStats | null;
  loading: boolean;
}

function StatCard({
  label,
  value,
  icon: Icon,
  loading,
  accent,
}: {
  label: string;
  value: string | number;
  icon: React.ElementType;
  loading: boolean;
  accent?: string;
}) {
  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between pb-2">
        <CardTitle className="text-sm font-medium text-muted-foreground">
          {label}
        </CardTitle>
        <Icon className="size-4 text-muted-foreground" />
      </CardHeader>
      <CardContent>
        {loading ? (
          <Skeleton className="h-7 w-16" />
        ) : (
          <div className={`text-2xl font-bold ${accent ?? ""}`}>{value}</div>
        )}
      </CardContent>
    </Card>
  );
}

export function SchedulerStatsCards({ stats, loading }: SchedulerStatsCardsProps) {
  const t = useTranslations("scheduler");
  const successRate =
    stats && stats.successRate24h > 0
      ? Math.round(stats.successRate24h)
      : stats && stats.totalRuns24h > 0
        ? Math.round(((stats.totalRuns24h - stats.failedRuns24h) / stats.totalRuns24h) * 100)
        : null;

  return (
    <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4 xl:grid-cols-8">
      <StatCard
        label={t("stats.totalJobs")}
        value={stats?.totalJobs ?? 0}
        icon={CalendarClock}
        loading={loading}
      />
      <StatCard
        label={t("stats.enabled")}
        value={stats?.enabledJobs ?? 0}
        icon={Power}
        loading={loading}
      />
      <StatCard
        label={t("stats.paused")}
        value={stats?.pausedJobs ?? 0}
        icon={PauseCircle}
        loading={loading}
      />
      <StatCard
        label={t("stats.activeRuns")}
        value={stats?.activeRuns ?? 0}
        icon={Activity}
        loading={loading}
        accent={stats && stats.activeRuns > 0 ? "text-blue-600 dark:text-blue-400" : undefined}
      />
      <StatCard
        label={t("stats.queueDepth")}
        value={stats?.queueDepth ?? 0}
        icon={Gauge}
        loading={loading}
      />
      <StatCard
        label={t("stats.failed24h")}
        value={stats?.failedRuns24h ?? 0}
        icon={XCircle}
        loading={loading}
        accent={stats && stats.failedRuns24h > 0 ? "text-red-600 dark:text-red-400" : undefined}
      />
      <StatCard
        label={t("stats.avgDuration")}
        value={stats?.averageDurationMs ? `${Math.round(stats.averageDurationMs)}ms` : "-"}
        icon={Timer}
        loading={loading}
      />
      <StatCard
        label={t("stats.successRate24h")}
        value={successRate != null ? `${successRate}%` : "-"}
        icon={CheckCircle2}
        loading={loading}
        accent={
          successRate != null && successRate < 80
            ? "text-red-600 dark:text-red-400"
            : successRate != null
              ? "text-green-600 dark:text-green-400"
              : undefined
        }
      />
    </div>
  );
}
