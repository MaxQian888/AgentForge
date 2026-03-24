jest.mock("@/lib/stores/auth-store", () => ({
  useAuthStore: {
    getState: () => ({ accessToken: "test-token" }),
  },
}));

import { useTaskStore } from "./task-store";

describe("useTaskStore", () => {
  const fetchMock = jest.fn();
  const mockJsonResponse = (data: unknown, status = 200) =>
    ({
      ok: status >= 200 && status < 300,
      status,
      json: async () => data,
    }) as Response;
  const baseTask = {
    id: "task-1",
    projectId: "project-1",
    title: "Implement timeline view",
    description: "",
    status: "in_progress",
    priority: "high",
    assigneeId: "member-1",
    assigneeType: "human",
    assigneeName: "Alice",
    cost: 4.5,
    plannedStartAt: "2026-03-25T09:00:00.000Z",
    plannedEndAt: "2026-03-27T18:00:00.000Z",
    createdAt: "2026-03-24T10:00:00.000Z",
    updatedAt: "2026-03-24T12:00:00.000Z",
  } as const;

  beforeEach(() => {
    fetchMock.mockReset();
    global.fetch = fetchMock as unknown as typeof fetch;
    useTaskStore.setState({
      tasks: [],
      loading: false,
      error: null,
    });
  });

  it("fetches paginated project tasks and keeps planning fields", async () => {
    fetchMock.mockResolvedValueOnce(
      mockJsonResponse({
        items: [
          {
            id: "task-1",
            projectId: "project-1",
            title: "Implement timeline view",
            description: "",
            status: "in_progress",
            priority: "high",
            assigneeId: "member-1",
            assigneeType: "human",
            assigneeName: "Alice",
            cost: 4.5,
            plannedStartAt: "2026-03-25T09:00:00.000Z",
            plannedEndAt: "2026-03-27T18:00:00.000Z",
            progress: {
              lastActivityAt: "2026-03-24T11:45:00.000Z",
              lastActivitySource: "agent_heartbeat",
              lastTransitionAt: "2026-03-24T10:30:00.000Z",
              healthStatus: "warning",
              riskReason: "no_recent_update",
              riskSinceAt: "2026-03-24T11:00:00.000Z",
              lastAlertState: "warning:no_recent_update",
              lastAlertAt: "2026-03-24T11:05:00.000Z",
              lastRecoveredAt: null,
            },
            createdAt: "2026-03-24T10:00:00.000Z",
            updatedAt: "2026-03-24T12:00:00.000Z",
          },
        ],
        total: 1,
        page: 1,
        limit: 20,
      })
    );

    await useTaskStore.getState().fetchTasks("project-1");

    expect(useTaskStore.getState().tasks).toEqual([
      expect.objectContaining({
        id: "task-1",
        projectId: "project-1",
        plannedStartAt: "2026-03-25T09:00:00.000Z",
        plannedEndAt: "2026-03-27T18:00:00.000Z",
        progress: {
          lastActivityAt: "2026-03-24T11:45:00.000Z",
          lastActivitySource: "agent_heartbeat",
          lastTransitionAt: "2026-03-24T10:30:00.000Z",
          healthStatus: "warning",
          riskReason: "no_recent_update",
          riskSinceAt: "2026-03-24T11:00:00.000Z",
          lastAlertState: "warning:no_recent_update",
          lastAlertAt: "2026-03-24T11:05:00.000Z",
          lastRecoveredAt: null,
        },
      }),
    ]);
  });

  it("stores a retryable error when the project task load fails", async () => {
    fetchMock.mockResolvedValueOnce(mockJsonResponse({ message: "boom" }, 500));

    await useTaskStore.getState().fetchTasks("project-1");

    expect(useTaskStore.getState().error).toBe("Unable to load tasks");
    expect(useTaskStore.getState().loading).toBe(false);
  });

  it("posts board status transitions through the task transition endpoint", async () => {
    useTaskStore.setState({
      tasks: [
        {
          ...baseTask,
          spentUsd: 4.5,
        },
      ],
      loading: false,
    });
    fetchMock.mockResolvedValueOnce(
      mockJsonResponse({
        ...baseTask,
        status: "done",
        updatedAt: "2026-03-24T12:30:00.000Z",
      })
    );

    await useTaskStore.getState().transitionTask("task-1", "done");

    expect(fetchMock).toHaveBeenCalledWith(
      "http://localhost:7777/api/v1/tasks/task-1/transition",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ status: "done" }),
        headers: expect.objectContaining({
          Authorization: "Bearer test-token",
          "Content-Type": "application/json",
        }),
      })
    );
    expect(useTaskStore.getState().tasks).toEqual([
      expect.objectContaining({ id: "task-1", status: "done" }),
    ]);
  });

  it("persists planning field updates through the shared task update endpoint", async () => {
    useTaskStore.setState({
      tasks: [
        {
          ...baseTask,
          spentUsd: 4.5,
        },
      ],
      loading: false,
    });
    fetchMock.mockResolvedValueOnce(
      mockJsonResponse({
        ...baseTask,
        plannedStartAt: "2026-03-30T09:00:00.000Z",
        plannedEndAt: "2026-04-01T18:00:00.000Z",
        updatedAt: "2026-03-24T13:00:00.000Z",
      })
    );

    await useTaskStore.getState().updateTask("task-1", {
      plannedStartAt: "2026-03-30T09:00:00.000Z",
      plannedEndAt: "2026-04-01T18:00:00.000Z",
    });

    expect(fetchMock).toHaveBeenCalledWith(
      "http://localhost:7777/api/v1/tasks/task-1",
      expect.objectContaining({
        method: "PUT",
        body: JSON.stringify({
          plannedStartAt: "2026-03-30T09:00:00.000Z",
          plannedEndAt: "2026-04-01T18:00:00.000Z",
        }),
        headers: expect.objectContaining({
          Authorization: "Bearer test-token",
          "Content-Type": "application/json",
        }),
      })
    );
    expect(useTaskStore.getState().tasks).toEqual([
      expect.objectContaining({
        id: "task-1",
        plannedStartAt: "2026-03-30T09:00:00.000Z",
        plannedEndAt: "2026-04-01T18:00:00.000Z",
      }),
    ]);
  });

  it("accepts wrapped dispatch responses when assigning a task", async () => {
    useTaskStore.setState({
      tasks: [
        {
          ...baseTask,
          spentUsd: 4.5,
        },
      ],
      loading: false,
    });
    fetchMock.mockResolvedValueOnce(
      mockJsonResponse({
        task: {
          ...baseTask,
          assigneeId: "member-2",
          assigneeType: "agent",
          assigneeName: "Agent Smith",
          updatedAt: "2026-03-24T13:30:00.000Z",
        },
        dispatch: {
          status: "started",
          run: {
            id: "run-1",
            taskId: "task-1",
          },
        },
      })
    );

    await useTaskStore.getState().assignTask("task-1", "member-2", "agent");

    expect(fetchMock).toHaveBeenCalledWith(
      "http://localhost:7777/api/v1/tasks/task-1/assign",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ assigneeId: "member-2", assigneeType: "agent" }),
      })
    );
    expect(useTaskStore.getState().tasks).toEqual([
      expect.objectContaining({
        id: "task-1",
        assigneeId: "member-2",
        assigneeType: "agent",
        assigneeName: "Agent Smith",
      }),
    ]);
  });
});
