jest.mock("next-intl", () => ({
  useTranslations: () => (key: string) => {
    const map: Record<string, string> = {
      "jobTable.colJob": "Job",
      "jobTable.colSchedule": "Schedule",
      "jobTable.colStatus": "Status",
      "jobTable.colNextRun": "Next Run",
      "jobTable.noJobs": "No scheduler jobs.",
      "jobTable.disabled": "Disabled",
      "jobTable.paused": "Paused",
      "jobTable.na": "N/A",
      "jobTable.notScheduled": "Not scheduled",
    };
    return map[key] ?? key;
  },
}));

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SchedulerJobTable } from "./scheduler-job-table";

describe("SchedulerJobTable", () => {
  beforeEach(() => {
    jest.spyOn(Date, "now").mockReturnValue(
      new Date("2026-03-30T12:00:00.000Z").getTime(),
    );
  });

  afterEach(() => {
    jest.restoreAllMocks();
  });

  it("renders loading placeholders before jobs are available", () => {
    const { container } = render(
      <SchedulerJobTable
        jobs={[]}
        selectedJobKey={null}
        loading
        onSelectJob={jest.fn()}
      />,
    );

    expect(container.querySelectorAll('[data-slot="skeleton"]')).toHaveLength(4);
  });

  it("renders an empty state when there are no jobs", () => {
    render(
      <SchedulerJobTable
        jobs={[]}
        selectedJobKey={null}
        loading={false}
        onSelectJob={jest.fn()}
      />,
    );

    expect(screen.getByText("No scheduler jobs.")).toBeInTheDocument();
  });

  it("renders truthful paused and active statuses, highlights the selected row, and notifies selection", async () => {
    const user = userEvent.setup();
    const onSelectJob = jest.fn();

    const { container } = render(
      <SchedulerJobTable
        jobs={[
          {
            jobKey: "scheduler.cleanup",
            name: "Cleanup",
            schedule: "*/5 * * * *",
            enabled: false,
            scope: "project",
            executionMode: "single",
            overlapPolicy: "skip",
            controlState: "paused",
            lastRunStatus: "failed",
            nextRunAt: undefined,
            lastRunSummary: "",
            lastError: "",
            config: "{}",
            supportedActions: [],
            upcomingRuns: [],
            createdAt: "",
            updatedAt: "",
          },
          {
            jobKey: "scheduler.sync",
            name: "Sync",
            schedule: "0 * * * *",
            enabled: true,
            scope: "system",
            executionMode: "single",
            overlapPolicy: "skip",
            controlState: "active",
            activeRun: {
              runId: "run-1",
              triggerSource: "manual",
              status: "running",
              startedAt: "2026-03-30T11:59:00.000Z",
              summary: "Syncing",
              errorMessage: "",
            },
            lastRunStatus: "succeeded",
            nextRunAt: "2026-03-30T13:00:00.000Z",
            lastRunSummary: "",
            lastError: "",
            config: "{}",
            supportedActions: [],
            upcomingRuns: [],
            createdAt: "",
            updatedAt: "",
          },
        ]}
        selectedJobKey="scheduler.cleanup"
        loading={false}
        onSelectJob={onSelectJob}
      />,
    );

    expect(screen.getByText("Cleanup")).toBeInTheDocument();
    expect(screen.getByText("scheduler.cleanup")).toBeInTheDocument();
    expect(screen.getByText("Paused")).toBeInTheDocument();
    expect(screen.getByText("running")).toBeInTheDocument();
    expect(screen.getByText("N/A")).toBeInTheDocument();
    expect(container.querySelector('tr[data-state="selected"]')).toBeInTheDocument();

    await user.click(screen.getByText("Cleanup"));
    expect(onSelectJob).toHaveBeenCalledWith("scheduler.cleanup");
  });
});
