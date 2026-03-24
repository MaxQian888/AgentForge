"use client";

import { Draggable } from "@hello-pangea/dnd";
import { cn } from "@/lib/utils";
import { Badge } from "@/components/ui/badge";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import type { Task, TaskPriority } from "@/lib/stores/task-store";

const priorityColors: Record<TaskPriority, string> = {
  urgent: "bg-red-500/15 text-red-700 dark:text-red-400",
  high: "bg-orange-500/15 text-orange-700 dark:text-orange-400",
  medium: "bg-blue-500/15 text-blue-700 dark:text-blue-400",
  low: "bg-zinc-500/15 text-zinc-600 dark:text-zinc-400",
};

const progressColors = {
  healthy: "bg-emerald-500/15 text-emerald-700 dark:text-emerald-400",
  warning: "bg-amber-500/15 text-amber-700 dark:text-amber-400",
  stalled: "bg-rose-500/15 text-rose-700 dark:text-rose-400",
} as const;

function formatProgressHealth(label: NonNullable<Task["progress"]>["healthStatus"]) {
  switch (label) {
    case "warning":
      return "At risk";
    case "stalled":
      return "Stalled";
    default:
      return "Healthy";
  }
}

function formatProgressReason(reason: string) {
  switch (reason) {
    case "no_recent_update":
      return "No recent update";
    case "no_assignee":
      return "No assignee";
    case "awaiting_review":
      return "Awaiting review";
    default:
      return reason || "Needs attention";
  }
}

interface TaskCardProps {
  task: Task;
  index: number;
  onClick: () => void;
}

export function TaskCard({ task, index, onClick }: TaskCardProps) {
  return (
    <Draggable draggableId={task.id} index={index}>
      {(provided, snapshot) => (
        <div
          ref={provided.innerRef}
          {...provided.draggableProps}
          {...provided.dragHandleProps}
          onClick={onClick}
          className={cn(
            "cursor-pointer rounded-md border bg-card p-3 shadow-sm transition-shadow hover:shadow-md",
            snapshot.isDragging && "shadow-lg ring-2 ring-primary/20"
          )}
        >
          <p className="mb-2 text-sm font-medium leading-snug">
            {task.title}
          </p>
          {task.progress && task.progress.healthStatus !== "healthy" && (
            <div className="mb-2 flex items-center gap-2">
              <Badge
                variant="secondary"
                className={cn("text-[11px]", progressColors[task.progress.healthStatus])}
              >
                {formatProgressHealth(task.progress.healthStatus)}
              </Badge>
              <span className="text-[11px] text-muted-foreground">
                {formatProgressReason(task.progress.riskReason)}
              </span>
            </div>
          )}
          <div className="flex items-center justify-between">
            <Badge
              variant="secondary"
              className={cn("text-xs", priorityColors[task.priority])}
            >
              {task.priority}
            </Badge>
            <div className="flex items-center gap-2">
              {task.cost != null && (
                <span className="text-xs text-muted-foreground">
                  ${task.cost.toFixed(2)}
                </span>
              )}
              {task.assigneeName && (
                <Avatar className="size-5">
                  <AvatarFallback className="text-[10px]">
                    {task.assigneeName[0]?.toUpperCase()}
                  </AvatarFallback>
                </Avatar>
              )}
            </div>
          </div>
        </div>
      )}
    </Draggable>
  );
}
