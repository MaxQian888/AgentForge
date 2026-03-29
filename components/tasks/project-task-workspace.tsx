"use client";

import { useEffect, useMemo } from "react";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { TaskContextRail } from "./task-context-rail";
import { TaskWorkspaceMain } from "./task-workspace-main";
import {
  buildContextRailState,
  type TaskContextRailSelectionState,
} from "@/lib/tasks/task-context-rail";
import type { TeamMember } from "@/lib/dashboard/summary";
import type { Agent } from "@/lib/stores/agent-store";
import type { Sprint, SprintMetrics } from "@/lib/stores/sprint-store";
import { filterTasksForWorkspace } from "@/lib/tasks/task-workspace";
import { cn } from "@/lib/utils";
import type { Notification } from "@/lib/stores/notification-store";
import type {
  Task,
  TaskDecompositionResult,
  TaskStatus,
} from "@/lib/stores/task-store";
import { useTaskWorkspaceStore } from "@/lib/stores/task-workspace-store";
import { useCustomFieldStore } from "@/lib/stores/custom-field-store";
import { useSavedViewStore } from "@/lib/stores/saved-view-store";

interface ProjectTaskWorkspaceProps {
  projectId: string;
  tasks: Task[];
  loading: boolean;
  error: string | null;
  realtimeConnected: boolean;
  notifications: Notification[];
  members: TeamMember[];
  agents: Agent[];
  sprints: Sprint[];
  sprintMetrics: SprintMetrics | null;
  sprintMetricsLoading: boolean;
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
  onTaskAssign: (
    taskId: string,
    assigneeId: string,
    assigneeType: "human" | "agent"
  ) => Promise<void> | void;
  onTaskDecompose?: (
    taskId: string
  ) => Promise<TaskDecompositionResult | null> | TaskDecompositionResult | null | void;
  onSpawnAgent?: (
    taskId: string,
    memberId: string
  ) => Promise<void> | void;
  onTaskDelete?: (taskId: string) => Promise<void> | void;
  onBulkStatusChange?: (ids: string[], status: TaskStatus) => void;
  onBulkAssign?: (ids: string[], assigneeId: string, assigneeType: "human" | "agent") => void;
  onBulkDelete?: (ids: string[]) => void;
  onSprintFilterChange?: (sprintId: string | "all") => void;
}

export function ProjectTaskWorkspace({
  projectId,
  tasks,
  loading,
  error,
  realtimeConnected,
  notifications,
  members,
  agents,
  sprints,
  sprintMetrics,
  sprintMetricsLoading,
  onRetry,
  onTaskOpen,
  onTaskStatusChange,
  onTaskScheduleChange,
  onTaskSave,
  onTaskAssign,
  onTaskDecompose,
  onSpawnAgent,
  onTaskDelete,
  onBulkStatusChange,
  onBulkAssign,
  onBulkDelete,
  onSprintFilterChange,
}: ProjectTaskWorkspaceProps) {
  const t = useTranslations("tasks");
  const filters = useTaskWorkspaceStore((state) => state.filters);
  const selectedTaskId = useTaskWorkspaceStore((state) => state.selectedTaskId);
  const selectTask = useTaskWorkspaceStore((state) => state.selectTask);
  const resetFilters = useTaskWorkspaceStore((state) => state.resetFilters);
  const contextRailDisplay = useTaskWorkspaceStore(
    (state) => state.contextRailDisplay
  );
  const setContextRailDisplay = useTaskWorkspaceStore(
    (state) => state.setContextRailDisplay
  );
  const fetchDefinitions = useCustomFieldStore((state) => state.fetchDefinitions);
  const fetchTaskValues = useCustomFieldStore((state) => state.fetchTaskValues);
  const fetchViews = useSavedViewStore((state) => state.fetchViews);

  const filteredTasks = useMemo(
    () => filterTasksForWorkspace(tasks, filters),
    [tasks, filters]
  );

  useEffect(() => {
    if (selectedTaskId && !tasks.some((task) => task.id === selectedTaskId)) {
      selectTask(null);
    }
  }, [selectedTaskId, selectTask, tasks]);

  useEffect(() => {
    void fetchDefinitions(projectId);
    void fetchViews(projectId);
    for (const task of tasks) {
      void fetchTaskValues(projectId, task.id);
    }
  }, [fetchDefinitions, fetchTaskValues, fetchViews, projectId, tasks]);

  const rail = useMemo(
    () =>
      buildContextRailState({
        tasks,
        filteredTasks,
        selectedTaskId,
        projectId,
        notifications,
        agents,
      }),
    [agents, filteredTasks, notifications, projectId, selectedTaskId, tasks]
  );

  return (
    <div
      className={cn(
        "grid gap-4",
        contextRailDisplay === "expanded"
          ? "xl:grid-cols-[minmax(0,1fr)_360px]"
          : "xl:grid-cols-[minmax(0,1fr)_120px]"
      )}
    >
      <TaskWorkspaceMain
        projectId={projectId}
        tasks={tasks}
        sprints={sprints}
        sprintMetrics={sprintMetrics}
        sprintMetricsLoading={sprintMetricsLoading}
        loading={loading}
        error={error}
        realtimeConnected={realtimeConnected}
        onRetry={onRetry}
        onTaskOpen={onTaskOpen}
        onTaskStatusChange={onTaskStatusChange}
        onTaskScheduleChange={onTaskScheduleChange}
        onTaskSave={onTaskSave}
        members={members}
        onBulkStatusChange={onBulkStatusChange}
        onBulkAssign={onBulkAssign}
        onBulkDelete={onBulkDelete}
        onSprintFilterChange={onSprintFilterChange}
      />
      {contextRailDisplay === "expanded" ? (
        <div className="flex flex-col gap-3">
          <div className="flex justify-end">
            <Button
              type="button"
              size="sm"
              variant="outline"
              onClick={() => setContextRailDisplay("collapsed")}
            >
              {t("workspace.collapseRail")}
            </Button>
          </div>
          <TaskContextRail
            selectionState={rail.selectionState as TaskContextRailSelectionState}
            selectedTask={rail.selectedTask}
            counts={rail.counts}
            dependencySummary={rail.dependencySummary}
            costSummary={rail.costSummary}
            alerts={rail.alerts}
            realtimeState={realtimeConnected ? "live" : "degraded"}
            tasks={tasks}
            members={members}
            agents={agents}
            sprints={sprints}
            onTaskSave={onTaskSave}
            onTaskAssign={onTaskAssign}
            onTaskStatusChange={onTaskStatusChange}
            onTaskDecompose={onTaskDecompose}
            onSpawnAgent={onSpawnAgent}
            onTaskDelete={onTaskDelete}
            onResetFilters={resetFilters}
          />
        </div>
      ) : (
        <Card className="h-fit">
          <CardContent className="flex flex-col gap-3 px-4 py-4">
            <Button
              type="button"
              size="sm"
              variant="outline"
              onClick={() => setContextRailDisplay("expanded")}
            >
              {t("workspace.expandRail")}
            </Button>
            <div className="text-xs text-muted-foreground">
              {rail.selectedTask ? t("workspace.selectedTask", { title: rail.selectedTask.title }) : t("workspace.noTaskSelected")}
            </div>
            <div className="text-xs text-muted-foreground">
              {realtimeConnected ? t("workspace.realtimeLive") : t("workspace.realtimeDegraded")}
            </div>
            <div className="text-xs text-muted-foreground">
              {t("workspace.stalledCount", { count: rail.counts.stalled })}
            </div>
            <div className="text-xs text-muted-foreground">
              {t("workspace.recentAlerts", { count: rail.alerts.length })}
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  );
}
