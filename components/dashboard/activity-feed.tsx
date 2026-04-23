"use client";

import { useMemo, useState } from "react";
import { useTranslations } from "next-intl";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { StatusDot } from "@/components/shared/status-dot";

interface ActivityEvent {
  id: string;
  type: string;
  title: string;
  timestamp: string;
  status: string;
}

interface ActivityFeedProps {
  events: ActivityEvent[];
}

type ActivityTypeFilter = "all" | "task" | "review" | "agent" | "system";
type ActivityTimeRange = "all" | "last24h" | "last7d";

function relativeTime(
  timestamp: string,
  t: ReturnType<typeof useTranslations>,
): string {
  const diff = Date.now() - new Date(timestamp).getTime();
  const seconds = Math.floor(diff / 1000);
  if (seconds < 60) return t("activityFeed.timeAgo.seconds", { seconds });
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return t("activityFeed.timeAgo.minutes", { minutes });
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return t("activityFeed.timeAgo.hours", { hours });
  const days = Math.floor(hours / 24);
  return t("activityFeed.timeAgo.days", { days });
}

function categorizeActivityType(type: string): ActivityTypeFilter {
  if (type.includes("task")) return "task";
  if (type.includes("review")) return "review";
  if (type.includes("agent")) return "agent";
  return "system";
}

function matchesTimeRange(timestamp: string, range: ActivityTimeRange) {
  if (range === "all") {
    return true;
  }

  const diff = Date.now() - new Date(timestamp).getTime();
  const dayMs = 24 * 60 * 60 * 1000;

  if (range === "last24h") {
    return diff <= dayMs;
  }

  return diff <= 7 * dayMs;
}

export function ActivityFeed({ events }: ActivityFeedProps) {
  const t = useTranslations("dashboard");
  const [typeFilter, setTypeFilter] = useState<ActivityTypeFilter>("all");
  const [timeRange, setTimeRange] = useState<ActivityTimeRange>("all");
  const filtered = useMemo(
    () =>
      events.filter((event) => {
        const matchesType =
          typeFilter === "all" ||
          categorizeActivityType(event.type) === typeFilter;
        const matchesRange = matchesTimeRange(event.timestamp, timeRange);
        return matchesType && matchesRange;
      }),
    [events, timeRange, typeFilter],
  );
  const visible = filtered.slice(0, 8);

  return (
    <Card>
      <CardHeader className="pb-3">
        <div className="flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
          <div className="space-y-1">
            <CardTitle className="text-sm font-medium">
              {t("activityFeed.title")}
            </CardTitle>
            <p className="text-xs text-muted-foreground">
              {t("activityFeed.count", { count: filtered.length })}
            </p>
          </div>
          <div className="flex flex-wrap gap-3">
            <div className="flex flex-col gap-1 text-xs text-muted-foreground">
              <span>{t("activityFeed.typeLabel")}</span>
              <Select value={typeFilter} onValueChange={(v) => setTypeFilter(v as ActivityTypeFilter)}>
                <SelectTrigger className="h-8 text-sm">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">{t("activityFeed.type.all")}</SelectItem>
                  <SelectItem value="task">{t("activityFeed.type.task")}</SelectItem>
                  <SelectItem value="review">{t("activityFeed.type.review")}</SelectItem>
                  <SelectItem value="agent">{t("activityFeed.type.agent")}</SelectItem>
                  <SelectItem value="system">{t("activityFeed.type.system")}</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="flex flex-col gap-1 text-xs text-muted-foreground">
              <span>{t("activityFeed.timeRangeLabel")}</span>
              <Select value={timeRange} onValueChange={(v) => setTimeRange(v as ActivityTimeRange)}>
                <SelectTrigger className="h-8 text-sm">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">{t("activityFeed.timeRange.all")}</SelectItem>
                  <SelectItem value="last24h">{t("activityFeed.timeRange.last24h")}</SelectItem>
                  <SelectItem value="last7d">{t("activityFeed.timeRange.last7d")}</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
        </div>
      </CardHeader>
      <CardContent>
        {visible.length === 0 ? (
          <p className="text-sm text-muted-foreground">
            {t("activityFeed.empty")}
          </p>
        ) : (
          <div className="space-y-2">
            {visible.map((event) => (
              <div
                key={event.id}
                className="flex items-center gap-2 text-sm"
              >
                <StatusDot status={event.status} size="sm" />
                <span className="min-w-0 flex-1 truncate">{event.title}</span>
                <span className="shrink-0 text-xs text-muted-foreground">
                  {relativeTime(event.timestamp, t)}
                </span>
              </div>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
