"use client";

import { useMemo, useState } from "react";
import { DragDropContext, type DropResult } from "@hello-pangea/dnd";
import { Column } from "./column";
import type { TaskWorkspaceDisplayOptions } from "@/lib/stores/task-workspace-store";
import type { Task, TaskStatus } from "@/lib/stores/task-store";
import type { LinkedDocItem } from "@/components/tasks/linked-docs-panel";

const columns: TaskStatus[] = [
  "inbox",
  "triaged",
  "assigned",
  "in_progress",
  "blocked",
  "in_review",
  "changes_requested",
  "done",
  "cancelled",
  "budget_exceeded",
];

interface BoardProps {
  tasks: Task[];
  selectedTaskId: string | null;
  displayOptions: TaskWorkspaceDisplayOptions;
  linkedDocsByTask?: Record<string, LinkedDocItem[]>;
  onTaskClick: (task: Task) => void;
  onTaskStatusChange: (
    taskId: string,
    nextStatus: TaskStatus
  ) => Promise<void> | void;
}

export function Board({
  tasks,
  selectedTaskId,
  displayOptions,
  linkedDocsByTask = {},
  onTaskClick,
  onTaskStatusChange,
}: BoardProps) {
  const [error, setError] = useState<string | null>(null);

  const grouped = useMemo(() => {
    const map: Record<TaskStatus, Task[]> = {
      inbox: [],
      triaged: [],
      assigned: [],
      in_progress: [],
      blocked: [],
      in_review: [],
      changes_requested: [],
      done: [],
      cancelled: [],
      budget_exceeded: [],
    };

    for (const task of tasks) {
      map[task.status]?.push(task);
    }

    return map;
  }, [tasks]);

  const onDragEnd = async (result: DropResult) => {
    if (!result.destination) return;

    const newStatus = result.destination.droppableId as TaskStatus;
    if (newStatus === result.source.droppableId) return;

    setError(null);
    try {
      await onTaskStatusChange(result.draggableId, newStatus);
    } catch (dragError) {
      setError(
        dragError instanceof Error
          ? dragError.message
          : "Failed to update task status."
      );
    }
  };

  return (
    <div className="flex flex-col gap-3">
      {error ? (
        <div className="rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {error}
        </div>
      ) : null}

      <DragDropContext onDragEnd={(result) => void onDragEnd(result)}>
        <div className="flex gap-4 overflow-x-auto pb-4">
          {columns.map((status) => (
            <Column
              key={status}
              status={status}
              tasks={grouped[status]}
              selectedTaskId={selectedTaskId}
              displayOptions={displayOptions}
              linkedDocsByTask={linkedDocsByTask}
              onTaskClick={onTaskClick}
            />
          ))}
        </div>
      </DragDropContext>
    </div>
  );
}
