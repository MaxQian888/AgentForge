"use client";

import { useEffect, useState } from "react";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import { useTranslations } from "next-intl";
import { ChevronDown, ChevronRight } from "lucide-react";
import {
  usePluginStore,
  type PluginEventType,
  type PluginEventRecord,
} from "@/lib/stores/plugin-store";

interface PluginEventTimelineProps {
  pluginId: string;
}

const eventTypeColors: Record<string, string> = {
  installed: "bg-blue-500/15 text-blue-700 dark:text-blue-400",
  enabled: "bg-blue-500/15 text-blue-700 dark:text-blue-400",
  disabled: "bg-blue-500/15 text-blue-700 dark:text-blue-400",
  activated: "bg-blue-500/15 text-blue-700 dark:text-blue-400",
  activating: "bg-blue-500/15 text-blue-700 dark:text-blue-400",
  deactivated: "bg-blue-500/15 text-blue-700 dark:text-blue-400",
  mcp_discovery: "bg-purple-500/15 text-purple-700 dark:text-purple-400",
  mcp_interaction: "bg-purple-500/15 text-purple-700 dark:text-purple-400",
  health: "bg-green-500/15 text-green-700 dark:text-green-400",
  restarted: "bg-green-500/15 text-green-700 dark:text-green-400",
  failed: "bg-red-500/15 text-red-700 dark:text-red-400",
  uninstalled: "bg-red-500/15 text-red-700 dark:text-red-400",
};

const eventSourceColors: Record<string, string> = {
  "control-plane": "bg-zinc-500/15 text-zinc-600 dark:text-zinc-400",
  "ts-bridge": "bg-cyan-500/15 text-cyan-700 dark:text-cyan-400",
  "go-runtime": "bg-orange-500/15 text-orange-700 dark:text-orange-400",
  operator: "bg-violet-500/15 text-violet-700 dark:text-violet-400",
};

function getEventColor(eventType: PluginEventType): string {
  return eventTypeColors[eventType] ?? "bg-zinc-500/15 text-zinc-600 dark:text-zinc-400";
}

function getSourceColor(source: string): string {
  return eventSourceColors[source] ?? "bg-zinc-500/15 text-zinc-600 dark:text-zinc-400";
}

export function PluginEventTimeline({ pluginId }: PluginEventTimelineProps) {
  const t = useTranslations("plugins");
  const events = usePluginStore((s) => s.events[pluginId]);
  const fetchEvents = usePluginStore((s) => s.fetchEvents);
  const [expandedIds, setExpandedIds] = useState<Set<string>>(new Set());
  const [limit, setLimit] = useState(50);

  useEffect(() => {
    if (!events) {
      void fetchEvents(pluginId, limit);
    }
  }, [pluginId, events, fetchEvents, limit]);

  const handleLimitChange = (newLimit: number) => {
    setLimit(newLimit);
    void fetchEvents(pluginId, newLimit);
  };

  const toggleExpanded = (id: string) => {
    setExpandedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  };

  const formatTimestamp = (ts?: string): string => {
    if (!ts) return t("eventTimeline.unknownTime");
    try {
      return new Date(ts).toLocaleString();
    } catch {
      return ts;
    }
  };

  const sortedEvents: PluginEventRecord[] = events
    ? [...events].sort((a, b) => {
        const ta = a.created_at ?? "";
        const tb = b.created_at ?? "";
        return tb.localeCompare(ta);
      })
    : [];

  return (
    <div className="flex flex-col gap-3">
      <div className="flex items-center justify-between">
        <p className="text-sm font-medium">{t("eventTimeline.title")}</p>
        <select
          className="h-8 rounded-md border bg-background px-2 text-xs"
          value={limit}
          onChange={(e) => handleLimitChange(Number(e.target.value))}
        >
          <option value={25}>{t("eventTimeline.events25")}</option>
          <option value={50}>{t("eventTimeline.events50")}</option>
          <option value={100}>{t("eventTimeline.events100")}</option>
        </select>
      </div>

      {sortedEvents.length === 0 ? (
        <div className="rounded-md border border-dashed px-4 py-8 text-center text-sm text-muted-foreground">
          {t("eventTimeline.noEvents")}
        </div>
      ) : (
        <div className="flex flex-col gap-2">
          {sortedEvents.map((event) => {
            const isExpanded = expandedIds.has(event.id);
            const hasPayload =
              event.payload && Object.keys(event.payload).length > 0;

            return (
              <div
                key={event.id}
                className="rounded-lg border border-border/60 p-3 text-sm"
              >
                <div
                  className={cn(
                    "flex items-start gap-2",
                    hasPayload && "cursor-pointer",
                  )}
                  onClick={() => hasPayload && toggleExpanded(event.id)}
                >
                  {hasPayload ? (
                    isExpanded ? (
                      <ChevronDown className="mt-0.5 size-3.5 shrink-0 text-muted-foreground" />
                    ) : (
                      <ChevronRight className="mt-0.5 size-3.5 shrink-0 text-muted-foreground" />
                    )
                  ) : (
                    <div className="size-3.5 shrink-0" />
                  )}

                  <div className="flex flex-1 flex-col gap-1.5">
                    <div className="flex flex-wrap items-center gap-1.5">
                      <Badge
                        variant="secondary"
                        className={cn("text-xs", getEventColor(event.event_type))}
                      >
                        {event.event_type}
                      </Badge>
                      <Badge
                        variant="secondary"
                        className={cn("text-xs", getSourceColor(event.event_source))}
                      >
                        {event.event_source}
                      </Badge>
                    </div>

                    {event.summary ? (
                      <p className="text-muted-foreground">{event.summary}</p>
                    ) : null}

                    <p className="text-xs text-muted-foreground/70">
                      {formatTimestamp(event.created_at)}
                    </p>
                  </div>
                </div>

                {isExpanded && hasPayload ? (
                  <pre className="mt-2 overflow-x-auto rounded-md bg-muted/40 p-2 text-xs">
                    {JSON.stringify(event.payload, null, 2)}
                  </pre>
                ) : null}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
