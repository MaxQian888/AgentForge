"use client";

import { useMemo, useState } from "react";
import {
  DragDropContext,
  Draggable,
  Droppable,
  type DropResult,
} from "@hello-pangea/dnd";
import { Board } from "@/components/kanban/board";
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
  getRescheduledPlanningWindow,
  type TaskViewMode,
} from "@/lib/tasks/task-workspace";
import { useTaskWorkspaceStore } from "@/lib/stores/task-workspace-store";
import type { Task, TaskPriority, TaskStatus } from "@/lib/stores/task-store";

interface ProjectTaskWorkspaceProps {
  tasks: Task[];
  loading: boolean;
  error: string | null;
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

function assigneeOptions(tasks: Task[]): Array<{ value: string; label: string }> {
  const seen = new Map<string, string>();
  for (const task of tasks) {
    if (task.assigneeId) {
      seen.set(task.assigneeId, task.assigneeName ?? task.assigneeId);
    }
  }
  return Array.from(seen.entries()).map(([value, label]) => ({ value, label }));
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
  tasks,
  onTaskOpen,
}: {
  tasks: Task[];
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
          <TableHead className="text-right">Action</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {tasks.map((task) => (
          <TableRow key={task.id}>
            <TableCell>
              <div className="font-medium">{task.title}</div>
              <div className="text-xs text-muted-foreground">{task.description}</div>
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
  onTaskOpen,
}: {
  task: Task;
  index: number;
  onTaskOpen: (taskId: string) => void;
}) {
  return (
    <Draggable draggableId={task.id} index={index}>
      {(provided, snapshot) => (
        <button
          ref={provided.innerRef}
          {...provided.draggableProps}
          {...provided.dragHandleProps}
          className={`w-full rounded-md border px-2 py-1 text-left text-sm ${
            snapshot.isDragging ? "bg-accent" : "bg-background"
          }`}
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
  dateKeys,
  droppablePrefix,
  onTaskOpen,
  onTaskScheduleChange,
}: {
  tasks: Task[];
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
  tasks,
  loading,
  error,
  onRetry,
  onTaskOpen,
  onTaskStatusChange,
  onTaskScheduleChange,
}: ProjectTaskWorkspaceProps) {
  const viewMode = useTaskWorkspaceStore((state) => state.viewMode);
  const filters = useTaskWorkspaceStore((state) => state.filters);
  const setViewMode = useTaskWorkspaceStore((state) => state.setViewMode);
  const setSearch = useTaskWorkspaceStore((state) => state.setSearch);
  const setStatus = useTaskWorkspaceStore((state) => state.setStatus);
  const setPriority = useTaskWorkspaceStore((state) => state.setPriority);
  const setAssigneeId = useTaskWorkspaceStore((state) => state.setAssigneeId);
  const setPlanning = useTaskWorkspaceStore((state) => state.setPlanning);
  const resetFilters = useTaskWorkspaceStore((state) => state.resetFilters);
  const selectTask = useTaskWorkspaceStore((state) => state.selectTask);

  const filteredTasks = useMemo(
    () => filterTasksForWorkspace(tasks, filters),
    [tasks, filters]
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
        return <ListView tasks={filteredTasks} onTaskOpen={handleTaskOpen} />;
      case "timeline":
        return (
          <TimelineView
            tasks={filteredTasks}
            onTaskOpen={handleTaskOpen}
            onTaskScheduleChange={onTaskScheduleChange}
          />
        );
      case "calendar":
        return (
          <CalendarView
            tasks={filteredTasks}
            onTaskOpen={handleTaskOpen}
            onTaskScheduleChange={onTaskScheduleChange}
          />
        );
      case "board":
      default:
        return (
          <Board
            tasks={filteredTasks}
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
          <Tabs value={viewMode} onValueChange={(value) => setViewMode(value as TaskViewMode)}>
            <TabsList>
              <TabsTrigger value="board">Board</TabsTrigger>
              <TabsTrigger value="list">List</TabsTrigger>
              <TabsTrigger value="timeline">Timeline</TabsTrigger>
              <TabsTrigger value="calendar">Calendar</TabsTrigger>
            </TabsList>
          </Tabs>
        </div>

        <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-5">
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
      </CardHeader>

      <CardContent className="px-6">
        <Tabs value={viewMode}>
          <TabsContent value={viewMode}>{renderView(viewMode)}</TabsContent>
        </Tabs>
      </CardContent>
    </Card>
  );
}
