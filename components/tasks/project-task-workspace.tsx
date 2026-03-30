"use client";

import { useEffect, useMemo, useState } from "react";
import { useTranslations } from "next-intl";
import { PanelLeftClose, PanelRightClose } from "lucide-react";
import { Button } from "@/components/ui/button";
import { ProjectWorkspaceSidebar } from "./project-workspace-sidebar";
import { TaskContextRail } from "./task-context-rail";
import { TaskWorkspaceMain, type BulkActionResult } from "./task-workspace-main";
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

type WorkspaceLayout = "desktop" | "medium" | "narrow";
type CompactPanel = "none" | "sidebar" | "details";

function getLayout(): WorkspaceLayout {
  if (typeof window === "undefined") return "desktop";
  if (window.innerWidth >= 1280) return "desktop";
  if (window.innerWidth >= 768) return "medium";
  return "narrow";
}

interface ProjectTaskWorkspaceProps {
  projectId: string;
  projectName: string;
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
  onCreateTask?: () => void;
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
  onBulkStatusChange?: (ids: string[], status: TaskStatus) => Promise<BulkActionResult | void> | BulkActionResult | void;
  onBulkAssign?: (ids: string[], assigneeId: string, assigneeType: "human" | "agent") => Promise<BulkActionResult | void> | BulkActionResult | void;
  onBulkDelete?: (ids: string[]) => Promise<BulkActionResult | void> | BulkActionResult | void;
  onSprintFilterChange?: (sprintId: string | "all") => void;
}

export function ProjectTaskWorkspace({
  projectId,
  projectName,
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
  onCreateTask,
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
  const valuesByTask = useCustomFieldStore((state) => state.valuesByTask);
  const fetchViews = useSavedViewStore((state) => state.fetchViews);

  const [layout, setLayout] = useState<WorkspaceLayout>(() => getLayout());
  const [compactPanel, setCompactPanel] = useState<CompactPanel>("none");

  useEffect(() => {
    const handleResize = () => {
      const nextLayout = getLayout();
      setLayout(nextLayout);
      if (nextLayout === "desktop") {
        setCompactPanel("none");
      }
    };
    handleResize();
    window.addEventListener("resize", handleResize);
    return () => window.removeEventListener("resize", handleResize);
  }, []);

  const filteredTasks = useMemo(
    () => filterTasksForWorkspace(tasks, filters, { valuesByTask }),
    [tasks, filters, valuesByTask]
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

  const showRail = contextRailDisplay === "expanded";

  const sidebar = (
    <ProjectWorkspaceSidebar
      projectId={projectId}
      projectName={projectName}
      tasks={tasks}
      sprints={sprints}
      filteredCount={filteredTasks.length}
      realtimeConnected={realtimeConnected}
      onCreateTask={onCreateTask}
      onSprintFilterChange={onSprintFilterChange}
    />
  );

  const mainContent = (
    <div className="relative flex h-full flex-col">
      {/* Toggle buttons for context rail on desktop */}
      {layout === "desktop" && !showRail ? (
        <button
          type="button"
          className="absolute right-0 top-3 z-10 rounded-l-md border border-r-0 bg-background px-1.5 py-1.5 text-muted-foreground shadow-sm transition-colors hover:bg-accent hover:text-foreground"
          onClick={() => setContextRailDisplay("expanded")}
          title={t("sidebar.showDetails")}
        >
          <PanelRightClose className="size-4" />
        </button>
      ) : null}
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
        onCreateTask={onCreateTask}
        members={members}
        onBulkStatusChange={onBulkStatusChange}
        onBulkAssign={onBulkAssign}
        onBulkDelete={onBulkDelete}
        onSprintFilterChange={onSprintFilterChange}
      />
    </div>
  );

  const contextRail = (
    <div className="flex h-full flex-col">
      <div className="flex items-center justify-between border-b px-3 py-2">
        <span className="text-xs font-medium text-muted-foreground">
          {rail.selectedTask
            ? t("workspace.selectedTask", { title: rail.selectedTask.title })
            : t("workspace.noTaskSelected")}
        </span>
        <button
          type="button"
          className="rounded-md p-1 text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
          onClick={() => setContextRailDisplay("collapsed")}
          title={t("sidebar.collapseDetails")}
        >
          <PanelLeftClose className="size-4" />
        </button>
      </div>
      <div className="flex-1 overflow-y-auto">
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
    </div>
  );

  // Desktop: 3-column grid
  if (layout === "desktop") {
    return (
      <div className="flex h-full overflow-hidden border-t bg-card">
        <div
          className={cn(
            "grid h-full w-full divide-x divide-border",
            showRail
              ? "grid-cols-[260px_minmax(0,1fr)_320px]"
              : "grid-cols-[260px_minmax(0,1fr)]"
          )}
        >
          <div className="overflow-y-auto">{sidebar}</div>
          <div className="overflow-y-auto">{mainContent}</div>
          {showRail ? <div className="overflow-hidden">{contextRail}</div> : null}
        </div>
      </div>
    );
  }

  // Medium / Narrow: stacked with toggle buttons
  return (
    <div className="flex h-full flex-col overflow-hidden">
      <div className="flex items-center gap-2 border-b bg-card px-3 py-2">
        <Button
          type="button"
          size="sm"
          variant={compactPanel === "sidebar" ? "secondary" : "outline"}
          className="h-7 text-xs"
          onClick={() =>
            setCompactPanel((c) => (c === "sidebar" ? "none" : "sidebar"))
          }
        >
          {t("sidebar.showSidebar")}
        </Button>
        <Button
          type="button"
          size="sm"
          variant={compactPanel === "details" ? "secondary" : "outline"}
          className="h-7 text-xs"
          onClick={() =>
            setCompactPanel((c) => (c === "details" ? "none" : "details"))
          }
        >
          {t("sidebar.showDetails")}
        </Button>
      </div>

      {compactPanel === "sidebar" ? (
        <div className="border-b bg-card">{sidebar}</div>
      ) : null}
      {compactPanel === "details" ? (
        <div className="max-h-[50vh] overflow-y-auto border-b bg-card">
          {contextRail}
        </div>
      ) : null}

      <div className="min-h-0 flex-1 overflow-auto bg-card">
        {mainContent}
      </div>
    </div>
  );
}
