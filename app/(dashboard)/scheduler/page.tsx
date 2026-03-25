"use client";

import { useEffect, useMemo } from "react";
import { CalendarClock, Play, Power, RefreshCw } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { useSchedulerStore } from "@/lib/stores/scheduler-store";

function formatDate(value?: string): string {
  if (!value) {
    return "Not available";
  }
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) {
    return value;
  }
  return parsed.toLocaleString();
}

function statusTone(status?: string): "default" | "secondary" | "destructive" {
  if (status === "succeeded") {
    return "default";
  }
  if (status === "failed") {
    return "destructive";
  }
  return "secondary";
}

export default function SchedulerPage() {
  const jobs = useSchedulerStore((s) => s.jobs);
  const runsByJobKey = useSchedulerStore((s) => s.runsByJobKey);
  const draftSchedules = useSchedulerStore((s) => s.draftSchedules);
  const selectedJobKey = useSchedulerStore((s) => s.selectedJobKey);
  const loading = useSchedulerStore((s) => s.loading);
  const actionJobKey = useSchedulerStore((s) => s.actionJobKey);
  const error = useSchedulerStore((s) => s.error);
  const fetchJobs = useSchedulerStore((s) => s.fetchJobs);
  const fetchRuns = useSchedulerStore((s) => s.fetchRuns);
  const updateJob = useSchedulerStore((s) => s.updateJob);
  const triggerJob = useSchedulerStore((s) => s.triggerJob);
  const selectJob = useSchedulerStore((s) => s.selectJob);
  const setDraftSchedule = useSchedulerStore((s) => s.setDraftSchedule);

  useEffect(() => {
    void fetchJobs();
  }, [fetchJobs]);

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
          <h1 className="text-2xl font-bold">Scheduler Control Plane</h1>
          <p className="text-sm text-muted-foreground">
            Manage built-in background jobs, inspect run history, and trigger
            operator reruns without leaving the dashboard.
          </p>
        </div>
        <Button
          variant="outline"
          className="gap-2"
          onClick={() => void fetchJobs()}
          disabled={loading}
        >
          <RefreshCw className="size-4" />
          Refresh jobs
        </Button>
      </div>

      {error ? (
        <div className="rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {error}
        </div>
      ) : null}

      <div className="grid gap-6 xl:grid-cols-[1.3fr_0.9fr]">
        <Card>
          <CardHeader>
            <CardTitle>Registered jobs</CardTitle>
            <CardDescription>
              Stable built-ins with their effective cadence, health, and next due time.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Job</TableHead>
                  <TableHead>Schedule</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Next run</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {jobs.map((job) => (
                  <TableRow
                    key={job.jobKey}
                    className="cursor-pointer"
                    data-state={selectedJob?.jobKey === job.jobKey ? "selected" : undefined}
                    onClick={() => selectJob(job.jobKey)}
                  >
                    <TableCell>
                      <div className="font-medium">{job.name}</div>
                      <div className="text-xs text-muted-foreground">{job.jobKey}</div>
                    </TableCell>
                    <TableCell className="font-mono text-xs">{job.schedule}</TableCell>
                    <TableCell>
                      <div className="flex flex-col gap-1">
                        <Badge variant={statusTone(job.lastRunStatus)}>
                          {job.lastRunStatus ?? "never-run"}
                        </Badge>
                        {!job.enabled ? (
                          <span className="text-xs text-muted-foreground">Disabled</span>
                        ) : null}
                      </div>
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground">
                      {formatDate(job.nextRunAt)}
                    </TableCell>
                  </TableRow>
                ))}
                {jobs.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={4} className="text-center text-sm text-muted-foreground">
                      {loading ? "Loading scheduler jobs..." : "No scheduler jobs registered yet."}
                    </TableCell>
                  </TableRow>
                ) : null}
              </TableBody>
            </Table>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Selected job</CardTitle>
            <CardDescription>
              Adjust the effective cron, toggle automatic execution, and review recent runs.
            </CardDescription>
          </CardHeader>
          <CardContent className="flex flex-col gap-4">
            {selectedJob ? (
              <>
                <div className="flex items-start justify-between gap-3 rounded-lg border p-4">
                  <div className="space-y-1">
                    <div className="flex items-center gap-2">
                      <CalendarClock className="size-4 text-muted-foreground" />
                      <span className="font-medium">{selectedJob.name}</span>
                    </div>
                    <p className="text-xs text-muted-foreground">{selectedJob.jobKey}</p>
                    <p className="text-xs text-muted-foreground">
                      Last run: {formatDate(selectedJob.lastRunAt)}
                    </p>
                    <p className="text-xs text-muted-foreground">
                      Last summary: {selectedJob.lastRunSummary || "No summary recorded"}
                    </p>
                    {selectedJob.lastError ? (
                      <p className="text-xs text-destructive">{selectedJob.lastError}</p>
                    ) : null}
                  </div>
                  <div className="flex gap-2">
                    <Button
                      variant="outline"
                      className="gap-2"
                      onClick={() => void triggerJob(selectedJob.jobKey)}
                      disabled={actionJobKey === selectedJob.jobKey}
                    >
                      <Play className="size-4" />
                      Run now
                    </Button>
                    <Button
                      variant={selectedJob.enabled ? "secondary" : "default"}
                      className="gap-2"
                      onClick={() =>
                        void updateJob(selectedJob.jobKey, { enabled: !selectedJob.enabled })
                      }
                      disabled={actionJobKey === selectedJob.jobKey}
                    >
                      <Power className="size-4" />
                      {selectedJob.enabled ? "Disable job" : "Enable job"}
                    </Button>
                  </div>
                </div>

                <div className="space-y-2">
                  <label htmlFor="job-schedule" className="text-sm font-medium">
                    Schedule expression
                  </label>
                  <div className="flex gap-2">
                    <Input
                      id="job-schedule"
                      aria-label="Schedule expression"
                      value={draftSchedules[selectedJob.jobKey] ?? selectedJob.schedule}
                      onChange={(event) =>
                        setDraftSchedule(selectedJob.jobKey, event.target.value)
                      }
                    />
                    <Button
                      onClick={() =>
                        void updateJob(selectedJob.jobKey, {
                          schedule:
                            draftSchedules[selectedJob.jobKey] ?? selectedJob.schedule,
                        })
                      }
                      disabled={actionJobKey === selectedJob.jobKey}
                    >
                      Save schedule
                    </Button>
                  </div>
                </div>

                <div className="space-y-2">
                  <h2 className="text-sm font-semibold">Recent runs</h2>
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>Status</TableHead>
                        <TableHead>Trigger</TableHead>
                        <TableHead>Started</TableHead>
                        <TableHead>Summary</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {runs.map((run) => (
                        <TableRow key={run.runId}>
                          <TableCell>
                            <Badge variant={statusTone(run.status)}>{run.status}</Badge>
                          </TableCell>
                          <TableCell className="text-xs">{run.triggerSource}</TableCell>
                          <TableCell className="text-xs text-muted-foreground">
                            {formatDate(run.startedAt)}
                          </TableCell>
                          <TableCell className="text-xs">
                            {run.summary || run.errorMessage || "No summary"}
                          </TableCell>
                        </TableRow>
                      ))}
                      {runs.length === 0 ? (
                        <TableRow>
                          <TableCell
                            colSpan={4}
                            className="text-center text-sm text-muted-foreground"
                          >
                            No run history recorded for this job yet.
                          </TableCell>
                        </TableRow>
                      ) : null}
                    </TableBody>
                  </Table>
                </div>
              </>
            ) : (
              <div className="text-sm text-muted-foreground">
                Select a scheduler job to inspect details and history.
              </div>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
