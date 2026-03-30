"use client";

import { useTranslations } from "next-intl";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
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

function relativeTime(timestamp: string): string {
  const diff = Date.now() - new Date(timestamp).getTime();
  const seconds = Math.floor(diff / 1000);
  if (seconds < 60) return `${seconds}s ago`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

export function ActivityFeed({ events }: ActivityFeedProps) {
  const t = useTranslations("dashboard");
  const visible = events.slice(0, 8);

  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="text-sm font-medium">
          {t("activityFeed.title")}
        </CardTitle>
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
                  {relativeTime(event.timestamp)}
                </span>
              </div>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
