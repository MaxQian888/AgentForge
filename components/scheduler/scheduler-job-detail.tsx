"use client";

import { useMemo, useState } from "react";
import { useTranslations } from "next-intl";
import {
  CalendarClock,
  Pause,
  Play,
  RotateCcw,
  Save,
  ShieldAlert,
  Trash2,
  XCircle,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Separator } from "@/components/ui/separator";
import { SchedulerStatusBadge } from "./scheduler-status-badge";
import { SchedulerRunHistory } from "./scheduler-run-history";
import { describeCron } from "@/lib/cron-description";
import { formatRelativeTime } from "@/lib/format-relative-time";
import type {
  SchedulerJob,
  SchedulerJobAction,
  SchedulerJobRun,
  SchedulerRunHistoryFilters,
} from "@/lib/stores/scheduler-store";

interface SchedulerJobDetailProps {
  job: SchedulerJob;
  runs: SchedulerJobRun[];
  draftSchedule: string;
  actionLoading: boolean;
  onUpdateJob: (input: { enabled?: boolean; schedule?: string }) => void;
  onTriggerJob: () => void;
  onPauseJob: () => void;
  onResumeJob: () => void;
  onCancelJob: () => void;
  onCleanupRuns: () => void;
  onFetchRuns: (filters?: SchedulerRunHistoryFilters) => void;
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

function supportFor(job: SchedulerJob, action: SchedulerJobAction) {
  return job.supportedActions?.find((item) => item.action === action);
}

export function SchedulerJobDetail({
  job,
  runs,
  draftSchedule,
  actionLoading,
  onUpdateJob,
  onTriggerJob,
  onPauseJob,
  onResumeJob,
  onCancelJob,
  onCleanupRuns,
  onFetchRuns,
  onSetDraftSchedule,
}: SchedulerJobDetailProps) {
  const t = useTranslations("scheduler");
  const [statusFilter, setStatusFilter] = useState("");
  const [triggerFilter, setTriggerFilter] = useState("");
  const config = parseConfig(job.config);
  const cronDesc = describeCron(draftSchedule || job.schedule);
  const scheduleChanged = draftSchedule !== job.schedule;
  const triggerSupport = supportFor(job, "trigger");
  const pauseSupport = supportFor(job, "pause");
  const resumeSupport = supportFor(job, "resume");
  const cancelSupport = supportFor(job, "cancel");
  const cleanupSupport = supportFor(job, "cleanup");
  const detailStatus = job.controlState === "paused"
    ? "paused"
    : job.activeRun?.status ?? job.lastRunStatus;
  const unsupportedMessages = useMemo(
    () =>
      (job.supportedActions ?? [])
        .filter((action) => !action.enabled && action.reason)
        .map((action) => `${action.action}: ${action.reason}`),
    [job.supportedActions],
  );

  return (
    <>
      <div className="flex items-start justify-between gap-3">
        <div className="space-y-1">
          <div className="flex items-center gap-2">
            <CalendarClock className="size-4 text-muted-foreground" />
            <span className="font-semibold">{job.name}</span>
            <SchedulerStatusBadge status={detailStatus} />
          </div>
          <p className="text-xs text-muted-foreground">{job.jobKey}</p>
        </div>
        <div className="flex flex-wrap justify-end gap-2">
          <Button
            variant="outline"
            size="sm"
            className="gap-1.5"
            onClick={onTriggerJob}
            disabled={actionLoading || triggerSupport?.enabled === false}
            title={triggerSupport?.reason}
          >
            <Play className="size-3.5" />
            {t("jobDetail.runNow")}
          </Button>
          <Button
            variant={job.controlState === "paused" ? "default" : "secondary"}
            size="sm"
            className="gap-1.5"
            onClick={job.controlState === "paused" ? onResumeJob : onPauseJob}
            disabled={
              actionLoading ||
              (job.controlState === "paused"
                ? resumeSupport?.enabled === false
                : pauseSupport?.enabled === false)
            }
            title={job.controlState === "paused" ? resumeSupport?.reason : pauseSupport?.reason}
          >
            {job.controlState === "paused" ? (
              <RotateCcw className="size-3.5" />
            ) : (
              <Pause className="size-3.5" />
            )}
            {job.controlState === "paused" ? t("jobDetail.resume") : t("jobDetail.pause")}
          </Button>
          <Button
            variant="outline"
            size="sm"
            className="gap-1.5"
            onClick={onCancelJob}
            disabled={actionLoading || cancelSupport?.enabled === false}
            title={cancelSupport?.reason}
          >
            <XCircle className="size-3.5" />
            {t("jobDetail.cancelRun")}
          </Button>
        </div>
      </div>

      {unsupportedMessages.length > 0 && (
        <div className="rounded-lg border border-amber-500/20 bg-amber-500/5 p-3 text-xs text-amber-800 dark:text-amber-200">
          <div className="mb-1 flex items-center gap-2 font-medium">
            <ShieldAlert className="size-3.5" />
            {t("jobDetail.unsupportedActions")}
          </div>
          <ul className="space-y-1">
            {unsupportedMessages.map((message) => (
              <li key={message}>{message}</li>
            ))}
          </ul>
        </div>
      )}

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
          {(config || job.configMetadata) && (
            <TabsTrigger value="config">{t("jobDetail.tabConfig")}</TabsTrigger>
          )}
        </TabsList>

        <TabsContent value="overview" className="space-y-4 pt-4">
          <div className="grid grid-cols-2 gap-4 text-sm">
            <div>
              <span className="text-muted-foreground">{t("jobDetail.lastRun")}</span>
              <p className="font-medium">{formatRelativeTime(job.lastRunAt)}</p>
            </div>
            <div>
              <span className="text-muted-foreground">{t("jobDetail.controlState")}</span>
              <p className="font-medium capitalize">{t(`controlStateLabels.${job.controlState ?? "active"}`)}</p>
            </div>
            <div>
              <span className="text-muted-foreground">{t("jobDetail.nextRun")}</span>
              <p className="font-medium">
                {job.controlState !== "paused" && job.nextRunAt
                  ? formatRelativeTime(job.nextRunAt)
                  : t("jobDetail.notScheduled")}
              </p>
            </div>
            <div>
              <span className="text-muted-foreground">{t("jobDetail.scope")}</span>
              <p className="font-medium capitalize">{job.scope ? t(`scopeLabels.${job.scope}`) : "-"}</p>
            </div>
            <div>
              <span className="text-muted-foreground">{t("jobDetail.overlapPolicy")}</span>
              <p className="font-medium capitalize">{job.overlapPolicy ? t(`overlapPolicyLabels.${job.overlapPolicy}`) : "-"}</p>
            </div>
            <div>
              <span className="text-muted-foreground">{t("jobDetail.executionMode")}</span>
              <p className="font-medium capitalize">{job.executionMode ? t(`executionModeLabels.${job.executionMode}`) : "-"}</p>
            </div>
          </div>

          {job.activeRun && (
            <div className="rounded-lg border bg-blue-500/5 p-3 text-sm">
              <span className="text-xs font-medium text-muted-foreground">{t("jobDetail.activeRun")}</span>
              <div className="mt-2 flex flex-wrap items-center gap-2">
                <SchedulerStatusBadge status={job.activeRun.status} />
                <span className="text-xs text-muted-foreground">
                  {formatRelativeTime(job.activeRun.startedAt)}
                </span>
                <span className="text-xs">{job.activeRun.summary || job.activeRun.errorMessage || "-"}</span>
              </div>
            </div>
          )}

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
                {cronDesc && <p className="text-xs text-muted-foreground">{cronDesc}</p>}
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

          {job.upcomingRuns && job.upcomingRuns.length > 0 && (
            <div className="space-y-2">
              <span className="text-sm font-medium">{t("jobDetail.upcomingRuns")}</span>
              <ul className="space-y-1 text-xs text-muted-foreground">
                {job.upcomingRuns.map((run) => (
                  <li key={run.runAt}>{new Date(run.runAt).toLocaleString()}</li>
                ))}
              </ul>
            </div>
          )}
        </TabsContent>

        <TabsContent value="history" className="space-y-4 pt-4">
          <div className="flex flex-wrap items-center gap-2">
            <label className="text-xs text-muted-foreground">{t("runHistory.filterStatus")}</label>
            <select
              aria-label={t("runHistory.filterStatus")}
              className="rounded-md border bg-background px-2 py-1 text-xs"
              value={statusFilter}
              onChange={(event) => setStatusFilter(event.target.value)}
            >
              <option value="">{t("runHistory.filterAll")}</option>
              <option value="failed">{t("runStatusOptions.failed")}</option>
              <option value="running">{t("runStatusOptions.running")}</option>
              <option value="cancel_requested">{t("runStatusOptions.cancel_requested")}</option>
              <option value="cancelled">{t("runStatusOptions.cancelled")}</option>
            </select>
            <label className="text-xs text-muted-foreground">{t("runHistory.filterTrigger")}</label>
            <select
              aria-label={t("runHistory.filterTrigger")}
              className="rounded-md border bg-background px-2 py-1 text-xs"
              value={triggerFilter}
              onChange={(event) => setTriggerFilter(event.target.value)}
            >
              <option value="">{t("runHistory.filterAll")}</option>
              <option value="manual">{t("triggerSourceLabels.manual")}</option>
              <option value="cron">{t("triggerSourceLabels.cron")}</option>
            </select>
            <Button
              variant="outline"
              size="sm"
              onClick={() =>
                onFetchRuns({
                  status: statusFilter ? (statusFilter as SchedulerRunHistoryFilters["status"]) : undefined,
                  triggerSource: triggerFilter || undefined,
                  limit: 20,
                })
              }
            >
              {t("runHistory.applyFilters")}
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={() => {
                setStatusFilter("");
                setTriggerFilter("");
                onFetchRuns();
              }}
            >
              {t("runHistory.resetFilters")}
            </Button>
            <Button
              variant="outline"
              size="sm"
              className="gap-1.5"
              onClick={onCleanupRuns}
              disabled={actionLoading || cleanupSupport?.enabled === false}
              title={cleanupSupport?.reason}
            >
              <Trash2 className="size-3.5" />
              {t("jobDetail.cleanupHistory")}
            </Button>
          </div>
          <SchedulerRunHistory runs={runs} />
        </TabsContent>

        {(config || job.configMetadata) && (
          <TabsContent value="config" className="pt-4">
            <div className="space-y-4">
              {job.configMetadata && (
                <div className="rounded-lg border bg-muted/50 p-4 text-xs">
                  <div className="mb-2 font-medium">{t("jobDetail.configFields")}</div>
                  {job.configMetadata.fields?.length ? (
                    <ul className="space-y-2">
                      {job.configMetadata.fields.map((field) => (
                        <li key={field.key}>
                          <div className="font-medium">{field.label}</div>
                          <div className="text-muted-foreground">{field.helpText || field.type}</div>
                        </li>
                      ))}
                    </ul>
                  ) : (
                    <span className="text-muted-foreground">
                      {job.configMetadata.reason || t("jobDetail.configManagedByBackend")}
                    </span>
                  )}
                </div>
              )}
              {config && (
                <pre className="max-h-64 overflow-auto rounded-lg border bg-muted/50 p-4 text-xs">
                  {JSON.stringify(config, null, 2)}
                </pre>
              )}
            </div>
          </TabsContent>
        )}
      </Tabs>
    </>
  );
}

export function SchedulerJobDetailEmpty() {
  const t = useTranslations("scheduler");
  return (
    <div className="flex flex-col items-center justify-center py-12 text-center">
      <CalendarClock className="mb-3 size-10 text-muted-foreground" />
      <p className="text-sm text-muted-foreground">{t("jobDetail.selectJob")}</p>
    </div>
  );
}
