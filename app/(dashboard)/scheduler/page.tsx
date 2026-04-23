"use client";

import { useEffect, useMemo, useState } from "react";
import { useTranslations } from "next-intl";
import { Plus, RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { SectionCard } from "@/components/shared/section-card";
import {
  filterSchedulerJobs,
  useSchedulerStore,
} from "@/lib/stores/scheduler-store";
import { SchedulerStatsCards } from "@/components/scheduler/scheduler-stats-cards";
import { SchedulerJobTable } from "@/components/scheduler/scheduler-job-table";
import {
  SchedulerJobDetail,
  SchedulerJobDetailEmpty,
} from "@/components/scheduler/scheduler-job-detail";
import { SchedulerJobFilters } from "@/components/scheduler/scheduler-job-filters";
import { SchedulerJobCreateDialog } from "@/components/scheduler/scheduler-job-create-dialog";
import { SchedulerUpcomingCalendar } from "@/components/scheduler/scheduler-upcoming-calendar";
import { PageHeader } from "@/components/shared/page-header";
import { ErrorBanner } from "@/components/shared/error-banner";
import { useBreadcrumbs } from "@/hooks/use-breadcrumbs";

export default function SchedulerPage() {
  const tc = useTranslations("common");
  useBreadcrumbs([{ label: tc("nav.group.operations"), href: "/" }, { label: tc("nav.scheduler") }]);
  const t = useTranslations("scheduler");
  const jobs = useSchedulerStore((s) => s.jobs);
  const runsByJobKey = useSchedulerStore((s) => s.runsByJobKey);
  const draftSchedules = useSchedulerStore((s) => s.draftSchedules);
  const selectedJobKey = useSchedulerStore((s) => s.selectedJobKey);
  const stats = useSchedulerStore((s) => s.stats);
  const loading = useSchedulerStore((s) => s.loading);
  const actionJobKey = useSchedulerStore((s) => s.actionJobKey);
  const error = useSchedulerStore((s) => s.error);
  const listFilters = useSchedulerStore((s) => s.listFilters);
  const fetchJobs = useSchedulerStore((s) => s.fetchJobs);
  const fetchRuns = useSchedulerStore((s) => s.fetchRuns);
  const fetchStats = useSchedulerStore((s) => s.fetchStats);
  const updateJob = useSchedulerStore((s) => s.updateJob);
  const createJob = useSchedulerStore((s) => s.createJob);
  const triggerJob = useSchedulerStore((s) => s.triggerJob);
  const pauseJob = useSchedulerStore((s) => s.pauseJob);
  const resumeJob = useSchedulerStore((s) => s.resumeJob);
  const cancelJob = useSchedulerStore((s) => s.cancelJob);
  const cleanupRuns = useSchedulerStore((s) => s.cleanupRuns);
  const selectJob = useSchedulerStore((s) => s.selectJob);
  const setDraftSchedule = useSchedulerStore((s) => s.setDraftSchedule);
  const setListFilters = useSchedulerStore((s) => s.setListFilters);
  const resetListFilters = useSchedulerStore((s) => s.resetListFilters);

  const [createOpen, setCreateOpen] = useState(false);

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

  const filteredJobs = useMemo(
    () => filterSchedulerJobs(jobs, listFilters),
    [jobs, listFilters],
  );

  const selectedJob = useMemo(
    () =>
      filteredJobs.find((job) => job.jobKey === selectedJobKey) ??
      jobs.find((job) => job.jobKey === selectedJobKey) ??
      filteredJobs[0] ??
      jobs[0] ??
      null,
    [filteredJobs, jobs, selectedJobKey]
  );
  const runs = selectedJob ? runsByJobKey[selectedJob.jobKey] ?? [] : [];

  return (
    <div className="flex flex-col gap-[var(--space-section-gap)]">
      <PageHeader
        title={t("title")}
        description={t("subtitle")}
        actions={
          <div className="flex items-center gap-[var(--space-stack-sm)]">
            <Button
              size="sm"
              className="gap-2"
              onClick={() => setCreateOpen(true)}
            >
              <Plus className="size-4" />
              {t("createJob")}
            </Button>
            <Button
              variant="outline"
              size="sm"
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
        }
      />

      {error && (
        <ErrorBanner
          message={error}
          onRetry={() => {
            void fetchJobs();
            void fetchStats();
          }}
        />
      )}

      <SchedulerStatsCards stats={stats} loading={loading && !stats} />

      <Tabs
        defaultValue="queue"
        className="flex flex-col gap-[var(--space-stack-md)]"
      >
        <TabsList>
          <TabsTrigger value="queue">{t("tabs.queue")}</TabsTrigger>
          <TabsTrigger value="calendar">{t("tabs.calendar")}</TabsTrigger>
        </TabsList>

        <TabsContent
          value="queue"
          className="flex flex-col gap-[var(--space-stack-md)]"
        >
          <SchedulerJobFilters
            jobs={jobs}
            filters={listFilters}
            onFiltersChange={setListFilters}
            onReset={resetListFilters}
          />

          <div className="grid grid-cols-1 gap-[var(--space-grid-gap)] lg:grid-cols-[1.3fr_0.9fr]">
            <SectionCard
              title={t("registeredJobs")}
              description={t("registeredJobsDesc")}
            >
              <SchedulerJobTable
                jobs={filteredJobs}
                selectedJobKey={selectedJob?.jobKey ?? null}
                loading={loading}
                onSelectJob={selectJob}
              />
            </SectionCard>

            <SectionCard
              title={t("jobDetails")}
              description={t("jobDetailsDesc")}
            >
              {selectedJob ? (
                <SchedulerJobDetail
                  job={selectedJob}
                  runs={runs}
                  draftSchedule={draftSchedules[selectedJob.jobKey] ?? selectedJob.schedule}
                  actionLoading={actionJobKey === selectedJob.jobKey}
                  onUpdateJob={(input) => void updateJob(selectedJob.jobKey, input)}
                  onTriggerJob={() => void triggerJob(selectedJob.jobKey)}
                  onPauseJob={() => void pauseJob(selectedJob.jobKey)}
                  onResumeJob={() => void resumeJob(selectedJob.jobKey)}
                  onCancelJob={() => void cancelJob(selectedJob.jobKey)}
                  onCleanupRuns={() => void cleanupRuns(selectedJob.jobKey, { retainRecent: 10 })}
                  onFetchRuns={(filters) => void fetchRuns(selectedJob.jobKey, filters)}
                  onSetDraftSchedule={(schedule) =>
                    setDraftSchedule(selectedJob.jobKey, schedule)
                  }
                />
              ) : (
                <SchedulerJobDetailEmpty />
              )}
            </SectionCard>
          </div>
        </TabsContent>

        <TabsContent value="calendar">
          <SectionCard
            title={t("calendar.title")}
            description={t("calendar.description")}
          >
            <SchedulerUpcomingCalendar jobs={jobs} />
          </SectionCard>
        </TabsContent>
      </Tabs>

      <SchedulerJobCreateDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        onCreate={createJob}
        actionLoading={loading}
      />
    </div>
  );
}
