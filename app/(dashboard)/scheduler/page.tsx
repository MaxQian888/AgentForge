"use client";

import { useEffect, useMemo } from "react";
import { useTranslations } from "next-intl";
import { RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { useSchedulerStore } from "@/lib/stores/scheduler-store";
import { SchedulerStatsCards } from "@/components/scheduler/scheduler-stats-cards";
import { SchedulerJobTable } from "@/components/scheduler/scheduler-job-table";
import {
  SchedulerJobDetail,
  SchedulerJobDetailEmpty,
} from "@/components/scheduler/scheduler-job-detail";

export default function SchedulerPage() {
  const t = useTranslations("scheduler");
  const jobs = useSchedulerStore((s) => s.jobs);
  const runsByJobKey = useSchedulerStore((s) => s.runsByJobKey);
  const draftSchedules = useSchedulerStore((s) => s.draftSchedules);
  const selectedJobKey = useSchedulerStore((s) => s.selectedJobKey);
  const stats = useSchedulerStore((s) => s.stats);
  const loading = useSchedulerStore((s) => s.loading);
  const actionJobKey = useSchedulerStore((s) => s.actionJobKey);
  const error = useSchedulerStore((s) => s.error);
  const fetchJobs = useSchedulerStore((s) => s.fetchJobs);
  const fetchRuns = useSchedulerStore((s) => s.fetchRuns);
  const fetchStats = useSchedulerStore((s) => s.fetchStats);
  const updateJob = useSchedulerStore((s) => s.updateJob);
  const triggerJob = useSchedulerStore((s) => s.triggerJob);
  const selectJob = useSchedulerStore((s) => s.selectJob);
  const setDraftSchedule = useSchedulerStore((s) => s.setDraftSchedule);

  useEffect(() => {
    void fetchJobs();
    void fetchStats();
  }, [fetchJobs, fetchStats]);

  useEffect(() => {
    if (!selectedJobKey) {
      return;
    }
    void fetchRuns(selectedJobKey);
  }, [fetchRuns, selectedJobKey]);

  const selectedJob = useMemo(
    () => jobs.find((job) => job.jobKey === selectedJobKey) ?? jobs[0] ?? null,
    [jobs, selectedJobKey]
  );
  const runs = selectedJob ? runsByJobKey[selectedJob.jobKey] ?? [] : [];

  return (
    <div className="flex flex-col gap-6">
      <div className="flex items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold">{t("title")}</h1>
          <p className="text-sm text-muted-foreground">
            {t("subtitle")}
          </p>
        </div>
        <Button
          variant="outline"
          className="gap-2"
          onClick={() => {
            void fetchJobs();
            void fetchStats();
          }}
          disabled={loading}
        >
          <RefreshCw className="size-4" />
          {t("refresh")}
        </Button>
      </div>

      {error && (
        <div className="rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {error}
        </div>
      )}

      <SchedulerStatsCards stats={stats} loading={loading && !stats} />

      <div className="grid gap-6 lg:grid-cols-[1.3fr_0.9fr]">
        <Card>
          <CardHeader>
            <CardTitle>{t("registeredJobs")}</CardTitle>
            <CardDescription>
              {t("registeredJobsDesc")}
            </CardDescription>
          </CardHeader>
          <CardContent>
            <SchedulerJobTable
              jobs={jobs}
              selectedJobKey={selectedJob?.jobKey ?? null}
              loading={loading}
              onSelectJob={selectJob}
            />
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>{t("jobDetails")}</CardTitle>
            <CardDescription>
              {t("jobDetailsDesc")}
            </CardDescription>
          </CardHeader>
          <CardContent className="flex flex-col gap-4">
            {selectedJob ? (
              <SchedulerJobDetail
                job={selectedJob}
                runs={runs}
                draftSchedule={draftSchedules[selectedJob.jobKey] ?? selectedJob.schedule}
                actionLoading={actionJobKey === selectedJob.jobKey}
                onUpdateJob={(input) => void updateJob(selectedJob.jobKey, input)}
                onTriggerJob={() => void triggerJob(selectedJob.jobKey)}
                onSetDraftSchedule={(schedule) =>
                  setDraftSchedule(selectedJob.jobKey, schedule)
                }
              />
            ) : (
              <SchedulerJobDetailEmpty />
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
