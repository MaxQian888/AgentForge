"use client";

import { useState } from "react";
import { Draggable } from "@hello-pangea/dnd";
import Link from "next/link";
import { MoreHorizontal } from "lucide-react";
import { cn } from "@/lib/utils";
import { Badge } from "@/components/ui/badge";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  Popover,
  PopoverContent,
  PopoverDescription,
  PopoverHeader,
  PopoverTitle,
  PopoverTrigger,
} from "@/components/ui/popover";
import type { Task, TaskPriority, TaskStatus } from "@/lib/stores/task-store";
import type { LinkedDocItem } from "@/components/tasks/linked-docs-panel";
import { buildDocsHref } from "@/lib/route-hrefs";
import { HighlightedText } from "@/components/shared/highlighted-text";

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

function focusSiblingBoardCard(
  current: HTMLElement,
  key: "ArrowUp" | "ArrowDown" | "ArrowLeft" | "ArrowRight"
): boolean {
  const currentColumn = current.closest<HTMLElement>("[data-board-column]");
  if (!currentColumn) {
    return false;
  }

  const cardsInColumn = Array.from(
    currentColumn.querySelectorAll<HTMLElement>('[data-board-task-card="true"]')
  );
  const currentIndex = cardsInColumn.indexOf(current);
  if (currentIndex === -1) {
    return false;
  }

  if (key === "ArrowUp" || key === "ArrowDown") {
    const nextIndex = key === "ArrowUp" ? currentIndex - 1 : currentIndex + 1;
    const nextCard =
      key === "ArrowUp" ? cardsInColumn[currentIndex - 1] : cardsInColumn[currentIndex + 1];

    if (!nextCard) {
      return false;
    }

    nextCard.dataset.boardNavIndex = String(nextIndex);
    nextCard.focus();
    return true;
  }

  const boardColumns = Array.from(
    document.querySelectorAll<HTMLElement>("[data-board-column]")
  );
  const currentColumnIndex = boardColumns.indexOf(currentColumn);
  if (currentColumnIndex === -1) {
    return false;
  }

  const adjacentColumn =
    key === "ArrowLeft"
      ? boardColumns[currentColumnIndex - 1]
      : boardColumns[currentColumnIndex + 1];
  if (!adjacentColumn) {
    return false;
  }

  const adjacentCards = Array.from(
    adjacentColumn.querySelectorAll<HTMLElement>('[data-board-task-card="true"]')
  );
  if (adjacentCards.length === 0) {
    return false;
  }

  const requestedRow = Number(current.dataset.boardNavIndex ?? currentIndex);
  const targetIndex = Math.min(requestedRow, adjacentCards.length - 1);
  const targetCard = adjacentCards[targetIndex];
  if (!targetCard) {
    return false;
  }

  targetCard.dataset.boardNavIndex = String(requestedRow);
  targetCard.focus();
  return true;
}

function formatDueDate(value: string | null): string | null {
  if (!value) {
    return null;
  }

  return new Date(value).toISOString().slice(0, 10);
}

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
  isMultiSelected?: boolean;
  isPending?: boolean;
  density: "comfortable" | "compact";
  showDescription: boolean;
  linkedDocs?: LinkedDocItem[];
  subtaskStats?: { total: number; done: number };
  searchQuery?: string;
  onClick: () => void;
  onToggleSelect?: (taskId: string) => void;
  onQuickStatusChange?: (taskId: string, status: TaskStatus) => void;
  onQuickPriorityChange?: (taskId: string, priority: TaskPriority) => void;
}

export function TaskCard({
  task,
  index,
  isSelected,
  isMultiSelected = false,
  isPending = false,
  density,
  showDescription,
  linkedDocs = [],
  subtaskStats,
  searchQuery,
  onClick,
  onToggleSelect,
  onQuickStatusChange,
  onQuickPriorityChange,
}: TaskCardProps) {
  const previewDoc = linkedDocs[0];
  const [docPreviewOpen, setDocPreviewOpen] = useState(false);
  const dueDate = formatDueDate(task.plannedEndAt);

  return (
    <Draggable draggableId={task.id} index={index} isDragDisabled={isPending}>
      {(provided, snapshot) => (
        <div
          ref={provided.innerRef}
          {...provided.draggableProps}
          {...provided.dragHandleProps}
          onClick={(event) => {
            if ((event.metaKey || event.ctrlKey) && onToggleSelect) {
              event.preventDefault();
              event.stopPropagation();
              onToggleSelect(task.id);
              return;
            }
            onClick();
          }}
          data-task-id={task.id}
          data-board-task-card="true"
          data-selected={isSelected ? "true" : "false"}
          aria-busy={isPending}
          role="button"
          tabIndex={0}
          className={cn(
            "cursor-pointer rounded-md border bg-card shadow-sm transition-shadow hover:shadow-md",
            density === "compact" ? "p-2.5" : "p-3",
            isSelected && "ring-2 ring-primary/25 border-primary/40",
            isMultiSelected && "ring-2 ring-blue-500/30 border-blue-500/40 bg-blue-500/5",
            isPending && "cursor-wait opacity-80",
            snapshot.isDragging && "shadow-lg ring-2 ring-primary/20",
            "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/35 focus-visible:border-primary/50"
          )}
          onKeyDown={(event) => {
            if (event.key === "Enter" || event.key === " ") {
              event.preventDefault();
              onClick();
              return;
            }

            if (
              event.key === "ArrowUp" ||
              event.key === "ArrowDown" ||
              event.key === "ArrowLeft" ||
              event.key === "ArrowRight"
            ) {
              const moved = focusSiblingBoardCard(
                event.currentTarget,
                event.key
              );

              if (moved) {
                event.preventDefault();
              }
            }
          }}
        >
          <div className="flex items-start gap-2">
            {onToggleSelect ? (
              <input
                type="checkbox"
                checked={isMultiSelected}
                className="mt-0.5 size-4 shrink-0 rounded border-border"
                onClick={(e) => e.stopPropagation()}
                onChange={() => onToggleSelect(task.id)}
              />
            ) : null}
            <p className="mb-2 text-sm font-medium leading-snug flex-1">
              <HighlightedText text={task.title} query={searchQuery} />
            </p>
            {(onQuickStatusChange || onQuickPriorityChange) ? (
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <button
                    type="button"
                    className="shrink-0 rounded p-0.5 opacity-0 transition-opacity hover:bg-accent/60 group-hover:opacity-100 [div:hover>&]:opacity-100"
                    onClick={(e) => e.stopPropagation()}
                  >
                    <MoreHorizontal className="size-4 text-muted-foreground" />
                  </button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end" onClick={(e) => e.stopPropagation()}>
                  {onQuickPriorityChange ? (
                    <>
                      <DropdownMenuLabel className="text-xs">Priority</DropdownMenuLabel>
                      {(["urgent", "high", "medium", "low"] as TaskPriority[]).map((p) => (
                        <DropdownMenuItem
                          key={p}
                          disabled={p === task.priority}
                          onClick={() => onQuickPriorityChange(task.id, p)}
                        >
                          <span className={cn("mr-2 inline-block size-2 rounded-full", priorityColors[p].split(" ")[0])} />
                          {p}
                        </DropdownMenuItem>
                      ))}
                      <DropdownMenuSeparator />
                    </>
                  ) : null}
                  {onQuickStatusChange ? (
                    <>
                      <DropdownMenuLabel className="text-xs">Status</DropdownMenuLabel>
                      {(["inbox", "triaged", "assigned", "in_progress", "in_review", "done", "cancelled"] as TaskStatus[]).map((s) => (
                        <DropdownMenuItem
                          key={s}
                          disabled={s === task.status}
                          onClick={() => onQuickStatusChange(task.id, s)}
                        >
                          {s.replace(/_/g, " ")}
                        </DropdownMenuItem>
                      ))}
                    </>
                  ) : null}
                </DropdownMenuContent>
              </DropdownMenu>
            ) : null}
          </div>
          {showDescription && task.description ? (
            <p className="mb-2 text-xs text-muted-foreground">
              <HighlightedText text={task.description} query={searchQuery} />
            </p>
          ) : null}
          {task.labels && task.labels.length > 0 && (
            <div className="mb-2 flex flex-wrap gap-1">
              {task.labels.slice(0, 3).map((label) => (
                <Badge
                  key={label}
                  variant="secondary"
                  className="text-[10px] px-1.5 py-0"
                >
                  {label}
                </Badge>
              ))}
              {task.labels.length > 3 && (
                <span className="text-[10px] text-muted-foreground">
                  +{task.labels.length - 3}
                </span>
              )}
            </div>
          )}
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
          {dueDate ? (
            <div className="mb-2">
              <Badge variant="outline" className="text-[10px] text-muted-foreground">
                {`Due ${dueDate}`}
              </Badge>
            </div>
          ) : null}
          <div className="flex items-center justify-between">
            <Badge
              variant="secondary"
              className={cn("text-xs", priorityColors[task.priority])}
            >
              {task.priority}
            </Badge>
            <div className="flex items-center gap-2">
              {isPending ? (
                <span className="text-[11px] font-medium text-muted-foreground">
                  Saving...
                </span>
              ) : null}
              {previewDoc ? (
                <Popover open={docPreviewOpen} onOpenChange={setDocPreviewOpen}>
                  <PopoverTrigger asChild>
                    <button
                      type="button"
                      aria-label={`Show linked docs for ${task.title}`}
                      className="rounded-full border border-border/60 px-2 py-0.5 text-[11px] text-muted-foreground hover:bg-accent/40"
                      onClick={(event) => event.stopPropagation()}
                      onMouseEnter={() => setDocPreviewOpen(true)}
                      onMouseLeave={() => setDocPreviewOpen(false)}
                      onFocus={() => setDocPreviewOpen(true)}
                      onBlur={() => setDocPreviewOpen(false)}
                    >
                      Docs {linkedDocs.length}
                    </button>
                  </PopoverTrigger>
                  <PopoverContent
                    onClick={(event) => event.stopPropagation()}
                    onMouseDown={(event) => event.stopPropagation()}
                    onMouseEnter={() => setDocPreviewOpen(true)}
                    onMouseLeave={() => setDocPreviewOpen(false)}
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
              {subtaskStats && subtaskStats.total > 0 ? (
                <span className="text-[11px] text-muted-foreground">
                  {subtaskStats.done}/{subtaskStats.total}
                </span>
              ) : null}
              {task.budgetUsd > 0 ? (
                <span
                  className={cn(
                    "text-xs",
                    task.spentUsd / task.budgetUsd >= 1
                      ? "text-red-600 dark:text-red-400"
                      : task.spentUsd / task.budgetUsd >= 0.8
                        ? "text-amber-600 dark:text-amber-400"
                        : "text-muted-foreground"
                  )}
                >
                  ${task.spentUsd.toFixed(0)} / ${task.budgetUsd.toFixed(0)}
                </span>
              ) : task.cost != null ? (
                <span className="text-xs text-muted-foreground">
                  ${task.cost.toFixed(2)}
                </span>
              ) : null}
              {task.assigneeName ? (
                <Avatar className="size-5">
                  <AvatarFallback className="text-[10px]">
                    {task.assigneeName[0]?.toUpperCase()}
                  </AvatarFallback>
                </Avatar>
              ) : (
                <Badge variant="outline" className="text-[10px] text-muted-foreground">
                  Unassigned
                </Badge>
              )}
            </div>
          </div>
        </div>
      )}
    </Draggable>
  );
}
