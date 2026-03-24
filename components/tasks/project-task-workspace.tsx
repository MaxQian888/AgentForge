"use client";

import { useMemo } from "react";
import { TaskContextRail } from "./task-context-rail";
import { TaskWorkspaceMain } from "./task-workspace-main";
import {
  buildContextRailState,
  type TaskContextRailSelectionState,
} from "@/lib/tasks/task-context-rail";
import { filterTasksForWorkspace } from "@/lib/tasks/task-workspace";
import type { Notification } from "@/lib/stores/notification-store";
import type { Task, TaskStatus } from "@/lib/stores/task-store";
import { useTaskWorkspaceStore } from "@/lib/stores/task-workspace-store";

interface ProjectTaskWorkspaceProps {
  projectId: string;
  tasks: Task[];
  loading: boolean;
  error: string | null;
  realtimeConnected: boolean;
  notifications: Notification[];
  onRetry: () => void;
  onTaskOpen: (taskId: string) => void;
  onTaskStatusChange: (
    taskId: string,
    nextStatus: TaskStatus
  ) => Promise<void> | void;
  onTaskScheduleChange: (
    taskId: string,
    changes: { plannedStartAt: string; plannedEndAt: string }
  ) => Promise<void> | void;
  onTaskSave: (taskId: string, data: Partial<Task>) => Promise<void> | void;
}

export function ProjectTaskWorkspace({
  projectId,
  tasks,
  loading,
  error,
  realtimeConnected,
  notifications,
  onRetry,
  onTaskOpen,
  onTaskStatusChange,
  onTaskScheduleChange,
  onTaskSave,
}: ProjectTaskWorkspaceProps) {
  const filters = useTaskWorkspaceStore((state) => state.filters);
  const selectedTaskId = useTaskWorkspaceStore((state) => state.selectedTaskId);

  const filteredTasks = useMemo(
    () => filterTasksForWorkspace(tasks, filters),
    [tasks, filters]
  );

  const rail = useMemo(
    () =>
      buildContextRailState({
        tasks,
        filteredTasks,
        selectedTaskId,
        projectId,
        notifications,
      }),
    [filteredTasks, notifications, projectId, selectedTaskId, tasks]
  );

  return (
    <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_360px]">
      <TaskWorkspaceMain
        tasks={tasks}
        loading={loading}
        error={error}
        onRetry={onRetry}
        onTaskOpen={onTaskOpen}
        onTaskStatusChange={onTaskStatusChange}
        onTaskScheduleChange={onTaskScheduleChange}
      />
      <TaskContextRail
        selectionState={rail.selectionState as TaskContextRailSelectionState}
        selectedTask={rail.selectedTask}
        counts={rail.counts}
        alerts={rail.alerts}
        realtimeState={realtimeConnected ? "live" : "degraded"}
        onTaskSave={onTaskSave}
        onTaskStatusChange={onTaskStatusChange}
      />
    </div>
  );
}
