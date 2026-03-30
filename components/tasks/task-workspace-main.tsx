"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useTranslations } from "next-intl";
import {
  DragDropContext,
  Draggable,
  Droppable,
  type DropResult,
} from "@hello-pangea/dnd";
import { Board } from "@/components/kanban/board";
import { BulkActionToolbar } from "@/components/tasks/bulk-action-toolbar";
import { TaskDependencyGraph } from "@/components/tasks/task-dependency-graph";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  filterTasksForWorkspace,
  getRescheduledPlanningWindow,
  type TaskViewMode,
} from "@/lib/tasks/task-workspace";
import type { Sprint, SprintMetrics } from "@/lib/stores/sprint-store";
import { BurndownChart } from "@/components/sprint/burndown-chart";
import { getTaskDependencyState } from "@/lib/tasks/task-dependencies";
import { useTaskWorkspaceStore } from "@/lib/stores/task-workspace-store";
import { cn } from "@/lib/utils";
import { useCustomFieldStore } from "@/lib/stores/custom-field-store";
import { useDocsStore, flattenDocsTree } from "@/lib/stores/docs-store";
import { useEntityLinkStore } from "@/lib/stores/entity-link-store";
import { FieldValueCell } from "@/components/fields/field-value-cell";
import { Skeleton } from "@/components/ui/skeleton";
import { RoadmapView } from "@/components/milestones/roadmap-view";
import type { Task, TaskPriority, TaskStatus } from "@/lib/stores/task-store";
import type { LinkedDocItem } from "./linked-docs-panel";
import Link from "next/link";
import { buildDocsHref } from "@/lib/route-hrefs";

export interface BulkActionFailure {
  taskId: string;
  message: string;
}

export interface BulkActionResult {
  failed?: BulkActionFailure[];
}

interface ProjectTaskWorkspaceProps {
  projectId: string;
  tasks: Task[];
  sprints: Sprint[];
  sprintMetrics: SprintMetrics | null;
  sprintMetricsLoading: boolean;
  loading: boolean;
  error: string | null;
  realtimeConnected: boolean;
  members?: import("@/lib/dashboard/summary").TeamMember[];
  onRetry: () => void;
  onTaskOpen: (taskId: string) => void;
  onTaskStatusChange: (
    taskId: string,
    nextStatus: TaskStatus
  ) => Promise<void> | void;
  onTaskScheduleChange: (
    taskId: string,
    changes: { plannedStartAt: string; plannedEndAt: string }
  ) => Promise<void> | void;
  onTaskSave?: (taskId: string, data: Partial<Task>) => Promise<void> | void;
  onCreateTask?: () => void;
  onSprintFilterChange?: (sprintId: string | "all") => void;
  onBulkStatusChange?: (ids: string[], status: TaskStatus) => Promise<BulkActionResult | void> | BulkActionResult | void;
  onBulkAssign?: (ids: string[], assigneeId: string, assigneeType: "human" | "agent") => Promise<BulkActionResult | void> | BulkActionResult | void;
  onBulkDelete?: (ids: string[]) => Promise<BulkActionResult | void> | BulkActionResult | void;
}

function formatDateKey(value: string | Date): string {
  const date = typeof value === "string" ? new Date(value) : value;
  return date.toISOString().slice(0, 10);
}

function startOfMonth(date: Date): Date {
  return new Date(Date.UTC(date.getUTCFullYear(), date.getUTCMonth(), 1));
}

function endOfMonth(date: Date): Date {
  return new Date(Date.UTC(date.getUTCFullYear(), date.getUTCMonth() + 1, 0));
}

function addDays(date: Date, days: number): Date {
  const next = new Date(date);
  next.setUTCDate(next.getUTCDate() + days);
  return next;
}

function startOfWeek(date: Date): Date {
  const normalized = new Date(
    Date.UTC(date.getUTCFullYear(), date.getUTCMonth(), date.getUTCDate())
  );
  const day = normalized.getUTCDay();
  const delta = day === 0 ? -6 : 1 - day;
  return addDays(normalized, delta);
}

function addWeeks(date: Date, weeks: number): Date {
  return addDays(date, weeks * 7);
}

function addMonths(date: Date, months: number): Date {
  return new Date(Date.UTC(date.getUTCFullYear(), date.getUTCMonth() + months, 1));
}

function differenceInDays(end: Date, start: Date): number {
  const startDate = Date.UTC(
    start.getUTCFullYear(),
    start.getUTCMonth(),
    start.getUTCDate()
  );
  const endDate = Date.UTC(
    end.getUTCFullYear(),
    end.getUTCMonth(),
    end.getUTCDate()
  );

  return Math.round((endDate - startDate) / (24 * 60 * 60 * 1000));
}

function differenceInWeeks(end: Date, start: Date): number {
  return Math.round(
    differenceInDays(startOfWeek(end), startOfWeek(start)) / 7
  );
}

function differenceInMonths(end: Date, start: Date): number {
  return (
    (end.getUTCFullYear() - start.getUTCFullYear()) * 12 +
    (end.getUTCMonth() - start.getUTCMonth())
  );
}

function formatPlanningState(task: Task, unscheduledLabel: string): string {
  if (task.plannedStartAt && task.plannedEndAt) {
    return `${formatDateKey(task.plannedStartAt)} → ${formatDateKey(task.plannedEndAt)}`;
  }
  return unscheduledLabel;
}

function formatProgressHealthKey(task: Task): string | null {
  switch (task.progress?.healthStatus) {
    case "stalled":
      return "health.stalled";
    case "warning":
      return "health.atRisk";
    default:
      return null;
  }
}

function formatProgressReasonKey(task: Task): string | null {
  switch (task.progress?.riskReason) {
    case "no_recent_update":
      return "risk.noRecentUpdate";
    case "no_assignee":
      return "risk.noAssignee";
    case "awaiting_review":
      return "risk.awaitingReview";
    default:
      return null;
  }
}

function getProgressBadgeClass(task: Task): string {
  switch (task.progress?.healthStatus) {
    case "stalled":
      return "bg-red-500/15 text-red-700 dark:text-red-300";
    case "warning":
      return "bg-amber-500/15 text-amber-700 dark:text-amber-300";
    default:
      return "bg-emerald-500/15 text-emerald-700 dark:text-emerald-300";
  }
}

function formatSprintStatusKey(status: Sprint["status"]): string {
  switch (status) {
    case "active":
      return "sprint.statusActive";
    case "planning":
      return "sprint.statusPlanning";
    case "closed":
      return "sprint.statusClosed";
    default:
      return status;
  }
}

function formatSprintRange(sprint: Sprint): string {
  return `${formatDateKey(sprint.startDate)} -> ${formatDateKey(sprint.endDate)}`;
}

function SprintOverview({
  sprints,
  sprintMetrics,
  sprintMetricsLoading,
}: {
  sprints: Sprint[];
  sprintMetrics: SprintMetrics | null;
  sprintMetricsLoading: boolean;
}) {
  const t = useTranslations("tasks");

  if (sprints.length === 0) {
    return null;
  }

  const activeSprint =
    sprintMetrics?.sprint ?? sprints.find((sprint) => sprint.status === "active") ?? sprints[0];

  return (
    <div className="grid gap-3 xl:grid-cols-[minmax(0,1fr)_320px]">
      <Card className="gap-3">
        <CardHeader className="px-4">
          <CardTitle className="text-base">{t("sprint.overview")}</CardTitle>
          <CardDescription>
            {t("sprint.overviewDescription")}
          </CardDescription>
        </CardHeader>
        <CardContent className="grid gap-3 px-4">
          <div className="flex flex-wrap items-center gap-2">
            <span className="text-sm font-medium">{activeSprint.name}</span>
            <Badge variant="secondary">{t(formatSprintStatusKey(activeSprint.status))}</Badge>
            <Badge variant="outline">{formatSprintRange(activeSprint)}</Badge>
          </div>
          {sprintMetricsLoading ? (
            <div className="text-sm text-muted-foreground">{t("sprint.loadingMetrics")}</div>
          ) : sprintMetrics ? (
            <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
              <div className="rounded-md border border-border/60 bg-muted/20 px-3 py-2">
                <div className="text-xs text-muted-foreground">{t("sprint.completion")}</div>
                <div className="text-lg font-semibold">
                  {sprintMetrics.completionRate.toFixed(2)}%
                </div>
              </div>
              <div className="rounded-md border border-border/60 bg-muted/20 px-3 py-2">
                <div className="text-xs text-muted-foreground">{t("sprint.remaining")}</div>
                <div className="text-lg font-semibold">{sprintMetrics.remainingTasks}</div>
              </div>
              <div className="rounded-md border border-border/60 bg-muted/20 px-3 py-2">
                <div className="text-xs text-muted-foreground">{t("sprint.velocity")}</div>
                <div className="text-lg font-semibold">
                  {sprintMetrics.velocityPerWeek.toFixed(2)}/wk
                </div>
              </div>
              <div className="rounded-md border border-border/60 bg-muted/20 px-3 py-2">
                <div className="text-xs text-muted-foreground">{t("sprint.taskSpend")}</div>
                <div className="text-lg font-semibold">
                  ${sprintMetrics.taskSpentUsd.toFixed(2)}
                </div>
              </div>
            </div>
          ) : (
            <div className="text-sm text-muted-foreground">
              {t("sprint.selectSprint")}
            </div>
          )}
        </CardContent>
      </Card>

      <Card className="gap-3">
        <CardHeader className="px-4">
          <CardTitle className="text-base">{t("sprint.burndown")}</CardTitle>
          <CardDescription>
            {t("sprint.velocity")} {sprintMetrics ? `${sprintMetrics.velocityPerWeek.toFixed(2)}/wk` : "--"}
          </CardDescription>
        </CardHeader>
        <CardContent className="px-4">
          <BurndownChart
            burndown={sprintMetrics?.burndown ?? []}
            plannedTasks={sprintMetrics?.plannedTasks ?? 0}
          />
        </CardContent>
      </Card>
    </div>
  );
}

function EmptyState({
  title,
  description,
  actionLabel,
  onAction,
}: {
  title: string;
  description: string;
  actionLabel?: string;
  onAction?: () => void;
}) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>{title}</CardTitle>
        <CardDescription>{description}</CardDescription>
      </CardHeader>
      {actionLabel && onAction ? (
        <CardContent>
          <Button type="button" onClick={onAction}>
            {actionLabel}
          </Button>
        </CardContent>
      ) : null}
    </Card>
  );
}

export function TaskListView({
  projectId,
  tasks,
  allTasks,
  selectedTaskId,
  filters,
  density,
  showDescriptions,
  showLinkedDocs,
  customFields,
  valuesByTask,
  linkedDocsByTask,
  onTaskOpen,
  onTaskStatusChange,
  onTaskSave,
  onSetCustomFieldFilter,
}: {
  projectId: string;
  tasks: Task[];
  allTasks: Task[];
  selectedTaskId: string | null;
  filters: ReturnType<typeof useTaskWorkspaceStore.getState>["filters"];
  density: "comfortable" | "compact";
  showDescriptions: boolean;
  showLinkedDocs: boolean;
  customFields: ReturnType<typeof useCustomFieldStore.getState>["definitionsByProject"][string];
  valuesByTask: Record<string, ReturnType<typeof useCustomFieldStore.getState>["valuesByTask"][string]>;
  linkedDocsByTask: Record<string, LinkedDocItem[]>;
  onTaskOpen: (taskId: string) => void;
  onTaskStatusChange: (taskId: string, nextStatus: TaskStatus) => Promise<void> | void;
  onTaskSave?: (taskId: string, data: Partial<Task>) => Promise<void> | void;
  onSetCustomFieldFilter: (fieldId: string, value: string | "all") => void;
}) {
  const t = useTranslations("tasks");
  const [collapsedParents, setCollapsedParents] = useState<Set<string>>(new Set());
  const [sortState, setSortState] = useState<{
    key: "task" | "status" | "priority" | "assignee" | "planning";
    direction: "asc" | "desc";
  } | null>(null);
  const [statusOverrides, setStatusOverrides] = useState<Record<string, TaskStatus>>({});
  const [priorityOverrides, setPriorityOverrides] = useState<Record<string, TaskPriority>>({});
  const [rowErrors, setRowErrors] = useState<Record<string, string>>({});
  const [visibleCustomFieldIds, setVisibleCustomFieldIds] = useState<string[]>([]);
  const [activeCustomFilterFieldId, setActiveCustomFilterFieldId] = useState<string>("all");
  const [customSortFieldId, setCustomSortFieldId] = useState<string>("all");
  const [groupByFieldId, setGroupByFieldId] = useState<string>("all");
  const selectedTaskIds = useTaskWorkspaceStore((state) => state.selectedTaskIds);
  const toggleTaskSelection = useTaskWorkspaceStore((state) => state.toggleTaskSelection);
  const inlineStatusOptions: TaskStatus[] = [
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
  const inlinePriorityOptions: TaskPriority[] = ["urgent", "high", "medium", "low"];

  useEffect(() => {
    const tasksById = new Map(tasks.map((task) => [task.id, task]));

    setStatusOverrides((current) => {
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

    setPriorityOverrides((current) => {
      let changed = false;
      const next = { ...current };
      for (const [taskId, priority] of Object.entries(current)) {
        const task = tasksById.get(taskId);
        if (!task || task.priority === priority) {
          delete next[taskId];
          changed = true;
        }
      }
      return changed ? next : current;
    });
  }, [tasks]);

  useEffect(() => {
    setVisibleCustomFieldIds((current) => {
      if (current.length === 0) {
        return customFields.map((field) => field.id);
      }

      const currentSet = new Set(current);
      const next = customFields
        .filter((field) => currentSet.has(field.id) || !current.includes(field.id))
        .map((field) => field.id);

      return next;
    });
  }, [customFields]);

  const listTasks = useMemo(
    () =>
      tasks.map((task) => ({
        ...task,
        status: statusOverrides[task.id] ?? task.status,
        priority: priorityOverrides[task.id] ?? task.priority,
      })),
    [priorityOverrides, statusOverrides, tasks],
  );
  const visibleCustomFields = useMemo(
    () => customFields.filter((field) => visibleCustomFieldIds.includes(field.id)),
    [customFields, visibleCustomFieldIds],
  );
  const customFieldValueForTask = useCallback(
    (taskId: string, fieldId: string): string => {
      const value = valuesByTask[taskId]?.find((item) => item.fieldDefId === fieldId)?.value;
      if (value == null || value === "") {
        return "Unset";
      }
      return String(value);
    },
    [valuesByTask],
  );
  const customFieldOptions = useMemo(() => {
    const map: Record<string, string[]> = {};
    for (const field of customFields) {
      const values = Array.from(
        new Set(tasks.map((task) => customFieldValueForTask(task.id, field.id)))
      ).sort((left, right) => left.localeCompare(right, undefined, { sensitivity: "base" }));
      map[field.id] = values;
    }
    return map;
  }, [customFieldValueForTask, customFields, tasks]);

  useEffect(() => {
    const entries = Object.entries(filters.customFieldFilters ?? {});
    if (entries.length === 0) {
      setActiveCustomFilterFieldId("all");
      return;
    }
    setActiveCustomFilterFieldId(entries[0][0]);
  }, [filters.customFieldFilters]);

  // Build hierarchical task list: parents first, children indented underneath
  const hierarchicalTasks = useMemo(() => {
    const parentTasks = listTasks.filter((t) => !t.parentId);
    const childrenByParent = new Map<string, Task[]>();
    for (const task of listTasks) {
      if (task.parentId) {
        const existing = childrenByParent.get(task.parentId) ?? [];
        existing.push(task);
        childrenByParent.set(task.parentId, existing);
      }
    }
    const result: Array<{ task: Task; depth: number; hasChildren: boolean }> = [];
    for (const parent of parentTasks) {
      const children = childrenByParent.get(parent.id) ?? [];
      result.push({ task: parent, depth: 0, hasChildren: children.length > 0 });
      if (!collapsedParents.has(parent.id)) {
        for (const child of children) {
          result.push({ task: child, depth: 1, hasChildren: false });
        }
      }
    }
    // Also include orphan children (whose parent is not in filtered list)
    const includedIds = new Set(result.map((r) => r.task.id));
    for (const task of listTasks) {
      if (!includedIds.has(task.id)) {
        result.push({ task, depth: 0, hasChildren: false });
      }
    }
    return result;
  }, [listTasks, collapsedParents]);

  const sortedTasks = useMemo(() => {
    const items = [...hierarchicalTasks];
    if (customSortFieldId !== "all") {
      return items.sort((left, right) =>
        customFieldValueForTask(left.task.id, customSortFieldId).localeCompare(
          customFieldValueForTask(right.task.id, customSortFieldId),
          undefined,
          { sensitivity: "base", numeric: true }
        )
      );
    }

    if (!sortState) {
      return items;
    }

    const direction = sortState.direction === "asc" ? 1 : -1;
    const compareValues = (left: string, right: string) =>
      left.localeCompare(right, undefined, { sensitivity: "base", numeric: true });

    return items.sort((left, right) => {
      switch (sortState.key) {
        case "status":
          return compareValues(left.task.status, right.task.status) * direction;
        case "priority":
          return compareValues(left.task.priority, right.task.priority) * direction;
        case "assignee":
          return compareValues(
            left.task.assigneeName ?? t("list.unassigned"),
            right.task.assigneeName ?? t("list.unassigned"),
          ) * direction;
        case "planning":
          return compareValues(
            formatPlanningState(left.task, t("planning.unscheduled")),
            formatPlanningState(right.task, t("planning.unscheduled")),
          ) * direction;
        case "task":
        default:
          return compareValues(left.task.title, right.task.title) * direction;
      }
    });
  }, [customFieldValueForTask, customSortFieldId, hierarchicalTasks, sortState, t]);

  const groupedRows = useMemo(() => {
    if (groupByFieldId === "all") {
      return [{ key: "", label: "", rows: sortedTasks }];
    }

    const groups = new Map<string, typeof sortedTasks>();
    for (const row of sortedTasks) {
      const groupKey = customFieldValueForTask(row.task.id, groupByFieldId);
      const existing = groups.get(groupKey) ?? [];
      existing.push(row);
      groups.set(groupKey, existing);
    }

    return Array.from(groups.entries())
      .sort(([left], [right]) =>
        left.localeCompare(right, undefined, { sensitivity: "base", numeric: true })
      )
      .map(([key, rows]) => ({
        key,
        label: key,
        rows,
      }));
  }, [customFieldValueForTask, groupByFieldId, sortedTasks]);

  const toggleSort = (key: NonNullable<typeof sortState>["key"]) => {
    setSortState((current) => {
      if (current?.key === key) {
        return {
          key,
          direction: current.direction === "asc" ? "desc" : "asc",
        };
      }
      return { key, direction: "asc" };
    });
  };

  const sortIndicator = (key: NonNullable<typeof sortState>["key"]) => {
    if (sortState?.key !== key) {
      return null;
    }
    return sortState.direction === "asc" ? "↑" : "↓";
  };

  const toggleCollapse = (parentId: string) => {
    setCollapsedParents((prev) => {
      const next = new Set(prev);
      if (next.has(parentId)) {
        next.delete(parentId);
      } else {
        next.add(parentId);
      }
      return next;
    });
  };

  const handleInlineStatusChange = async (task: Task, nextStatus: TaskStatus) => {
    if (task.status === nextStatus) {
      return;
    }

    setRowErrors((current) => {
      const next = { ...current };
      delete next[task.id];
      return next;
    });
    setStatusOverrides((current) => ({
      ...current,
      [task.id]: nextStatus,
    }));

    try {
      await onTaskStatusChange(task.id, nextStatus);
    } catch (error) {
      setStatusOverrides((current) => {
        const next = { ...current };
        delete next[task.id];
        return next;
      });
      setRowErrors((current) => ({
        ...current,
        [task.id]:
          error instanceof Error ? error.message : "Failed to update task status.",
      }));
    }
  };

  const handleInlinePriorityChange = async (task: Task, nextPriority: TaskPriority) => {
    if (!onTaskSave || task.priority === nextPriority) {
      return;
    }

    setRowErrors((current) => {
      const next = { ...current };
      delete next[task.id];
      return next;
    });
    setPriorityOverrides((current) => ({
      ...current,
      [task.id]: nextPriority,
    }));

    try {
      await onTaskSave(task.id, { priority: nextPriority });
    } catch (error) {
      setPriorityOverrides((current) => {
        const next = { ...current };
        delete next[task.id];
        return next;
      });
      setRowErrors((current) => ({
        ...current,
        [task.id]:
          error instanceof Error ? error.message : "Failed to update task priority.",
      }));
    }
  };

  const toggleCustomFieldVisibility = (fieldId: string) => {
    setVisibleCustomFieldIds((current) =>
      current.includes(fieldId)
        ? current.filter((id) => id !== fieldId)
        : [...current, fieldId]
    );
  };

  const activeCustomFilterValue =
    activeCustomFilterFieldId !== "all"
      ? filters.customFieldFilters?.[activeCustomFilterFieldId] ?? "all"
      : "all";

  return (
    <div className="flex flex-col gap-3">
      {customFields.length > 0 ? (
        <div className="flex flex-wrap items-center gap-2">
          <span className="text-xs font-medium text-muted-foreground">{t("list.columns")}</span>
          {customFields.map((field) => {
            const isVisible = visibleCustomFieldIds.includes(field.id);
            return (
              <Button
                key={field.id}
                type="button"
                size="sm"
                variant={isVisible ? "secondary" : "outline"}
                aria-label={`Toggle custom column ${field.name}`}
                onClick={() => toggleCustomFieldVisibility(field.id)}
              >
                {field.name}
              </Button>
            );
          })}
        </div>
      ) : null}
      {customFields.length > 0 ? (
        <div className="grid gap-3 md:grid-cols-3">
          <label className="flex flex-col gap-1 text-xs font-medium">
            {t("list.customFieldFilterField")}
            <select
              aria-label={t("list.customFieldFilterField")}
              className="h-8 rounded-md border bg-background px-2 text-xs"
              value={activeCustomFilterFieldId}
              onChange={(event) => {
                const nextFieldId = event.target.value;
                if (activeCustomFilterFieldId !== "all") {
                  onSetCustomFieldFilter(activeCustomFilterFieldId, "all");
                }
                setActiveCustomFilterFieldId(nextFieldId);
              }}
            >
              <option value="all">{t("list.customFieldNone")}</option>
              {customFields.map((field) => (
                <option key={field.id} value={field.id}>
                  {field.name}
                </option>
              ))}
            </select>
          </label>
          <label className="flex flex-col gap-1 text-xs font-medium">
            {t("list.customFieldFilterValue")}
            <select
              aria-label={t("list.customFieldFilterValue")}
              className="h-8 rounded-md border bg-background px-2 text-xs"
              disabled={activeCustomFilterFieldId === "all"}
              value={activeCustomFilterValue}
              onChange={(event) => {
                if (activeCustomFilterFieldId === "all") {
                  return;
                }
                onSetCustomFieldFilter(
                  activeCustomFilterFieldId,
                  event.target.value as string | "all"
                );
              }}
            >
              <option value="all">{t("list.customFieldNone")}</option>
              {(activeCustomFilterFieldId !== "all"
                ? customFieldOptions[activeCustomFilterFieldId] ?? []
                : []
              ).map((option) => (
                <option key={option} value={option}>
                  {option}
                </option>
              ))}
            </select>
          </label>
          <label className="flex flex-col gap-1 text-xs font-medium">
            {t("list.customFieldSort")}
            <select
              aria-label={t("list.customFieldSort")}
              className="h-8 rounded-md border bg-background px-2 text-xs"
              value={customSortFieldId}
              onChange={(event) => setCustomSortFieldId(event.target.value)}
            >
              <option value="all">{t("list.customFieldNone")}</option>
              {customFields.map((field) => (
                <option key={field.id} value={field.id}>
                  {field.name}
                </option>
              ))}
            </select>
          </label>
          <label className="flex flex-col gap-1 text-xs font-medium md:col-span-3">
            {t("list.customFieldGroup")}
            <select
              aria-label={t("list.customFieldGroup")}
              className="h-8 rounded-md border bg-background px-2 text-xs md:max-w-xs"
              value={groupByFieldId}
              onChange={(event) => setGroupByFieldId(event.target.value)}
            >
              <option value="all">{t("list.customFieldNone")}</option>
              {customFields.map((field) => (
                <option key={field.id} value={field.id}>
                  {field.name}
                </option>
              ))}
            </select>
          </label>
        </div>
      ) : null}

      <Table>
        <TableHeader>
          <TableRow>
          <TableHead className="w-8">
            <input
              type="checkbox"
              className="size-4 rounded border-border"
              checked={selectedTaskIds.length > 0 && selectedTaskIds.length === tasks.length}
              onChange={() => {
                if (selectedTaskIds.length === tasks.length) {
                  useTaskWorkspaceStore.getState().clearSelection();
                } else {
                  useTaskWorkspaceStore.getState().selectAllVisible(tasks.map((t) => t.id));
                }
              }}
            />
          </TableHead>
          <TableHead aria-sort={sortState?.key === "task" ? (sortState.direction === "asc" ? "ascending" : "descending") : "none"}>
            <button type="button" className="inline-flex items-center gap-1" aria-label={`Sort by ${t("list.task")}`} onClick={() => toggleSort("task")}>
              {t("list.task")} {sortIndicator("task")}
            </button>
          </TableHead>
          <TableHead aria-sort={sortState?.key === "status" ? (sortState.direction === "asc" ? "ascending" : "descending") : "none"}>
            <button type="button" className="inline-flex items-center gap-1" aria-label={`Sort by ${t("list.status")}`} onClick={() => toggleSort("status")}>
              {t("list.status")} {sortIndicator("status")}
            </button>
          </TableHead>
          <TableHead>{t("list.progress")}</TableHead>
          <TableHead aria-sort={sortState?.key === "priority" ? (sortState.direction === "asc" ? "ascending" : "descending") : "none"}>
            <button type="button" className="inline-flex items-center gap-1" aria-label={`Sort by ${t("list.priority")}`} onClick={() => toggleSort("priority")}>
              {t("list.priority")} {sortIndicator("priority")}
            </button>
          </TableHead>
          <TableHead aria-sort={sortState?.key === "assignee" ? (sortState.direction === "asc" ? "ascending" : "descending") : "none"}>
            <button type="button" className="inline-flex items-center gap-1" aria-label={`Sort by ${t("list.assignee")}`} onClick={() => toggleSort("assignee")}>
              {t("list.assignee")} {sortIndicator("assignee")}
            </button>
          </TableHead>
          <TableHead aria-sort={sortState?.key === "planning" ? (sortState.direction === "asc" ? "ascending" : "descending") : "none"}>
            <button type="button" className="inline-flex items-center gap-1" aria-label={`Sort by ${t("list.planning")}`} onClick={() => toggleSort("planning")}>
              {t("list.planning")} {sortIndicator("planning")}
            </button>
          </TableHead>
          {showLinkedDocs ? <TableHead>{t("list.linkedDocs")}</TableHead> : null}
          {visibleCustomFields.map((field) => (
            <TableHead key={field.id}>{field.name}</TableHead>
          ))}
          <TableHead className="text-right">{t("list.action")}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {groupedRows.flatMap(({ key, label, rows }) => [
            ...(groupByFieldId !== "all"
              ? [
                  <TableRow key={`group-${key}`} data-group-header={key}>
                    <TableCell
                      colSpan={
                        8 +
                        (showLinkedDocs ? 1 : 0) +
                        visibleCustomFields.length
                      }
                      className="bg-muted/30 text-xs font-medium text-muted-foreground"
                    >
                      {customFields.find((field) => field.id === groupByFieldId)?.name}: {label}
                    </TableCell>
                  </TableRow>,
                ]
              : []),
            ...rows.map(({ task, depth, hasChildren }) => (
              <TableRow
              key={task.id}
              data-task-id={task.id}
              data-selected={task.id === selectedTaskId ? "true" : "false"}
              className={cn(
                task.id === selectedTaskId && "bg-accent/40 hover:bg-accent/50",
                selectedTaskIds.includes(task.id) && "bg-blue-500/5"
              )}
            >
              <TableCell>
                <input
                  type="checkbox"
                  className="size-4 rounded border-border"
                  checked={selectedTaskIds.includes(task.id)}
                  onChange={() => toggleTaskSelection(task.id)}
                />
              </TableCell>
              <TableCell>
                <div className={cn("flex flex-col", density === "compact" ? "gap-0.5" : "gap-1")} style={{ paddingLeft: depth * 24 }}>
                  <div className="flex items-center gap-1.5">
                    {hasChildren ? (
                      <button
                        type="button"
                        className="text-xs text-muted-foreground hover:text-foreground"
                        onClick={(e) => { e.stopPropagation(); toggleCollapse(task.id); }}
                      >
                        {collapsedParents.has(task.id) ? "▸" : "▾"}
                      </button>
                    ) : depth > 0 ? (
                      <span className="w-3 text-xs text-muted-foreground">└</span>
                    ) : null}
                    <span className="font-medium">{task.title}</span>
                  </div>
                  {showDescriptions ? (
                    <div className="text-xs text-muted-foreground">{task.description}</div>
                  ) : null}
                  {(() => {
                    const dependencyState = getTaskDependencyState(task, allTasks);

                    if (dependencyState.state === "blocked") {
                      return (
                        <span className="text-xs text-amber-700 dark:text-amber-300">
                          {t("list.blockedByDependency")}
                        </span>
                      );
                    }
                    if (dependencyState.state === "ready_to_unblock") {
                      return (
                        <span className="text-xs text-emerald-700 dark:text-emerald-300">
                          {t("list.readyToUnblock")}
                        </span>
                      );
                    }
                    if (dependencyState.blockedTasks.length > 0) {
                      return (
                        <span className="text-xs text-muted-foreground">
                          {t("list.blocksDownstream", { count: dependencyState.blockedTasks.length })}
                        </span>
                      );
                    }
                    return null;
                  })()}
                  {rowErrors[task.id] ? (
                    <div className="text-xs text-destructive">{rowErrors[task.id]}</div>
                  ) : null}
                </div>
              </TableCell>
              <TableCell>
                <select
                  aria-label={`Status for ${task.title}`}
                  className="h-8 rounded-md border bg-background px-2 text-xs"
                  value={task.status}
                  onChange={(event) =>
                    void handleInlineStatusChange(task, event.target.value as TaskStatus)
                  }
                >
                  {inlineStatusOptions.map((status) => (
                    <option key={status} value={status}>
                      {status}
                    </option>
                  ))}
                </select>
              </TableCell>
              <TableCell>
                {formatProgressHealthKey(task) ? (
                  <div className="flex flex-col gap-1">
                    <Badge
                      variant="secondary"
                      className={getProgressBadgeClass(task)}
                    >
                      {t(formatProgressHealthKey(task)!)}
                    </Badge>
                    {formatProgressReasonKey(task) ? (
                      <span className="text-xs text-muted-foreground">
                        {t(formatProgressReasonKey(task)!)}
                      </span>
                    ) : null}
                  </div>
                ) : (
                  <span className="text-xs text-muted-foreground">{t("list.healthy")}</span>
                )}
              </TableCell>
              <TableCell>
                <select
                  aria-label={`Priority for ${task.title}`}
                  className="h-8 rounded-md border bg-background px-2 text-xs"
                  value={task.priority}
                  onChange={(event) =>
                    void handleInlinePriorityChange(task, event.target.value as TaskPriority)
                  }
                >
                  {inlinePriorityOptions.map((priority) => (
                    <option key={priority} value={priority}>
                      {priority}
                    </option>
                  ))}
                </select>
              </TableCell>
              <TableCell>{task.assigneeName ?? t("list.unassigned")}</TableCell>
              <TableCell>{formatPlanningState(task, t("planning.unscheduled"))}</TableCell>
              {showLinkedDocs ? (
                <TableCell>
                  <div className="flex flex-wrap gap-1">
                    {(linkedDocsByTask[task.id] ?? []).map((doc) => (
                      <Link
                        key={doc.id}
                        href={buildDocsHref(doc.pageId)}
                        className="inline-flex items-center rounded-full border border-border/60 px-2 py-0.5 text-xs font-medium text-foreground transition-colors hover:bg-accent hover:text-accent-foreground"
                      >
                        {doc.title}
                      </Link>
                    ))}
                    {(linkedDocsByTask[task.id] ?? []).length === 0 ? (
                      <span className="text-xs text-muted-foreground">{t("list.none")}</span>
                    ) : null}
                  </div>
                </TableCell>
              ) : null}
              {visibleCustomFields.map((field) => (
                <TableCell key={field.id}>
                  <FieldValueCell
                    projectId={projectId}
                    taskId={task.id}
                    field={field}
                    value={valuesByTask[task.id]?.find((item) => item.fieldDefId === field.id) ?? null}
                  />
                </TableCell>
              ))}
              <TableCell className="text-right">
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => onTaskOpen(task.id)}
                >
                  {t("list.openTask", { title: task.title })}
                </Button>
              </TableCell>
              </TableRow>
            )),
          ])}
        </TableBody>
      </Table>
    </div>
  );
}

type TimelineGranularity = "day" | "week" | "month";
type CalendarMode = "month" | "week";

function PlanningTaskChip({
  task,
  index,
  isSelected,
  density,
  onTaskOpen,
  span = 1,
  testId,
  granularity,
  mode,
}: {
  task: Task;
  index: number;
  isSelected: boolean;
  density: "comfortable" | "compact";
  onTaskOpen: (taskId: string) => void;
  span?: number;
  testId?: string;
  granularity?: TimelineGranularity;
  mode?: CalendarMode;
}) {
  const t = useTranslations("tasks");
  const spanWidth =
    span > 1 ? `calc(${span * 100}% + ${(span - 1) * 0.75}rem)` : undefined;

  return (
    <Draggable draggableId={task.id} index={index}>
      {(provided, snapshot) => (
        <button
          ref={provided.innerRef}
          {...provided.draggableProps}
          {...provided.dragHandleProps}
          data-task-id={task.id}
          data-selected={isSelected ? "true" : "false"}
          data-testid={testId}
          data-span={String(span)}
          data-granularity={granularity}
          data-mode={mode}
          className={cn(
            "relative z-10 w-full rounded-md border text-left text-sm",
            density === "compact" ? "px-2 py-1" : "px-2.5 py-1.5",
            snapshot.isDragging ? "bg-accent" : "bg-background",
            isSelected && "border-primary/40 ring-2 ring-primary/25"
          )}
          style={spanWidth ? { width: spanWidth } : undefined}
          onClick={() => onTaskOpen(task.id)}
          type="button"
        >
          <div className="font-medium">{task.title}</div>
          <div className="text-xs text-muted-foreground">
            {task.assigneeName ?? t("list.unassigned")}
          </div>
        </button>
      )}
    </Draggable>
  );
}

function timelineSlotKey(
  task: Task,
  granularity: TimelineGranularity
): string | null {
  if (!task.plannedStartAt) {
    return null;
  }

  const start = new Date(task.plannedStartAt);
  switch (granularity) {
    case "week":
      return formatDateKey(startOfWeek(start));
    case "month":
      return formatDateKey(startOfMonth(start));
    case "day":
    default:
      return formatDateKey(start);
  }
}

function timelineSpan(task: Task, granularity: TimelineGranularity): number {
  if (!task.plannedStartAt || !task.plannedEndAt) {
    return 1;
  }

  const start = new Date(task.plannedStartAt);
  const end = new Date(task.plannedEndAt);

  switch (granularity) {
    case "week":
      return Math.max(1, differenceInWeeks(end, start) + 1);
    case "month":
      return Math.max(1, differenceInMonths(end, start) + 1);
    case "day":
    default:
      return Math.max(1, differenceInDays(end, start) + 1);
  }
}

function timelineSlotKeys(
  baseline: string,
  granularity: TimelineGranularity
): string[] {
  const start = new Date(formatDateKey(baseline) + "T00:00:00.000Z");

  switch (granularity) {
    case "week": {
      const weekStart = startOfWeek(start);
      return Array.from({ length: 6 }, (_, index) =>
        formatDateKey(addWeeks(weekStart, index))
      );
    }
    case "month": {
      const monthStart = startOfMonth(start);
      return Array.from({ length: 4 }, (_, index) =>
        formatDateKey(addMonths(monthStart, index))
      );
    }
    case "day":
    default:
      return Array.from({ length: 7 }, (_, index) =>
        formatDateKey(addDays(start, index))
      );
  }
}

function formatTimelineSlotLabel(
  slotKey: string,
  granularity: TimelineGranularity
): string {
  switch (granularity) {
    case "week":
      return `Week of ${slotKey}`;
    case "month":
      return slotKey.slice(0, 7);
    case "day":
    default:
      return slotKey;
  }
}

function calendarSlotKeys(baseline: string, mode: CalendarMode): string[] {
  const start = new Date(formatDateKey(baseline) + "T00:00:00.000Z");

  if (mode === "week") {
    const weekStart = startOfWeek(start);
    return Array.from({ length: 7 }, (_, index) =>
      formatDateKey(addDays(weekStart, index))
    );
  }

  const monthStart = startOfMonth(start);
  const monthEnd = endOfMonth(monthStart);
  const keys: string[] = [];

  for (let cursor = monthStart; cursor <= monthEnd; cursor = addDays(cursor, 1)) {
    keys.push(formatDateKey(cursor));
  }

  return keys;
}

function formatCalendarSlotLabel(slotKey: string, mode: CalendarMode): string {
  if (mode === "week") {
    return slotKey;
  }

  return String(new Date(slotKey + "T00:00:00.000Z").getUTCDate());
}

function calendarSpan(task: Task): number {
  if (!task.plannedStartAt || !task.plannedEndAt) {
    return 1;
  }

  return Math.max(
    1,
    differenceInDays(new Date(task.plannedEndAt), new Date(task.plannedStartAt)) +
      1
  );
}

function PlanningBoard({
  tasks,
  selectedTaskId,
  density,
  slotKeys,
  columnCount,
  droppablePrefix,
  onTaskOpen,
  onTaskScheduleChange,
  slotLabelFormatter,
  resolveSlotKey,
  getTaskSpan,
  chipMetadata,
}: {
  tasks: Task[];
  selectedTaskId: string | null;
  density: "comfortable" | "compact";
  slotKeys: string[];
  columnCount: number;
  droppablePrefix: "timeline" | "calendar";
  onTaskOpen: (taskId: string) => void;
  onTaskScheduleChange: (
    taskId: string,
    changes: { plannedStartAt: string; plannedEndAt: string }
  ) => Promise<void> | void;
  slotLabelFormatter: (slotKey: string) => string;
  resolveSlotKey: (task: Task) => string | null;
  getTaskSpan: (task: Task) => number;
  chipMetadata?: {
    granularity?: TimelineGranularity;
    mode?: CalendarMode;
  };
}) {
  const t = useTranslations("tasks");
  const [error, setError] = useState<string | null>(null);

  const scheduledBySlot = useMemo(() => {
    const map = new Map<string, Task[]>();
    for (const key of slotKeys) {
      map.set(key, []);
    }
    for (const task of tasks) {
      if (!task.plannedStartAt || !task.plannedEndAt) continue;
      const key = resolveSlotKey(task);
      if (!key) continue;
      if (!map.has(key)) {
        map.set(key, []);
      }
      map.get(key)?.push(task);
    }
    return map;
  }, [resolveSlotKey, slotKeys, tasks]);

  const unscheduledTasks = tasks.filter(
    (task) => !task.plannedStartAt || !task.plannedEndAt
  );

  const onDragEnd = async (result: DropResult) => {
    if (!result.destination) return;

    const [destinationPrefix, destinationDate] =
      result.destination.droppableId.split(":");
    if (destinationPrefix !== droppablePrefix || !destinationDate) return;
    if (destinationDate === "unscheduled") return;

    const task = tasks.find((item) => item.id === result.draggableId);
    if (!task) return;

    setError(null);
    try {
      await onTaskScheduleChange(
        task.id,
        getRescheduledPlanningWindow(task, destinationDate)
      );
    } catch (scheduleError) {
      setError(
        scheduleError instanceof Error
          ? scheduleError.message
          : t("planning.failedUpdate")
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
        <div
          className="grid gap-3"
          style={{
            gridTemplateColumns: `repeat(${columnCount}, minmax(0, 1fr))`,
          }}
        >
          {slotKeys.map((slotKey) => (
            <Card key={slotKey} className="gap-3 py-4">
              <CardHeader className="px-4">
                <CardTitle className="text-sm">
                  {slotLabelFormatter(slotKey)}
                </CardTitle>
              </CardHeader>
              <CardContent className="px-4">
                <Droppable droppableId={`${droppablePrefix}:${slotKey}`}>
                  {(provided, snapshot) => (
                    <div
                      ref={provided.innerRef}
                      {...provided.droppableProps}
                      className={`flex min-h-28 flex-col gap-2 rounded-md border border-dashed p-2 ${
                        snapshot.isDraggingOver ? "bg-accent/40" : "bg-muted/30"
                      }`}
                    >
                      {(scheduledBySlot.get(slotKey) ?? []).map((task, index) => (
                        <PlanningTaskChip
                          key={task.id}
                          task={task}
                          index={index}
                          isSelected={task.id === selectedTaskId}
                          density={density}
                          onTaskOpen={onTaskOpen}
                          span={getTaskSpan(task)}
                          testId={`${droppablePrefix}-bar-${task.id}`}
                          granularity={chipMetadata?.granularity}
                          mode={chipMetadata?.mode}
                        />
                      ))}
                      {provided.placeholder}
                    </div>
                  )}
                </Droppable>
              </CardContent>
            </Card>
          ))}
        </div>

        <Card>
          <CardHeader>
            <CardTitle>{t("planning.unscheduled")}</CardTitle>
            <CardDescription>
              {t("planning.unscheduledDescription")}
            </CardDescription>
          </CardHeader>
          <CardContent>
            <Droppable droppableId={`${droppablePrefix}:unscheduled`}>
              {(provided, snapshot) => (
                <div
                  ref={provided.innerRef}
                  {...provided.droppableProps}
                  className={`flex min-h-20 flex-col gap-2 rounded-md border border-dashed p-3 ${
                    snapshot.isDraggingOver ? "bg-accent/40" : "bg-muted/30"
                  }`}
                >
                  {unscheduledTasks.map((task, index) => (
                    <PlanningTaskChip
                      key={task.id}
                      task={task}
                      index={index}
                      isSelected={task.id === selectedTaskId}
                      density={density}
                      onTaskOpen={onTaskOpen}
                    />
                  ))}
                  {unscheduledTasks.length === 0 ? (
                    <p className="text-sm text-muted-foreground">
                      {t("planning.allScheduled")}
                    </p>
                  ) : null}
                  {provided.placeholder}
                </div>
              )}
            </Droppable>
          </CardContent>
        </Card>
      </DragDropContext>
    </div>
  );
}

export function TaskTimelineView(props: {
  tasks: Task[];
  selectedTaskId: string | null;
  density: "comfortable" | "compact";
  onTaskOpen: (taskId: string) => void;
  onTaskScheduleChange: (
    taskId: string,
    changes: { plannedStartAt: string; plannedEndAt: string }
  ) => Promise<void> | void;
}) {
  const t = useTranslations("tasks");
  const [granularity, setGranularity] = useState<TimelineGranularity>("day");
  const baseline =
    props.tasks.find((task) => task.plannedStartAt)?.plannedStartAt ??
    new Date().toISOString();
  const slotKeys = useMemo(
    () => timelineSlotKeys(baseline, granularity),
    [baseline, granularity]
  );

  return (
    <div data-testid="timeline-view" className="space-y-3">
      <div className="flex flex-wrap justify-end gap-2">
        {(["day", "week", "month"] as const).map((scale) => (
          <Button
            key={scale}
            type="button"
            size="sm"
            variant={granularity === scale ? "default" : "outline"}
            aria-pressed={granularity === scale}
            onClick={() => setGranularity(scale)}
          >
            {t(`planning.scale.${scale}`)}
          </Button>
        ))}
      </div>
      <PlanningBoard
        {...props}
        slotKeys={slotKeys}
        columnCount={slotKeys.length}
        droppablePrefix="timeline"
        slotLabelFormatter={(slotKey) =>
          formatTimelineSlotLabel(slotKey, granularity)
        }
        resolveSlotKey={(task) => timelineSlotKey(task, granularity)}
        getTaskSpan={(task) => timelineSpan(task, granularity)}
        chipMetadata={{ granularity }}
      />
    </div>
  );
}

export function TaskCalendarView(props: {
  tasks: Task[];
  selectedTaskId: string | null;
  density: "comfortable" | "compact";
  onTaskOpen: (taskId: string) => void;
  onTaskScheduleChange: (
    taskId: string,
    changes: { plannedStartAt: string; plannedEndAt: string }
  ) => Promise<void> | void;
}) {
  const t = useTranslations("tasks");
  const [mode, setMode] = useState<CalendarMode>("month");
  const baseline =
    props.tasks.find((task) => task.plannedStartAt)?.plannedStartAt ??
    new Date().toISOString();
  const slotKeys = useMemo(
    () => calendarSlotKeys(baseline, mode),
    [baseline, mode]
  );

  return (
    <div data-testid="calendar-view" className="space-y-3">
      <div className="flex flex-wrap justify-end gap-2">
        {(["month", "week"] as const).map((calendarMode) => (
          <Button
            key={calendarMode}
            type="button"
            size="sm"
            variant={mode === calendarMode ? "default" : "outline"}
            aria-pressed={mode === calendarMode}
            onClick={() => setMode(calendarMode)}
          >
            {t(`planning.scale.${calendarMode}`)}
          </Button>
        ))}
      </div>
      <PlanningBoard
        {...props}
        slotKeys={slotKeys}
        columnCount={7}
        droppablePrefix="calendar"
        slotLabelFormatter={(slotKey) => formatCalendarSlotLabel(slotKey, mode)}
        resolveSlotKey={(task) =>
          task.plannedStartAt ? formatDateKey(task.plannedStartAt) : null
        }
        getTaskSpan={calendarSpan}
        chipMetadata={{ mode }}
      />
    </div>
  );
}

export function TaskWorkspaceMain({
  projectId,
  tasks,
  sprints,
  sprintMetrics,
  sprintMetricsLoading,
  loading,
  error,
  onRetry,
  onTaskOpen,
  onTaskStatusChange,
  onTaskScheduleChange,
  onTaskSave,
  onCreateTask,
  members = [],
  onBulkStatusChange,
  onBulkAssign,
  onBulkDelete,
}: ProjectTaskWorkspaceProps) {
  const t = useTranslations("tasks");
  const viewMode = useTaskWorkspaceStore((state) => state.viewMode);
  const filters = useTaskWorkspaceStore((state) => state.filters);
  const selectedTaskId = useTaskWorkspaceStore((state) => state.selectedTaskId);
  const displayOptions = useTaskWorkspaceStore((state) => state.displayOptions);
  const setCustomFieldFilter = useTaskWorkspaceStore((state) => state.setCustomFieldFilter);
  const selectTask = useTaskWorkspaceStore((state) => state.selectTask);
  const selectedTaskIds = useTaskWorkspaceStore((state) => state.selectedTaskIds);
  const selectAllVisible = useTaskWorkspaceStore((state) => state.selectAllVisible);
  const toggleTaskSelection = useTaskWorkspaceStore((state) => state.toggleTaskSelection);
  const clearSelection = useTaskWorkspaceStore((state) => state.clearSelection);
  const definitionsByProject = useCustomFieldStore((state) => state.definitionsByProject);
  const valuesByTask = useCustomFieldStore((state) => state.valuesByTask);
  const [bulkFailures, setBulkFailures] = useState<BulkActionFailure[]>([]);
  const docsTree = useDocsStore((state) => state.tree);
  const linksByEntity = useEntityLinkStore((state) => state.linksByEntity);

  const filteredTasks = useMemo(
    () => filterTasksForWorkspace(tasks, filters, { valuesByTask }),
    [tasks, filters, valuesByTask]
  );
  const customFields = useMemo(
    () => definitionsByProject[projectId] ?? [],
    [definitionsByProject, projectId]
  );
  const docsById = useMemo(() => {
    const map = new Map<string, ReturnType<typeof flattenDocsTree>[number]>();
    for (const doc of flattenDocsTree(docsTree)) {
      map.set(doc.id, doc);
    }
    return map;
  }, [docsTree]);
  const linkedDocsByTask = useMemo<Record<string, LinkedDocItem[]>>(() => {
    const result: Record<string, LinkedDocItem[]> = {};
    for (const task of tasks) {
      const links = linksByEntity[`task:${task.id}`] ?? [];
      result[task.id] = links
        .filter((link) => link.targetType === "wiki_page")
        .map((link) => {
          const doc = docsById.get(link.targetId);
          return {
            id: link.id,
            pageId: link.targetId,
            title: doc?.title ?? link.targetId,
            linkType: link.linkType,
            updatedAt: doc?.updatedAt ?? new Date().toISOString(),
            preview: doc?.contentText ?? "",
          };
        });
    }
    return result;
  }, [docsById, linksByEntity, tasks]);
  const taskTitleById = useMemo(
    () => new Map(tasks.map((task) => [task.id, task.title])),
    [tasks]
  );
  const handleTaskOpen = (taskId: string) => {
    selectTask(taskId);
    onTaskOpen(taskId);
  };

  const applyBulkResult = (
    result: BulkActionResult | void,
    options?: { preserveFailedSelection?: boolean }
  ) => {
    const failures = result?.failed ?? [];
    setBulkFailures(failures);

    if (options?.preserveFailedSelection) {
      if (failures.length > 0) {
        selectAllVisible(failures.map((failure) => failure.taskId));
      } else {
        clearSelection();
      }
    }
  };

  const renderView = (mode: TaskViewMode) => {
    if (loading) {
      return (
        <div className="flex flex-col gap-3">
          {mode === "board" ? (
            <div data-testid="board-loading-skeleton" className="flex gap-4">
              {[1, 2, 3, 4].map((col) => (
                <div key={col} className="flex w-72 shrink-0 flex-col gap-2 rounded-lg border bg-muted/50 p-3">
                  <Skeleton className="h-5 w-20" />
                  <Skeleton className="h-24 w-full rounded-md" />
                  <Skeleton className="h-24 w-full rounded-md" />
                  <Skeleton className="h-24 w-full rounded-md" />
                </div>
              ))}
            </div>
          ) : (
            <div className="flex flex-col gap-2">
              {Array.from({ length: 8 }).map((_, i) => (
                <Skeleton key={i} className="h-12 w-full rounded-md" />
              ))}
            </div>
          )}
        </div>
      );
    }

    if (error) {
      return (
        <Card>
          <CardHeader>
            <CardTitle>{t("empty.unableToLoad")}</CardTitle>
            <CardDescription>{error}</CardDescription>
          </CardHeader>
          <CardContent>
            <Button onClick={onRetry}>{t("empty.retry")}</Button>
          </CardContent>
        </Card>
      );
    }

    if (tasks.length === 0) {
      return (
        <EmptyState
          title={t("empty.noTasks")}
          description={t("empty.noTasksDescription")}
          actionLabel={onCreateTask ? t("empty.createTaskAction") : undefined}
          onAction={onCreateTask}
        />
      );
    }

    if (filteredTasks.length === 0) {
      return (
        <EmptyState
          title={t("empty.noMatch")}
          description={t("empty.noMatchDescription")}
          actionLabel={onCreateTask ? t("empty.createTaskAction") : undefined}
          onAction={onCreateTask}
        />
      );
    }

    switch (mode) {
      case "list":
        return (
          <TaskListView
            projectId={projectId}
            tasks={filteredTasks}
            allTasks={tasks}
            selectedTaskId={selectedTaskId}
            filters={filters}
            density={displayOptions.density}
            showDescriptions={displayOptions.showDescriptions}
            showLinkedDocs={displayOptions.showLinkedDocs}
            customFields={customFields}
            valuesByTask={valuesByTask}
            linkedDocsByTask={linkedDocsByTask}
            onTaskOpen={handleTaskOpen}
            onTaskStatusChange={onTaskStatusChange}
            onTaskSave={onTaskSave}
            onSetCustomFieldFilter={setCustomFieldFilter}
          />
        );
      case "timeline":
        return (
          <TaskTimelineView
            tasks={filteredTasks}
            selectedTaskId={selectedTaskId}
            density={displayOptions.density}
            onTaskOpen={handleTaskOpen}
            onTaskScheduleChange={onTaskScheduleChange}
          />
        );
      case "calendar":
        return (
          <TaskCalendarView
            tasks={filteredTasks}
            selectedTaskId={selectedTaskId}
            density={displayOptions.density}
            onTaskOpen={handleTaskOpen}
            onTaskScheduleChange={onTaskScheduleChange}
          />
        );
      case "dependencies":
        return (
          <TaskDependencyGraph
            tasks={filteredTasks}
            onTaskClick={handleTaskOpen}
          />
        );
      case "roadmap":
        return (
          <RoadmapView
            projectId={projectId}
            tasks={filteredTasks}
            sprints={sprints}
          />
        );
      case "board":
      default:
        return (
          <Board
            tasks={filteredTasks}
            allTasks={tasks}
            selectedTaskId={selectedTaskId}
            selectedTaskIds={selectedTaskIds}
            displayOptions={displayOptions}
            linkedDocsByTask={linkedDocsByTask}
            onTaskClick={(task) => handleTaskOpen(task.id)}
            onTaskStatusChange={onTaskStatusChange}
            onToggleTaskSelection={toggleTaskSelection}
            onQuickStatusChange={(taskId, status) => void onTaskStatusChange(taskId, status)}
            onQuickPriorityChange={(taskId, priority) => void onTaskSave?.(taskId, { priority })}
          />
        );
    }
  };

  return (
    <div className="flex h-full flex-col gap-3 p-4">
      <SprintOverview
        sprints={sprints}
        sprintMetrics={sprintMetrics}
        sprintMetricsLoading={sprintMetricsLoading}
      />

      {selectedTaskIds.length > 0 ? (
        <div>
          {bulkFailures.length > 0 ? (
            <div className="mb-3 rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
              <p className="font-medium">{t("bulk.partialFailureTitle")}</p>
              <ul className="mt-1 list-disc pl-5">
                {bulkFailures.map((failure) => (
                  <li key={`${failure.taskId}:${failure.message}`}>
                    {(taskTitleById.get(failure.taskId) ?? failure.taskId)}: {failure.message}
                  </li>
                ))}
              </ul>
            </div>
          ) : null}
          <BulkActionToolbar
            selectedCount={selectedTaskIds.length}
            members={members}
            onBulkStatusChange={async (status) => {
              const result = await onBulkStatusChange?.(selectedTaskIds, status);
              applyBulkResult(result);
            }}
            onBulkAssign={async (assigneeId, assigneeType) => {
              const result = await onBulkAssign?.(selectedTaskIds, assigneeId, assigneeType);
              applyBulkResult(result);
            }}
            onBulkDelete={async () => {
              const result = await onBulkDelete?.(selectedTaskIds);
              applyBulkResult(result, { preserveFailedSelection: true });
            }}
            onClearSelection={() => {
              setBulkFailures([]);
              clearSelection();
            }}
          />
        </div>
      ) : null}

      <div className="min-h-0 flex-1 overflow-auto">
        {renderView(viewMode)}
      </div>
    </div>
  );
}
