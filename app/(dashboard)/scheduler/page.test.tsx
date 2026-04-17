import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import SchedulerPage from "./page";

const fetchJobs = jest.fn();
const fetchRuns = jest.fn();
const fetchStats = jest.fn();
const updateJob = jest.fn();
const createJob = jest.fn().mockResolvedValue(true);
const triggerJob = jest.fn();
const selectJob = jest.fn();
const setDraftSchedule = jest.fn();
const setListFilters = jest.fn();
const resetListFilters = jest.fn();

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
        durationMs: 3000,
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
  stats: {
    totalJobs: 6,
    enabledJobs: 5,
    disabledJobs: 1,
    failedJobs: 0,
    activeRuns: 0,
    totalRuns24h: 42,
    failedRuns24h: 0,
  },
  loading: false,
  actionJobKey: null,
  error: null as string | null,
  listFilters: { status: "all", scope: "all" },
  fetchJobs,
  fetchRuns,
  fetchStats,
  updateJob,
  createJob,
  triggerJob,
  selectJob,
  setDraftSchedule,
  setListFilters,
  resetListFilters,
  upsertJob: jest.fn(),
  recordRun: jest.fn(),
};

jest.mock("@/lib/stores/scheduler-store", () => ({
  useSchedulerStore: (selector: (state: typeof storeState) => unknown) => selector(storeState),
  filterSchedulerJobs: (jobs: unknown[]) => jobs,
  DEFAULT_SCHEDULER_JOB_LIST_FILTERS: { status: "all", scope: "all" },
}));

jest.mock("@/components/scheduler/scheduler-stats-cards", () => ({
  SchedulerStatsCards: () => <div data-testid="scheduler-stats" />,
}));

jest.mock("@/components/scheduler/scheduler-job-table", () => ({
  SchedulerJobTable: ({
    onSelectJob,
  }: {
    onSelectJob: (jobKey: string) => void;
  }) => (
    <button type="button" onClick={() => onSelectJob("task-progress-detector")}>
      select-job
    </button>
  ),
}));

jest.mock("@/components/scheduler/scheduler-job-filters", () => ({
  SchedulerJobFilters: ({
    onFiltersChange,
    onReset,
  }: {
    onFiltersChange: (f: { status: string }) => void;
    onReset: () => void;
  }) => (
    <div>
      <button type="button" onClick={() => onFiltersChange({ status: "failed" })}>
        filter-status
      </button>
      <button type="button" onClick={onReset}>
        reset-filters
      </button>
    </div>
  ),
}));

jest.mock("@/components/scheduler/scheduler-job-create-dialog", () => ({
  SchedulerJobCreateDialog: ({
    open,
    onCreate,
    onOpenChange,
  }: {
    open: boolean;
    onCreate: (input: { jobKey: string; name: string; schedule: string }) => Promise<boolean>;
    onOpenChange: (open: boolean) => void;
  }) =>
    open ? (
      <div>
        <button
          type="button"
          onClick={async () => {
            await onCreate({ jobKey: "new", name: "New", schedule: "0 * * * *" });
            onOpenChange(false);
          }}
        >
          submit-create
        </button>
      </div>
    ) : null,
}));

jest.mock("@/components/scheduler/scheduler-upcoming-calendar", () => ({
  SchedulerUpcomingCalendar: () => <div data-testid="scheduler-calendar" />,
}));

jest.mock("@/components/scheduler/scheduler-job-detail", () => ({
  SchedulerJobDetail: ({
    onUpdateJob,
    onTriggerJob,
    onSetDraftSchedule,
  }: {
    onUpdateJob: (input: { enabled: boolean }) => void;
    onTriggerJob: () => void;
    onSetDraftSchedule: (schedule: string) => void;
  }) => (
    <div>
      <button type="button" onClick={() => onSetDraftSchedule("0 * * * *")}>
        set-schedule
      </button>
      <button type="button" onClick={() => onUpdateJob({ enabled: false })}>
        update-job
      </button>
      <button type="button" onClick={onTriggerJob}>
        Run now
      </button>
    </div>
  ),
  SchedulerJobDetailEmpty: () => <div data-testid="scheduler-job-detail-empty" />,
}));

jest.mock("@/components/shared/error-banner", () => ({
  ErrorBanner: ({
    message,
    onRetry,
  }: {
    message: string;
    onRetry: () => void;
  }) => (
    <button type="button" onClick={onRetry}>
      {message}
    </button>
  ),
}));

describe("SchedulerPage", () => {
  beforeEach(() => {
    fetchJobs.mockReset();
    fetchRuns.mockReset();
    fetchStats.mockReset();
    updateJob.mockReset();
    createJob.mockReset().mockResolvedValue(true);
    triggerJob.mockReset();
    selectJob.mockReset();
    setDraftSchedule.mockReset();
    setListFilters.mockReset();
    resetListFilters.mockReset();
    storeState.error = null;
    storeState.selectedJobKey = "task-progress-detector";
    storeState.listFilters = { status: "all", scope: "all" };
  });

  it("loads jobs and renders scheduler management controls", () => {
    render(<SchedulerPage />);

    expect(fetchJobs).toHaveBeenCalled();
    expect(fetchStats).toHaveBeenCalled();
    expect(fetchRuns).toHaveBeenCalledWith("task-progress-detector");
    expect(screen.getByText("Scheduler Control Plane")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "select-job" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Run now" })).toBeInTheDocument();
  });

  it("updates the draft schedule and can trigger a manual run", async () => {
    const user = userEvent.setup();
    render(<SchedulerPage />);

    await user.click(screen.getByRole("button", { name: "set-schedule" }));
    await user.click(screen.getByRole("button", { name: "Run now" }));

    expect(setDraftSchedule).toHaveBeenCalledWith("task-progress-detector", "0 * * * *");
    expect(triggerJob).toHaveBeenCalledWith("task-progress-detector");
  });

  it("refreshes the page, selects a job, updates it, and retries from the error banner", async () => {
    const user = userEvent.setup();
    storeState.error = "scheduler unavailable";

    render(<SchedulerPage />);

    await user.click(screen.getByRole("button", { name: "Refresh" }));
    await user.click(screen.getByRole("button", { name: "scheduler unavailable" }));
    await user.click(screen.getByRole("button", { name: "select-job" }));
    await user.click(screen.getByRole("button", { name: "update-job" }));

    expect(fetchJobs).toHaveBeenCalledTimes(3);
    expect(fetchStats).toHaveBeenCalledTimes(3);
    expect(selectJob).toHaveBeenCalledWith("task-progress-detector");
    expect(updateJob).toHaveBeenCalledWith("task-progress-detector", { enabled: false });
  });

  it("opens the create dialog, submits a new job, and applies list filters", async () => {
    const user = userEvent.setup();
    render(<SchedulerPage />);

    await user.click(screen.getByRole("button", { name: "filter-status" }));
    await user.click(screen.getByRole("button", { name: "reset-filters" }));
    expect(setListFilters).toHaveBeenCalledWith({ status: "failed" });
    expect(resetListFilters).toHaveBeenCalled();

    await user.click(screen.getByRole("button", { name: /Create Job/i }));
    await user.click(screen.getByRole("button", { name: "submit-create" }));
    expect(createJob).toHaveBeenCalledWith({
      jobKey: "new",
      name: "New",
      schedule: "0 * * * *",
    });
  });
});
