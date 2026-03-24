import type { Notification } from "@/lib/stores/notification-store";
import type { Agent } from "@/lib/stores/agent-store";
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
  budgetUsd: 5,
  spentUsd: 3.5,
  agentBranch: "",
  agentWorktree: "",
  agentSessionId: "",
  blockedBy: [],
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
  status: "blocked",
  priority: "medium",
  assigneeId: null,
  assigneeType: null,
  assigneeName: null,
  cost: null,
  spentUsd: 0,
  blockedBy: ["task-1"],
  plannedStartAt: null,
  plannedEndAt: null,
  progress: {
    ...baseTask.progress!,
    healthStatus: "warning",
    riskReason: "no_assignee",
  },
};

const completedTask: Task = {
  ...baseTask,
  id: "task-3",
  title: "Ship task workspace",
  status: "done",
  cost: 4.25,
  budgetUsd: 4,
  spentUsd: 4.25,
  blockedBy: [],
};

const readyTask: Task = {
  ...baseTask,
  id: "task-4",
  title: "Polish rollout",
  status: "blocked",
  cost: 0,
  spentUsd: 0,
  blockedBy: ["task-3"],
};

const agents: Agent[] = [
  {
    id: "agent-1",
    taskId: "task-1",
    taskTitle: "Implement timeline view",
    memberId: "member-2",
    roleId: "frontend-agent",
    roleName: "Frontend agent",
    status: "running",
    provider: "anthropic",
    model: "claude-sonnet-4-6",
    turns: 5,
    cost: 1.75,
    budget: 3,
    worktreePath: "",
    branchName: "agent/task-1",
    sessionId: "session-1",
    lastActivity: "2026-03-24T10:10:00.000Z",
    startedAt: "2026-03-24T10:00:00.000Z",
    createdAt: "2026-03-24T10:00:00.000Z",
    completedAt: null,
    canResume: true,
    memoryStatus: "available",
  },
];

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
      agents,
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
      agents,
    });

    expect(result.selectionState).toBe("summary");
    expect(result.selectedTask).toBeNull();
  });

  it("derives health counts and project-scoped progress alerts", () => {
    const result = buildContextRailState({
      tasks: [baseTask, unscheduledTask, completedTask, readyTask],
      filteredTasks: [baseTask, unscheduledTask, completedTask, readyTask],
      selectedTaskId: null,
      projectId: "project-1",
      notifications,
      agents,
    });

    expect(result.counts).toEqual({
      healthy: 0,
      warning: 1,
      stalled: 3,
      unscheduled: 1,
    });
    expect(result.alerts).toEqual([
      expect.objectContaining({
        id: "notification-1",
        type: "task_progress_stalled",
      }),
    ]);
    expect(result.dependencySummary).toEqual({
      blocked: 1,
      readyToUnblock: 1,
    });
    expect(result.costSummary).toEqual({
      totalSpentUsd: 7.75,
      totalBudgetUsd: 19,
      budgetedTaskCount: 4,
      overBudgetTaskCount: 1,
      activeRunCostUsd: 1.75,
      activeRunBudgetUsd: 3,
      activeRunCount: 1,
    });
  });
});
