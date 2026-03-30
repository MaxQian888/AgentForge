jest.mock("next-intl", () => ({
  useTranslations: () => (
    key: string,
    values?: Record<string, string>,
  ) => {
    const map: Record<string, string> = {
      "jobDetail.runNow": "Run Now",
      "jobDetail.disable": "Disable",
      "jobDetail.enable": "Enable",
      "jobDetail.tabOverview": "Overview",
      "jobDetail.tabHistory": "History",
      "jobDetail.tabConfig": "Config",
      "jobDetail.lastRun": "Last Run",
      "jobDetail.nextRun": "Next Run",
      "jobDetail.notScheduled": "Not scheduled",
      "jobDetail.scope": "Scope",
      "jobDetail.overlapPolicy": "Overlap Policy",
      "jobDetail.lastSummary": "Last Summary",
      "jobDetail.lastError": "Last Error",
      "jobDetail.scheduleExpression": "Schedule Expression",
      "jobDetail.save": "Save",
      "jobDetail.disableJobTitle": "Disable Job",
      "jobDetail.enableJobTitle": "Enable Job",
      "jobDetail.cancel": "Cancel",
      "jobDetail.selectJob": "Select a job",
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

import { render, screen } from "@testing-library/react";
import { fireEvent, within } from "@testing-library/react";
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

  it("renders overview information, saves schedule edits, and triggers runs", async () => {
    const user = userEvent.setup();
    const onUpdateJob = jest.fn();
    const onTriggerJob = jest.fn();
    const onSetDraftSchedule = jest.fn();

    render(
      <SchedulerJobDetail
        job={{
          jobKey: "scheduler.cleanup",
          name: "Cleanup",
          scope: "project",
          schedule: "0 0 * * *",
          enabled: true,
          executionMode: "single",
          overlapPolicy: "skip",
          lastRunStatus: "succeeded",
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
        onSetDraftSchedule={onSetDraftSchedule}
      />,
    );

    expect(screen.getByText("Cleanup")).toBeInTheDocument();
    expect(screen.getByText("scheduler.cleanup")).toBeInTheDocument();
    expect(screen.getByText("Finished cleanly")).toBeInTheDocument();
    expect(screen.getByText("Previous failure")).toBeInTheDocument();
    expect(screen.getByText("Every 15 minutes")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Run Now" }));
    expect(onTriggerJob).toHaveBeenCalled();

    await user.click(screen.getByRole("button", { name: "Save" }));
    expect(onUpdateJob).toHaveBeenCalledWith({ schedule: "*/15 * * * *" });

    const scheduleInput = screen.getByRole("textbox", {
      name: "Schedule expression",
    });
    fireEvent.change(scheduleInput, { target: { value: "0 */6 * * *" } });
    expect(onSetDraftSchedule).toHaveBeenLastCalledWith("0 */6 * * *");
  });

  it("switches to config and history tabs and confirms disable actions", async () => {
    const user = userEvent.setup();
    const onUpdateJob = jest.fn();

    render(
      <SchedulerJobDetail
        job={{
          jobKey: "scheduler.cleanup",
          name: "Cleanup",
          scope: "project",
          schedule: "0 0 * * *",
          enabled: true,
          executionMode: "single",
          overlapPolicy: "skip",
          lastRunStatus: "failed",
          lastRunAt: "2026-03-30T11:00:00.000Z",
          nextRunAt: undefined,
          lastRunSummary: "",
          lastError: "",
          config: '{"retries":2}',
          createdAt: "",
          updatedAt: "",
        }}
        runs={[
          {
            runId: "run-1",
            jobKey: "scheduler.cleanup",
            triggerSource: "manual",
            status: "failed",
            startedAt: "2026-03-30T11:00:00.000Z",
            durationMs: 1500,
            summary: "",
            errorMessage: "Bridge unavailable",
            metrics: "{}",
            createdAt: "",
            updatedAt: "",
          },
        ]}
        draftSchedule="0 0 * * *"
        actionLoading={false}
        onUpdateJob={onUpdateJob}
        onTriggerJob={jest.fn()}
        onSetDraftSchedule={jest.fn()}
      />,
    );

    await user.click(screen.getByRole("tab", { name: /History/i }));
    expect(screen.getByText("Bridge unavailable")).toBeInTheDocument();

    await user.click(screen.getByRole("tab", { name: "Config" }));
    expect(screen.getByText(/"retries": 2/)).toBeInTheDocument();

    await user.click(screen.getAllByRole("button", { name: "Disable" })[0]);
    expect(screen.getByText("Disable Job")).toBeInTheDocument();
    expect(screen.getByText("Stop Cleanup")).toBeInTheDocument();

    const dialog = screen.getByRole("dialog");
    await user.click(within(dialog).getByRole("button", { name: "Disable" }));
    expect(onUpdateJob).toHaveBeenCalledWith({ enabled: false });
  });
});

describe("SchedulerJobDetailEmpty", () => {
  it("renders the empty selection message", () => {
    render(<SchedulerJobDetailEmpty />);
    expect(screen.getByText("Select a job")).toBeInTheDocument();
  });
});
