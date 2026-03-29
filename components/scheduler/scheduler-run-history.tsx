"use client";

import { useTranslations } from "next-intl";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { SchedulerStatusBadge } from "./scheduler-status-badge";
import { formatRelativeTime } from "@/lib/format-relative-time";
import { formatDuration } from "@/lib/format-duration";
import type { SchedulerJobRun } from "@/lib/stores/scheduler-store";

interface SchedulerRunHistoryProps {
  runs: SchedulerJobRun[];
}

function parseMetrics(raw: string): Record<string, unknown> | null {
  if (!raw || raw === "{}") {
    return null;
  }
  try {
    const parsed = JSON.parse(raw) as Record<string, unknown>;
    if (Object.keys(parsed).length === 0) {
      return null;
    }
    return parsed;
  } catch {
    return null;
  }
}

function MetricsDisplay({ raw }: { raw: string }) {
  const metrics = parseMetrics(raw);
  if (!metrics) {
    return <span className="text-muted-foreground">-</span>;
  }
  return (
    <div className="flex flex-wrap gap-1">
      {Object.entries(metrics).map(([key, value]) => (
        <span
          key={key}
          className="inline-flex items-center rounded-md bg-muted px-1.5 py-0.5 text-[10px] font-medium"
        >
          {key}: {String(value)}
        </span>
      ))}
    </div>
  );
}

export function SchedulerRunHistory({ runs }: SchedulerRunHistoryProps) {
  const t = useTranslations("scheduler");
  if (runs.length === 0) {
    return (
      <div className="py-8 text-center text-sm text-muted-foreground">
        {t("runHistory.noRuns")}
      </div>
    );
  }

  return (
    <TooltipProvider>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t("runHistory.colStatus")}</TableHead>
            <TableHead>{t("runHistory.colTrigger")}</TableHead>
            <TableHead>{t("runHistory.colStarted")}</TableHead>
            <TableHead>{t("runHistory.colDuration")}</TableHead>
            <TableHead>{t("runHistory.colSummary")}</TableHead>
            <TableHead>{t("runHistory.colMetrics")}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {runs.map((run) => (
            <TableRow key={run.runId}>
              <TableCell>
                <SchedulerStatusBadge status={run.status} />
              </TableCell>
              <TableCell className="text-xs capitalize">{run.triggerSource}</TableCell>
              <TableCell>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <span className="text-xs text-muted-foreground">
                      {formatRelativeTime(run.startedAt)}
                    </span>
                  </TooltipTrigger>
                  <TooltipContent>
                    {new Date(run.startedAt).toLocaleString()}
                  </TooltipContent>
                </Tooltip>
              </TableCell>
              <TableCell className="text-xs font-mono">
                {formatDuration(run.durationMs)}
              </TableCell>
              <TableCell className="max-w-[200px] truncate text-xs">
                {run.errorMessage ? (
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <span className="text-destructive">{run.errorMessage}</span>
                    </TooltipTrigger>
                    <TooltipContent className="max-w-xs">
                      {run.errorMessage}
                    </TooltipContent>
                  </Tooltip>
                ) : (
                  run.summary || <span className="text-muted-foreground">-</span>
                )}
              </TableCell>
              <TableCell className="text-xs">
                <MetricsDisplay raw={run.metrics} />
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </TooltipProvider>
  );
}
