jest.mock("@/lib/stores/auth-store", () => ({
  useAuthStore: {
    getState: jest.fn(() => ({ accessToken: "test-token" })),
  },
}));

import { useSprintStore } from "./sprint-store";

const authStoreModule = jest.requireMock("@/lib/stores/auth-store") as {
  useAuthStore: {
    getState: jest.Mock<{ accessToken: string | null }, []>;
  };
};

describe("useSprintStore", () => {
  const fetchMock = jest.fn();
  const mockJsonResponse = (data: unknown, status = 200) =>
    ({
      ok: status >= 200 && status < 300,
      status,
      json: async () => data,
    }) as Response;

  beforeEach(() => {
    fetchMock.mockReset();
    global.fetch = fetchMock as unknown as typeof fetch;
    authStoreModule.useAuthStore.getState.mockReturnValue({
      accessToken: "test-token",
    });
    useSprintStore.setState({
      sprintsByProject: {},
      metricsBySprintId: {},
      loadingByProject: {},
      metricsLoadingBySprintId: {},
      errorByProject: {},
      metricsErrorBySprintId: {},
    });
  });

  it("fetches sprints for a project", async () => {
    fetchMock.mockResolvedValueOnce(
      mockJsonResponse([
        {
          id: "sprint-1",
          projectId: "project-1",
          name: "Sprint Alpha",
          startDate: "2026-03-24T00:00:00.000Z",
          endDate: "2026-03-30T23:59:59.000Z",
          status: "active",
          totalBudgetUsd: 20,
          spentUsd: 8,
          createdAt: "2026-03-20T10:00:00.000Z",
        },
      ])
    );

    await useSprintStore.getState().fetchSprints("project-1");

    expect(useSprintStore.getState().sprintsByProject["project-1"]).toEqual([
      expect.objectContaining({
        id: "sprint-1",
        status: "active",
      }),
    ]);
  });

  it("creates a sprint and adds it to state", async () => {
    const newSprint = {
      id: "sprint-new",
      projectId: "project-1",
      name: "Sprint Beta",
      startDate: "2026-04-01T00:00:00.000Z",
      endDate: "2026-04-14T23:59:59.000Z",
      status: "planning" as const,
      totalBudgetUsd: 50,
      spentUsd: 0,
      createdAt: "2026-03-24T10:00:00.000Z",
    };

    fetchMock.mockResolvedValueOnce(mockJsonResponse(newSprint));

    const result = await useSprintStore.getState().createSprint("project-1", {
      name: "Sprint Beta",
      startDate: "2026-04-01T00:00:00.000Z",
      endDate: "2026-04-14T23:59:59.000Z",
      totalBudgetUsd: 50,
    });

    expect(result).toEqual(expect.objectContaining({ id: "sprint-new" }));
    expect(useSprintStore.getState().sprintsByProject["project-1"]).toEqual([
      expect.objectContaining({ id: "sprint-new", name: "Sprint Beta" }),
    ]);
  });

  it("updates a sprint and replaces it in state", async () => {
    // Seed an existing sprint
    useSprintStore.setState({
      sprintsByProject: {
        "project-1": [
          {
            id: "sprint-1",
            projectId: "project-1",
            name: "Sprint Alpha",
            startDate: "2026-03-24T00:00:00.000Z",
            endDate: "2026-03-30T23:59:59.000Z",
            status: "planning",
            totalBudgetUsd: 20,
            spentUsd: 0,
            createdAt: "2026-03-20T10:00:00.000Z",
          },
        ],
      },
    });

    const updated = {
      id: "sprint-1",
      projectId: "project-1",
      name: "Sprint Alpha",
      startDate: "2026-03-24T00:00:00.000Z",
      endDate: "2026-03-30T23:59:59.000Z",
      status: "active" as const,
      totalBudgetUsd: 20,
      spentUsd: 0,
      createdAt: "2026-03-20T10:00:00.000Z",
    };

    fetchMock.mockResolvedValueOnce(mockJsonResponse(updated));

    const result = await useSprintStore
      .getState()
      .updateSprint("project-1", "sprint-1", { status: "active" });

    expect(result.status).toBe("active");
    expect(useSprintStore.getState().sprintsByProject["project-1"]?.[0]?.status).toBe("active");
  });

  it("fetches sprint metrics for a specific sprint", async () => {
    fetchMock.mockResolvedValueOnce(
      mockJsonResponse({
        sprint: {
          id: "sprint-1",
          projectId: "project-1",
          name: "Sprint Alpha",
          startDate: "2026-03-24T00:00:00.000Z",
          endDate: "2026-03-30T23:59:59.000Z",
          status: "active",
          totalBudgetUsd: 20,
          spentUsd: 8,
          createdAt: "2026-03-20T10:00:00.000Z",
        },
        plannedTasks: 3,
        completedTasks: 2,
        remainingTasks: 1,
        completionRate: 66.67,
        velocityPerWeek: 2,
        taskBudgetUsd: 16,
        taskSpentUsd: 9.5,
        burndown: [
          { date: "2026-03-24", remainingTasks: 3, completedTasks: 0 },
          { date: "2026-03-25", remainingTasks: 2, completedTasks: 1 },
        ],
      })
    );

    await useSprintStore.getState().fetchSprintMetrics("project-1", "sprint-1");

    expect(useSprintStore.getState().metricsBySprintId["sprint-1"]).toEqual(
      expect.objectContaining({
        sprint: expect.objectContaining({ id: "sprint-1" }),
        completedTasks: 2,
        burndown: [
          expect.objectContaining({ remainingTasks: 3 }),
          expect.objectContaining({ remainingTasks: 2 }),
        ],
      })
    );
  });

  it("stores sprint list and metrics failures per project", async () => {
    fetchMock
      .mockRejectedValueOnce(new Error("sprints unavailable"))
      .mockRejectedValueOnce(new Error("metrics unavailable"));

    await useSprintStore.getState().fetchSprints("project-1");
    await useSprintStore.getState().fetchSprintMetrics("project-1", "sprint-1");

    expect(useSprintStore.getState()).toMatchObject({
      loadingByProject: { "project-1": false },
      metricsLoadingBySprintId: { "sprint-1": false },
      errorByProject: { "project-1": "sprints unavailable" },
      metricsErrorBySprintId: { "sprint-1": "metrics unavailable" },
    });
  });

  it("upserts sprints into the correct project bucket", () => {
    useSprintStore.getState().upsertSprint({
      id: "sprint-1",
      projectId: "project-1",
      name: "Sprint Alpha",
      startDate: "2026-03-24T00:00:00.000Z",
      endDate: "2026-03-30T23:59:59.000Z",
      status: "active",
      totalBudgetUsd: 20,
      spentUsd: 8,
      createdAt: "2026-03-20T10:00:00.000Z",
    });
    useSprintStore.getState().upsertSprint({
      id: "sprint-1",
      projectId: "project-1",
      name: "Sprint Alpha Updated",
      startDate: "2026-03-24T00:00:00.000Z",
      endDate: "2026-03-30T23:59:59.000Z",
      status: "closed",
      totalBudgetUsd: 20,
      spentUsd: 20,
      createdAt: "2026-03-20T10:00:00.000Z",
    });

    expect(useSprintStore.getState().sprintsByProject["project-1"]).toEqual([
      expect.objectContaining({
        name: "Sprint Alpha Updated",
        status: "closed",
      }),
    ]);
  });

  it("returns early or throws when auth is missing", async () => {
    authStoreModule.useAuthStore.getState.mockReturnValue({
      accessToken: null,
    });

    await useSprintStore.getState().fetchSprints("project-1");
    await useSprintStore.getState().fetchSprintMetrics("project-1", "sprint-1");
    await expect(
      useSprintStore.getState().createSprint("project-1", {
        name: "Blocked",
        startDate: "2026-04-01T00:00:00.000Z",
        endDate: "2026-04-14T23:59:59.000Z",
        totalBudgetUsd: 50,
      }),
    ).rejects.toThrow("Not authenticated");
    await expect(
      useSprintStore.getState().updateSprint("project-1", "sprint-1", {
        status: "active",
      }),
    ).rejects.toThrow("Not authenticated");

    expect(fetchMock).not.toHaveBeenCalled();
  });
});
