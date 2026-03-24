import type { Notification } from "@/lib/stores/notification-store";
import type { Task } from "@/lib/stores/task-store";

export type TaskContextRailSelectionState =
  | "summary"
  | "selected_visible"
  | "hidden_by_filter";

export interface TaskHealthCounts {
  healthy: number;
  warning: number;
  stalled: number;
  unscheduled: number;
}

export interface BuildContextRailStateInput {
  tasks: Task[];
  filteredTasks: Task[];
  selectedTaskId: string | null;
  projectId: string;
  notifications: Notification[];
}

export interface TaskContextRailState {
  selectedTask: Task | null;
  selectionState: TaskContextRailSelectionState;
  counts: TaskHealthCounts;
  alerts: Notification[];
}

function summarizeTaskHealth(tasks: Task[]): TaskHealthCounts {
  return tasks.reduce<TaskHealthCounts>(
    (counts, task) => {
      if (!task.plannedStartAt || !task.plannedEndAt) {
        counts.unscheduled += 1;
      }

      switch (task.progress?.healthStatus) {
        case "warning":
          counts.warning += 1;
          break;
        case "stalled":
          counts.stalled += 1;
          break;
        default:
          counts.healthy += 1;
          break;
      }

      return counts;
    },
    {
      healthy: 0,
      warning: 0,
      stalled: 0,
      unscheduled: 0,
    }
  );
}

function notificationBelongsToProject(notification: Notification, projectId: string): boolean {
  return Boolean(notification.href?.includes(`/project?id=${projectId}`));
}

function selectProjectProgressAlerts(
  notifications: Notification[],
  projectId: string
): Notification[] {
  return notifications.filter((notification) => {
    const isProgressAlert =
      notification.type === "task_progress_warning" ||
      notification.type === "task_progress_stalled" ||
      notification.type === "task_progress_recovered";

    return isProgressAlert && notificationBelongsToProject(notification, projectId);
  });
}

export function buildContextRailState(
  input: BuildContextRailStateInput
): TaskContextRailState {
  const selectedTask =
    input.tasks.find((task) => task.id === input.selectedTaskId) ?? null;
  const visibleSelectedTask =
    selectedTask != null &&
    input.filteredTasks.some((task) => task.id === selectedTask.id);

  return {
    selectedTask,
    selectionState: !selectedTask
      ? "summary"
      : visibleSelectedTask
        ? "selected_visible"
        : "hidden_by_filter",
    counts: summarizeTaskHealth(input.tasks),
    alerts: selectProjectProgressAlerts(input.notifications, input.projectId),
  };
}
