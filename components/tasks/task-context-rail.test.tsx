import { render, screen } from "@testing-library/react";
import { TaskContextRail } from "./task-context-rail";
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
  spentUsd: 2.5,
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

describe("TaskContextRail", () => {
  it("renders summary mode when no task is selected", () => {
    render(
      <TaskContextRail
        selectionState="summary"
        selectedTask={null}
        counts={{ healthy: 3, warning: 1, stalled: 2, unscheduled: 4 }}
        alerts={[stalledAlert]}
        realtimeState="live"
      />
    );

    expect(screen.getByText("Progress health")).toBeInTheDocument();
    expect(screen.getByText("Healthy 3")).toBeInTheDocument();
    expect(screen.getByText("Warning 1")).toBeInTheDocument();
    expect(screen.getByText("Stalled 2")).toBeInTheDocument();
    expect(screen.getByText("Unscheduled 4")).toBeInTheDocument();
    expect(
      screen.getByText("Task stalled: Implement timeline view")
    ).toBeInTheDocument();
  });

  it("renders a hidden-by-filter message when the selected task is not visible", () => {
    render(
      <TaskContextRail
        selectionState="hidden_by_filter"
        selectedTask={task}
        counts={{ healthy: 0, warning: 1, stalled: 0, unscheduled: 0 }}
        alerts={[recoveredAlert]}
        realtimeState="degraded"
      />
    );

    expect(screen.getByText(/outside the current filters/i)).toBeInTheDocument();
    expect(screen.getByText("Task recovered: Calendar polish")).toBeInTheDocument();
    expect(screen.getByText(/realtime updates unavailable/i)).toBeInTheDocument();
  });

  it("renders selected-task mode with editable and read-only task fields", () => {
    render(
      <TaskContextRail
        selectionState="selected_visible"
        selectedTask={task}
        counts={{ healthy: 0, warning: 1, stalled: 0, unscheduled: 0 }}
        alerts={[]}
        realtimeState="live"
        onTaskSave={jest.fn()}
        onTaskStatusChange={jest.fn()}
      />
    );

    expect(screen.getByDisplayValue("Implement timeline view")).toBeInTheDocument();
    expect(screen.getByDisplayValue("2026-03-25")).toBeInTheDocument();
    expect(screen.getByText(/Alice/)).toBeInTheDocument();
    expect(screen.getAllByText(/No recent update/)).not.toHaveLength(0);
    expect(screen.getByText(/Last activity/i)).toBeInTheDocument();
  });
});
