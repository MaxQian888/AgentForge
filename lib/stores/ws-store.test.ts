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

import { useDashboardStore } from "./dashboard-store";
import { useNotificationStore } from "./notification-store";
import { useTaskStore } from "./task-store";
import { useWSStore } from "./ws-store";

describe("useWSStore", () => {
  beforeEach(() => {
    useTaskStore.setState({ tasks: [], loading: false });
    useNotificationStore.setState({ notifications: [], unreadCount: 0 });
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
    useWSStore.getState().disconnect();
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
});
