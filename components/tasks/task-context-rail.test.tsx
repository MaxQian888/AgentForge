import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { TaskContextRail } from "./task-context-rail";
import type { TeamMember } from "@/lib/dashboard/summary";
import type { Agent } from "@/lib/stores/agent-store";
import type { Notification } from "@/lib/stores/notification-store";
import type { Task } from "@/lib/stores/task-store";

const task: Task = {
  id: "task-1",
  projectId: "project-1",
  title: "Implement timeline view",
  description: "Build the horizontal planning lane.",
  status: "in_progress",
  priority: "high",
  assigneeId: "member-1",
  assigneeType: "human",
  assigneeName: "Alice",
  cost: 2.5,
  budgetUsd: 5,
  spentUsd: 2.5,
  agentBranch: "",
  agentWorktree: "",
  agentSessionId: "",
  blockedBy: [],
  plannedStartAt: "2026-03-25T09:00:00.000Z",
  plannedEndAt: "2026-03-27T18:00:00.000Z",
  progress: {
    lastActivityAt: "2026-03-24T09:15:00.000Z",
    lastActivitySource: "detector",
    lastTransitionAt: "2026-03-24T09:10:00.000Z",
    healthStatus: "stalled",
    riskReason: "no_recent_update",
    riskSinceAt: "2026-03-24T09:20:00.000Z",
    lastAlertState: "stalled:no_recent_update",
    lastAlertAt: "2026-03-24T09:25:00.000Z",
    lastRecoveredAt: null,
  },
  createdAt: "2026-03-24T09:00:00.000Z",
  updatedAt: "2026-03-24T09:30:00.000Z",
};

const blockedTask: Task = {
  ...task,
  id: "task-2",
  title: "Calendar polish",
  status: "blocked",
  assigneeId: null,
  assigneeType: null,
  assigneeName: null,
  cost: 0.25,
  budgetUsd: 2,
  spentUsd: 0.25,
  blockedBy: ["task-1"],
  plannedStartAt: null,
  plannedEndAt: null,
};

const stalledAlert: Notification = {
  id: "alert-1",
  type: "task_progress_stalled",
  title: "Task stalled: Implement timeline view",
  message: "Implement timeline view is stalled.",
  href: "/project?id=project-1#task-task-1",
  read: false,
  createdAt: "2026-03-24T12:00:00.000Z",
};

const recoveredAlert: Notification = {
  id: "alert-2",
  type: "task_progress_recovered",
  title: "Task recovered: Calendar polish",
  message: "Calendar polish is healthy again.",
  href: "/project?id=project-1#task-task-2",
  read: false,
  createdAt: "2026-03-24T12:05:00.000Z",
};

const members: TeamMember[] = [
  {
    id: "member-1",
    projectId: "project-1",
    name: "Alice",
    type: "human",
    typeLabel: "Human",
    role: "Frontend engineer",
    email: "",
    avatarUrl: "",
    skills: ["frontend", "timeline"],
    isActive: true,
    status: "active",
    createdAt: "2026-03-24T09:00:00.000Z",
    lastActivityAt: null,
    workload: {
      assignedTasks: 0,
      inProgressTasks: 0,
      inReviewTasks: 0,
      activeAgentRuns: 0,
    },
  },
];

const agents: Agent[] = [];

describe("TaskContextRail", () => {
  it("renders summary mode when no task is selected", () => {
    render(
      <TaskContextRail
        selectionState="summary"
        selectedTask={null}
        counts={{ healthy: 3, warning: 1, stalled: 2, unscheduled: 4 }}
        dependencySummary={{ blocked: 1, readyToUnblock: 0 }}
        costSummary={{
          totalSpentUsd: 2.75,
          totalBudgetUsd: 7,
          budgetedTaskCount: 2,
          overBudgetTaskCount: 0,
          activeRunCostUsd: 1.1,
          activeRunBudgetUsd: 2.5,
          activeRunCount: 1,
        }}
        alerts={[stalledAlert]}
        realtimeState="live"
        tasks={[task, blockedTask]}
        members={members}
        agents={agents}
      />
    );

    expect(screen.getByText("Progress health")).toBeInTheDocument();
    expect(screen.getByText("Healthy 3")).toBeInTheDocument();
    expect(screen.getByText("Warning 1")).toBeInTheDocument();
    expect(screen.getByText("Stalled 2")).toBeInTheDocument();
    expect(screen.getByText("Unscheduled 4")).toBeInTheDocument();
    expect(screen.getByText("Blocked 1")).toBeInTheDocument();
    expect(screen.getByText("Task spend $2.75 / $7.00")).toBeInTheDocument();
    expect(
      screen.getByText("Task stalled: Implement timeline view")
    ).toBeInTheDocument();
  });

  it("renders a hidden-by-filter message when the selected task is not visible", () => {
    const onResetFilters = jest.fn();

    render(
      <TaskContextRail
        selectionState="hidden_by_filter"
        selectedTask={task}
        counts={{ healthy: 0, warning: 1, stalled: 0, unscheduled: 0 }}
        dependencySummary={{ blocked: 1, readyToUnblock: 0 }}
        costSummary={{
          totalSpentUsd: 2.75,
          totalBudgetUsd: 7,
          budgetedTaskCount: 2,
          overBudgetTaskCount: 0,
          activeRunCostUsd: 0,
          activeRunBudgetUsd: 0,
          activeRunCount: 0,
        }}
        alerts={[recoveredAlert]}
        realtimeState="degraded"
        tasks={[task, blockedTask]}
        members={members}
        agents={agents}
        onResetFilters={onResetFilters}
      />
    );

    expect(screen.getByText(/outside the current filters/i)).toBeInTheDocument();
    expect(screen.getByText("Task recovered: Calendar polish")).toBeInTheDocument();
    expect(screen.getByText(/realtime updates unavailable/i)).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Reset filters" })
    ).toBeInTheDocument();
  });

  it("lets the user recover a hidden task by resetting current filters", async () => {
    const user = userEvent.setup();
    const onResetFilters = jest.fn();

    render(
      <TaskContextRail
        selectionState="hidden_by_filter"
        selectedTask={task}
        counts={{ healthy: 0, warning: 1, stalled: 0, unscheduled: 0 }}
        dependencySummary={{ blocked: 1, readyToUnblock: 0 }}
        costSummary={{
          totalSpentUsd: 2.75,
          totalBudgetUsd: 7,
          budgetedTaskCount: 2,
          overBudgetTaskCount: 0,
          activeRunCostUsd: 0,
          activeRunBudgetUsd: 0,
          activeRunCount: 0,
        }}
        alerts={[recoveredAlert]}
        realtimeState="live"
        tasks={[task, blockedTask]}
        members={members}
        agents={agents}
        onResetFilters={onResetFilters}
      />
    );

    await user.click(screen.getByRole("button", { name: "Reset filters" }));

    expect(onResetFilters).toHaveBeenCalledTimes(1);
  });

  it("renders selected-task mode with editable and read-only task fields", () => {
    render(
      <TaskContextRail
        selectionState="selected_visible"
        selectedTask={blockedTask}
        counts={{ healthy: 0, warning: 1, stalled: 0, unscheduled: 0 }}
        dependencySummary={{ blocked: 1, readyToUnblock: 0 }}
        costSummary={{
          totalSpentUsd: 2.75,
          totalBudgetUsd: 7,
          budgetedTaskCount: 2,
          overBudgetTaskCount: 0,
          activeRunCostUsd: 0,
          activeRunBudgetUsd: 0,
          activeRunCount: 0,
        }}
        alerts={[]}
        realtimeState="live"
        tasks={[task, blockedTask]}
        members={members}
        agents={agents}
        onTaskSave={jest.fn()}
        onTaskAssign={jest.fn()}
        onTaskStatusChange={jest.fn()}
      />
    );

    expect(screen.getByDisplayValue("Calendar polish")).toBeInTheDocument();
    expect(screen.getByText("Design blocker")).toBeInTheDocument();
    expect(screen.getAllByText(/Implement timeline view/).length).toBeGreaterThan(0);
    expect(screen.getAllByText(/Last activity/i).length).toBeGreaterThan(0);
    expect(screen.getByText("Smart assignment")).toBeInTheDocument();
  });
});
