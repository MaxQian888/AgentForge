jest.mock("@/lib/stores/auth-store", () => ({
  useAuthStore: {
    getState: jest.fn(() => ({ token: "test-token" })),
  },
}));

import { useDashboardStore } from "./dashboard-store";

const authStoreModule = jest.requireMock("@/lib/stores/auth-store") as {
  useAuthStore: {
    getState: jest.Mock<{ token?: string | null; accessToken?: string | null }, []>;
  };
};

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
    authStoreModule.useAuthStore.getState.mockReturnValue({ token: "test-token" });
    useDashboardStore.setState({
      summary: null,
      projects: [],
      selectedProjectId: null,
      activeDashboardIdByProject: {},
      dashboardsLoadingByProject: {},
      dashboardsErrorByProject: {},
      dashboardsByProject: {},
      widgetsByDashboard: {},
      widgetData: {},
      widgetRequestStateByKey: {},
      tasks: [],
      members: [],
      agents: [],
      activity: [],
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

  it("loads bootstrap readiness inputs for the selected project scope", async () => {
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
            settings: {
              codingAgent: {
                runtime: "",
                provider: "",
                model: "",
              },
            },
          },
        ]),
      )
      .mockResolvedValueOnce(
        mockJsonResponse({
          items: [],
          total: 0,
          page: 1,
          limit: 20,
        }),
      )
      .mockResolvedValueOnce(mockJsonResponse([]))
      .mockResolvedValueOnce(mockJsonResponse([]))
      .mockResolvedValueOnce(mockJsonResponse([]))
      .mockResolvedValueOnce(mockJsonResponse([{ id: "template-1" }]))
      .mockResolvedValueOnce(mockJsonResponse([{ id: "wf-template-1" }]))
      .mockResolvedValueOnce(mockJsonResponse([]));

    await useDashboardStore.getState().fetchSummary({ projectId: "project-1" });

    expect(fetchMock).toHaveBeenCalledWith(
      "http://localhost:7777/api/v1/projects/project-1/wiki/templates",
      expect.any(Object),
    );
    expect(fetchMock).toHaveBeenCalledWith(
      "http://localhost:7777/api/v1/workflow-templates",
      expect.any(Object),
    );
    expect(fetchMock).toHaveBeenCalledWith(
      "http://localhost:7777/api/v1/projects/project-1/sprints",
      expect.any(Object),
    );
    expect(useDashboardStore.getState().summary?.bootstrap).toEqual(
      expect.objectContaining({
        unresolvedCount: 4,
      }),
    );
  });

  it("builds an all-projects summary when there is no selectable project", async () => {
    fetchMock.mockResolvedValueOnce(
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
      ]),
    );

    await useDashboardStore.getState().fetchSummary({ projectId: "missing-project" });

    expect(useDashboardStore.getState()).toMatchObject({
      selectedProjectId: null,
      tasks: [],
      members: [],
      agents: [],
      activity: [],
      summary: expect.objectContaining({
        scope: {
          projectId: null,
          projectName: "All Projects",
          projectsCount: 1,
        },
      }),
    });
  });

  it("treats an explicit null projectId as aggregate scope across all projects", async () => {
    fetchMock.mockImplementation((input: RequestInfo | URL) => {
      const url = String(input);

      if (url.endsWith("/api/v1/projects")) {
        return Promise.resolve(
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
            {
              id: "project-2",
              name: "Ops",
              slug: "ops",
              description: "Ops project",
              repoUrl: "",
              defaultBranch: "main",
              createdAt: "2026-03-20T10:00:00.000Z",
            },
          ]),
        );
      }

      if (url.endsWith("/api/v1/projects/project-1/tasks")) {
        return Promise.resolve(
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
          }),
        );
      }

      if (url.endsWith("/api/v1/projects/project-2/tasks")) {
        return Promise.resolve(
          mockJsonResponse({
            items: [
              {
                id: "task-2",
                projectId: "project-2",
                title: "Deploy ops",
                description: "",
                status: "assigned",
                priority: "medium",
                assigneeId: "member-2",
                assigneeType: "human",
                spentUsd: 1.5,
                createdAt: "2026-03-23T10:00:00.000Z",
                updatedAt: "2026-03-24T10:00:00.000Z",
              },
            ],
            total: 1,
            page: 1,
            limit: 20,
          }),
        );
      }

      if (url.endsWith("/api/v1/projects/project-1/members")) {
        return Promise.resolve(
          mockJsonResponse([
            {
              id: "member-1",
              projectId: "project-1",
              name: "Alice",
              type: "human",
              role: "frontend",
              email: "alice@example.com",
              avatarUrl: "",
              skills: ["react"],
              isActive: true,
              createdAt: "2026-03-20T10:00:00.000Z",
            },
          ]),
        );
      }

      if (url.endsWith("/api/v1/projects/project-2/members")) {
        return Promise.resolve(
          mockJsonResponse([
            {
              id: "member-2",
              projectId: "project-2",
              name: "Bob",
              type: "human",
              role: "ops",
              email: "bob@example.com",
              avatarUrl: "",
              skills: ["deploy"],
              isActive: true,
              createdAt: "2026-03-20T10:00:00.000Z",
            },
          ]),
        );
      }

      if (url.endsWith("/api/v1/agents")) {
        return Promise.resolve(
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
          ]),
        );
      }

      if (url.endsWith("/api/v1/notifications")) {
        return Promise.resolve(
          mockJsonResponse([
            {
              id: "notification-1",
              targetId: "member-1",
              type: "review_completed",
              title: "Deep review completed",
              body: "Task task-1 is waiting for a reviewer.",
              createdAt: "2026-03-24T10:30:00.000Z",
            },
          ]),
        );
      }

      throw new Error(`Unexpected fetch: ${url}`);
    });

    await useDashboardStore.getState().fetchSummary({
      projectId: null,
      now: "2026-03-24T12:00:00.000Z",
    });

    expect(useDashboardStore.getState()).toMatchObject({
      selectedProjectId: null,
      tasks: [
        expect.objectContaining({ id: "task-1" }),
        expect.objectContaining({ id: "task-2" }),
      ],
      members: [
        expect.objectContaining({ id: "member-1" }),
        expect.objectContaining({ id: "member-2" }),
      ],
      summary: expect.objectContaining({
        scope: {
          projectId: null,
          projectName: "All Projects",
          projectsCount: 2,
        },
      }),
    });
  });

  it("stores a fatal dashboard summary error when the project list fails", async () => {
    fetchMock.mockRejectedValueOnce(new Error("projects unavailable"));

    await useDashboardStore.getState().fetchSummary();

    expect(useDashboardStore.getState()).toMatchObject({
      loading: false,
      error: "projects unavailable",
      tasks: [],
      members: [],
      agents: [],
      activity: [],
    });
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

  it("updates dashboards, widgets, and active dashboard state in place", async () => {
    useDashboardStore.setState({
      summary: null,
      projects: [],
      selectedProjectId: null,
      activeDashboardIdByProject: { "project-1": "dashboard-1" },
      dashboardsLoadingByProject: {},
      dashboardsErrorByProject: {},
      dashboardsByProject: {
        "project-1": [
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
        ],
      },
      widgetsByDashboard: { "dashboard-1": [] },
      widgetData: {},
      widgetRequestStateByKey: {},
      tasks: [],
      members: [],
      agents: [],
      activity: [],
      loading: false,
      error: null,
      sectionErrors: {},
    });
    fetchMock
      .mockResolvedValueOnce(
        mockJsonResponse({
          id: "dashboard-1",
          projectId: "project-1",
          name: "Updated Overview",
          layout: [{ x: 0 }],
          createdBy: "user-1",
          createdAt: "2026-03-20T10:00:00.000Z",
          updatedAt: "2026-03-21T10:00:00.000Z",
          widgets: [],
        }),
      )
      .mockResolvedValueOnce(
        mockJsonResponse({
          id: "widget-1",
          dashboardId: "dashboard-1",
          widgetType: "throughput_chart",
          config: { days: 7 },
          position: { x: 0, y: 0, w: 2, h: 2 },
          createdAt: "2026-03-20T10:00:00.000Z",
          updatedAt: "2026-03-20T10:00:00.000Z",
        }),
      )
      .mockResolvedValueOnce(mockJsonResponse({}));

    await useDashboardStore
      .getState()
      .updateDashboard("project-1", "dashboard-1", { name: "Updated Overview" });
    useDashboardStore.getState().setActiveDashboard("project-1", "dashboard-1");
    await useDashboardStore.getState().saveWidget("project-1", "dashboard-1", {
      widgetType: "throughput_chart",
      config: { days: 7 },
      position: { x: 0, y: 0, w: 2, h: 2 },
    });
    await useDashboardStore
      .getState()
      .deleteWidget("project-1", "dashboard-1", "widget-1");

    expect(useDashboardStore.getState()).toMatchObject({
      activeDashboardIdByProject: { "project-1": "dashboard-1" },
      dashboardsByProject: {
        "project-1": [expect.objectContaining({ name: "Updated Overview" })],
      },
      widgetsByDashboard: {
        "dashboard-1": [],
      },
    });
  });

  it("applies task, agent, and activity updates while respecting the selected project", () => {
    useDashboardStore.setState({
      summary: null,
      projects: [
        {
          id: "project-1",
          name: "AgentForge",
          slug: "agentforge",
          description: "",
          repoUrl: "",
          defaultBranch: "main",
          createdAt: "2026-03-20T10:00:00.000Z",
        },
      ],
      selectedProjectId: "project-1",
      activeDashboardIdByProject: {},
      dashboardsLoadingByProject: {},
      dashboardsErrorByProject: {},
      dashboardsByProject: {},
      widgetsByDashboard: {},
      widgetData: {},
      widgetRequestStateByKey: {},
      tasks: [],
      members: [],
      agents: [],
      activity: [],
      loading: false,
      error: null,
      sectionErrors: {},
    });

    useDashboardStore.getState().applyTaskUpdate({
      id: "task-1",
      projectId: "project-2",
      title: "Ignore me",
      status: "assigned",
      priority: "low",
      assigneeId: null,
      assigneeType: null,
      spentUsd: 0,
      createdAt: "2026-03-24T10:00:00.000Z",
      updatedAt: "2026-03-24T10:00:00.000Z",
    });
    useDashboardStore.getState().applyTaskUpdate({
      id: "task-2",
      projectId: "project-1",
      title: "Keep me",
      status: "in_progress",
      priority: "high",
      assigneeId: "member-1",
      assigneeType: "human",
      spentUsd: 2,
      createdAt: "2026-03-24T10:00:00.000Z",
      updatedAt: "2026-03-24T10:00:00.000Z",
    });
    useDashboardStore.getState().applyAgentUpdate({
      id: "run-1",
      taskId: "task-2",
      memberId: "member-1",
      status: "running",
      costUsd: 0.5,
      turnCount: 1,
      startedAt: "2026-03-24T10:00:00.000Z",
      createdAt: "2026-03-24T10:00:00.000Z",
      updatedAt: "2026-03-24T10:00:00.000Z",
    });
    useDashboardStore.getState().applyAgentUpdate({
      id: "run-1",
      taskId: "task-2",
      memberId: "member-1",
      status: "completed",
      costUsd: 0.5,
      turnCount: 1,
      startedAt: "2026-03-24T10:00:00.000Z",
      createdAt: "2026-03-24T10:00:00.000Z",
      updatedAt: "2026-03-24T10:05:00.000Z",
    });
    useDashboardStore.getState().applyActivityNotification({
      id: "notification-1",
      type: "task_progress_stalled",
      title: "Task stalled",
      message: "Task stalled.",
      createdAt: "2026-03-24T11:00:00.000Z",
      targetId: "member-1",
    });
    useDashboardStore.getState().applyActivityNotification({
      id: "notification-1",
      type: "task_progress_stalled",
      title: "Task stalled",
      message: "Task stalled again.",
      createdAt: "2026-03-24T11:05:00.000Z",
      targetId: "member-1",
    });

    expect(useDashboardStore.getState()).toMatchObject({
      tasks: [expect.objectContaining({ id: "task-2" })],
      agents: [expect.objectContaining({ id: "run-1", status: "completed" })],
      activity: [expect.objectContaining({ id: "notification-1", message: "Task stalled again." })],
    });
  });

  it("returns early without an auth token", async () => {
    authStoreModule.useAuthStore.getState.mockReturnValue({
      token: null,
      accessToken: null,
    });

    await useDashboardStore.getState().fetchSummary();
    await useDashboardStore.getState().fetchDashboards("project-1");
    await expect(
      useDashboardStore.getState().fetchWidgetData("project-1", "throughput_chart"),
    ).resolves.toBeNull();
    await useDashboardStore
      .getState()
      .createDashboard("project-1", { name: "Skipped", layout: [] });
    await useDashboardStore
      .getState()
      .updateDashboard("project-1", "dashboard-1", { name: "Skipped" });
    await useDashboardStore
      .getState()
      .deleteDashboard("project-1", "dashboard-1");
    await useDashboardStore.getState().saveWidget("project-1", "dashboard-1", {
      widgetType: "throughput_chart",
    });
    await useDashboardStore
      .getState()
      .deleteWidget("project-1", "dashboard-1", "widget-1");

    expect(fetchMock).not.toHaveBeenCalled();
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
