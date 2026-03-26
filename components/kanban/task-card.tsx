"use client";

import { Draggable } from "@hello-pangea/dnd";
import Link from "next/link";
import { cn } from "@/lib/utils";
import { Badge } from "@/components/ui/badge";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import {
  Popover,
  PopoverContent,
  PopoverDescription,
  PopoverHeader,
  PopoverTitle,
  PopoverTrigger,
} from "@/components/ui/popover";
import type { Task, TaskPriority } from "@/lib/stores/task-store";
import type { LinkedDocItem } from "@/components/tasks/linked-docs-panel";
import { buildDocsHref } from "@/lib/route-hrefs";

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
  isSelected: boolean;
  density: "comfortable" | "compact";
  showDescription: boolean;
  linkedDocs?: LinkedDocItem[];
  onClick: () => void;
}

export function TaskCard({
  task,
  index,
  isSelected,
  density,
  showDescription,
  linkedDocs = [],
  onClick,
}: TaskCardProps) {
  const previewDoc = linkedDocs[0];

  return (
    <Draggable draggableId={task.id} index={index}>
      {(provided, snapshot) => (
        <div
          ref={provided.innerRef}
          {...provided.draggableProps}
          {...provided.dragHandleProps}
          onClick={onClick}
          data-task-id={task.id}
          data-selected={isSelected ? "true" : "false"}
          className={cn(
            "cursor-pointer rounded-md border bg-card shadow-sm transition-shadow hover:shadow-md",
            density === "compact" ? "p-2.5" : "p-3",
            isSelected && "ring-2 ring-primary/25 border-primary/40",
            snapshot.isDragging && "shadow-lg ring-2 ring-primary/20"
          )}
        >
          <p className="mb-2 text-sm font-medium leading-snug">
            {task.title}
          </p>
          {showDescription && task.description ? (
            <p className="mb-2 text-xs text-muted-foreground">
              {task.description}
            </p>
          ) : null}
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
              {previewDoc ? (
                <Popover>
                  <PopoverTrigger asChild>
                    <button
                      type="button"
                      aria-label={`Show linked docs for ${task.title}`}
                      className="rounded-full border border-border/60 px-2 py-0.5 text-[11px] text-muted-foreground hover:bg-accent/40"
                      onClick={(event) => event.stopPropagation()}
                    >
                      Docs {linkedDocs.length}
                    </button>
                  </PopoverTrigger>
                  <PopoverContent
                    onClick={(event) => event.stopPropagation()}
                    onMouseDown={(event) => event.stopPropagation()}
                  >
                    <PopoverHeader>
                      <PopoverTitle>{previewDoc.title}</PopoverTitle>
                      <PopoverDescription>{previewDoc.linkType}</PopoverDescription>
                    </PopoverHeader>
                    {previewDoc.preview ? (
                      <div className="mt-3 whitespace-pre-wrap text-xs text-muted-foreground">
                        {previewDoc.preview.split("\n").slice(0, 3).join("\n")}
                      </div>
                    ) : null}
                    <div className="mt-3">
                      <Link
                        href={buildDocsHref(previewDoc.pageId)}
                        className="text-xs font-medium text-primary hover:underline"
                        onClick={(event) => event.stopPropagation()}
                      >
                        View
                      </Link>
                    </div>
                  </PopoverContent>
                </Popover>
              ) : null}
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
