jest.mock("@/lib/stores/auth-store", () => ({
  useAuthStore: {
    getState: () => ({ token: "test-token" }),
  },
}));

import { useDashboardStore } from "./dashboard-store";

describe("useDashboardStore", () => {
  const fetchMock = jest.fn();
  const mockJsonResponse = (data: unknown) =>
    ({
      ok: true,
      status: 200,
      json: async () => data,
    }) as Response;

  beforeEach(() => {
    fetchMock.mockReset();
    global.fetch = fetchMock as unknown as typeof fetch;
    useDashboardStore.setState({
      summary: null,
      projects: [],
      selectedProjectId: null,
      activeDashboardIdByProject: {},
      loading: false,
      error: null,
      sectionErrors: {},
    });
  });

  it("loads a dashboard summary and records partial section failures without dropping healthy sections", async () => {
    fetchMock
      .mockResolvedValueOnce(
        mockJsonResponse([
          {
            id: "project-1",
            name: "AgentForge",
            slug: "agentforge",
            description: "Main project",
            repoUrl: "",
            defaultBranch: "main",
            createdAt: "2026-03-20T10:00:00.000Z",
          },
        ])
      )
      .mockResolvedValueOnce(
        mockJsonResponse({
          items: [
            {
              id: "task-1",
              projectId: "project-1",
              title: "Review queue",
              description: "",
              status: "in_review",
              priority: "high",
              assigneeId: "member-1",
              assigneeType: "human",
              spentUsd: 3.5,
              createdAt: "2026-03-23T10:00:00.000Z",
              updatedAt: "2026-03-24T10:00:00.000Z",
            },
          ],
          total: 1,
          page: 1,
          limit: 20,
        })
      )
      .mockResolvedValueOnce(
        mockJsonResponse([
          {
            id: "member-1",
            projectId: "project-1",
            name: "Alice",
            type: "human",
            role: "frontend-developer",
            email: "alice@example.com",
            avatarUrl: "",
            skills: ["react"],
            isActive: true,
            createdAt: "2026-03-20T10:00:00.000Z",
          },
        ])
      )
      .mockResolvedValueOnce(
        mockJsonResponse([
          {
            id: "run-1",
            taskId: "task-1",
            memberId: "member-1",
            status: "running",
            provider: "anthropic",
            model: "sonnet",
            inputTokens: 10,
            outputTokens: 20,
            cacheReadTokens: 0,
            costUsd: 5.25,
            turnCount: 3,
            errorMessage: "",
            startedAt: "2026-03-24T09:00:00.000Z",
            createdAt: "2026-03-24T09:00:00.000Z",
          },
        ])
      )
      .mockRejectedValueOnce(new Error("notifications unavailable"));

    await useDashboardStore
      .getState()
      .fetchSummary({ projectId: "project-1", now: "2026-03-24T12:00:00.000Z" });

    const state = useDashboardStore.getState();

    expect(state.summary).not.toBeNull();
    expect(state.summary?.headline.pendingReviews).toBe(1);
    expect(state.summary?.team.totalMembers).toBe(1);
    expect(state.sectionErrors.activity).toContain("notifications unavailable");
    expect(state.error).toBeNull();
  });

  it("normalizes notification activity payloads for dashboard consumers", async () => {
    fetchMock
      .mockResolvedValueOnce(
        mockJsonResponse([
          {
            id: "project-1",
            name: "AgentForge",
            slug: "agentforge",
            description: "Main project",
            repoUrl: "",
            defaultBranch: "main",
            createdAt: "2026-03-20T10:00:00.000Z",
          },
        ])
      )
      .mockResolvedValueOnce(
        mockJsonResponse({
          items: [],
          total: 0,
          page: 1,
          limit: 20,
        })
      )
      .mockResolvedValueOnce(mockJsonResponse([]))
      .mockResolvedValueOnce(mockJsonResponse([]))
      .mockResolvedValueOnce(
        mockJsonResponse([
          {
            id: "notification-1",
            targetId: "member-1",
            type: "task_progress_stalled",
            title: "Task stalled: Review queue",
            body: "Task Review queue is stalled (awaiting_review).",
            data: "{\"href\":\"/project?id=project-1#task-task-1\"}",
            isRead: false,
            createdAt: "2026-03-24T10:30:00.000Z",
          },
        ])
      );

    await useDashboardStore.getState().fetchSummary({
      projectId: "project-1",
      now: "2026-03-24T12:00:00.000Z",
    });

    expect(useDashboardStore.getState().activity).toEqual([
      expect.objectContaining({
        id: "notification-1",
        type: "task_progress_stalled",
        message: "Task Review queue is stalled (awaiting_review).",
        targetId: "member-1",
      }),
    ]);
  });

  it("loads dashboards and widget data for project dashboards", async () => {
    fetchMock.mockResolvedValueOnce(
      mockJsonResponse([
        {
          id: "dashboard-1",
          projectId: "project-1",
          name: "Sprint Overview",
          layout: [],
          createdBy: "user-1",
          createdAt: "2026-03-20T10:00:00.000Z",
          updatedAt: "2026-03-20T10:00:00.000Z",
          widgets: [
            {
              id: "widget-1",
              dashboardId: "dashboard-1",
              widgetType: "throughput_chart",
              config: {},
              position: {},
              createdAt: "2026-03-20T10:00:00.000Z",
              updatedAt: "2026-03-20T10:00:00.000Z",
            },
          ],
        },
      ])
    );

    await useDashboardStore.getState().fetchDashboards("project-1");

    fetchMock.mockResolvedValueOnce(
      mockJsonResponse({
        widgetType: "throughput_chart",
        points: [{ date: "2026-03-24", count: 3 }],
      })
    );
    const widgetData = await useDashboardStore.getState().fetchWidgetData("project-1", "throughput_chart", {
      days: 7,
    });

    expect(useDashboardStore.getState().dashboardsByProject["project-1"]).toHaveLength(1);
    expect(useDashboardStore.getState().widgetsByDashboard["dashboard-1"]).toHaveLength(1);
    expect(widgetData).toEqual(
      expect.objectContaining({
        widgetType: "throughput_chart",
      })
    );
  });

  it("tracks the active dashboard per project when dashboards load, create, and delete", async () => {
    fetchMock.mockResolvedValueOnce(
      mockJsonResponse([
        {
          id: "dashboard-1",
          projectId: "project-1",
          name: "Sprint Overview",
          layout: [],
          createdBy: "user-1",
          createdAt: "2026-03-20T10:00:00.000Z",
          updatedAt: "2026-03-20T10:00:00.000Z",
          widgets: [],
        },
        {
          id: "dashboard-2",
          projectId: "project-1",
          name: "Review Watch",
          layout: [],
          createdBy: "user-1",
          createdAt: "2026-03-20T10:00:00.000Z",
          updatedAt: "2026-03-20T10:00:00.000Z",
          widgets: [],
        },
      ])
    );

    await useDashboardStore.getState().fetchDashboards("project-1");

    let workspaceState = useDashboardStore.getState() as unknown as {
      activeDashboardIdByProject?: Record<string, string | null>;
    };
    expect(workspaceState.activeDashboardIdByProject?.["project-1"]).toBe("dashboard-1");

    fetchMock.mockResolvedValueOnce(
      mockJsonResponse({
        id: "dashboard-3",
        projectId: "project-1",
        name: "Budget Watch",
        layout: [],
        createdBy: "user-1",
        createdAt: "2026-03-20T10:00:00.000Z",
        updatedAt: "2026-03-20T10:00:00.000Z",
        widgets: [],
      })
    );

    await useDashboardStore
      .getState()
      .createDashboard("project-1", { name: "Budget Watch", layout: [] });

    workspaceState = useDashboardStore.getState() as unknown as {
      activeDashboardIdByProject?: Record<string, string | null>;
    };
    expect(workspaceState.activeDashboardIdByProject?.["project-1"]).toBe("dashboard-3");

    fetchMock.mockResolvedValueOnce(mockJsonResponse({}));
    await useDashboardStore.getState().deleteDashboard("project-1", "dashboard-3");

    workspaceState = useDashboardStore.getState() as unknown as {
      activeDashboardIdByProject?: Record<string, string | null>;
    };
    expect(workspaceState.activeDashboardIdByProject?.["project-1"]).toBe("dashboard-1");
  });

  it("records widget request failures without throwing away the widget key", async () => {
    fetchMock.mockRejectedValueOnce(new Error("widget endpoint unavailable"));

    const result = await useDashboardStore
      .getState()
      .fetchWidgetData("project-1", "throughput_chart", {});

    const state = useDashboardStore.getState() as unknown as {
      widgetRequestStateByKey?: Record<
        string,
        { status: string; error: string | null }
      >;
    };

    expect(result).toBeNull();
    expect(
      state.widgetRequestStateByKey?.["project-1:throughput_chart:{}"]
    ).toEqual({
      status: "error",
      error: "widget endpoint unavailable",
    });
  });

  it("records dashboard loading errors per project", async () => {
    fetchMock.mockRejectedValueOnce(new Error("dashboard list unavailable"));

    await useDashboardStore.getState().fetchDashboards("project-1");

    const state = useDashboardStore.getState() as unknown as {
      dashboardsLoadingByProject?: Record<string, boolean>;
      dashboardsErrorByProject?: Record<string, string | null>;
    };

    expect(state.dashboardsLoadingByProject?.["project-1"]).toBe(false);
    expect(state.dashboardsErrorByProject?.["project-1"]).toBe(
      "dashboard list unavailable"
    );
  });
});
