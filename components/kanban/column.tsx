"use client";

import { Droppable } from "@hello-pangea/dnd";
import { cn } from "@/lib/utils";
import { ScrollArea } from "@/components/ui/scroll-area";
import { TaskCard } from "./task-card";
import type { TaskWorkspaceDisplayOptions } from "@/lib/stores/task-workspace-store";
import type { Task, TaskStatus } from "@/lib/stores/task-store";

const columnLabels: Record<TaskStatus, string> = {
  inbox: "Inbox",
  triaged: "Triaged",
  assigned: "Assigned",
  in_progress: "In Progress",
  blocked: "Blocked",
  in_review: "In Review",
  changes_requested: "Changes Requested",
  done: "Done",
  cancelled: "Cancelled",
  budget_exceeded: "Budget Exceeded",
};

interface ColumnProps {
  status: TaskStatus;
  tasks: Task[];
  selectedTaskId: string | null;
  displayOptions: TaskWorkspaceDisplayOptions;
  onTaskClick: (task: Task) => void;
}

export function Column({
  status,
  tasks,
  selectedTaskId,
  displayOptions,
  onTaskClick,
}: ColumnProps) {
  return (
    <div className="flex w-72 shrink-0 flex-col rounded-lg border bg-muted/50">
      <div className="flex items-center justify-between px-3 py-2">
        <h3 className="text-sm font-semibold">{columnLabels[status]}</h3>
        <span className="rounded-full bg-muted px-2 py-0.5 text-xs font-medium text-muted-foreground">
          {tasks.length}
        </span>
      </div>
      <Droppable droppableId={status}>
        {(provided, snapshot) => (
          <ScrollArea className="flex-1">
            <div
              ref={provided.innerRef}
              {...provided.droppableProps}
              className={cn(
                "flex min-h-[120px] flex-col gap-2 p-2",
                snapshot.isDraggingOver && "bg-accent/50"
              )}
            >
              {tasks.map((task, i) => (
                <TaskCard
                  key={task.id}
                  task={task}
                  index={i}
                  isSelected={task.id === selectedTaskId}
                  density={displayOptions.density}
                  showDescription={displayOptions.showDescriptions}
                  onClick={() => onTaskClick(task)}
                />
              ))}
              {provided.placeholder}
            </div>
          </ScrollArea>
        )}
      </Droppable>
    </div>
  );
}
