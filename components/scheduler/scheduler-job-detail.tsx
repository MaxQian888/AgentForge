"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { CalendarClock, Play, Power, Save } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Separator } from "@/components/ui/separator";
import { SchedulerStatusBadge } from "./scheduler-status-badge";
import { SchedulerRunHistory } from "./scheduler-run-history";
import { describeCron } from "@/lib/cron-description";
import { formatRelativeTime } from "@/lib/format-relative-time";
import type {
  SchedulerJob,
  SchedulerJobRun,
} from "@/lib/stores/scheduler-store";

interface SchedulerJobDetailProps {
  job: SchedulerJob;
  runs: SchedulerJobRun[];
  draftSchedule: string;
  actionLoading: boolean;
  onUpdateJob: (input: { enabled?: boolean; schedule?: string }) => void;
  onTriggerJob: () => void;
  onSetDraftSchedule: (schedule: string) => void;
}

function parseConfig(raw: string): Record<string, unknown> | null {
  if (!raw || raw === "{}") {
    return null;
  }
  try {
    return JSON.parse(raw) as Record<string, unknown>;
  } catch {
    return null;
  }
}

export function SchedulerJobDetail({
  job,
  runs,
  draftSchedule,
  actionLoading,
  onUpdateJob,
  onTriggerJob,
  onSetDraftSchedule,
}: SchedulerJobDetailProps) {
  const t = useTranslations("scheduler");
  const [confirmToggle, setConfirmToggle] = useState(false);
  const config = parseConfig(job.config);
  const cronDesc = describeCron(draftSchedule || job.schedule);
  const scheduleChanged = draftSchedule !== job.schedule;

  return (
    <>
      <div className="flex items-start justify-between gap-3">
        <div className="space-y-1">
          <div className="flex items-center gap-2">
            <CalendarClock className="size-4 text-muted-foreground" />
            <span className="font-semibold">{job.name}</span>
            <SchedulerStatusBadge status={job.enabled ? job.lastRunStatus : "disabled"} />
          </div>
          <p className="text-xs text-muted-foreground">{job.jobKey}</p>
        </div>
        <div className="flex gap-2">
          <Button
            variant="outline"
            size="sm"
            className="gap-1.5"
            onClick={onTriggerJob}
            disabled={actionLoading}
          >
            <Play className="size-3.5" />
            {t("jobDetail.runNow")}
          </Button>
          <Button
            variant={job.enabled ? "secondary" : "default"}
            size="sm"
            className="gap-1.5"
            onClick={() => setConfirmToggle(true)}
            disabled={actionLoading}
          >
            <Power className="size-3.5" />
            {job.enabled ? t("jobDetail.disable") : t("jobDetail.enable")}
          </Button>
        </div>
      </div>

      <Separator />

      <Tabs defaultValue="overview">
        <TabsList>
          <TabsTrigger value="overview">{t("jobDetail.tabOverview")}</TabsTrigger>
          <TabsTrigger value="history">
            {t("jobDetail.tabHistory")}
            {runs.length > 0 && (
              <span className="ml-1.5 rounded-full bg-muted px-1.5 py-0.5 text-[10px] font-medium">
                {runs.length}
              </span>
            )}
          </TabsTrigger>
          {config && <TabsTrigger value="config">{t("jobDetail.tabConfig")}</TabsTrigger>}
        </TabsList>

        <TabsContent value="overview" className="space-y-4 pt-4">
          <div className="grid grid-cols-2 gap-4 text-sm">
            <div>
              <span className="text-muted-foreground">{t("jobDetail.lastRun")}</span>
              <p className="font-medium">{formatRelativeTime(job.lastRunAt)}</p>
            </div>
            <div>
              <span className="text-muted-foreground">{t("jobDetail.nextRun")}</span>
              <p className="font-medium">
                {job.enabled && job.nextRunAt
                  ? formatRelativeTime(job.nextRunAt)
                  : t("jobDetail.notScheduled")}
              </p>
            </div>
            <div>
              <span className="text-muted-foreground">{t("jobDetail.scope")}</span>
              <p className="font-medium capitalize">{job.scope}</p>
            </div>
            <div>
              <span className="text-muted-foreground">{t("jobDetail.overlapPolicy")}</span>
              <p className="font-medium capitalize">{job.overlapPolicy}</p>
            </div>
          </div>

          {job.lastRunSummary && (
            <div className="rounded-lg border bg-muted/50 p-3 text-sm">
              <span className="text-xs font-medium text-muted-foreground">{t("jobDetail.lastSummary")}</span>
              <p className="mt-1">{job.lastRunSummary}</p>
            </div>
          )}

          {job.lastError && (
            <div className="rounded-lg border border-destructive/30 bg-destructive/5 p-3 text-sm text-destructive">
              <span className="text-xs font-medium">{t("jobDetail.lastError")}</span>
              <p className="mt-1">{job.lastError}</p>
            </div>
          )}

          <Separator />

          <div className="space-y-2">
            <label htmlFor="job-schedule" className="text-sm font-medium">
              {t("jobDetail.scheduleExpression")}
            </label>
            <div className="flex gap-2">
              <div className="flex-1 space-y-1">
                <Input
                  id="job-schedule"
                  aria-label="Schedule expression"
                  value={draftSchedule}
                  onChange={(e) => onSetDraftSchedule(e.target.value)}
                />
                {cronDesc && (
                  <p className="text-xs text-muted-foreground">{cronDesc}</p>
                )}
              </div>
              <Button
                size="sm"
                className="gap-1.5"
                onClick={() => onUpdateJob({ schedule: draftSchedule })}
                disabled={actionLoading || !scheduleChanged}
              >
                <Save className="size-3.5" />
                {t("jobDetail.save")}
              </Button>
            </div>
          </div>
        </TabsContent>

        <TabsContent value="history" className="pt-4">
          <SchedulerRunHistory runs={runs} />
        </TabsContent>

        {config && (
          <TabsContent value="config" className="pt-4">
            <pre className="max-h-64 overflow-auto rounded-lg border bg-muted/50 p-4 text-xs">
              {JSON.stringify(config, null, 2)}
            </pre>
          </TabsContent>
        )}
      </Tabs>

      <Dialog open={confirmToggle} onOpenChange={setConfirmToggle}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {job.enabled ? t("jobDetail.disableJobTitle") : t("jobDetail.enableJobTitle")}
            </DialogTitle>
            <DialogDescription>
              {job.enabled
                ? t("jobDetail.disableJobDesc", { name: job.name })
                : t("jobDetail.enableJobDesc", { name: job.name, schedule: describeCron(job.schedule) })}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setConfirmToggle(false)}>
              {t("jobDetail.cancel")}
            </Button>
            <Button
              variant={job.enabled ? "destructive" : "default"}
              onClick={() => {
                onUpdateJob({ enabled: !job.enabled });
                setConfirmToggle(false);
              }}
            >
              {job.enabled ? t("jobDetail.disable") : t("jobDetail.enable")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}

export function SchedulerJobDetailEmpty() {
  const t = useTranslations("scheduler");
  return (
    <div className="flex flex-col items-center justify-center py-12 text-center">
      <CalendarClock className="mb-3 size-10 text-muted-foreground" />
      <p className="text-sm text-muted-foreground">
        {t("jobDetail.selectJob")}
      </p>
    </div>
  );
}
