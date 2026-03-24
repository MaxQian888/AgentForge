import type { Task } from "@/lib/stores/task-store";

export type TaskDependencyStateLabel =
  | "clear"
  | "blocked"
  | "ready_to_unblock";

export interface TaskDependencyReference {
  id: string;
  title: string;
  status: Task["status"] | "missing";
  isComplete: boolean;
  isMissing: boolean;
}

export interface TaskDownstreamReference {
  id: string;
  title: string;
  status: Task["status"];
}

export interface TaskDependencyState {
  state: TaskDependencyStateLabel;
  blockers: TaskDependencyReference[];
  blockedTasks: TaskDownstreamReference[];
}

export interface TaskDependencySummary {
  blocked: number;
  readyToUnblock: number;
}

function isTaskTerminal(task: Task): boolean {
  return task.status === "done" || task.status === "cancelled";
}

function isDependencyComplete(task: Task | undefined): boolean {
  return task?.status === "done";
}

export function getTaskDependencyState(
  task: Task,
  tasks: Task[]
): TaskDependencyState {
  const blockers = task.blockedBy.map<TaskDependencyReference>((blockerId) => {
    const blocker = tasks.find((item) => item.id === blockerId);
    return {
      id: blockerId,
      title: blocker?.title ?? blockerId,
      status: blocker?.status ?? "missing",
      isComplete: isDependencyComplete(blocker),
      isMissing: !blocker,
    };
  });

  const blockedTasks = tasks
    .filter((candidate) => candidate.blockedBy.includes(task.id))
    .map<TaskDownstreamReference>((candidate) => ({
      id: candidate.id,
      title: candidate.title,
      status: candidate.status,
    }));

  if (blockers.length === 0) {
    return {
      state: "clear",
      blockers,
      blockedTasks,
    };
  }

  const unresolvedBlockers = blockers.filter((blocker) => !blocker.isComplete);

  return {
    state:
      unresolvedBlockers.length === 0 ? "ready_to_unblock" : "blocked",
    blockers,
    blockedTasks,
  };
}

export function summarizeTaskDependencies(tasks: Task[]): TaskDependencySummary {
  return tasks.reduce<TaskDependencySummary>(
    (summary, task) => {
      if (isTaskTerminal(task) || task.blockedBy.length === 0) {
        return summary;
      }

      const state = getTaskDependencyState(task, tasks).state;
      if (state === "blocked") {
        summary.blocked += 1;
      } else if (state === "ready_to_unblock") {
        summary.readyToUnblock += 1;
      }

      return summary;
    },
    {
      blocked: 0,
      readyToUnblock: 0,
    }
  );
}
