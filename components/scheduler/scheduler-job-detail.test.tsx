jest.mock("next-intl", () => ({
  useTranslations: () => (
    key: string,
    values?: Record<string, string>,
  ) => {
    const map: Record<string, string> = {
      "jobDetail.runNow": "Run Now",
      "jobDetail.pause": "Pause",
      "jobDetail.resume": "Resume",
      "jobDetail.cancelRun": "Cancel Run",
      "jobDetail.cleanupHistory": "Cleanup History",
      "jobDetail.unsupportedActions": "Unsupported Actions",
      "jobDetail.tabOverview": "Overview",
      "jobDetail.tabHistory": "History",
      "jobDetail.tabConfig": "Config",
      "jobDetail.lastRun": "Last Run",
      "jobDetail.controlState": "Control State",
      "jobDetail.nextRun": "Next Run",
      "jobDetail.notScheduled": "Not scheduled",
      "jobDetail.scope": "Scope",
      "jobDetail.overlapPolicy": "Overlap Policy",
      "jobDetail.executionMode": "Execution Mode",
      "jobDetail.activeRun": "Active Run",
      "jobDetail.lastSummary": "Last Summary",
      "jobDetail.lastError": "Last Error",
      "jobDetail.scheduleExpression": "Schedule Expression",
      "jobDetail.save": "Save",
      "jobDetail.upcomingRuns": "Upcoming Runs",
      "jobDetail.configFields": "Config Fields",
      "jobDetail.configManagedByBackend": "Managed by backend",
      "jobDetail.selectJob": "Select a job",
      "statusLabels.succeeded": "Succeeded",
      "statusLabels.failed": "Failed",
      "statusLabels.running": "Running",
      "statusLabels.cancel_requested": "Cancel Requested",
      "statusLabels.cancelled": "Cancelled",
      "statusLabels.pending": "Pending",
      "statusLabels.skipped": "Skipped",
      "statusLabels.paused": "Paused",
      "statusLabels.disabled": "Disabled",
      "statusLabels.never-run": "Never Run",
      "controlStateLabels.active": "Active",
      "controlStateLabels.paused": "Paused",
      "overlapPolicyLabels.skip": "Skip",
      "overlapPolicyLabels.allow": "Allow",
      "executionModeLabels.in_process": "In Process",
      "executionModeLabels.os_registered": "OS Registered",
      "runStatusOptions.failed": "Failed",
      "runStatusOptions.running": "Running",
      "runStatusOptions.cancel_requested": "Cancel Requested",
      "runStatusOptions.cancelled": "Cancelled",
      "triggerSourceLabels.manual": "Manual",
      "triggerSourceLabels.cron": "Cron",
      "triggerSourceLabels.system": "System",
      "triggerSourceLabels.startup": "Startup",
      "triggerSourceLabels.reconcile": "Reconcile",
      "scopeLabels.system": "System",
      "scopeLabels.project": "Project",
      "runHistory.filterStatus": "Status Filter",
      "runHistory.filterTrigger": "Trigger Filter",
      "runHistory.filterAll": "All",
      "runHistory.applyFilters": "Apply Filters",
      "runHistory.resetFilters": "Reset Filters",
      "runHistory.noRuns": "No runs",
      "runHistory.colStatus": "Status",
      "runHistory.colTrigger": "Trigger",
      "runHistory.colStarted": "Started",
      "runHistory.colDuration": "Duration",
      "runHistory.colSummary": "Summary",
      "runHistory.colMetrics": "Metrics",
    };
    if (key === "jobDetail.disableJobDesc") {
      return `Stop ${values?.name ?? ""}`;
    }
    if (key === "jobDetail.enableJobDesc") {
      return `Resume ${values?.name ?? ""} on ${values?.schedule ?? ""}`;
    }
    return map[key] ?? key;
  },
}));

import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SchedulerJobDetail, SchedulerJobDetailEmpty } from "./scheduler-job-detail";

describe("SchedulerJobDetail", () => {
  beforeEach(() => {
    jest.spyOn(Date, "now").mockReturnValue(
      new Date("2026-03-30T12:00:00.000Z").getTime(),
    );
  });

  afterEach(() => {
    jest.restoreAllMocks();
  });

  it("renders operator controls, schedule preview, and config metadata", async () => {
    const user = userEvent.setup();
    const onUpdateJob = jest.fn();
    const onTriggerJob = jest.fn();
    const onPauseJob = jest.fn();
    const onResumeJob = jest.fn();
    const onCancelJob = jest.fn();
    const onCleanupRuns = jest.fn();
    const onFetchRuns = jest.fn();
    const onSetDraftSchedule = jest.fn();

    render(
      <SchedulerJobDetail
        job={{
          jobKey: "scheduler.cleanup",
          name: "Cleanup",
          scope: "project",
          schedule: "0 0 * * *",
          enabled: true,
          executionMode: "in_process",
          overlapPolicy: "skip",
          lastRunStatus: "succeeded",
          controlState: "active",
          activeRun: {
            runId: "run-active",
            triggerSource: "manual",
            status: "running",
            startedAt: "2026-03-30T11:30:00.000Z",
            summary: "Processing files",
            errorMessage: "",
          },
          supportedActions: [
            { action: "pause", enabled: true },
            { action: "trigger", enabled: true },
            { action: "cancel", enabled: true },
            { action: "cleanup", enabled: false, reason: "cleanup is disabled while a run is active" },
          ],
          configMetadata: {
            editable: true,
            fields: [
              { key: "schedule", label: "Schedule", type: "string", helpText: "Cron expression" },
            ],
          },
          upcomingRuns: [
            { runAt: "2026-03-31T00:00:00.000Z" },
            { runAt: "2026-04-01T00:00:00.000Z" },
          ],
          lastRunAt: "2026-03-30T11:00:00.000Z",
          nextRunAt: "2026-03-31T00:00:00.000Z",
          lastRunSummary: "Finished cleanly",
          lastError: "Previous failure",
          config: '{"retries":2}',
          createdAt: "",
          updatedAt: "",
        }}
        runs={[
          {
            runId: "run-1",
            jobKey: "scheduler.cleanup",
            triggerSource: "manual",
            status: "succeeded",
            startedAt: "2026-03-30T11:00:00.000Z",
            durationMs: 1500,
            summary: "Finished cleanly",
            errorMessage: "",
            metrics: '{"processed":10}',
            createdAt: "",
            updatedAt: "",
          },
        ]}
        draftSchedule="*/15 * * * *"
        actionLoading={false}
        onUpdateJob={onUpdateJob}
        onTriggerJob={onTriggerJob}
        onPauseJob={onPauseJob}
        onResumeJob={onResumeJob}
        onCancelJob={onCancelJob}
        onCleanupRuns={onCleanupRuns}
        onFetchRuns={onFetchRuns}
        onSetDraftSchedule={onSetDraftSchedule}
      />,
    );

    expect(screen.getByText("Cleanup")).toBeInTheDocument();
    expect(screen.getByText("Processing files")).toBeInTheDocument();
    expect(screen.getByText("Upcoming Runs")).toBeInTheDocument();
    expect(screen.getByText("cleanup: cleanup is disabled while a run is active")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Run Now" }));
    expect(onTriggerJob).toHaveBeenCalled();

    await user.click(screen.getByRole("button", { name: "Pause" }));
    expect(onPauseJob).toHaveBeenCalled();

    await user.click(screen.getByRole("button", { name: "Cancel Run" }));
    expect(onCancelJob).toHaveBeenCalled();

    await user.click(screen.getByRole("button", { name: "Save" }));
    expect(onUpdateJob).toHaveBeenCalledWith({ schedule: "*/15 * * * *" });

    const scheduleInput = screen.getByRole("textbox", {
      name: "Schedule expression",
    });
    fireEvent.change(scheduleInput, { target: { value: "0 */6 * * *" } });
    expect(onSetDraftSchedule).toHaveBeenLastCalledWith("0 */6 * * *");

    await user.click(screen.getByRole("tab", { name: /History/i }));
    await user.selectOptions(screen.getByLabelText("Status Filter"), "failed");
    await user.selectOptions(screen.getByLabelText("Trigger Filter"), "manual");
    await user.click(screen.getByRole("button", { name: "Apply Filters" }));
    expect(onFetchRuns).toHaveBeenCalledWith({
      status: "failed",
      triggerSource: "manual",
      limit: 20,
    });

    await user.click(screen.getByRole("tab", { name: /Config/i }));
    expect(screen.getByText("Config Fields")).toBeInTheDocument();
  });

  it("renders paused jobs truthfully and allows resume or cleanup reset", async () => {
    const user = userEvent.setup();
    const onResumeJob = jest.fn();
    const onFetchRuns = jest.fn();

    render(
      <SchedulerJobDetail
        job={{
          jobKey: "scheduler.cleanup",
          name: "Cleanup",
          scope: "project",
          schedule: "0 0 * * *",
          enabled: false,
          executionMode: "in_process",
          overlapPolicy: "skip",
          lastRunStatus: "failed",
          controlState: "paused",
          supportedActions: [
            { action: "resume", enabled: true },
            { action: "trigger", enabled: false, reason: "job is paused" },
            { action: "cancel", enabled: false, reason: "no active run" },
            { action: "cleanup", enabled: true },
          ],
          configMetadata: { editable: false, reason: "Managed by backend" },
          upcomingRuns: [],
          lastRunAt: "2026-03-30T11:00:00.000Z",
          nextRunAt: undefined,
          lastRunSummary: "",
          lastError: "",
          config: "{}",
          createdAt: "",
          updatedAt: "",
        }}
        runs={[]}
        draftSchedule="0 0 * * *"
        actionLoading={false}
        onUpdateJob={jest.fn()}
        onTriggerJob={jest.fn()}
        onPauseJob={jest.fn()}
        onResumeJob={onResumeJob}
        onCancelJob={jest.fn()}
        onCleanupRuns={jest.fn()}
        onFetchRuns={onFetchRuns}
        onSetDraftSchedule={jest.fn()}
      />,
    );

    expect(screen.getAllByText("Paused")).toHaveLength(2);
    expect(screen.getByTitle("job is paused")).toBeDisabled();

    await user.click(screen.getByRole("button", { name: "Resume" }));
    expect(onResumeJob).toHaveBeenCalled();

    await user.click(screen.getByRole("tab", { name: /History/i }));
    await user.click(screen.getByRole("button", { name: "Reset Filters" }));
    expect(onFetchRuns).toHaveBeenCalledWith();

    await user.click(screen.getByRole("tab", { name: /Config/i }));
    expect(screen.getByText("Managed by backend")).toBeInTheDocument();
  });
});

describe("SchedulerJobDetailEmpty", () => {
  it("renders the empty selection message", () => {
    render(<SchedulerJobDetailEmpty />);
    expect(screen.getByText("Select a job")).toBeInTheDocument();
  });
});

