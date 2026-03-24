import type { Notification } from "@/lib/stores/notification-store";
import type { Task } from "@/lib/stores/task-store";
import { buildContextRailState } from "./task-context-rail";

const baseTask: Task = {
  id: "task-1",
  projectId: "project-1",
  title: "Implement timeline view",
  description: "Build the workspace timeline lane.",
  status: "in_progress",
  priority: "high",
  assigneeId: "member-1",
  assigneeType: "human",
  assigneeName: "Alice",
  cost: 3.5,
  spentUsd: 3.5,
  plannedStartAt: "2026-03-25T09:00:00.000Z",
  plannedEndAt: "2026-03-27T18:00:00.000Z",
  progress: {
    lastActivityAt: "2026-03-24T10:00:00.000Z",
    lastActivitySource: "agent_heartbeat",
    lastTransitionAt: "2026-03-24T09:30:00.000Z",
    healthStatus: "stalled",
    riskReason: "no_recent_update",
    riskSinceAt: "2026-03-24T09:45:00.000Z",
    lastAlertState: "stalled:no_recent_update",
    lastAlertAt: "2026-03-24T09:50:00.000Z",
    lastRecoveredAt: null,
  },
  createdAt: "2026-03-24T09:00:00.000Z",
  updatedAt: "2026-03-24T10:05:00.000Z",
};

const unscheduledTask: Task = {
  ...baseTask,
  id: "task-2",
  title: "Calendar polish",
  status: "triaged",
  priority: "medium",
  assigneeId: null,
  assigneeType: null,
  assigneeName: null,
  cost: null,
  spentUsd: 0,
  plannedStartAt: null,
  plannedEndAt: null,
  progress: {
    ...baseTask.progress!,
    healthStatus: "warning",
    riskReason: "no_assignee",
  },
};

const notifications: Notification[] = [
  {
    id: "notification-1",
    targetId: "member-1",
    type: "task_progress_stalled",
    title: "Task stalled: Implement timeline view",
    message: "Implement timeline view is stalled.",
    href: "/project?id=project-1#task-task-1",
    read: false,
    createdAt: "2026-03-24T10:15:00.000Z",
  },
  {
    id: "notification-2",
    targetId: "member-1",
    type: "task_progress_recovered",
    title: "Task recovered: Search flow",
    message: "Recovered outside this project.",
    href: "/project?id=project-2#task-task-9",
    read: false,
    createdAt: "2026-03-24T10:20:00.000Z",
  },
];

describe("buildContextRailState", () => {
  it("keeps a selected task active even when current filters hide it", () => {
    const result = buildContextRailState({
      tasks: [baseTask, unscheduledTask],
      filteredTasks: [unscheduledTask],
      selectedTaskId: "task-1",
      projectId: "project-1",
      notifications,
    });

    expect(result.selectionState).toBe("hidden_by_filter");
    expect(result.selectedTask?.id).toBe("task-1");
  });

  it("falls back to summary mode when the selected task is not in the task source", () => {
    const result = buildContextRailState({
      tasks: [baseTask],
      filteredTasks: [baseTask],
      selectedTaskId: "task-missing",
      projectId: "project-1",
      notifications,
    });

    expect(result.selectionState).toBe("summary");
    expect(result.selectedTask).toBeNull();
  });

  it("derives health counts and project-scoped progress alerts", () => {
    const result = buildContextRailState({
      tasks: [baseTask, unscheduledTask],
      filteredTasks: [baseTask, unscheduledTask],
      selectedTaskId: null,
      projectId: "project-1",
      notifications,
    });

    expect(result.counts).toEqual({
      healthy: 0,
      warning: 1,
      stalled: 1,
      unscheduled: 1,
    });
    expect(result.alerts).toEqual([
      expect.objectContaining({
        id: "notification-1",
        type: "task_progress_stalled",
      }),
    ]);
  });
});
