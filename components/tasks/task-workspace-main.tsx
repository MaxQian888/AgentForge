"use client";

import { useMemo, useState } from "react";
import {
  DragDropContext,
  Draggable,
  Droppable,
  type DropResult,
} from "@hello-pangea/dnd";
import { Board } from "@/components/kanban/board";
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
import { Input } from "@/components/ui/input";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  filterTasksForWorkspace,
  type TaskDependencyFilter,
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
import { ViewSwitcher } from "@/components/views/view-switcher";
import { RoadmapView } from "@/components/milestones/roadmap-view";
import type { Task, TaskPriority, TaskStatus } from "@/lib/stores/task-store";
import type { LinkedDocItem } from "./linked-docs-panel";

interface ProjectTaskWorkspaceProps {
  projectId: string;
  tasks: Task[];
  sprints: Sprint[];
  sprintMetrics: SprintMetrics | null;
  sprintMetricsLoading: boolean;
  loading: boolean;
  error: string | null;
  realtimeConnected: boolean;
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
  onSprintFilterChange?: (sprintId: string | "all") => void;
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

function formatPlanningState(task: Task): string {
  if (task.plannedStartAt && task.plannedEndAt) {
    return `${formatDateKey(task.plannedStartAt)} → ${formatDateKey(task.plannedEndAt)}`;
  }
  return "Unscheduled";
}

function formatProgressHealth(task: Task): string | null {
  switch (task.progress?.healthStatus) {
    case "stalled":
      return "Stalled";
    case "warning":
      return "At risk";
    default:
      return null;
  }
}

function formatProgressReason(task: Task): string | null {
  switch (task.progress?.riskReason) {
    case "no_recent_update":
      return "No recent update";
    case "no_assignee":
      return "No assignee";
    case "awaiting_review":
      return "Awaiting review";
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

function statusOptions(): Array<"all" | TaskStatus> {
  return ["all", "inbox", "triaged", "assigned", "in_progress", "in_review", "done"].filter(
    (value, index, items) => items.indexOf(value) === index
  ) as Array<"all" | TaskStatus>;
}

function priorityOptions(): Array<"all" | TaskPriority> {
  return ["all", "urgent", "high", "medium", "low"];
}

function dependencyOptions(): Array<{ value: TaskDependencyFilter; label: string }> {
  return [
    { value: "all", label: "all" },
    { value: "blocked", label: "blocked" },
    { value: "ready_to_unblock", label: "ready to unblock" },
  ];
}

function assigneeOptions(tasks: Task[]): Array<{ value: string; label: string }> {
  const seen = new Map<string, string>();
  for (const task of tasks) {
    if (task.assigneeId) {
      seen.set(task.assigneeId, task.assigneeName ?? task.assigneeId);
    }
  }
  return Array.from(seen.entries()).map(([value, label]) => ({ value, label }));
}

function sprintOptions(sprints: Sprint[]): Array<{ value: string; label: string }> {
  return sprints.map((sprint) => ({
    value: sprint.id,
    label: sprint.name,
  }));
}

function formatSprintStatus(status: Sprint["status"]): string {
  switch (status) {
    case "active":
      return "Active";
    case "planning":
      return "Planning";
    case "closed":
      return "Closed";
    default:
      return status;
  }
}

function formatSprintRange(sprint: Sprint): string {
  return `${formatDateKey(sprint.startDate)} -> ${formatDateKey(sprint.endDate)}`;
}

function getActiveFilterChips(
  filters: ReturnType<typeof useTaskWorkspaceStore.getState>["filters"],
  tasks: Task[],
  sprints: Sprint[]
): Array<{
  key: string;
  label: string;
  clearLabel: string;
  onClear: (
    setSearch: (search: string) => void,
    setStatus: (status: "all" | TaskStatus) => void,
    setPriority: (priority: "all" | TaskPriority) => void,
    setAssigneeId: (assigneeId: string | "all") => void,
    setSprintId: (sprintId: string | "all") => void,
    setPlanning: (
      planning: "all" | "scheduled" | "unscheduled"
    ) => void,
    setDependency: (dependency: TaskDependencyFilter) => void
  ) => void;
}> {
  const chips: Array<{
    key: string;
    label: string;
    clearLabel: string;
    onClear: (
      setSearch: (search: string) => void,
      setStatus: (status: "all" | TaskStatus) => void,
      setPriority: (priority: "all" | TaskPriority) => void,
      setAssigneeId: (assigneeId: string | "all") => void,
      setSprintId: (sprintId: string | "all") => void,
      setPlanning: (
        planning: "all" | "scheduled" | "unscheduled"
      ) => void,
      setDependency: (dependency: TaskDependencyFilter) => void
    ) => void;
  }> = [];

  if (filters.search.trim()) {
    chips.push({
      key: "search",
      label: `search: ${filters.search.trim()}`,
      clearLabel: `Clear filter search "${filters.search.trim()}"`,
      onClear: (clearSearch) => clearSearch(""),
    });
  }

  if (filters.status !== "all") {
    chips.push({
      key: "status",
      label: `status: ${filters.status}`,
      clearLabel: `Clear filter status "${filters.status}"`,
      onClear: (_clearSearch, clearStatus) => clearStatus("all"),
    });
  }

  if (filters.priority !== "all") {
    chips.push({
      key: "priority",
      label: `priority: ${filters.priority}`,
      clearLabel: `Clear filter priority "${filters.priority}"`,
      onClear: (_clearSearch, _clearStatus, clearPriority) => clearPriority("all"),
    });
  }

  if (filters.assigneeId !== "all") {
    const assigneeLabel =
      assigneeOptions(tasks).find((option) => option.value === filters.assigneeId)?.label ??
      filters.assigneeId;

    chips.push({
      key: "assignee",
      label: `assignee: ${assigneeLabel}`,
      clearLabel: `Clear filter assignee "${assigneeLabel}"`,
      onClear: (_clearSearch, _clearStatus, _clearPriority, clearAssignee) =>
        clearAssignee("all"),
    });
  }

  if (filters.sprintId !== "all") {
    const sprintLabel =
      sprintOptions(sprints).find((option) => option.value === filters.sprintId)?.label ??
      filters.sprintId;

    chips.push({
      key: "sprint",
      label: `sprint: ${sprintLabel}`,
      clearLabel: `Clear filter sprint "${sprintLabel}"`,
      onClear: (
        _clearSearch,
        _clearStatus,
        _clearPriority,
        _clearAssignee,
        clearSprint
      ) => clearSprint("all"),
    });
  }

  if (filters.planning !== "all") {
    chips.push({
      key: "planning",
      label: `planning: ${filters.planning}`,
      clearLabel: `Clear filter planning "${filters.planning}"`,
      onClear: (
        _clearSearch,
        _clearStatus,
        _clearPriority,
        _clearAssignee,
        _clearSprint,
        clearPlanning
      ) => clearPlanning("all"),
    });
  }

  if (filters.dependency !== "all") {
    chips.push({
      key: "dependency",
      label: `dependencies: ${filters.dependency}`,
      clearLabel: `Clear filter dependencies "${filters.dependency}"`,
      onClear: (
        _clearSearch,
        _clearStatus,
        _clearPriority,
        _clearAssignee,
        _clearSprint,
        _clearPlanning,
        clearDependency
      ) => clearDependency("all"),
    });
  }

  return chips;
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
  if (sprints.length === 0) {
    return null;
  }

  const activeSprint =
    sprintMetrics?.sprint ?? sprints.find((sprint) => sprint.status === "active") ?? sprints[0];

  return (
    <div className="grid gap-3 xl:grid-cols-[minmax(0,1fr)_320px]">
      <Card className="gap-3">
        <CardHeader className="px-4">
          <CardTitle className="text-base">Sprint overview</CardTitle>
          <CardDescription>
            Track current cycle scope, burndown progress, and delivery velocity in one place.
          </CardDescription>
        </CardHeader>
        <CardContent className="grid gap-3 px-4">
          <div className="flex flex-wrap items-center gap-2">
            <span className="text-sm font-medium">{activeSprint.name}</span>
            <Badge variant="secondary">{formatSprintStatus(activeSprint.status)}</Badge>
            <Badge variant="outline">{formatSprintRange(activeSprint)}</Badge>
          </div>
          {sprintMetricsLoading ? (
            <div className="text-sm text-muted-foreground">Loading sprint metrics...</div>
          ) : sprintMetrics ? (
            <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
              <div className="rounded-md border border-border/60 bg-muted/20 px-3 py-2">
                <div className="text-xs text-muted-foreground">Completion</div>
                <div className="text-lg font-semibold">
                  {sprintMetrics.completionRate.toFixed(2)}%
                </div>
              </div>
              <div className="rounded-md border border-border/60 bg-muted/20 px-3 py-2">
                <div className="text-xs text-muted-foreground">Remaining</div>
                <div className="text-lg font-semibold">{sprintMetrics.remainingTasks}</div>
              </div>
              <div className="rounded-md border border-border/60 bg-muted/20 px-3 py-2">
                <div className="text-xs text-muted-foreground">Velocity</div>
                <div className="text-lg font-semibold">
                  {sprintMetrics.velocityPerWeek.toFixed(2)}/wk
                </div>
              </div>
              <div className="rounded-md border border-border/60 bg-muted/20 px-3 py-2">
                <div className="text-xs text-muted-foreground">Task spend</div>
                <div className="text-lg font-semibold">
                  ${sprintMetrics.taskSpentUsd.toFixed(2)}
                </div>
              </div>
            </div>
          ) : (
            <div className="text-sm text-muted-foreground">
              Select a sprint to load burndown and velocity metrics.
            </div>
          )}
        </CardContent>
      </Card>

      <Card className="gap-3">
        <CardHeader className="px-4">
          <CardTitle className="text-base">Burndown</CardTitle>
          <CardDescription>
            Velocity {sprintMetrics ? `${sprintMetrics.velocityPerWeek.toFixed(2)}/wk` : "--"}
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
}: {
  title: string;
  description: string;
}) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>{title}</CardTitle>
        <CardDescription>{description}</CardDescription>
      </CardHeader>
    </Card>
  );
}

function ListView({
  projectId,
  tasks,
  allTasks,
  selectedTaskId,
  density,
  showDescriptions,
  showLinkedDocs,
  customFields,
  valuesByTask,
  linkedDocsByTask,
  onTaskOpen,
}: {
  projectId: string;
  tasks: Task[];
  allTasks: Task[];
  selectedTaskId: string | null;
  density: "comfortable" | "compact";
  showDescriptions: boolean;
  showLinkedDocs: boolean;
  customFields: ReturnType<typeof useCustomFieldStore.getState>["definitionsByProject"][string];
  valuesByTask: Record<string, ReturnType<typeof useCustomFieldStore.getState>["valuesByTask"][string]>;
  linkedDocsByTask: Record<string, LinkedDocItem[]>;
  onTaskOpen: (taskId: string) => void;
}) {
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>Task</TableHead>
          <TableHead>Status</TableHead>
          <TableHead>Progress</TableHead>
          <TableHead>Priority</TableHead>
          <TableHead>Assignee</TableHead>
          <TableHead>Planning</TableHead>
          {showLinkedDocs ? <TableHead>Linked Docs</TableHead> : null}
          {customFields.map((field) => (
            <TableHead key={field.id}>{field.name}</TableHead>
          ))}
          <TableHead className="text-right">Action</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {tasks.map((task) => (
          <TableRow
            key={task.id}
            data-task-id={task.id}
            data-selected={task.id === selectedTaskId ? "true" : "false"}
            className={cn(
              task.id === selectedTaskId && "bg-accent/40 hover:bg-accent/50"
            )}
          >
            <TableCell>
              <div className={cn("flex flex-col", density === "compact" ? "gap-0.5" : "gap-1")}>
                <div className="font-medium">{task.title}</div>
                {showDescriptions ? (
                  <div className="text-xs text-muted-foreground">{task.description}</div>
                ) : null}
                {(() => {
                  const dependencyState = getTaskDependencyState(task, allTasks);

                  if (dependencyState.state === "blocked") {
                    return (
                      <span className="text-xs text-amber-700 dark:text-amber-300">
                        Blocked by dependency
                      </span>
                    );
                  }
                  if (dependencyState.state === "ready_to_unblock") {
                    return (
                      <span className="text-xs text-emerald-700 dark:text-emerald-300">
                        Ready to unblock
                      </span>
                    );
                  }
                  if (dependencyState.blockedTasks.length > 0) {
                    return (
                      <span className="text-xs text-muted-foreground">
                        Blocks {dependencyState.blockedTasks.length} downstream
                      </span>
                    );
                  }
                  return null;
                })()}
              </div>
            </TableCell>
            <TableCell>{task.status}</TableCell>
            <TableCell>
              {formatProgressHealth(task) ? (
                <div className="flex flex-col gap-1">
                  <Badge
                    variant="secondary"
                    className={getProgressBadgeClass(task)}
                  >
                    {formatProgressHealth(task)}
                  </Badge>
                  {formatProgressReason(task) ? (
                    <span className="text-xs text-muted-foreground">
                      {formatProgressReason(task)}
                    </span>
                  ) : null}
                </div>
              ) : (
                <span className="text-xs text-muted-foreground">Healthy</span>
              )}
            </TableCell>
            <TableCell>
              <Badge variant="secondary">{task.priority}</Badge>
            </TableCell>
            <TableCell>{task.assigneeName ?? "Unassigned"}</TableCell>
            <TableCell>{formatPlanningState(task)}</TableCell>
            {showLinkedDocs ? (
              <TableCell>
                <div className="flex flex-wrap gap-1">
                  {(linkedDocsByTask[task.id] ?? []).map((doc) => (
                    <Badge key={doc.id} variant="outline">
                      {doc.title}
                    </Badge>
                  ))}
                  {(linkedDocsByTask[task.id] ?? []).length === 0 ? (
                    <span className="text-xs text-muted-foreground">None</span>
                  ) : null}
                </div>
              </TableCell>
            ) : null}
            {customFields.map((field) => (
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
                Open {task.title}
              </Button>
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}

function PlanningTaskChip({
  task,
  index,
  isSelected,
  density,
  onTaskOpen,
}: {
  task: Task;
  index: number;
  isSelected: boolean;
  density: "comfortable" | "compact";
  onTaskOpen: (taskId: string) => void;
}) {
  return (
    <Draggable draggableId={task.id} index={index}>
      {(provided, snapshot) => (
        <button
          ref={provided.innerRef}
          {...provided.draggableProps}
          {...provided.dragHandleProps}
          data-task-id={task.id}
          data-selected={isSelected ? "true" : "false"}
          className={cn(
            "w-full rounded-md border text-left text-sm",
            density === "compact" ? "px-2 py-1" : "px-2.5 py-1.5",
            snapshot.isDragging ? "bg-accent" : "bg-background",
            isSelected && "border-primary/40 ring-2 ring-primary/25"
          )}
          onClick={() => onTaskOpen(task.id)}
          type="button"
        >
          <div className="font-medium">{task.title}</div>
          <div className="text-xs text-muted-foreground">
            {task.assigneeName ?? "Unassigned"}
          </div>
        </button>
      )}
    </Draggable>
  );
}

function PlanningBoard({
  tasks,
  selectedTaskId,
  density,
  dateKeys,
  droppablePrefix,
  onTaskOpen,
  onTaskScheduleChange,
}: {
  tasks: Task[];
  selectedTaskId: string | null;
  density: "comfortable" | "compact";
  dateKeys: string[];
  droppablePrefix: "timeline" | "calendar";
  onTaskOpen: (taskId: string) => void;
  onTaskScheduleChange: (
    taskId: string,
    changes: { plannedStartAt: string; plannedEndAt: string }
  ) => Promise<void> | void;
}) {
  const [error, setError] = useState<string | null>(null);

  const scheduledByDay = useMemo(() => {
    const map = new Map<string, Task[]>();
    for (const key of dateKeys) {
      map.set(key, []);
    }
    for (const task of tasks) {
      if (!task.plannedStartAt || !task.plannedEndAt) continue;
      const key = formatDateKey(task.plannedStartAt);
      if (!map.has(key)) {
        map.set(key, []);
      }
      map.get(key)?.push(task);
    }
    return map;
  }, [dateKeys, tasks]);

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
          : "Failed to update planning window."
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
        <div className="grid gap-3 md:grid-cols-4">
          {dateKeys.map((dateKey) => (
            <Card key={dateKey} className="gap-3 py-4">
              <CardHeader className="px-4">
                <CardTitle className="text-sm">{dateKey}</CardTitle>
              </CardHeader>
              <CardContent className="px-4">
                <Droppable droppableId={`${droppablePrefix}:${dateKey}`}>
                  {(provided, snapshot) => (
                    <div
                      ref={provided.innerRef}
                      {...provided.droppableProps}
                      className={`flex min-h-28 flex-col gap-2 rounded-md border border-dashed p-2 ${
                        snapshot.isDraggingOver ? "bg-accent/40" : "bg-muted/30"
                      }`}
                    >
                      {(scheduledByDay.get(dateKey) ?? []).map((task, index) => (
                        <PlanningTaskChip
                          key={task.id}
                          task={task}
                          index={index}
                          isSelected={task.id === selectedTaskId}
                          density={density}
                          onTaskOpen={onTaskOpen}
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
            <CardTitle>Unscheduled</CardTitle>
            <CardDescription>
              Tasks without a planning window stay visible here until scheduled.
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
                      All visible tasks are scheduled.
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

function TimelineView(props: {
  tasks: Task[];
  selectedTaskId: string | null;
  density: "comfortable" | "compact";
  onTaskOpen: (taskId: string) => void;
  onTaskScheduleChange: (
    taskId: string,
    changes: { plannedStartAt: string; plannedEndAt: string }
  ) => Promise<void> | void;
}) {
  const baseline =
    props.tasks.find((task) => task.plannedStartAt)?.plannedStartAt ?? new Date().toISOString();
  const start = new Date(formatDateKey(baseline) + "T00:00:00.000Z");
  const dateKeys = Array.from({ length: 7 }, (_, index) =>
    formatDateKey(addDays(start, index))
  );

  return (
    <PlanningBoard
      {...props}
      dateKeys={dateKeys}
      droppablePrefix="timeline"
    />
  );
}

function CalendarView(props: {
  tasks: Task[];
  selectedTaskId: string | null;
  density: "comfortable" | "compact";
  onTaskOpen: (taskId: string) => void;
  onTaskScheduleChange: (
    taskId: string,
    changes: { plannedStartAt: string; plannedEndAt: string }
  ) => Promise<void> | void;
}) {
  const baseline =
    props.tasks.find((task) => task.plannedStartAt)?.plannedStartAt ?? new Date().toISOString();
  const monthStart = startOfMonth(new Date(baseline));
  const monthEnd = endOfMonth(monthStart);
  const dateKeys: string[] = [];

  for (let cursor = monthStart; cursor <= monthEnd; cursor = addDays(cursor, 1)) {
    dateKeys.push(formatDateKey(cursor));
  }

  return (
    <PlanningBoard
      {...props}
      dateKeys={dateKeys}
      droppablePrefix="calendar"
    />
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
  realtimeConnected,
  onRetry,
  onTaskOpen,
  onTaskStatusChange,
  onTaskScheduleChange,
  onSprintFilterChange,
}: ProjectTaskWorkspaceProps) {
  const viewMode = useTaskWorkspaceStore((state) => state.viewMode);
  const filters = useTaskWorkspaceStore((state) => state.filters);
  const selectedTaskId = useTaskWorkspaceStore((state) => state.selectedTaskId);
  const displayOptions = useTaskWorkspaceStore((state) => state.displayOptions);
  const setViewMode = useTaskWorkspaceStore((state) => state.setViewMode);
  const setSearch = useTaskWorkspaceStore((state) => state.setSearch);
  const setStatus = useTaskWorkspaceStore((state) => state.setStatus);
  const setPriority = useTaskWorkspaceStore((state) => state.setPriority);
  const setAssigneeId = useTaskWorkspaceStore((state) => state.setAssigneeId);
  const setSprintId = useTaskWorkspaceStore((state) => state.setSprintId);
  const setPlanning = useTaskWorkspaceStore((state) => state.setPlanning);
  const setDependency = useTaskWorkspaceStore((state) => state.setDependency);
  const setDensity = useTaskWorkspaceStore((state) => state.setDensity);
  const setShowDescriptions = useTaskWorkspaceStore((state) => state.setShowDescriptions);
  const setShowLinkedDocs = useTaskWorkspaceStore((state) => state.setShowLinkedDocs);
  const resetFilters = useTaskWorkspaceStore((state) => state.resetFilters);
  const selectTask = useTaskWorkspaceStore((state) => state.selectTask);
  const definitionsByProject = useCustomFieldStore((state) => state.definitionsByProject);
  const valuesByTask = useCustomFieldStore((state) => state.valuesByTask);
  const docsTree = useDocsStore((state) => state.tree);
  const linksByEntity = useEntityLinkStore((state) => state.linksByEntity);

  const filteredTasks = useMemo(
    () => filterTasksForWorkspace(tasks, filters),
    [tasks, filters]
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
  const activeFilterChips = useMemo(
    () => getActiveFilterChips(filters, tasks, sprints),
    [filters, sprints, tasks]
  );

  const handleTaskOpen = (taskId: string) => {
    selectTask(taskId);
    onTaskOpen(taskId);
  };

  const renderView = (mode: TaskViewMode) => {
    if (loading) {
      return <EmptyState title="Loading tasks" description="Fetching the project task workspace." />;
    }

    if (error) {
      return (
        <Card>
          <CardHeader>
            <CardTitle>Unable to load tasks</CardTitle>
            <CardDescription>{error}</CardDescription>
          </CardHeader>
          <CardContent>
            <Button onClick={onRetry}>Retry</Button>
          </CardContent>
        </Card>
      );
    }

    if (tasks.length === 0) {
      return (
        <EmptyState
          title="No tasks yet"
          description="Create the first task to start using Board, List, Timeline, and Calendar views."
        />
      );
    }

    if (filteredTasks.length === 0) {
      return (
        <EmptyState
          title="No tasks match the current filters"
          description="Adjust search or filters to bring tasks back into view."
        />
      );
    }

    switch (mode) {
      case "list":
        return (
          <ListView
            projectId={projectId}
            tasks={filteredTasks}
            allTasks={tasks}
            selectedTaskId={selectedTaskId}
            density={displayOptions.density}
            showDescriptions={displayOptions.showDescriptions}
            showLinkedDocs={displayOptions.showLinkedDocs}
            customFields={customFields}
            valuesByTask={valuesByTask}
            linkedDocsByTask={linkedDocsByTask}
            onTaskOpen={handleTaskOpen}
          />
        );
      case "timeline":
        return (
          <TimelineView
            tasks={filteredTasks}
            selectedTaskId={selectedTaskId}
            density={displayOptions.density}
            onTaskOpen={handleTaskOpen}
            onTaskScheduleChange={onTaskScheduleChange}
          />
        );
      case "calendar":
        return (
          <CalendarView
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
            selectedTaskId={selectedTaskId}
            displayOptions={displayOptions}
            linkedDocsByTask={linkedDocsByTask}
            onTaskClick={(task) => handleTaskOpen(task.id)}
            onTaskStatusChange={onTaskStatusChange}
          />
        );
    }
  };

  return (
    <Card className="gap-4">
      <CardHeader className="gap-4 px-6">
        <div className="flex flex-col gap-2 lg:flex-row lg:items-center lg:justify-between">
          <div>
            <CardTitle>Task Workspace</CardTitle>
            <CardDescription>
              One project-scoped workspace for Board, List, Timeline, and Calendar views.
            </CardDescription>
          </div>
          <div className="flex flex-wrap items-center gap-3">
            <ViewSwitcher projectId={projectId} />
            <Badge variant={realtimeConnected ? "secondary" : "outline"}>
              {realtimeConnected ? "Realtime live" : "Live alerts paused"}
            </Badge>
            <Tabs value={viewMode} onValueChange={(value) => setViewMode(value as TaskViewMode)}>
              <TabsList>
                <TabsTrigger value="board">Board</TabsTrigger>
                <TabsTrigger value="list">List</TabsTrigger>
                <TabsTrigger value="timeline">Timeline</TabsTrigger>
                <TabsTrigger value="calendar">Calendar</TabsTrigger>
                <TabsTrigger value="dependencies">Dependencies</TabsTrigger>
                <TabsTrigger value="roadmap">Roadmap</TabsTrigger>
              </TabsList>
            </Tabs>
          </div>
        </div>

        <SprintOverview
          sprints={sprints}
          sprintMetrics={sprintMetrics}
          sprintMetricsLoading={sprintMetricsLoading}
        />

        <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-7">
          <label className="flex flex-col gap-2 text-sm font-medium">
            Search tasks
            <Input
              aria-label="Search tasks"
              value={filters.search}
              onChange={(event) => setSearch(event.target.value)}
              placeholder="Search title, description, or assignee"
            />
          </label>

          <label className="flex flex-col gap-2 text-sm font-medium">
            Status
            <select
              className="h-10 rounded-md border bg-background px-3 text-sm"
              value={filters.status}
              onChange={(event) => setStatus(event.target.value as "all" | TaskStatus)}
            >
              {statusOptions().map((status) => (
                <option key={status} value={status}>
                  {status}
                </option>
              ))}
            </select>
          </label>

          <label className="flex flex-col gap-2 text-sm font-medium">
            Priority
            <select
              className="h-10 rounded-md border bg-background px-3 text-sm"
              value={filters.priority}
              onChange={(event) => setPriority(event.target.value as "all" | TaskPriority)}
            >
              {priorityOptions().map((priority) => (
                <option key={priority} value={priority}>
                  {priority}
                </option>
              ))}
            </select>
          </label>

          <label className="flex flex-col gap-2 text-sm font-medium">
            Assignee
            <select
              className="h-10 rounded-md border bg-background px-3 text-sm"
              value={filters.assigneeId}
              onChange={(event) => setAssigneeId(event.target.value)}
            >
              <option value="all">all</option>
              {assigneeOptions(tasks).map((option) => (
                <option key={option.value} value={option.value}>
                  {option.label}
                </option>
              ))}
            </select>
          </label>

          <label className="flex flex-col gap-2 text-sm font-medium">
            Sprint
            <select
              aria-label="Sprint"
              className="h-10 rounded-md border bg-background px-3 text-sm"
              value={filters.sprintId}
              onChange={(event) => {
                const nextValue = event.target.value as string | "all";
                setSprintId(nextValue);
                onSprintFilterChange?.(nextValue);
              }}
            >
              <option value="all">all</option>
              {sprintOptions(sprints).map((option) => (
                <option key={option.value} value={option.value}>
                  {option.label}
                </option>
              ))}
            </select>
          </label>

          <label className="flex flex-col gap-2 text-sm font-medium">
            Planning
            <select
              className="h-10 rounded-md border bg-background px-3 text-sm"
              value={filters.planning}
              onChange={(event) =>
                setPlanning(
                  event.target.value as
                    | "all"
                    | "scheduled"
                    | "unscheduled"
                )
              }
            >
              <option value="all">all</option>
              <option value="scheduled">scheduled</option>
              <option value="unscheduled">unscheduled</option>
            </select>
          </label>

          <label className="flex flex-col gap-2 text-sm font-medium">
            Dependencies
            <select
              aria-label="Dependencies"
              className="h-10 rounded-md border bg-background px-3 text-sm"
              value={filters.dependency}
              onChange={(event) =>
                setDependency(event.target.value as TaskDependencyFilter)
              }
            >
              {dependencyOptions().map((option) => (
                <option key={option.value} value={option.value}>
                  {option.label}
                </option>
              ))}
            </select>
          </label>
        </div>

        <div className="flex items-center gap-2 text-sm text-muted-foreground">
          <span>{filteredTasks.length} visible tasks</span>
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={resetFilters}
          >
            Reset filters
          </Button>
        </div>

        {activeFilterChips.length > 0 ? (
          <div className="flex flex-wrap items-center gap-2 text-sm">
            <span className="text-muted-foreground">Active filters</span>
            {activeFilterChips.map((chip) => (
              <Button
                key={chip.key}
                type="button"
                size="sm"
                variant="outline"
                aria-label={chip.clearLabel}
                onClick={() =>
                  chip.onClear(
                    setSearch,
                    setStatus,
                    setPriority,
                    setAssigneeId,
                    setSprintId,
                    setPlanning,
                    setDependency
                  )
                }
              >
                {chip.label}
              </Button>
            ))}
          </div>
        ) : null}

        <div className="flex flex-wrap items-center gap-2 text-sm">
          <span className="text-muted-foreground">Display</span>
          <Button
            type="button"
            size="sm"
            variant={displayOptions.density === "comfortable" ? "secondary" : "outline"}
            onClick={() => setDensity("comfortable")}
          >
            Comfortable
          </Button>
          <Button
            type="button"
            size="sm"
            variant={displayOptions.density === "compact" ? "secondary" : "outline"}
            onClick={() => setDensity("compact")}
          >
            Compact
          </Button>
          <Button
            type="button"
            size="sm"
            variant="outline"
            onClick={() => setShowDescriptions(!displayOptions.showDescriptions)}
          >
            {displayOptions.showDescriptions ? "Hide descriptions" : "Show descriptions"}
          </Button>
          <Button
            type="button"
            size="sm"
            variant="outline"
            onClick={() => setShowLinkedDocs(!displayOptions.showLinkedDocs)}
          >
            {displayOptions.showLinkedDocs ? "Hide linked docs" : "Show linked docs"}
          </Button>
        </div>
      </CardHeader>

      <CardContent className="px-6">
        <Tabs value={viewMode}>
          <TabsContent value={viewMode}>{renderView(viewMode)}</TabsContent>
        </Tabs>
      </CardContent>
    </Card>
  );
}
