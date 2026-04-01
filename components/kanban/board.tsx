"use client";

import { useEffect, useMemo, useState } from "react";
import { DragDropContext, type DropResult } from "@hello-pangea/dnd";
import { Button } from "@/components/ui/button";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import { DEFAULT_BOARD_COLUMN_ORDER } from "@/lib/stores/task-workspace-store";
import { Column } from "./column";
import type { TaskWorkspaceDisplayOptions } from "@/lib/stores/task-workspace-store";
import type { Task, TaskPriority, TaskStatus } from "@/lib/stores/task-store";
import type { LinkedDocItem } from "@/components/tasks/linked-docs-panel";

interface BoardProps {
  tasks: Task[];
  allTasks: Task[];
  selectedTaskId: string | null;
  selectedTaskIds?: string[];
  displayOptions: TaskWorkspaceDisplayOptions;
  linkedDocsByTask?: Record<string, LinkedDocItem[]>;
  searchQuery?: string;
  onTaskClick: (task: Task) => void;
  onTaskStatusChange: (
    taskId: string,
    nextStatus: TaskStatus
  ) => Promise<void> | void;
  onToggleTaskSelection?: (taskId: string) => void;
  onQuickStatusChange?: (taskId: string, status: TaskStatus) => void;
  onQuickPriorityChange?: (taskId: string, priority: TaskPriority) => void;
  onQuickCreateTask?: (status: TaskStatus, title: string) => Promise<void> | void;
  onUpdateBoardColumns?: (
    boardColumnOrder: TaskStatus[],
    hiddenBoardColumns: TaskStatus[],
  ) => void;
}

export function Board({
  tasks,
  allTasks,
  selectedTaskId,
  selectedTaskIds = [],
  displayOptions,
  linkedDocsByTask = {},
  searchQuery,
  onTaskClick,
  onTaskStatusChange,
  onToggleTaskSelection,
  onQuickStatusChange,
  onQuickPriorityChange,
  onQuickCreateTask,
  onUpdateBoardColumns,
}: BoardProps) {
  const [error, setError] = useState<string | null>(null);
  const [optimisticStatuses, setOptimisticStatuses] = useState<Record<string, TaskStatus>>({});
  const [pendingTaskIds, setPendingTaskIds] = useState<string[]>([]);
  const [columnOrder, setColumnOrder] = useState<TaskStatus[]>(
    displayOptions.boardColumnOrder ?? DEFAULT_BOARD_COLUMN_ORDER,
  );
  const [hiddenColumns, setHiddenColumns] = useState<TaskStatus[]>(
    displayOptions.hiddenBoardColumns ?? [],
  );

  useEffect(() => {
    setColumnOrder(displayOptions.boardColumnOrder ?? DEFAULT_BOARD_COLUMN_ORDER);
  }, [displayOptions.boardColumnOrder]);

  useEffect(() => {
    setHiddenColumns(displayOptions.hiddenBoardColumns ?? []);
  }, [displayOptions.hiddenBoardColumns]);

  useEffect(() => {
    const tasksById = new Map(tasks.map((task) => [task.id, task]));

    setPendingTaskIds((current) =>
      current.filter((taskId) => tasksById.has(taskId)),
    );
    setOptimisticStatuses((current) => {
      let changed = false;
      const next = { ...current };

      for (const [taskId, status] of Object.entries(current)) {
        const task = tasksById.get(taskId);
        if (!task || task.status === status) {
          delete next[taskId];
          changed = true;
        }
      }

      return changed ? next : current;
    });
  }, [tasks]);

  const visibleTasks = useMemo(
    () =>
      tasks.map((task) => {
        const optimisticStatus = optimisticStatuses[task.id];
        if (!optimisticStatus || optimisticStatus === task.status) {
          return task;
        }

        return {
          ...task,
          status: optimisticStatus,
        };
      }),
    [optimisticStatuses, tasks],
  );

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

    for (const task of visibleTasks) {
      map[task.status]?.push(task);
    }

    return map;
  }, [visibleTasks]);

  const visibleColumns = useMemo(
    () => columnOrder.filter((status) => !hiddenColumns.includes(status)),
    [columnOrder, hiddenColumns],
  );

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

    const taskId = result.draggableId;
    const newStatus = result.destination.droppableId as TaskStatus;
    if (newStatus === result.source.droppableId) return;
    if (pendingTaskIds.includes(taskId)) return;

    setError(null);
    setOptimisticStatuses((current) => ({
      ...current,
      [taskId]: newStatus,
    }));
    setPendingTaskIds((current) =>
      current.includes(taskId) ? current : [...current, taskId],
    );

    try {
      await onTaskStatusChange(taskId, newStatus);
    } catch (dragError) {
      setOptimisticStatuses((current) => {
        const next = { ...current };
        delete next[taskId];
        return next;
      });
      setError(
        dragError instanceof Error
          ? dragError.message
          : "Failed to update task status."
      );
    } finally {
      setPendingTaskIds((current) => current.filter((id) => id !== taskId));
    }
  };

  const applyColumnConfig = (
    nextColumnOrder: TaskStatus[],
    nextHiddenColumns: TaskStatus[],
  ) => {
    setColumnOrder(nextColumnOrder);
    setHiddenColumns(nextHiddenColumns);
    onUpdateBoardColumns?.(nextColumnOrder, nextHiddenColumns);
  };

  const toggleColumnVisibility = (status: TaskStatus) => {
    const nextHiddenColumns = hiddenColumns.includes(status)
      ? hiddenColumns.filter((value) => value !== status)
      : [...hiddenColumns, status];
    applyColumnConfig(columnOrder, nextHiddenColumns);
  };

  const moveColumn = (status: TaskStatus, direction: -1 | 1) => {
    const index = columnOrder.indexOf(status);
    const nextIndex = index + direction;

    if (index === -1 || nextIndex < 0 || nextIndex >= columnOrder.length) {
      return;
    }

    const nextColumnOrder = [...columnOrder];
    const [current] = nextColumnOrder.splice(index, 1);
    nextColumnOrder.splice(nextIndex, 0, current);
    applyColumnConfig(nextColumnOrder, hiddenColumns);
  };

  return (
    <div className="flex flex-col gap-3">
      <div className="flex justify-end">
        <Popover>
          <PopoverTrigger asChild>
            <Button type="button" size="sm" variant="outline">
              Configure Columns
            </Button>
          </PopoverTrigger>
          <PopoverContent align="end" className="w-72">
            <div className="space-y-2">
              {columnOrder.map((status, index) => {
                const hidden = hiddenColumns.includes(status);
                return (
                  <div
                    key={status}
                    className="flex items-center justify-between gap-2 rounded-md border px-2 py-2"
                  >
                    <span className="text-sm">{status}</span>
                    <div className="flex gap-1">
                      <Button
                        type="button"
                        size="sm"
                        variant="ghost"
                        disabled={index === 0}
                        onClick={() => moveColumn(status, -1)}
                      >
                        {`Move ${status} left`}
                      </Button>
                      <Button
                        type="button"
                        size="sm"
                        variant="ghost"
                        disabled={index === columnOrder.length - 1}
                        onClick={() => moveColumn(status, 1)}
                      >
                        {`Move ${status} right`}
                      </Button>
                      <Button
                        type="button"
                        size="sm"
                        variant="ghost"
                        onClick={() => toggleColumnVisibility(status)}
                      >
                        {hidden ? `Show ${status}` : `Hide ${status}`}
                      </Button>
                    </div>
                  </div>
                );
              })}
            </div>
          </PopoverContent>
        </Popover>
      </div>
      {error ? (
        <div className="rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {error}
        </div>
      ) : null}

      <DragDropContext onDragEnd={(result) => void onDragEnd(result)}>
        <div className="flex gap-4 overflow-x-auto pb-4">
          {visibleColumns.map((status) => (
            <Column
              key={status}
              status={status}
              tasks={grouped[status]}
              selectedTaskId={selectedTaskId}
              selectedTaskIds={selectedTaskIds}
              pendingTaskIds={pendingTaskIds}
              displayOptions={displayOptions}
              linkedDocsByTask={linkedDocsByTask}
              subtaskStatsMap={subtaskStatsMap}
              searchQuery={searchQuery}
              onTaskClick={onTaskClick}
              onToggleTaskSelection={onToggleTaskSelection}
              onQuickStatusChange={onQuickStatusChange}
              onQuickPriorityChange={onQuickPriorityChange}
              onQuickCreateTask={onQuickCreateTask}
            />
          ))}
        </div>
      </DragDropContext>
    </div>
  );
}
