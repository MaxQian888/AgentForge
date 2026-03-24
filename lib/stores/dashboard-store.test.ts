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
});
