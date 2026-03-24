import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { TaskDetailContent } from "./task-detail-content";
import { TaskProgressSummary } from "./task-progress-summary";
import { TaskRecentAlerts } from "./task-recent-alerts";
import type {
  TaskContextRailSelectionState,
  TaskHealthCounts,
} from "@/lib/tasks/task-context-rail";
import type { Notification } from "@/lib/stores/notification-store";
import type { Task, TaskStatus } from "@/lib/stores/task-store";

export interface TaskContextRailProps {
  selectionState: TaskContextRailSelectionState;
  selectedTask: Task | null;
  counts: TaskHealthCounts;
  alerts: Notification[];
  realtimeState: "live" | "degraded";
  onTaskSave?: (taskId: string, data: Partial<Task>) => Promise<void> | void;
  onTaskStatusChange?: (
    taskId: string,
    nextStatus: TaskStatus
  ) => Promise<void> | void;
}

export function TaskContextRail({
  selectionState,
  selectedTask,
  counts,
  alerts,
  realtimeState,
  onTaskSave,
  onTaskStatusChange,
}: TaskContextRailProps) {
  return (
    <aside className="flex flex-col gap-4">
      <TaskProgressSummary counts={counts} realtimeState={realtimeState} />

      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-base">Task details</CardTitle>
        </CardHeader>
        <CardContent>
          {selectionState === "summary" || !selectedTask ? (
            <div className="text-sm text-muted-foreground">
              Select a task to inspect its details.
            </div>
          ) : (
            <div className="flex flex-col gap-4">
              {selectionState === "hidden_by_filter" ? (
                <div className="text-sm text-muted-foreground">
                  This task is outside the current filters, but it remains selected.
                </div>
              ) : null}
              <TaskDetailContent
                key={selectedTask.id}
                task={selectedTask}
                onTaskSave={onTaskSave}
                onTaskStatusChange={onTaskStatusChange}
              />
            </div>
          )}
        </CardContent>
      </Card>

      <TaskRecentAlerts alerts={alerts} />
    </aside>
  );
}
