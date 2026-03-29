"use client";

import { useTranslations } from "next-intl";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { TaskDetailContent } from "./task-detail-content";
import { TaskProgressSummary } from "./task-progress-summary";
import { TaskRecentAlerts } from "./task-recent-alerts";
import type {
  TaskContextRailSelectionState,
  TaskCostSummary,
  TaskHealthCounts,
} from "@/lib/tasks/task-context-rail";
import type { TaskDependencySummary } from "@/lib/tasks/task-dependencies";
import type { TeamMember } from "@/lib/dashboard/summary";
import type { Agent } from "@/lib/stores/agent-store";
import type { Notification } from "@/lib/stores/notification-store";
import type { Sprint } from "@/lib/stores/sprint-store";
import type {
  Task,
  TaskDecompositionResult,
  TaskStatus,
} from "@/lib/stores/task-store";

function formatRelativeTime(isoDate: string): string {
  const now = Date.now();
  const then = new Date(isoDate).getTime();
  const diffMs = now - then;
  if (diffMs < 0) return "just now";

  const minutes = Math.floor(diffMs / 60_000);
  if (minutes < 1) return "just now";
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

export interface TaskContextRailProps {
  selectionState: TaskContextRailSelectionState;
  selectedTask: Task | null;
  counts: TaskHealthCounts;
  dependencySummary: TaskDependencySummary;
  costSummary: TaskCostSummary;
  alerts: Notification[];
  realtimeState: "live" | "degraded";
  tasks: Task[];
  members: TeamMember[];
  agents: Agent[];
  sprints?: Sprint[];
  onTaskSave?: (taskId: string, data: Partial<Task>) => Promise<void> | void;
  onTaskAssign?: (
    taskId: string,
    assigneeId: string,
    assigneeType: "human" | "agent"
  ) => Promise<void> | void;
  onTaskStatusChange?: (
    taskId: string,
    nextStatus: TaskStatus
  ) => Promise<void> | void;
  onTaskDecompose?: (
    taskId: string
  ) => Promise<TaskDecompositionResult | null> | TaskDecompositionResult | null | void;
  onSpawnAgent?: (
    taskId: string,
    memberId: string
  ) => Promise<void> | void;
  onTaskDelete?: (taskId: string) => Promise<void> | void;
  onResetFilters?: () => void;
}

export function TaskContextRail({
  selectionState,
  selectedTask,
  counts,
  dependencySummary,
  costSummary,
  alerts,
  realtimeState,
  tasks,
  members,
  agents,
  sprints,
  onTaskSave,
  onTaskAssign,
  onTaskStatusChange,
  onTaskDecompose,
  onSpawnAgent,
  onTaskDelete,
  onResetFilters,
}: TaskContextRailProps) {
  const t = useTranslations("tasks");
  const stalledTasks = tasks.filter(
    (task) =>
      task.progress?.healthStatus === "stalled" ||
      task.progress?.healthStatus === "warning"
  );

  return (
    <aside className="flex flex-col gap-4">
      <TaskProgressSummary
        counts={counts}
        dependencySummary={dependencySummary}
        costSummary={costSummary}
        realtimeState={realtimeState}
      />

      {stalledTasks.length > 0 ? (
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-base">{t("attention.title")}</CardTitle>
          </CardHeader>
          <CardContent className="flex flex-col gap-3">
            {stalledTasks.slice(0, 5).map((st) => (
              <div
                key={st.id}
                className="flex flex-col gap-2 rounded-md border border-border/60 bg-muted/20 p-3 text-sm"
              >
                <div className="flex items-center justify-between gap-2">
                  <span className="truncate font-medium">{st.title}</span>
                  <Badge
                    variant="secondary"
                    className={
                      st.progress?.healthStatus === "stalled"
                        ? "bg-red-500/15 text-red-700 dark:text-red-300"
                        : "bg-amber-500/15 text-amber-700 dark:text-amber-300"
                    }
                  >
                    {st.progress?.healthStatus === "stalled" ? t("health.stalled") : t("health.atRisk")}
                  </Badge>
                </div>
                {st.progress?.lastActivityAt ? (
                  <div className="text-xs text-muted-foreground">
                    {t("attention.lastActivity", { time: formatRelativeTime(st.progress.lastActivityAt) })}
                  </div>
                ) : null}
                <div className="flex flex-wrap gap-1.5">
                  {onTaskAssign ? (
                    <Button
                      type="button"
                      size="sm"
                      variant="outline"
                      onClick={() => {
                        const firstActive = members.find((m) => m.isActive && m.id !== st.assigneeId);
                        if (firstActive) {
                          void onTaskAssign(st.id, firstActive.id, firstActive.type);
                        }
                      }}
                    >
                      {t("attention.reassign")}
                    </Button>
                  ) : null}
                  {onTaskStatusChange ? (
                    <Button
                      type="button"
                      size="sm"
                      variant="outline"
                      onClick={() => void onTaskStatusChange(st.id, "cancelled")}
                    >
                      {t("attention.cancelTask")}
                    </Button>
                  ) : null}
                </div>
              </div>
            ))}
          </CardContent>
        </Card>
      ) : null}

      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-base">{t("detail.title")}</CardTitle>
        </CardHeader>
        <CardContent>
          {selectionState === "summary" || !selectedTask ? (
            <div className="text-sm text-muted-foreground">
              {t("detail.selectPrompt")}
            </div>
          ) : (
            <div className="flex flex-col gap-4">
              {selectionState === "hidden_by_filter" ? (
                <div className="flex flex-wrap items-center gap-2 text-sm text-muted-foreground">
                  <span>
                    {t("detail.hiddenByFilter")}
                  </span>
                  {onResetFilters ? (
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      onClick={onResetFilters}
                    >
                      {t("detail.resetFilters")}
                    </Button>
                  ) : null}
                </div>
              ) : null}
              <TaskDetailContent
                key={selectedTask.id}
                task={selectedTask}
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
              />
            </div>
          )}
        </CardContent>
      </Card>

      <TaskRecentAlerts alerts={alerts} />
    </aside>
  );
}
