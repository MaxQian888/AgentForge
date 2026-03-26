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

    connect() {}

    close() {}

    subscribe() {}

    unsubscribe() {}

    emit(event: string, payload: unknown) {
      this.handlers.get(event)?.(payload);
    }
  }

  return { WSClient: MockWSClient };
});

import { useAgentStore } from "./agent-store";
import { useDashboardStore } from "./dashboard-store";
import { useDocsStore } from "./docs-store";
import { useNotificationStore } from "./notification-store";
import { useSchedulerStore } from "./scheduler-store";
import { useTaskStore } from "./task-store";
import { useWorkflowStore } from "./workflow-store";
import { useWSStore } from "./ws-store";

describe("useWSStore", () => {
  beforeEach(() => {
    useTaskStore.setState({ tasks: [], loading: false, error: null });
    useAgentStore.setState({
      agents: [],
      agentOutputs: new Map(),
      pool: { active: 0, max: 2, available: 2, pausedResumable: 0 },
      loading: false,
    });
    useNotificationStore.setState({ notifications: [], unreadCount: 0 });
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
    useWSStore.getState().disconnect();
  });

  it("tracks websocket connection degradation explicitly", () => {
    useWSStore.getState().connect("ws://localhost:7777/ws", "token");

    const MockWSClient = jest.requireMock("@/lib/ws-client").WSClient as {
      instances: Array<{ emit: (event: string, payload: unknown) => void }>;
    };
    const client = MockWSClient.instances.at(-1);

    client?.emit("connected", null);
    expect(useWSStore.getState().connected).toBe(true);

    client?.emit("disconnected", null);
    expect(useWSStore.getState().connected).toBe(false);
  });

  it("applies progress updates and notifications from websocket payload envelopes", () => {
    useWSStore.getState().connect("ws://localhost:7777/ws", "token");

    const MockWSClient = jest.requireMock("@/lib/ws-client").WSClient as {
      instances: Array<{ emit: (event: string, payload: unknown) => void }>;
    };
    const client = MockWSClient.instances.at(-1);
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

    const MockWSClient = jest.requireMock("@/lib/ws-client").WSClient as {
      instances: Array<{ emit: (event: string, payload: unknown) => void }>;
    };
    const client = MockWSClient.instances.at(-1);
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

  it("hydrates agent output envelopes and keeps pool stats in sync with live agent events", () => {
    useWSStore.getState().connect("ws://localhost:7777/ws", "token");

    const MockWSClient = jest.requireMock("@/lib/ws-client").WSClient as {
      instances: Array<{ emit: (event: string, payload: unknown) => void }>;
    };
    const client = MockWSClient.instances.at(-1);
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

    const MockWSClient = jest.requireMock("@/lib/ws-client").WSClient as {
      instances: Array<{ emit: (event: string, payload: unknown) => void }>;
    };
    const client = MockWSClient.instances.at(-1);
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

  it("stores workflow trigger activity from websocket envelopes", () => {
    useWSStore.getState().connect("ws://localhost:7777/ws", "token");

    const MockWSClient = jest.requireMock("@/lib/ws-client").WSClient as {
      instances: Array<{ emit: (event: string, payload: unknown) => void }>;
    };
    const client = MockWSClient.instances.at(-1);
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

  it("hydrates scheduler job updates and run history from websocket envelopes", () => {
    useWSStore.getState().connect("ws://localhost:7777/ws", "token");

    const MockWSClient = jest.requireMock("@/lib/ws-client").WSClient as {
      instances: Array<{ emit: (event: string, payload: unknown) => void }>;
    };
    const client = MockWSClient.instances.at(-1);
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

    const MockWSClient = jest.requireMock("@/lib/ws-client").WSClient as {
      instances: Array<{ emit: (event: string, payload: unknown) => void }>;
    };
    const client = MockWSClient.instances.at(-1);
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

  it("hydrates explicit agent pool summary updates from websocket envelopes", () => {
    useWSStore.getState().connect("ws://localhost:7777/ws", "token");

    const MockWSClient = jest.requireMock("@/lib/ws-client").WSClient as {
      instances: Array<{ emit: (event: string, payload: unknown) => void }>;
    };
    const client = MockWSClient.instances.at(-1);
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
});
