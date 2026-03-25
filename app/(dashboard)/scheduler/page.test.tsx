import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import SchedulerPage from "./page";

const fetchJobs = jest.fn();
const fetchRuns = jest.fn();
const updateJob = jest.fn();
const triggerJob = jest.fn();
const selectJob = jest.fn();
const setDraftSchedule = jest.fn();

const storeState = {
  jobs: [
    {
      jobKey: "task-progress-detector",
      name: "Task progress detector",
      scope: "system",
      schedule: "*/5 * * * *",
      enabled: true,
      executionMode: "in_process",
      overlapPolicy: "skip",
      lastRunStatus: "succeeded",
      lastRunAt: "2026-03-25T10:00:00.000Z",
      nextRunAt: "2026-03-25T10:05:00.000Z",
      lastRunSummary: "checked 12 tasks",
      lastError: "",
      config: "{}",
      createdAt: "2026-03-25T10:00:00.000Z",
      updatedAt: "2026-03-25T10:00:00.000Z",
    },
  ],
  runsByJobKey: {
    "task-progress-detector": [
      {
        runId: "run-1",
        jobKey: "task-progress-detector",
        triggerSource: "manual",
        status: "succeeded",
        startedAt: "2026-03-25T10:00:00.000Z",
        finishedAt: "2026-03-25T10:00:03.000Z",
        summary: "checked 12 tasks",
        errorMessage: "",
        metrics: "{}",
        createdAt: "2026-03-25T10:00:00.000Z",
        updatedAt: "2026-03-25T10:00:03.000Z",
      },
    ],
  },
  draftSchedules: {
    "task-progress-detector": "*/5 * * * *",
  },
  selectedJobKey: "task-progress-detector",
  loading: false,
  actionJobKey: null,
  error: null,
  fetchJobs,
  fetchRuns,
  updateJob,
  triggerJob,
  selectJob,
  setDraftSchedule,
  upsertJob: jest.fn(),
  recordRun: jest.fn(),
};

jest.mock("@/lib/stores/scheduler-store", () => ({
  useSchedulerStore: (selector: (state: typeof storeState) => unknown) => selector(storeState),
}));

describe("SchedulerPage", () => {
  beforeEach(() => {
    fetchJobs.mockReset();
    fetchRuns.mockReset();
    updateJob.mockReset();
    triggerJob.mockReset();
    selectJob.mockReset();
    setDraftSchedule.mockReset();
  });

  it("loads jobs and renders scheduler management controls", () => {
    render(<SchedulerPage />);

    expect(fetchJobs).toHaveBeenCalled();
    expect(fetchRuns).toHaveBeenCalledWith("task-progress-detector");
    expect(screen.getByText("Scheduler Control Plane")).toBeInTheDocument();
    expect(screen.getAllByText("Task progress detector").length).toBeGreaterThan(0);
    expect(screen.getByRole("button", { name: "Run now" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Disable job" })).toBeInTheDocument();
  });

  it("updates the draft schedule and can trigger a manual run", async () => {
    const user = userEvent.setup();
    render(<SchedulerPage />);

    await user.clear(screen.getByLabelText("Schedule expression"));
    await user.type(screen.getByLabelText("Schedule expression"), "0 * * * *");
    await user.click(screen.getByRole("button", { name: "Run now" }));

    expect(setDraftSchedule).toHaveBeenCalled();
    expect(triggerJob).toHaveBeenCalledWith("task-progress-detector");
  });
});
