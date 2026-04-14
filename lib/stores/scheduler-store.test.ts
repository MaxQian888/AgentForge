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
  useAuthStore: { getState: jest.fn(() => ({ accessToken: "test-token" })) },
}));

import { useSchedulerStore } from "./scheduler-store";

const authStoreModule = jest.requireMock("./auth-store") as {
  useAuthStore: {
    getState: jest.Mock<{ accessToken: string | null }, []>;
  };
};

beforeEach(() => {
  useSchedulerStore.setState({
    jobs: [],
    runsByJobKey: {},
    draftSchedules: {},
    stats: null,
    loading: false,
    actionJobKey: null,
    selectedJobKey: null,
    error: null,
  });
  mockGet.mockReset();
  mockPost.mockReset();
  mockPut.mockReset();
  authStoreModule.useAuthStore.getState.mockReturnValue({
    accessToken: "test-token",
  });
});

describe("useSchedulerStore", () => {
  it("fetches scheduler jobs, sorts them, and derives stats", async () => {
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
        controlState: "active",
        activeRun: {
          runId: "run-active",
          triggerSource: "manual",
          status: "running",
          startedAt: "2026-03-25T10:00:00.000Z",
          summary: "",
          errorMessage: "",
        },
        supportedActions: [{ action: "pause", enabled: true }],
        configMetadata: { editable: true, fields: [] },
        upcomingRuns: [{ runAt: "2026-03-25T10:05:00.000Z" }],
        createdAt: "2026-03-25T10:00:00.000Z",
        updatedAt: "2026-03-25T10:00:00.000Z",
      },
      {
        jobKey: "agent-report",
        name: "Agent report",
        scope: "system",
        schedule: "0 * * * *",
        enabled: false,
        executionMode: "in_process",
        overlapPolicy: "skip",
        lastRunStatus: "failed",
        lastRunSummary: "bridge offline",
        lastError: "bridge offline",
        config: "{}",
        controlState: "paused",
        supportedActions: [{ action: "resume", enabled: true }],
        configMetadata: { editable: true, fields: [] },
        upcomingRuns: [],
        createdAt: "2026-03-25T10:00:00.000Z",
        updatedAt: "2026-03-25T10:00:00.000Z",
      },
    ];
    mockGet.mockResolvedValueOnce({ data: jobs });

    await useSchedulerStore.getState().fetchJobs();

    expect(mockGet).toHaveBeenCalledWith("/api/v1/scheduler/jobs", {
      token: "test-token",
    });
    expect(useSchedulerStore.getState().jobs.map((job) => job.jobKey)).toEqual([
      "agent-report",
      "task-progress-detector",
    ]);
    expect(useSchedulerStore.getState()).toMatchObject({
      selectedJobKey: "agent-report",
      draftSchedules: {
        "agent-report": "0 * * * *",
        "task-progress-detector": "*/5 * * * *",
      },
      stats: {
        totalJobs: 2,
        enabledJobs: 1,
        disabledJobs: 1,
        pausedJobs: 1,
        failedJobs: 1,
        activeRuns: 1,
        queueDepth: 1,
      },
    });
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
      stats: null,
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
      { token: "test-token" },
    );
    expect(useSchedulerStore.getState().jobs[0]?.enabled).toBe(false);
    expect(useSchedulerStore.getState().jobs[0]?.schedule).toBe("0 * * * *");
  });

  it("stores update failures and clears the active action key", async () => {
    mockPut.mockRejectedValueOnce(new Error("boom"));

    await useSchedulerStore
      .getState()
      .updateJob("task-progress-detector", { enabled: false });

    expect(useSchedulerStore.getState()).toMatchObject({
      actionJobKey: null,
      error: "Unable to update scheduler job",
    });
  });

  it("triggers a manual run and prepends history for the selected job", async () => {
    useSchedulerStore.setState({
      jobs: [],
      runsByJobKey: { "task-progress-detector": [] },
      draftSchedules: {},
      stats: null,
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
      { token: "test-token" },
    );
    expect(
      useSchedulerStore.getState().runsByJobKey["task-progress-detector"],
    ).toHaveLength(1);
    expect(
      useSchedulerStore.getState().runsByJobKey["task-progress-detector"]?.[0]
        ?.runId,
    ).toBe("run-1");
  });

  it("stores trigger failures and clears the active action key", async () => {
    mockPost.mockRejectedValueOnce(new Error("boom"));

    await useSchedulerStore.getState().triggerJob("task-progress-detector");

    expect(useSchedulerStore.getState()).toMatchObject({
      actionJobKey: null,
      error: "Unable to trigger scheduler job",
    });
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
      { token: "test-token" },
    );
    expect(useSchedulerStore.getState().runsByJobKey["task-progress-detector"]).toEqual([
      expect.objectContaining({
        runId: "run-2",
        status: "failed",
      }),
    ]);
  });

  it("fetches filtered scheduler run history using query parameters", async () => {
    mockGet.mockResolvedValueOnce({ data: [] });

    await useSchedulerStore.getState().fetchRuns("task-progress-detector", {
      status: "failed",
      triggerSource: "manual",
      since: "2026-03-25T09:00:00.000Z",
      before: "2026-03-25T12:00:00.000Z",
      limit: 5,
    });

    expect(mockGet).toHaveBeenCalledWith(
      "/api/v1/scheduler/jobs/task-progress-detector/runs?status=failed&triggerSource=manual&since=2026-03-25T09%3A00%3A00.000Z&before=2026-03-25T12%3A00%3A00.000Z&limit=5",
      { token: "test-token" },
    );
  });

  it("stores run-history failures", async () => {
    mockGet.mockRejectedValueOnce(new Error("boom"));

    await useSchedulerStore.getState().fetchRuns("task-progress-detector");

    expect(useSchedulerStore.getState().error).toBe(
      "Unable to load scheduler run history",
    );
  });

  it("fetches scheduler stats and ignores stats failures", async () => {
    mockGet
      .mockResolvedValueOnce({
        data: {
          totalJobs: 4,
        enabledJobs: 3,
        disabledJobs: 1,
        pausedJobs: 1,
        failedJobs: 1,
        activeRuns: 2,
        queueDepth: 2,
        totalRuns24h: 9,
        successfulRuns24h: 7,
        failedRuns24h: 2,
        averageDurationMs: 1500,
        successRate24h: 77.7,
      },
    })
      .mockRejectedValueOnce(new Error("boom"));

    await useSchedulerStore.getState().fetchStats();
    expect(useSchedulerStore.getState().stats).toEqual(
      expect.objectContaining({
        totalJobs: 4,
        activeRuns: 2,
        successRate24h: 77.7,
      }),
    );

    await useSchedulerStore.getState().fetchStats();
    expect(useSchedulerStore.getState().stats).toEqual(
      expect.objectContaining({
        totalJobs: 4,
        activeRuns: 2,
      }),
    );
  });

  it("updates draft schedules, upserts jobs, and replaces existing runs in place", () => {
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
          controlState: "active",
          supportedActions: [],
          upcomingRuns: [],
          createdAt: "2026-03-25T10:00:00.000Z",
          updatedAt: "2026-03-25T10:00:00.000Z",
        },
      ],
      runsByJobKey: {
        "task-progress-detector": [
          {
            runId: "run-1",
            jobKey: "task-progress-detector",
            triggerSource: "cron",
            status: "running",
            startedAt: "2026-03-25T10:00:00.000Z",
            summary: "",
            errorMessage: "",
            metrics: "{}",
            createdAt: "2026-03-25T10:00:00.000Z",
            updatedAt: "2026-03-25T10:00:00.000Z",
          },
        ],
      },
      draftSchedules: { "task-progress-detector": "0 * * * *" },
      selectedJobKey: null,
      stats: null,
      loading: false,
      actionJobKey: null,
      error: null,
    });

    useSchedulerStore.getState().upsertJob({
      jobKey: "task-progress-detector",
      name: "Task progress detector",
      scope: "system",
      schedule: "*/10 * * * *",
      enabled: true,
      executionMode: "in_process",
      overlapPolicy: "skip",
      lastRunStatus: "failed",
      lastRunSummary: "failed",
      lastError: "failed",
      config: "{}",
      controlState: "active",
      supportedActions: [],
      upcomingRuns: [],
      createdAt: "2026-03-25T10:00:00.000Z",
      updatedAt: "2026-03-25T10:05:00.000Z",
    });
    useSchedulerStore.getState().recordRun({
      runId: "run-1",
      jobKey: "task-progress-detector",
      triggerSource: "cron",
      status: "failed",
      startedAt: "2026-03-25T10:00:00.000Z",
      finishedAt: "2026-03-25T10:01:00.000Z",
      summary: "failed",
      errorMessage: "failed",
      metrics: "{}",
      createdAt: "2026-03-25T10:00:00.000Z",
      updatedAt: "2026-03-25T10:01:00.000Z",
    });
    useSchedulerStore
      .getState()
      .setDraftSchedule("task-progress-detector", "manual draft");

    expect(useSchedulerStore.getState()).toMatchObject({
      draftSchedules: {
        "task-progress-detector": "manual draft",
      },
      jobs: [
        expect.objectContaining({
          lastRunStatus: "failed",
          schedule: "*/10 * * * *",
        }),
      ],
      runsByJobKey: {
        "task-progress-detector": [
          expect.objectContaining({
            runId: "run-1",
            status: "failed",
          }),
        ],
      },
    });
  });

  it("returns early without an access token or job key", async () => {
    authStoreModule.useAuthStore.getState.mockReturnValue({
      accessToken: null,
    });

    await useSchedulerStore.getState().fetchJobs();
    await useSchedulerStore.getState().fetchRuns("task-progress-detector");
    await useSchedulerStore.getState().fetchStats();
    await useSchedulerStore
      .getState()
      .updateJob("task-progress-detector", { enabled: false });
    await useSchedulerStore.getState().triggerJob("task-progress-detector");

    authStoreModule.useAuthStore.getState.mockReturnValue({
      accessToken: "test-token",
    });
    await useSchedulerStore.getState().fetchRuns("");
    await useSchedulerStore.getState().updateJob("", { enabled: false });
    await useSchedulerStore.getState().triggerJob("");

    expect(mockGet).not.toHaveBeenCalled();
    expect(mockPost).not.toHaveBeenCalled();
    expect(mockPut).not.toHaveBeenCalled();
  });

  it("uses dedicated pause, resume, cancel, and cleanup endpoints", async () => {
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
          lastRunStatus: "running",
          lastRunSummary: "",
          lastError: "",
          config: "{}",
          controlState: "active",
          supportedActions: [
            { action: "pause", enabled: true },
            { action: "resume", enabled: false, reason: "job is already active" },
            { action: "cancel", enabled: true },
            { action: "cleanup", enabled: true },
          ],
          activeRun: {
            runId: "run-1",
            triggerSource: "manual",
            status: "running",
            startedAt: "2026-03-25T10:00:00.000Z",
            summary: "",
            errorMessage: "",
          },
          configMetadata: { editable: true, fields: [] },
          upcomingRuns: [],
          createdAt: "2026-03-25T10:00:00.000Z",
          updatedAt: "2026-03-25T10:00:00.000Z",
        },
      ],
      runsByJobKey: { "task-progress-detector": [] },
      draftSchedules: {},
      stats: null,
      loading: false,
      actionJobKey: null,
      selectedJobKey: "task-progress-detector",
      error: null,
    });

    mockPost
      .mockResolvedValueOnce({
        data: {
          jobKey: "task-progress-detector",
          name: "Task progress detector",
          scope: "system",
          schedule: "*/5 * * * *",
          enabled: false,
          executionMode: "in_process",
          overlapPolicy: "skip",
          lastRunStatus: "running",
          lastRunSummary: "",
          lastError: "",
          config: "{}",
          controlState: "paused",
          supportedActions: [{ action: "resume", enabled: true }],
          configMetadata: { editable: true, fields: [] },
          upcomingRuns: [],
          createdAt: "2026-03-25T10:00:00.000Z",
          updatedAt: "2026-03-25T10:05:00.000Z",
        },
      })
      .mockResolvedValueOnce({
        data: {
          jobKey: "task-progress-detector",
          name: "Task progress detector",
          scope: "system",
          schedule: "*/5 * * * *",
          enabled: true,
          executionMode: "in_process",
          overlapPolicy: "skip",
          lastRunStatus: "running",
          lastRunSummary: "",
          lastError: "",
          config: "{}",
          controlState: "active",
          supportedActions: [{ action: "pause", enabled: true }],
          configMetadata: { editable: true, fields: [] },
          upcomingRuns: [],
          createdAt: "2026-03-25T10:00:00.000Z",
          updatedAt: "2026-03-25T10:06:00.000Z",
        },
      })
      .mockResolvedValueOnce({
        data: {
          runId: "run-1",
          jobKey: "task-progress-detector",
          triggerSource: "manual",
          status: "cancel_requested",
          startedAt: "2026-03-25T10:00:00.000Z",
          summary: "cancellation requested",
          errorMessage: "",
          metrics: "{}",
          createdAt: "2026-03-25T10:00:00.000Z",
          updatedAt: "2026-03-25T10:02:00.000Z",
        },
      })
      .mockResolvedValueOnce({ data: { deleted: 2 } });
    mockGet.mockResolvedValue({ data: [] });

    await useSchedulerStore.getState().pauseJob("task-progress-detector");
    await useSchedulerStore.getState().resumeJob("task-progress-detector");
    await useSchedulerStore.getState().cancelJob("task-progress-detector");
    await useSchedulerStore
      .getState()
      .cleanupRuns("task-progress-detector", { retainRecent: 10 });

    expect(mockPost).toHaveBeenNthCalledWith(
      1,
      "/api/v1/scheduler/jobs/task-progress-detector/pause",
      {},
      { token: "test-token" },
    );
    expect(mockPost).toHaveBeenNthCalledWith(
      2,
      "/api/v1/scheduler/jobs/task-progress-detector/resume",
      {},
      { token: "test-token" },
    );
    expect(mockPost).toHaveBeenNthCalledWith(
      3,
      "/api/v1/scheduler/jobs/task-progress-detector/cancel",
      {},
      { token: "test-token" },
    );
    expect(mockPost).toHaveBeenNthCalledWith(
      4,
      "/api/v1/scheduler/jobs/task-progress-detector/runs/cleanup",
      { retainRecent: 10 },
      { token: "test-token" },
    );
  });

  it("surfaces unsupported action reasons without calling the API", async () => {
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
          controlState: "active",
          supportedActions: [{ action: "resume", enabled: false, reason: "job is already active" }],
          configMetadata: { editable: true, fields: [] },
          upcomingRuns: [],
          createdAt: "2026-03-25T10:00:00.000Z",
          updatedAt: "2026-03-25T10:00:00.000Z",
        },
      ],
      runsByJobKey: {},
      draftSchedules: {},
      stats: null,
      loading: false,
      actionJobKey: null,
      selectedJobKey: null,
      error: null,
    });

    await useSchedulerStore.getState().resumeJob("task-progress-detector");

    expect(mockPost).not.toHaveBeenCalled();
    expect(useSchedulerStore.getState().error).toBe("job is already active");
  });
});
