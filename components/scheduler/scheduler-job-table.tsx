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
import { CalendarClock } from "lucide-react";
import { SchedulerStatusBadge } from "./scheduler-status-badge";
import { describeCron } from "@/lib/cron-description";
import { formatRelativeTime } from "@/lib/format-relative-time";
import type { SchedulerJob } from "@/lib/stores/scheduler-store";
import { Skeleton } from "@/components/ui/skeleton";

interface SchedulerJobTableProps {
  jobs: SchedulerJob[];
  selectedJobKey: string | null;
  loading: boolean;
  onSelectJob: (jobKey: string) => void;
}

export function SchedulerJobTable({
  jobs,
  selectedJobKey,
  loading,
  onSelectJob,
}: SchedulerJobTableProps) {
  const t = useTranslations("scheduler");
  if (loading && jobs.length === 0) {
    return (
      <div className="flex flex-col gap-3 p-4">
        {Array.from({ length: 4 }, (_, i) => (
          <div key={i} className="flex items-center gap-4">
            <Skeleton className="h-10 w-full" />
          </div>
        ))}
      </div>
    );
  }

  if (jobs.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center py-12 text-center">
        <CalendarClock className="mx-auto mb-4 size-12 text-muted-foreground" />
        <p className="text-muted-foreground">{t("jobTable.noJobs")}</p>
      </div>
    );
  }

  return (
    <TooltipProvider>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t("jobTable.colJob")}</TableHead>
            <TableHead>{t("jobTable.colSchedule")}</TableHead>
            <TableHead>{t("jobTable.colStatus")}</TableHead>
            <TableHead>{t("jobTable.colNextRun")}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {jobs.map((job) => (
            <TableRow
              key={job.jobKey}
              className="cursor-pointer"
              data-state={selectedJobKey === job.jobKey ? "selected" : undefined}
              onClick={() => onSelectJob(job.jobKey)}
            >
              <TableCell>
                <div className="font-medium">{job.name}</div>
                <div className="text-xs text-muted-foreground">{job.jobKey}</div>
              </TableCell>
              <TableCell>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <span className="cursor-help font-mono text-xs">
                      {job.schedule}
                    </span>
                  </TooltipTrigger>
                  <TooltipContent>{describeCron(job.schedule)}</TooltipContent>
                </Tooltip>
              </TableCell>
              <TableCell>
                <div className="flex flex-col gap-1">
                  <SchedulerStatusBadge status={job.lastRunStatus} />
                  {!job.enabled && (
                    <span className="text-xs text-muted-foreground">{t("jobTable.disabled")}</span>
                  )}
                </div>
              </TableCell>
              <TableCell>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <span className="text-sm text-muted-foreground">
                      {job.nextRunAt ? formatRelativeTime(job.nextRunAt) : t("jobTable.na")}
                    </span>
                  </TooltipTrigger>
                  <TooltipContent>
                    {job.nextRunAt ? new Date(job.nextRunAt).toLocaleString() : t("jobTable.notScheduled")}
                  </TooltipContent>
                </Tooltip>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </TooltipProvider>
  );
}
