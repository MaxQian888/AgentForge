jest.mock("@/lib/ws-client", () => {
  class MockWSClient {
    static instances: MockWSClient[] = [];
    handlers = new Map<string, (payload: unknown) => void>();

    constructor() {
      MockWSClient.instances.push(this);
    }

    on(event: string, handler: (payload: unknown) => void) {
      this.handlers.set(event, handler);
    }

    connect = jest.fn();

    close = jest.fn();

    subscribe = jest.fn();

    unsubscribe = jest.fn();

    emit(event: string, payload: unknown) {
      this.handlers.get(event)?.(payload);
    }
  }

  return { WSClient: MockWSClient };
});

const emitProjectedDesktopEventMock = jest.fn();

jest.mock("@/lib/platform-runtime", () => ({
  emitProjectedDesktopEvent: (...args: unknown[]) =>
    emitProjectedDesktopEventMock(...args),
}));

const toastWarningMock = jest.fn();

jest.mock("sonner", () => ({
  toast: {
    warning: (...args: unknown[]) => toastWarningMock(...args),
  },
}));

import { useAgentStore } from "./agent-store";
import { useDashboardStore } from "./dashboard-store";
import { useDocsStore } from "./docs-store";
import { useEntityLinkStore } from "./entity-link-store";
import { useNotificationStore } from "./notification-store";
import { useReviewStore } from "./review-store";
import { useSprintStore } from "./sprint-store";
import { useSchedulerStore } from "./scheduler-store";
import { useTaskCommentStore } from "./task-comment-store";
import { useTaskStore } from "./task-store";
import { useTeamStore } from "./team-store";
import { useWorkflowStore } from "./workflow-store";
import { useWSStore } from "./ws-store";
import { useLocaleStore } from "./locale-store";

function getLatestClient() {
  const MockWSClient = jest.requireMock("@/lib/ws-client").WSClient as {
    instances: Array<{
      emit: (event: string, payload: unknown) => void;
      close: jest.Mock;
      connect: jest.Mock;
      subscribe: jest.Mock;
      unsubscribe: jest.Mock;
    }>;
  };
  return MockWSClient.instances.at(-1);
}

describe("useWSStore", () => {
  beforeEach(() => {
    emitProjectedDesktopEventMock.mockReset();
    toastWarningMock.mockReset();
    useTaskStore.setState({ tasks: [], loading: false, error: null });
    useLocaleStore.setState({ locale: "en" });
    useReviewStore.setState({
      reviewsByTask: {},
      allReviews: [],
      allReviewsLoading: false,
      loading: false,
      error: null,
    });
    useAgentStore.setState({
      agents: [],
      agentOutputs: new Map(),
      agentToolCalls: new Map(),
      agentToolResults: new Map(),
      agentReasoning: new Map(),
      agentFileChanges: new Map(),
      agentTodos: new Map(),
      agentPartialMessages: new Map(),
      agentPermissionRequests: new Map(),
      pool: { active: 0, max: 2, available: 2, pausedResumable: 0 },
      loading: false,
    });
    useNotificationStore.setState({ notifications: [], unreadCount: 0 });
    useEntityLinkStore.setState({ linksByEntity: {}, loading: false, error: null });
    useTaskCommentStore.setState({ commentsByTask: {}, loading: false, error: null });
    useDocsStore.setState({
      projectId: "project-1",
      tree: [],
      currentPage: {
        id: "page-1",
        spaceId: "space-1",
        title: "Runbook",
        content: "[]",
        contentText: "",
        path: "/page-1",
        sortOrder: 0,
        isTemplate: false,
        isSystem: false,
        isPinned: false,
        createdAt: "2026-03-26T10:00:00.000Z",
        updatedAt: "2026-03-26T10:00:00.000Z",
      },
      comments: [],
      versions: [],
      templates: [],
      favorites: [],
      recentAccess: [],
      loading: false,
      saving: false,
      error: null,
    });
    useDashboardStore.setState({
      summary: null,
      projects: [{ id: "project-1", name: "AgentForge", slug: "agentforge", description: "", repoUrl: "", defaultBranch: "main", createdAt: "2026-03-24T09:00:00.000Z" }],
      selectedProjectId: "project-1",
      tasks: [],
      members: [],
      agents: [],
      activity: [],
      loading: false,
      error: null,
      sectionErrors: {},
    });
    useWorkflowStore.setState({
      config: null,
      loading: false,
      saving: false,
      error: null,
      recentActivityByProject: {},
    });
    useSchedulerStore.setState({
      jobs: [],
      runsByJobKey: {},
      draftSchedules: {},
      selectedJobKey: null,
      loading: false,
      actionJobKey: null,
      error: null,
    });
    useSprintStore.setState({
      sprintsByProject: {},
      metricsBySprintId: {},
      loadingByProject: {},
      metricsLoadingBySprintId: {},
      errorByProject: {},
      metricsErrorBySprintId: {},
    });
    useTeamStore.setState({
      teams: [],
      loading: false,
      error: null,
      loadingById: {},
      errorById: {},
    });
    useWSStore.getState().disconnect();
  });

  it("tracks websocket connection degradation explicitly", () => {
    useWSStore.getState().connect("ws://localhost:7777/ws", "token");

    const client = getLatestClient();

    client?.emit("connected", null);
    expect(useWSStore.getState().connected).toBe(true);

    client?.emit("disconnected", null);
    expect(useWSStore.getState().connected).toBe(false);
  });

  it("closes the previous websocket client and proxies subscribe/unsubscribe/disconnect", () => {
    useWSStore.getState().connect("ws://localhost:7777/ws", "token-1");
    const firstClient = getLatestClient();

    useWSStore.getState().connect("ws://localhost:7777/ws", "token-2");
    const secondClient = getLatestClient();

    useWSStore.getState().subscribe("project-1");
    useWSStore.getState().unsubscribe("project-1");
    useWSStore.getState().disconnect();

    expect(firstClient?.close).toHaveBeenCalledTimes(1);
    expect(secondClient?.connect).toHaveBeenCalledTimes(1);
    expect(secondClient?.subscribe).toHaveBeenCalledWith("project-1");
    expect(secondClient?.unsubscribe).toHaveBeenCalledWith("project-1");
    expect(secondClient?.close).toHaveBeenCalledTimes(1);
    expect(useWSStore.getState().connected).toBe(false);
  });

  it("safely ignores subscribe and unsubscribe before a client exists", () => {
    expect(() => {
      useWSStore.getState().subscribe("project-1");
      useWSStore.getState().unsubscribe("project-1");
      useWSStore.getState().disconnect();
    }).not.toThrow();
  });

  it("applies progress updates and notifications from websocket payload envelopes", () => {
    useWSStore.getState().connect("ws://localhost:7777/ws", "token");

    const client = getLatestClient();
    expect(client).toBeDefined();

    client?.emit("task.progress.updated", {
      type: "task.progress.updated",
      payload: {
        task: {
          id: "task-1",
          projectId: "project-1",
          title: "Implement detector",
          description: "",
          status: "in_progress",
          priority: "high",
          assigneeId: "member-1",
          assigneeType: "human",
          spentUsd: 2.5,
          progress: {
            lastActivityAt: "2026-03-24T11:00:00.000Z",
            lastActivitySource: "agent_heartbeat",
            lastTransitionAt: "2026-03-24T10:00:00.000Z",
            healthStatus: "stalled",
            riskReason: "no_recent_update",
            riskSinceAt: "2026-03-24T11:30:00.000Z",
            lastAlertState: "stalled:no_recent_update",
            lastAlertAt: "2026-03-24T11:35:00.000Z",
            lastRecoveredAt: null,
          },
          createdAt: "2026-03-24T09:00:00.000Z",
          updatedAt: "2026-03-24T12:00:00.000Z",
        },
      },
    });

    client?.emit("notification", {
      type: "notification",
      payload: {
        id: "notification-1",
        type: "task_progress_stalled",
        title: "Task stalled: Implement detector",
        body: "Task Implement detector is stalled.",
        createdAt: "2026-03-24T12:05:00.000Z",
        isRead: false,
        targetId: "member-1",
      },
    });

    expect(useTaskStore.getState().tasks[0]).toEqual(
      expect.objectContaining({
        id: "task-1",
        progress: expect.objectContaining({
          healthStatus: "stalled",
          riskReason: "no_recent_update",
        }),
      })
    );
    expect(useDashboardStore.getState().tasks[0]?.id).toBe("task-1");
    expect(useNotificationStore.getState().notifications[0]).toEqual(
      expect.objectContaining({
        id: "notification-1",
        message: "Task Implement detector is stalled.",
        read: false,
      })
    );
    expect(useDashboardStore.getState().activity[0]).toEqual(
      expect.objectContaining({
        id: "notification-1",
        message: "Task Implement detector is stalled.",
      })
    );
  });

  it("does not duplicate websocket replay notifications with the same identifier", () => {
    useWSStore.getState().connect("ws://localhost:7777/ws", "token");

    const client = getLatestClient();
    expect(client).toBeDefined();

    const payload = {
      type: "notification",
      payload: {
        id: "notification-dup",
        type: "task_progress_stalled",
        title: "Task stalled: Implement detector",
        body: "Task Implement detector is stalled.",
        createdAt: "2026-03-26T12:05:00.000Z",
        isRead: false,
        targetId: "member-1",
      },
    };

    client?.emit("notification", payload);
    client?.emit("notification", payload);

    expect(useNotificationStore.getState().notifications).toHaveLength(1);
    expect(useNotificationStore.getState().notifications[0]).toEqual(
      expect.objectContaining({
        id: "notification-dup",
        message: "Task Implement detector is stalled.",
      }),
    );
  });

  it("projects plugin lifecycle websocket events into the desktop event stream", () => {
    useWSStore.getState().connect("ws://localhost:7777/ws", "token");

    const client = getLatestClient();
    expect(client).toBeDefined();

    client?.emit("plugin.lifecycle", {
      type: "plugin.lifecycle",
      payload: {
        id: "evt-1",
        plugin_id: "github-tool",
        event_type: "activated",
        lifecycle_state: "active",
        summary: "GitHub Tool activated",
        created_at: "2026-03-28T10:00:00.000Z",
      },
    });

    expect(emitProjectedDesktopEventMock).toHaveBeenCalledWith(
      expect.objectContaining({
        type: "plugin.lifecycle",
        source: "plugin",
        timestamp: "2026-03-28T10:00:00.000Z",
        payload: expect.objectContaining({
          plugin_id: "github-tool",
          event_type: "activated",
        }),
      }),
    );
  });

  it("hydrates agent output envelopes and keeps pool stats in sync with live agent events", () => {
    useWSStore.getState().connect("ws://localhost:7777/ws", "token");

    const client = getLatestClient();
    expect(client).toBeDefined();

    client?.emit("agent.started", {
      type: "agent.started",
      payload: {
        id: "run-1",
        taskId: "task-1",
        taskTitle: "Implement orchestration",
        memberId: "member-1",
        roleName: "Planner",
        status: "running",
        turnCount: 1,
        costUsd: 0.2,
        budgetUsd: 5,
        createdAt: "2026-03-24T09:00:00.000Z",
        startedAt: "2026-03-24T09:00:00.000Z",
        lastActivityAt: "2026-03-24T09:00:00.000Z",
      },
    });

    client?.emit("agent.output", {
      type: "agent.output",
      payload: {
        agent_id: "run-1",
        line: "Planning the implementation.",
      },
    });

    client?.emit("agent.progress", {
      type: "agent.progress",
      payload: {
        id: "run-1",
        taskId: "task-1",
        memberId: "member-1",
        status: "paused",
        createdAt: "2026-03-24T09:00:00.000Z",
        startedAt: "2026-03-24T09:00:00.000Z",
        lastActivityAt: "2026-03-24T09:10:00.000Z",
        canResume: true,
      },
    });

    expect(useAgentStore.getState().agentOutputs.get("run-1")).toEqual([
      "Planning the implementation.",
    ]);
    expect(useAgentStore.getState().pool).toEqual(
      expect.objectContaining({
        active: 0,
        max: 2,
        available: 2,
        pausedResumable: 1,
        queued: 0,
        warm: 0,
      }),
    );
  });

  it("applies blocked dispatch envelopes so task views do not miss assignment failures", () => {
    useWSStore.getState().connect("ws://localhost:7777/ws", "token");

    const client = getLatestClient();
    expect(client).toBeDefined();

    client?.emit("task.dispatch_blocked", {
      type: "task.dispatch_blocked",
      payload: {
        task: {
          id: "task-2",
          projectId: "project-1",
          title: "Dispatch stalled task",
          description: "",
          status: "assigned",
          priority: "high",
          assigneeId: "member-1",
          assigneeType: "agent",
          spentUsd: 0,
          createdAt: "2026-03-24T09:00:00.000Z",
          updatedAt: "2026-03-24T09:15:00.000Z",
        },
        dispatch: {
          status: "blocked",
          reason: "agent pool is at capacity",
        },
      },
    });

    expect(useTaskStore.getState().tasks[0]).toEqual(
      expect.objectContaining({
        id: "task-2",
        status: "assigned",
      }),
    );
    expect(useDashboardStore.getState().tasks[0]).toEqual(
      expect.objectContaining({
        id: "task-2",
      }),
    );
  });

  it("shows a budget warning toast from websocket envelopes", () => {
    useWSStore.getState().connect("ws://localhost:7777/ws", "token");

    const client = getLatestClient();
    expect(client).toBeDefined();

    client?.emit("budget.warning", {
      type: "budget.warning",
      payload: {
        taskId: "task-budget",
        scope: "project",
        message: "project budget warning",
      },
    });

    expect(toastWarningMock).toHaveBeenCalledWith(
      expect.stringContaining("Budget warning"),
      expect.objectContaining({
        description: "project budget warning",
      }),
    );
  });

  it("stores workflow trigger activity from websocket envelopes", () => {
    useWSStore.getState().connect("ws://localhost:7777/ws", "token");

    const client = getLatestClient();
    expect(client).toBeDefined();

    client?.emit("workflow.trigger_fired", {
      type: "workflow.trigger_fired",
      projectId: "project-1",
      payload: {
        taskId: "task-3",
        action: "notify",
        from: "triaged",
        to: "assigned",
        config: { channel: "team" },
      },
    });

    expect(useWorkflowStore.getState().recentActivityByProject["project-1"]).toEqual([
      expect.objectContaining({
        taskId: "task-3",
        action: "notify",
        from: "triaged",
        to: "assigned",
      }),
    ]);
  });

  it("hydrates review stores from websocket review events", () => {
    useReviewStore.setState({
      reviewsByTask: { "task-1": [] },
      allReviews: [],
      allReviewsLoading: false,
      loading: false,
      error: null,
    });
    useWSStore.getState().connect("ws://localhost:7777/ws", "token");

    const client = getLatestClient();
    expect(client).toBeDefined();

    client?.emit("review.pending_human", {
      type: "review.pending_human",
      payload: {
        id: "review-1",
        taskId: "task-1",
        prUrl: "https://example.com/pr/1",
        prNumber: 1,
        layer: 2,
        status: "pending_human",
        riskLevel: "high",
        findings: [],
        summary: "needs approval",
        recommendation: "approve",
        costUsd: 0.3,
        createdAt: "2026-03-26T10:00:00.000Z",
        updatedAt: "2026-03-26T10:05:00.000Z",
      },
    });

    client?.emit("review.updated", {
      type: "review.updated",
      payload: {
        id: "review-1",
        taskId: "task-1",
        prUrl: "https://example.com/pr/1",
        prNumber: 1,
        layer: 2,
        status: "completed",
        riskLevel: "medium",
        findings: [],
        summary: "approved by human",
        recommendation: "approve",
        costUsd: 0.3,
        createdAt: "2026-03-26T10:00:00.000Z",
        updatedAt: "2026-03-26T10:07:00.000Z",
      },
    });

    expect(useReviewStore.getState().allReviews[0]).toEqual(
      expect.objectContaining({
        id: "review-1",
        status: "completed",
      }),
    );
    expect(useReviewStore.getState().reviewsByTask["task-1"][0]).toEqual(
      expect.objectContaining({
        id: "review-1",
      }),
    );
  });

  it("hydrates nested review payloads and agent events with fallback timestamps", () => {
    useReviewStore.setState({
      reviewsByTask: { "task-2": [] },
      allReviews: [],
      allReviewsLoading: false,
      loading: false,
      error: null,
    });
    useAgentStore.setState({
      agents: [],
      agentOutputs: new Map(),
      pool: null,
      loading: false,
    });
    useWSStore.getState().connect("ws://localhost:7777/ws", "token");
    const client = getLatestClient();

    client?.emit("review.completed", {
      payload: {
        review: {
          id: "review-nested",
          taskId: "task-2",
          prUrl: "https://example.com/pr/2",
          prNumber: 2,
          layer: 2,
          status: "completed",
          riskLevel: "low",
          findings: [],
          summary: "nested review",
          recommendation: "approve",
          costUsd: 0.1,
          createdAt: "2026-03-26T10:00:00.000Z",
          updatedAt: "2026-03-26T10:05:00.000Z",
        },
      },
    });
    client?.emit("agent.cost_update", {
      payload: {
        id: "run-fallback",
        taskId: "task-2",
        memberId: "member-2",
        status: "running",
      },
    });

    expect(useReviewStore.getState().allReviews[0]).toEqual(
      expect.objectContaining({
        id: "review-nested",
      }),
    );
    expect(useAgentStore.getState().agents[0]).toMatchObject({
      id: "run-fallback",
      createdAt: expect.any(String),
      startedAt: expect.any(String),
      lastActivity: expect.any(String),
    });
  });

  it("hydrates scheduler job updates and run history from websocket envelopes", () => {
    useWSStore.getState().connect("ws://localhost:7777/ws", "token");

    const client = getLatestClient();
    expect(client).toBeDefined();

    client?.emit("scheduler.job.updated", {
      type: "scheduler.job.updated",
      payload: {
        job: {
          jobKey: "task-progress-detector",
          name: "Task progress detector",
          scope: "system",
          schedule: "0 * * * *",
          enabled: false,
          executionMode: "os_registered",
          overlapPolicy: "skip",
          lastRunStatus: "failed",
          lastRunSummary: "bridge offline",
          lastError: "bridge offline",
          config: "{}",
          createdAt: "2026-03-25T10:00:00.000Z",
          updatedAt: "2026-03-25T11:00:00.000Z",
        },
      },
    });

    client?.emit("scheduler.run.completed", {
      type: "scheduler.run.completed",
      payload: {
        run: {
          runId: "run-1",
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
      },
    });

    expect(useSchedulerStore.getState().jobs).toEqual([
      expect.objectContaining({
        jobKey: "task-progress-detector",
        enabled: false,
        executionMode: "os_registered",
      }),
    ]);
    expect(useSchedulerStore.getState().runsByJobKey["task-progress-detector"]).toEqual([
      expect.objectContaining({
        runId: "run-1",
        triggerSource: "cron",
        status: "failed",
      }),
    ]);
  });

  it("refreshes docs workspace slices on wiki websocket events", () => {
    const refreshTree = jest
      .spyOn(useDocsStore.getState(), "refreshActiveProjectTree")
      .mockResolvedValue(undefined);
    const refreshComments = jest
      .spyOn(useDocsStore.getState(), "refreshActivePageComments")
      .mockResolvedValue(undefined);
    const fetchPage = jest
      .spyOn(useDocsStore.getState(), "fetchPage")
      .mockResolvedValue(undefined);
    const fetchVersions = jest
      .spyOn(useDocsStore.getState(), "fetchVersions")
      .mockResolvedValue(undefined);

    useWSStore.getState().connect("ws://localhost:7777/ws", "token");

    const client = getLatestClient();
    expect(client).toBeDefined();

    client?.emit("wiki.page.updated", {
      type: "wiki.page.updated",
      payload: { id: "page-1" },
    });
    client?.emit("wiki.comment.created", {
      type: "wiki.comment.created",
      payload: { pageId: "page-1" },
    });
    client?.emit("wiki.version.published", {
      type: "wiki.version.published",
      payload: { pageId: "page-1" },
    });

    expect(refreshTree).toHaveBeenCalled();
    expect(fetchPage).toHaveBeenCalledWith("project-1", "page-1");
    expect(refreshComments).toHaveBeenCalled();
    expect(fetchVersions).toHaveBeenCalledWith("project-1", "page-1");
  });

  it("refreshes docs tree for move, delete, and comment resolution events", () => {
    const refreshTree = jest
      .spyOn(useDocsStore.getState(), "refreshActiveProjectTree")
      .mockResolvedValue(undefined);
    const refreshComments = jest
      .spyOn(useDocsStore.getState(), "refreshActivePageComments")
      .mockResolvedValue(undefined);

    useWSStore.getState().connect("ws://localhost:7777/ws", "token");
    const client = getLatestClient();

    client?.emit("wiki.page.moved", {});
    client?.emit("wiki.page.deleted", {});
    client?.emit("wiki.comment.resolved", {});

    expect(refreshTree).toHaveBeenCalledTimes(2);
    expect(refreshComments).toHaveBeenCalledTimes(1);
  });

  it("updates link and task-comment stores from websocket envelopes", () => {
    useWSStore.getState().connect("ws://localhost:7777/ws", "token");

    const client = getLatestClient();
    expect(client).toBeDefined();

    client?.emit("link.created", {
      type: "link.created",
      projectId: "project-1",
      payload: {
        id: "link-1",
        projectId: "project-1",
        sourceType: "task",
        sourceId: "task-1",
        targetType: "wiki_page",
        targetId: "page-1",
        linkType: "requirement",
        createdBy: "user-1",
        createdAt: "2026-03-26T10:00:00.000Z",
      },
    });

    client?.emit("task_comment.created", {
      type: "task_comment.created",
      projectId: "project-1",
      payload: {
        id: "comment-1",
        taskId: "task-1",
        body: "hello",
        mentions: [],
        createdBy: "user-1",
        createdAt: "2026-03-26T10:01:00.000Z",
        updatedAt: "2026-03-26T10:01:00.000Z",
      },
    });

    expect(useEntityLinkStore.getState().linksByEntity["task:task-1"]).toEqual([
      expect.objectContaining({ id: "link-1" }),
    ]);
    expect(useTaskCommentStore.getState().commentsByTask["task-1"]).toEqual([
      expect.objectContaining({ id: "comment-1", body: "hello" }),
    ]);
  });

  it("hydrates explicit agent pool summary updates from websocket envelopes", () => {
    useWSStore.getState().connect("ws://localhost:7777/ws", "token");

    const client = getLatestClient();
    expect(client).toBeDefined();

    client?.emit("agent.pool.updated", {
      type: "agent.pool.updated",
      payload: {
        active: 1,
        max: 3,
        available: 2,
        pausedResumable: 0,
        queued: 2,
        warm: 1,
        degraded: false,
        queue: [
          {
            entryId: "queue-1",
            taskId: "task-queued-1",
            memberId: "member-1",
            status: "queued",
            reason: "agent pool is at capacity",
            createdAt: "2026-03-25T12:00:00.000Z",
            updatedAt: "2026-03-25T12:00:00.000Z",
          },
        ],
      },
    });

    expect(useAgentStore.getState().pool).toEqual(
      expect.objectContaining({
        active: 1,
        max: 3,
        queued: 2,
        warm: 1,
        queue: [expect.objectContaining({ entryId: "queue-1" })],
      }),
    );
  });

  it("applies task lifecycle, deletion, and progress recovery events", () => {
    useTaskStore.setState({
      tasks: [
        {
          id: "task-1",
          projectId: "project-1",
          title: "Original",
          description: "",
          status: "assigned",
          priority: "medium",
          assigneeId: null,
          assigneeType: null,
          assigneeName: null,
          cost: null,
          budgetUsd: 0,
          spentUsd: 0,
          agentBranch: "",
          agentWorktree: "",
          agentSessionId: "",
          labels: [],
          blockedBy: [],
          plannedStartAt: null,
          plannedEndAt: null,
          createdAt: "2026-03-24T09:00:00.000Z",
          updatedAt: "2026-03-24T09:00:00.000Z",
        },
      ],
      loading: false,
      error: null,
    });
    useWSStore.getState().connect("ws://localhost:7777/ws", "token");
    const client = getLatestClient();

    const taskPayload = {
      id: "task-1",
      projectId: "project-1",
      title: "Recovered task",
      description: "",
      status: "in_progress",
      priority: "high",
      spentUsd: 1.2,
      createdAt: "2026-03-24T09:00:00.000Z",
      updatedAt: "2026-03-24T09:30:00.000Z",
    };

    client?.emit("task.updated", { payload: { task: taskPayload } });
    client?.emit("task.transitioned", { payload: { task: { ...taskPayload, status: "in_review" } } });
    client?.emit("task.assigned", { payload: { task: { ...taskPayload, assigneeId: "member-1" } } });
    client?.emit("task.progress.alerted", { payload: { task: { ...taskPayload, progress: { healthStatus: "stalled" } } } });
    client?.emit("task.progress.recovered", { payload: { task: { ...taskPayload, progress: { healthStatus: "healthy" } } } });

    expect(useTaskStore.getState().tasks[0]).toEqual(
      expect.objectContaining({
        id: "task-1",
        progress: expect.objectContaining({
          healthStatus: "healthy",
        }),
      }),
    );

    client?.emit("task.deleted", { payload: { id: "task-1" } });
    expect(useTaskStore.getState().tasks).toEqual([]);
  });

  it("hydrates raw task-created, transitioned, assigned, and alerted envelopes", () => {
    useWSStore.getState().connect("ws://localhost:7777/ws", "token");
    const client = getLatestClient();

    client?.emit("task.created", {
      id: "task-raw",
      projectId: "project-1",
      title: "Created raw",
      description: "",
      status: "assigned",
      priority: "medium",
      createdAt: "2026-03-24T09:00:00.000Z",
      updatedAt: "2026-03-24T09:00:00.000Z",
    });
    client?.emit("task.transitioned", {
      payload: {
        task: {
          id: "task-raw",
          projectId: "project-1",
          title: "Created raw",
          description: "",
          status: "in_review",
          priority: "medium",
          createdAt: "2026-03-24T09:00:00.000Z",
          updatedAt: "2026-03-24T09:10:00.000Z",
        },
      },
    });
    client?.emit("task.assigned", {
      payload: {
        task: {
          id: "task-raw",
          projectId: "project-1",
          title: "Created raw",
          description: "",
          status: "in_review",
          priority: "medium",
          assigneeId: "member-2",
          assigneeType: "human",
          createdAt: "2026-03-24T09:00:00.000Z",
          updatedAt: "2026-03-24T09:15:00.000Z",
        },
      },
    });
    client?.emit("task.progress.updated", {
      payload: {
        task: {
          id: "task-raw",
          projectId: "project-1",
          title: "Created raw",
          description: "",
          status: "in_review",
          priority: "medium",
          createdAt: "2026-03-24T09:00:00.000Z",
          updatedAt: "2026-03-24T09:20:00.000Z",
          progress: {
            healthStatus: "warning",
          },
        },
      },
    });
    client?.emit("task.dispatch_blocked", {
      payload: {
        task: {
          id: "task-raw",
          projectId: "project-1",
          title: "Created raw",
          description: "",
          status: "assigned",
          priority: "medium",
          createdAt: "2026-03-24T09:00:00.000Z",
          updatedAt: "2026-03-24T09:25:00.000Z",
        },
      },
    });
    client?.emit("task.progress.alerted", {
      payload: {
        task: {
          id: "task-raw",
          projectId: "project-1",
          title: "Created raw",
          description: "",
          status: "assigned",
          priority: "medium",
          createdAt: "2026-03-24T09:00:00.000Z",
          updatedAt: "2026-03-24T09:30:00.000Z",
          progress: {
            healthStatus: "stalled",
            riskReason: "no_recent_update",
          },
        },
      },
    });

    expect(useTaskStore.getState().tasks).toEqual([
      expect.objectContaining({
        id: "task-raw",
        progress: expect.objectContaining({
          healthStatus: "stalled",
          riskReason: "no_recent_update",
        }),
      }),
    ]);
    expect(useDashboardStore.getState().tasks).toEqual([
      expect.objectContaining({
        id: "task-raw",
      }),
    ]);
  });

  it("applies budget exceeded, sprint, and team events", () => {
    useTaskStore.setState({
      tasks: [
        {
          id: "task-budget",
          projectId: "project-1",
          title: "Budgeted task",
          description: "",
          status: "in_progress",
          priority: "medium",
          assigneeId: null,
          assigneeType: null,
          assigneeName: null,
          cost: null,
          spentUsd: 1,
          budgetUsd: 5,
          agentBranch: "",
          agentWorktree: "",
          agentSessionId: "",
          labels: [],
          blockedBy: [],
          plannedStartAt: null,
          plannedEndAt: null,
          createdAt: "2026-03-24T09:00:00.000Z",
          updatedAt: "2026-03-24T09:00:00.000Z",
        },
      ],
      loading: false,
      error: null,
    });
    useWSStore.getState().connect("ws://localhost:7777/ws", "token");
    const client = getLatestClient();

    client?.emit("budget.exceeded", {
      payload: {
        taskId: "task-budget",
        spent: 6,
        budget: 5,
      },
    });
    client?.emit("sprint.updated", {
      payload: {
        id: "sprint-1",
        projectId: "project-1",
        name: "Sprint 1",
        goal: "",
        status: "active",
        startDate: "2026-03-24",
        endDate: "2026-03-31",
        createdAt: "2026-03-24T09:00:00.000Z",
        updatedAt: "2026-03-24T09:00:00.000Z",
      },
    });
    client?.emit("team.created", {
      payload: {
        id: "team-1",
        projectId: "project-1",
        taskId: "task-budget",
        taskTitle: "Budgeted task",
        name: "Runtime team",
        status: "planning",
        strategy: "pipeline",
        runtime: "codex",
        provider: "openai",
        model: "gpt-5-codex",
        coderRunIds: [],
        totalBudgetUsd: 5,
        totalSpentUsd: 1,
        createdAt: "2026-03-24T09:00:00.000Z",
        updatedAt: "2026-03-24T09:00:00.000Z",
      },
    });

    expect(useTaskStore.getState().tasks[0]).toEqual(
      expect.objectContaining({
        status: "budget_exceeded",
        spentUsd: 6,
        budgetUsd: 5,
      }),
    );
    expect(useSprintStore.getState().sprintsByProject["project-1"]).toEqual([
      expect.objectContaining({ id: "sprint-1" }),
    ]);
    expect(useTeamStore.getState().teams).toEqual([
      expect.objectContaining({ id: "team-1", strategy: "pipeline" }),
    ]);
  });

  it("refreshes docs tree when the active page context is missing", () => {
    const refreshTree = jest
      .spyOn(useDocsStore.getState(), "refreshActiveProjectTree")
      .mockResolvedValue(undefined);
    const fetchPage = jest
      .spyOn(useDocsStore.getState(), "fetchPage")
      .mockResolvedValue(undefined);

    useDocsStore.setState({
      projectId: "project-1",
      tree: [],
      currentPage: null,
      comments: [],
      versions: [],
      templates: [],
      favorites: [],
      recentAccess: [],
      loading: false,
      saving: false,
      error: null,
    });
    useWSStore.getState().connect("ws://localhost:7777/ws", "token");
    const client = getLatestClient();

    client?.emit("wiki.page.created", {
      payload: { id: "page-2" },
    });

    expect(refreshTree).toHaveBeenCalled();
    expect(fetchPage).not.toHaveBeenCalled();
  });

  it("removes deleted links and resolves task comments in place", () => {
    useEntityLinkStore.setState({
      linksByEntity: {
        "task:task-1": [
          {
            id: "link-1",
            projectId: "project-1",
            sourceType: "task",
            sourceId: "task-1",
            targetType: "wiki_page",
            targetId: "page-1",
            linkType: "requirement",
            anchorBlockId: null,
            createdBy: "user-1",
            createdAt: "2026-03-26T10:00:00.000Z",
            deletedAt: null,
          },
        ],
      },
      loading: false,
      error: null,
    });
    useTaskCommentStore.setState({
      commentsByTask: {
        "task-1": [
          {
            id: "comment-1",
            taskId: "task-1",
            parentCommentId: null,
            body: "hello",
            mentions: [],
            resolvedAt: null,
            createdBy: "user-1",
            createdAt: "2026-03-26T10:01:00.000Z",
            updatedAt: "2026-03-26T10:01:00.000Z",
            deletedAt: null,
          },
        ],
      },
      loading: false,
      error: null,
    });
    useWSStore.getState().connect("ws://localhost:7777/ws", "token");
    const client = getLatestClient();

    client?.emit("link.deleted", {
      payload: { id: "link-1" },
    });
    client?.emit("task_comment.resolved", {
      payload: {
        taskId: "task-1",
        id: "comment-1",
        resolved: true,
      },
    });

    expect(useEntityLinkStore.getState().linksByEntity["task:task-1"]).toEqual(
      [],
    );
    expect(
      useTaskCommentStore.getState().commentsByTask["task-1"][0]?.resolvedAt,
    ).not.toBeNull();
  });

  it("applies agent queue events (queued, cancelled, promoted, failed) via applyAgentEvent", () => {
    useWSStore.getState().connect("ws://localhost:7777/ws", "token");
    const client = getLatestClient();

    const basePayload = {
      id: "run-q1",
      taskId: "task-1",
      memberId: "member-1",
      status: "queued",
      createdAt: "2026-04-01T09:00:00.000Z",
      startedAt: "2026-04-01T09:00:00.000Z",
      lastActivityAt: "2026-04-01T09:00:00.000Z",
    };

    client?.emit("agent.queued", { payload: { ...basePayload, status: "queued" } });
    expect(useAgentStore.getState().agents[0]).toEqual(
      expect.objectContaining({ id: "run-q1", status: "queued" }),
    );

    client?.emit("agent.queue.promoted", { payload: { ...basePayload, status: "running" } });
    expect(useAgentStore.getState().agents[0]).toEqual(
      expect.objectContaining({ id: "run-q1", status: "running" }),
    );

    client?.emit("agent.queue.cancelled", { payload: { ...basePayload, status: "cancelled" } });
    expect(useAgentStore.getState().agents[0]).toEqual(
      expect.objectContaining({ id: "run-q1", status: "cancelled" }),
    );

    client?.emit("agent.queue.failed", { payload: { ...basePayload, status: "failed" } });
    expect(useAgentStore.getState().agents[0]).toEqual(
      expect.objectContaining({ id: "run-q1", status: "failed" }),
    );

    // Pool should stay in sync
    expect(useAgentStore.getState().pool).toEqual(
      expect.objectContaining({ active: 0, pausedResumable: 0 }),
    );
  });

  it("applies agent streaming events (tool_call, tool_result, reasoning, file_change, todo_update, partial_message, permission_request)", () => {
    useWSStore.getState().connect("ws://localhost:7777/ws", "token");
    const client = getLatestClient();

    client?.emit("agent.tool_call", {
      payload: {
        agentId: "agent-s1",
        toolName: "Read",
        toolCallId: "tc-1",
        input: { path: "/foo" },
        turnNumber: 1,
      },
    });
    expect(useAgentStore.getState().agentToolCalls.get("agent-s1")).toEqual([
      expect.objectContaining({ toolName: "Read", toolCallId: "tc-1" }),
    ]);

    client?.emit("agent.tool_result", {
      payload: {
        agentId: "agent-s1",
        toolName: "Read",
        toolCallId: "tc-1",
        output: "file contents",
        isError: false,
      },
    });
    expect(useAgentStore.getState().agentToolResults.get("agent-s1")).toEqual([
      expect.objectContaining({ toolName: "Read", output: "file contents", isError: false }),
    ]);

    client?.emit("agent.reasoning", {
      payload: { agentId: "agent-s1", content: "Thinking about the problem..." },
    });
    expect(useAgentStore.getState().agentReasoning.get("agent-s1")).toBe(
      "Thinking about the problem...",
    );

    client?.emit("agent.file_change", {
      payload: {
        agentId: "agent-s1",
        files: [{ path: "src/index.ts", changeType: "modified" }],
      },
    });
    expect(useAgentStore.getState().agentFileChanges.get("agent-s1")).toEqual([
      expect.objectContaining({ path: "src/index.ts", changeType: "modified" }),
    ]);

    client?.emit("agent.todo_update", {
      payload: {
        agentId: "agent-s1",
        todos: [{ id: "todo-1", content: "Fix bug", status: "in_progress" }],
      },
    });
    expect(useAgentStore.getState().agentTodos.get("agent-s1")).toEqual([
      expect.objectContaining({ id: "todo-1", content: "Fix bug" }),
    ]);

    client?.emit("agent.partial_message", {
      payload: { agentId: "agent-s1", content: "Partial output so far..." },
    });
    expect(useAgentStore.getState().agentPartialMessages.get("agent-s1")).toBe(
      "Partial output so far...",
    );

    client?.emit("agent.permission_request", {
      payload: {
        agentId: "agent-s1",
        requestId: "perm-1",
        toolName: "Bash",
      },
    });
    expect(useAgentStore.getState().agentPermissionRequests.get("agent-s1")).toEqual([
      expect.objectContaining({ requestId: "perm-1", toolName: "Bash" }),
    ]);
  });

  it("applies agent.snapshot via applyAgentEvent", () => {
    useWSStore.getState().connect("ws://localhost:7777/ws", "token");
    const client = getLatestClient();

    client?.emit("agent.snapshot", {
      payload: {
        id: "run-snap",
        taskId: "task-1",
        memberId: "member-1",
        status: "running",
        turnCount: 5,
        costUsd: 1.5,
        createdAt: "2026-04-01T09:00:00.000Z",
        startedAt: "2026-04-01T09:00:00.000Z",
        lastActivityAt: "2026-04-01T09:05:00.000Z",
      },
    });

    expect(useAgentStore.getState().agents[0]).toEqual(
      expect.objectContaining({ id: "run-snap", status: "running", turns: 5 }),
    );
  });

  it("applies review.created and review.fix_requested via applyReviewEvent", () => {
    useReviewStore.setState({
      reviewsByTask: { "task-1": [] },
      allReviews: [],
      allReviewsLoading: false,
      loading: false,
      error: null,
    });
    useWSStore.getState().connect("ws://localhost:7777/ws", "token");
    const client = getLatestClient();

    client?.emit("review.created", {
      payload: {
        id: "review-c1",
        taskId: "task-1",
        prUrl: "https://example.com/pr/10",
        prNumber: 10,
        layer: 1,
        status: "in_progress",
        riskLevel: "low",
        findings: [],
        summary: "new review",
        recommendation: "approve",
        costUsd: 0.1,
        createdAt: "2026-04-01T10:00:00.000Z",
        updatedAt: "2026-04-01T10:00:00.000Z",
      },
    });

    expect(useReviewStore.getState().allReviews[0]).toEqual(
      expect.objectContaining({ id: "review-c1", status: "in_progress" }),
    );

    client?.emit("review.fix_requested", {
      payload: {
        id: "review-c1",
        taskId: "task-1",
        prUrl: "https://example.com/pr/10",
        prNumber: 10,
        layer: 1,
        status: "fix_requested",
        riskLevel: "medium",
        findings: [],
        summary: "needs fixes",
        recommendation: "request_changes",
        costUsd: 0.2,
        createdAt: "2026-04-01T10:00:00.000Z",
        updatedAt: "2026-04-01T10:05:00.000Z",
      },
    });

    expect(useReviewStore.getState().allReviews[0]).toEqual(
      expect.objectContaining({ id: "review-c1", status: "fix_requested" }),
    );
  });

  it("applies task.dependency_resolved by upserting the task", () => {
    useWSStore.getState().connect("ws://localhost:7777/ws", "token");
    const client = getLatestClient();

    client?.emit("task.dependency_resolved", {
      payload: {
        task: {
          id: "task-dep",
          projectId: "project-1",
          title: "Unblocked task",
          description: "",
          status: "ready",
          priority: "medium",
          createdAt: "2026-04-01T09:00:00.000Z",
          updatedAt: "2026-04-01T09:10:00.000Z",
        },
      },
    });

    expect(useTaskStore.getState().tasks[0]).toEqual(
      expect.objectContaining({ id: "task-dep", status: "ready" }),
    );
    expect(useDashboardStore.getState().tasks[0]).toEqual(
      expect.objectContaining({ id: "task-dep" }),
    );
  });

  it("ignores malformed websocket envelopes without mutating stores", () => {
    useDashboardStore.setState({
      summary: null,
      projects: [],
      selectedProjectId: null,
      tasks: [],
      members: [],
      agents: [],
      activity: [],
      loading: false,
      error: null,
      sectionErrors: {},
    });
    useWSStore.getState().connect("ws://localhost:7777/ws", "token");
    const client = getLatestClient();

    client?.emit("task.updated", { payload: {} });
    client?.emit("task.created", null);
    client?.emit("task.deleted", { payload: {} });
    client?.emit("review.completed", { payload: {} });
    client?.emit("agent.output", { payload: { line: "missing agent id" } });
    client?.emit("agent.pool.updated", null);
    client?.emit("budget.warning", { payload: { scope: "task" } });
    client?.emit("budget.exceeded", { payload: {} });
    client?.emit("notification", null);
    client?.emit("plugin.lifecycle", null);
    client?.emit("workflow.trigger_fired", null);
    client?.emit("scheduler.job.updated", { payload: {} });
    client?.emit("sprint.updated", { payload: {} });
    client?.emit("team.created", { payload: {} });
    client?.emit("link.created", null);
    client?.emit("link.deleted", { payload: {} });
    client?.emit("task_comment.created", null);
    client?.emit("task_comment.resolved", { payload: {} });
    client?.emit("agent.queued", { payload: {} });
    client?.emit("agent.queue.cancelled", null);
    client?.emit("agent.queue.promoted", { payload: {} });
    client?.emit("agent.queue.failed", null);
    client?.emit("agent.tool_call", { payload: {} });
    client?.emit("agent.tool_result", null);
    client?.emit("agent.reasoning", { payload: { agentId: "x" } });
    client?.emit("agent.file_change", { payload: { agentId: "x" } });
    client?.emit("agent.todo_update", { payload: { agentId: "x" } });
    client?.emit("agent.partial_message", { payload: {} });
    client?.emit("agent.permission_request", { payload: { agentId: "x" } });
    client?.emit("agent.snapshot", null);
    client?.emit("review.created", { payload: {} });
    client?.emit("review.fix_requested", null);
    client?.emit("task.dependency_resolved", { payload: {} });

    expect(useTaskStore.getState().tasks).toEqual([]);
    expect(useNotificationStore.getState().notifications).toEqual([]);
    expect(useWorkflowStore.getState().recentActivityByProject).toEqual({});
    expect(emitProjectedDesktopEventMock).not.toHaveBeenCalled();
    expect(toastWarningMock).not.toHaveBeenCalled();
  });
});
