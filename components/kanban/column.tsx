"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { Droppable } from "@hello-pangea/dnd";
import { Plus } from "lucide-react";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { ScrollArea } from "@/components/ui/scroll-area";
import { TaskCard } from "./task-card";
import type { TaskWorkspaceDisplayOptions } from "@/lib/stores/task-workspace-store";
import type { Task, TaskPriority, TaskStatus } from "@/lib/stores/task-store";
import type { LinkedDocItem } from "@/components/tasks/linked-docs-panel";

interface ColumnProps {
  status: TaskStatus;
  tasks: Task[];
  selectedTaskId: string | null;
  selectedTaskIds?: string[];
  pendingTaskIds?: string[];
  displayOptions: TaskWorkspaceDisplayOptions;
  linkedDocsByTask: Record<string, LinkedDocItem[]>;
  subtaskStatsMap?: Record<string, { total: number; done: number }>;
  searchQuery?: string;
  onTaskClick: (task: Task) => void;
  onToggleTaskSelection?: (taskId: string) => void;
  onQuickStatusChange?: (taskId: string, status: TaskStatus) => void;
  onQuickPriorityChange?: (taskId: string, priority: TaskPriority) => void;
  onQuickCreateTask?: (status: TaskStatus, title: string) => Promise<void> | void;
}

export function Column({
  status,
  tasks,
  selectedTaskId,
  selectedTaskIds = [],
  pendingTaskIds = [],
  displayOptions,
  linkedDocsByTask,
  subtaskStatsMap = {},
  searchQuery,
  onTaskClick,
  onToggleTaskSelection,
  onQuickStatusChange,
  onQuickPriorityChange,
  onQuickCreateTask,
}: ColumnProps) {
  const t = useTranslations("tasks");
  const tc = useTranslations("common");
  const [quickCreateOpen, setQuickCreateOpen] = useState(false);
  const [quickCreateTitle, setQuickCreateTitle] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);

  const handleQuickCreate = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();

    const title = quickCreateTitle.trim();
    if (!title || !onQuickCreateTask) {
      return;
    }

    setSubmitting(true);
    setCreateError(null);

    try {
      await onQuickCreateTask(status, title);
      setQuickCreateTitle("");
      setQuickCreateOpen(false);
    } catch (error) {
      setCreateError(
        error instanceof Error ? error.message : t("board.createTaskError")
      );
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div
      className="flex w-72 shrink-0 flex-col rounded-lg border bg-muted/50"
      data-board-column={status}
    >
      <div className="flex items-center justify-between px-3 py-2">
        <h3 className="text-sm font-semibold">{t(`status.${status}`)}</h3>
        <div className="flex items-center gap-1">
          <span className="rounded-full bg-muted px-2 py-0.5 text-xs font-medium text-muted-foreground">
            {tasks.length}
          </span>
          {onQuickCreateTask ? (
            <Button
              type="button"
              size="icon"
              variant="ghost"
              className="size-7"
              aria-label={t("board.quickCreateInColumn", { column: t(`status.${status}`) })}
              onClick={() => {
                setCreateError(null);
                setQuickCreateOpen((open) => !open);
              }}
            >
              <Plus className="size-4" />
            </Button>
          ) : null}
        </div>
      </div>
      {quickCreateOpen ? (
        <form className="flex flex-col gap-2 px-3 pb-2" onSubmit={(event) => void handleQuickCreate(event)}>
          <Input
            aria-label={t("board.taskTitlePlaceholder")}
            className="h-8 bg-background text-xs"
            value={quickCreateTitle}
            onChange={(event) => setQuickCreateTitle(event.target.value)}
            placeholder={t("board.taskTitlePlaceholder")}
          />
          <div className="flex items-center justify-end gap-2">
            <Button
              type="button"
              size="sm"
              variant="ghost"
              className="h-7 px-2 text-xs"
              onClick={() => {
                setCreateError(null);
                setQuickCreateOpen(false);
                setQuickCreateTitle("");
              }}
            >
              {tc("action.cancel")}
            </Button>
            <Button
              type="submit"
              size="sm"
              className="h-7 px-2 text-xs"
              disabled={!quickCreateTitle.trim() || submitting}
            >
              {submitting ? t("board.addingTask") : t("board.addTask")}
            </Button>
          </div>
          {createError ? (
            <p className="text-xs text-destructive">{createError}</p>
          ) : null}
        </form>
      ) : null}
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
                  isMultiSelected={selectedTaskIds.includes(task.id)}
                  isPending={pendingTaskIds.includes(task.id)}
                  density={displayOptions.density}
                  showDescription={displayOptions.showDescriptions}
                  linkedDocs={linkedDocsByTask[task.id] ?? []}
                  subtaskStats={subtaskStatsMap[task.id]}
                  searchQuery={searchQuery}
                  onClick={() => onTaskClick(task)}
                  onToggleSelect={onToggleTaskSelection}
                  onQuickStatusChange={onQuickStatusChange}
                  onQuickPriorityChange={onQuickPriorityChange}
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
