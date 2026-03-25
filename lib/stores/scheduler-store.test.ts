import { useSchedulerStore } from "./scheduler-store";

const mockGet = jest.fn();
const mockPost = jest.fn();
const mockPut = jest.fn();

jest.mock("@/lib/api-client", () => ({
  createApiClient: () => ({
    get: mockGet,
    post: mockPost,
    put: mockPut,
  }),
}));

jest.mock("./auth-store", () => ({
  useAuthStore: { getState: () => ({ accessToken: "test-token" }) },
}));

beforeEach(() => {
  useSchedulerStore.setState({
    jobs: [],
    runsByJobKey: {},
    draftSchedules: {},
    loading: false,
    actionJobKey: null,
    selectedJobKey: null,
    error: null,
  });
  mockGet.mockReset();
  mockPost.mockReset();
  mockPut.mockReset();
});

describe("useSchedulerStore", () => {
  it("fetches scheduler jobs", async () => {
    const jobs = [
      {
        jobKey: "task-progress-detector",
        name: "Task progress detector",
        scope: "system",
        schedule: "*/5 * * * *",
        enabled: true,
        executionMode: "in_process",
        overlapPolicy: "skip",
        lastRunStatus: "succeeded",
        lastRunSummary: "checked 12 tasks",
        lastError: "",
        config: "{}",
        createdAt: "2026-03-25T10:00:00.000Z",
        updatedAt: "2026-03-25T10:00:00.000Z",
      },
    ];
    mockGet.mockResolvedValueOnce({ data: jobs });

    await useSchedulerStore.getState().fetchJobs();

    expect(mockGet).toHaveBeenCalledWith("/api/v1/scheduler/jobs", {
      token: "test-token",
    });
    expect(useSchedulerStore.getState().jobs).toEqual(jobs);
    expect(useSchedulerStore.getState().selectedJobKey).toBe("task-progress-detector");
  });

  it("updates a scheduler job and merges the response", async () => {
    useSchedulerStore.setState({
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
          lastRunSummary: "",
          lastError: "",
          config: "{}",
          createdAt: "2026-03-25T10:00:00.000Z",
          updatedAt: "2026-03-25T10:00:00.000Z",
        },
      ],
      runsByJobKey: {},
      draftSchedules: { "task-progress-detector": "*/5 * * * *" },
      loading: false,
      actionJobKey: null,
      selectedJobKey: "task-progress-detector",
      error: null,
    });

    mockPut.mockResolvedValueOnce({
      data: {
        jobKey: "task-progress-detector",
        name: "Task progress detector",
        scope: "system",
        schedule: "0 * * * *",
        enabled: false,
        executionMode: "in_process",
        overlapPolicy: "skip",
        lastRunStatus: "succeeded",
        lastRunSummary: "",
        lastError: "",
        config: "{}",
        createdAt: "2026-03-25T10:00:00.000Z",
        updatedAt: "2026-03-25T11:00:00.000Z",
      },
    });

    await useSchedulerStore
      .getState()
      .updateJob("task-progress-detector", { enabled: false, schedule: "0 * * * *" });

    expect(mockPut).toHaveBeenCalledWith(
      "/api/v1/scheduler/jobs/task-progress-detector",
      { enabled: false, schedule: "0 * * * *" },
      { token: "test-token" }
    );
    expect(useSchedulerStore.getState().jobs[0]?.enabled).toBe(false);
    expect(useSchedulerStore.getState().jobs[0]?.schedule).toBe("0 * * * *");
  });

  it("triggers a manual run and prepends history for the selected job", async () => {
    useSchedulerStore.setState({
      jobs: [],
      runsByJobKey: { "task-progress-detector": [] },
      draftSchedules: {},
      loading: false,
      actionJobKey: null,
      selectedJobKey: "task-progress-detector",
      error: null,
    });

    mockPost.mockResolvedValueOnce({
      data: {
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
    });

    await useSchedulerStore.getState().triggerJob("task-progress-detector");

    expect(mockPost).toHaveBeenCalledWith(
      "/api/v1/scheduler/jobs/task-progress-detector/trigger",
      {},
      { token: "test-token" }
    );
    expect(useSchedulerStore.getState().runsByJobKey["task-progress-detector"]).toHaveLength(1);
    expect(useSchedulerStore.getState().runsByJobKey["task-progress-detector"]?.[0]?.runId).toBe("run-1");
  });

  it("fetches scheduler run history for the selected job", async () => {
    mockGet.mockResolvedValueOnce({
      data: [
        {
          runId: "run-2",
          jobKey: "task-progress-detector",
          triggerSource: "cron",
          status: "failed",
          startedAt: "2026-03-25T11:00:00.000Z",
          finishedAt: "2026-03-25T11:00:03.000Z",
          summary: "bridge offline",
          errorMessage: "bridge offline",
          metrics: "{}",
          createdAt: "2026-03-25T11:00:00.000Z",
          updatedAt: "2026-03-25T11:00:03.000Z",
        },
      ],
    });

    await useSchedulerStore.getState().fetchRuns("task-progress-detector");

    expect(mockGet).toHaveBeenCalledWith(
      "/api/v1/scheduler/jobs/task-progress-detector/runs",
      { token: "test-token" }
    );
    expect(useSchedulerStore.getState().runsByJobKey["task-progress-detector"]).toEqual([
      expect.objectContaining({
        runId: "run-2",
        status: "failed",
      }),
    ]);
  });
});
