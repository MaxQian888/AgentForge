import type { Task, TaskPriority, TaskStatus } from "@/lib/stores/task-store";
import { getTaskDependencyState } from "./task-dependencies";

export type TaskViewMode = "board" | "list" | "timeline" | "calendar" | "dependencies";
export type TaskPlanningFilter = "all" | "scheduled" | "unscheduled";
export type TaskDependencyFilter = "all" | "blocked" | "ready_to_unblock";
export type TaskFilterOption<T extends string> = "all" | T;

export interface TaskWorkspaceFilters {
  search: string;
  status: TaskFilterOption<TaskStatus>;
  priority: TaskFilterOption<TaskPriority>;
  assigneeId: string | "all";
  sprintId: string | "all";
  planning: TaskPlanningFilter;
  dependency: TaskDependencyFilter;
}

export function createDefaultTaskWorkspaceFilters(): TaskWorkspaceFilters {
  return {
    search: "",
    status: "all",
    priority: "all",
    assigneeId: "all",
    sprintId: "all",
    planning: "all",
    dependency: "all",
  };
}

function taskMatchesSearch(task: Task, search: string): boolean {
  if (!search) return true;
  const normalized = search.trim().toLowerCase();
  if (!normalized) return true;

  return [task.title, task.description, task.assigneeName ?? ""].some((value) =>
    value.toLowerCase().includes(normalized)
  );
}

function taskPlanningState(task: Task): TaskPlanningFilter {
  return task.plannedStartAt && task.plannedEndAt ? "scheduled" : "unscheduled";
}

export function filterTasksForWorkspace(
  tasks: Task[],
  filters: TaskWorkspaceFilters
): Task[] {
  return tasks.filter((task) => {
    if (!taskMatchesSearch(task, filters.search)) return false;
    if (filters.status !== "all" && task.status !== filters.status) return false;
    if (filters.priority !== "all" && task.priority !== filters.priority) return false;
    if (filters.assigneeId !== "all" && task.assigneeId !== filters.assigneeId) {
      return false;
    }
    if (filters.sprintId !== "all" && task.sprintId !== filters.sprintId) {
      return false;
    }
    if (filters.planning !== "all" && taskPlanningState(task) !== filters.planning) {
      return false;
    }
    if (
      filters.dependency !== "all" &&
      getTaskDependencyState(task, tasks).state !== filters.dependency
    ) {
      return false;
    }
    return true;
  });
}

function addDays(date: Date, days: number): Date {
  const next = new Date(date);
  next.setUTCDate(next.getUTCDate() + days);
  return next;
}

function differenceInCalendarDays(end: Date, start: Date): number {
  const startDate = Date.UTC(
    start.getUTCFullYear(),
    start.getUTCMonth(),
    start.getUTCDate()
  );
  const endDate = Date.UTC(
    end.getUTCFullYear(),
    end.getUTCMonth(),
    end.getUTCDate()
  );

  return Math.round((endDate - startDate) / (24 * 60 * 60 * 1000));
}

function toIsoDateString(date: Date): string {
  return date.toISOString();
}

export function getRescheduledPlanningWindow(
  task: Task,
  targetDateKey: string
): { plannedStartAt: string; plannedEndAt: string } {
  const baseStart = task.plannedStartAt ? new Date(task.plannedStartAt) : null;
  const baseEnd = task.plannedEndAt ? new Date(task.plannedEndAt) : null;
  const targetStart = new Date(`${targetDateKey}T09:00:00.000Z`);

  if (!baseStart || !baseEnd) {
    return {
      plannedStartAt: toIsoDateString(targetStart),
      plannedEndAt: toIsoDateString(new Date(`${targetDateKey}T18:00:00.000Z`)),
    };
  }

  const durationDays = differenceInCalendarDays(baseEnd, baseStart);
  const targetEnd = addDays(targetStart, durationDays);
  targetEnd.setUTCHours(baseEnd.getUTCHours(), baseEnd.getUTCMinutes(), 0, 0);

  return {
    plannedStartAt: toIsoDateString(targetStart),
    plannedEndAt: toIsoDateString(targetEnd),
  };
}
