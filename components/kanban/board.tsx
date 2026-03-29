"use client";

import { useMemo, useState } from "react";
import { DragDropContext, type DropResult } from "@hello-pangea/dnd";
import { Column } from "./column";
import type { TaskWorkspaceDisplayOptions } from "@/lib/stores/task-workspace-store";
import type { Task, TaskPriority, TaskStatus } from "@/lib/stores/task-store";
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
  allTasks: Task[];
  selectedTaskId: string | null;
  selectedTaskIds?: string[];
  displayOptions: TaskWorkspaceDisplayOptions;
  linkedDocsByTask?: Record<string, LinkedDocItem[]>;
  onTaskClick: (task: Task) => void;
  onTaskStatusChange: (
    taskId: string,
    nextStatus: TaskStatus
  ) => Promise<void> | void;
  onToggleTaskSelection?: (taskId: string) => void;
  onQuickStatusChange?: (taskId: string, status: TaskStatus) => void;
  onQuickPriorityChange?: (taskId: string, priority: TaskPriority) => void;
}

export function Board({
  tasks,
  allTasks,
  selectedTaskId,
  selectedTaskIds = [],
  displayOptions,
  linkedDocsByTask = {},
  onTaskClick,
  onTaskStatusChange,
  onToggleTaskSelection,
  onQuickStatusChange,
  onQuickPriorityChange,
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

  const subtaskStatsMap = useMemo(() => {
    const map: Record<string, { total: number; done: number }> = {};
    for (const task of allTasks) {
      if (task.parentId) {
        if (!map[task.parentId]) {
          map[task.parentId] = { total: 0, done: 0 };
        }
        map[task.parentId].total++;
        if (task.status === "done") {
          map[task.parentId].done++;
        }
      }
    }
    return map;
  }, [allTasks]);

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
              selectedTaskIds={selectedTaskIds}
              displayOptions={displayOptions}
              linkedDocsByTask={linkedDocsByTask}
              subtaskStatsMap={subtaskStatsMap}
              onTaskClick={onTaskClick}
              onToggleTaskSelection={onToggleTaskSelection}
              onQuickStatusChange={onQuickStatusChange}
              onQuickPriorityChange={onQuickPriorityChange}
            />
          ))}
        </div>
      </DragDropContext>
    </div>
  );
}
