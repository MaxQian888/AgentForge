"use client";

import { useEffect, useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Separator } from "@/components/ui/separator";
import { normalizePlanningInput } from "@/lib/tasks/task-planning";
import type { Task, TaskPriority, TaskStatus } from "@/lib/stores/task-store";

const statuses: TaskStatus[] = [
  "inbox",
  "triaged",
  "assigned",
  "in_progress",
  "in_review",
  "done",
];

const priorities: TaskPriority[] = ["urgent", "high", "medium", "low"];

interface TaskDraft {
  title: string;
  description: string;
  priority: TaskPriority;
  plannedStartDate: string;
  plannedEndDate: string;
}

export interface TaskDetailContentProps {
  task: Task;
  onTaskSave?: (taskId: string, data: Partial<Task>) => Promise<void> | void;
  onTaskStatusChange?: (
    taskId: string,
    status: TaskStatus
  ) => Promise<void> | void;
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

function toDateInputValue(value: string | null): string {
  return value ? value.slice(0, 10) : "";
}

function getTaskDraft(task: Task): TaskDraft {
  return {
    title: task.title,
    description: task.description,
    priority: task.priority,
    plannedStartDate: toDateInputValue(task.plannedStartAt),
    plannedEndDate: toDateInputValue(task.plannedEndAt),
  };
}

export function TaskDetailContent({
  task,
  onTaskSave,
  onTaskStatusChange,
}: TaskDetailContentProps) {
  const initialDraft = getTaskDraft(task);
  const [title, setTitle] = useState(initialDraft.title);
  const [description, setDescription] = useState(initialDraft.description);
  const [priority, setPriority] = useState<TaskPriority>(initialDraft.priority);
  const [plannedStartDate, setPlannedStartDate] = useState(initialDraft.plannedStartDate);
  const [plannedEndDate, setPlannedEndDate] = useState(initialDraft.plannedEndDate);
  const [planningError, setPlanningError] = useState<string | null>(null);

  useEffect(() => {
    const nextDraft = getTaskDraft(task);
    setTitle(nextDraft.title);
    setDescription(nextDraft.description);
    setPriority(nextDraft.priority);
    setPlannedStartDate(nextDraft.plannedStartDate);
    setPlannedEndDate(nextDraft.plannedEndDate);
    setPlanningError(null);
  }, [task]);

  const handleSave = async () => {
    const planning = normalizePlanningInput({
      startDate: plannedStartDate,
      endDate: plannedEndDate,
    });

    if (planning.kind === "invalid") {
      setPlanningError("End date cannot be earlier than start date.");
      return;
    }

    setPlanningError(null);
    await onTaskSave?.(task.id, {
      title,
      description,
      priority,
      ...(planning.kind === "scheduled"
        ? {
            plannedStartAt: planning.plannedStartAt,
            plannedEndAt: planning.plannedEndAt,
          }
        : {
            plannedStartAt: null,
            plannedEndAt: null,
          }),
    });
  };

  const titleId = `task-detail-title-${task.id}`;
  const descriptionId = `task-detail-description-${task.id}`;
  const startDateId = `task-detail-start-${task.id}`;
  const endDateId = `task-detail-end-${task.id}`;

  return (
    <div className="flex flex-col gap-4 py-4">
      <div className="flex flex-col gap-2">
        <Label htmlFor={titleId}>Title</Label>
        <Input
          id={titleId}
          value={title}
          onChange={(event) => setTitle(event.target.value)}
        />
      </div>

      <div className="flex flex-col gap-2">
        <Label htmlFor={descriptionId}>Description</Label>
        <Input
          id={descriptionId}
          value={description}
          onChange={(event) => setDescription(event.target.value)}
        />
      </div>

      <div className="grid grid-cols-2 gap-4">
        <div className="flex flex-col gap-2">
          <Label>Status</Label>
          <Select
            value={task.status}
            onValueChange={(value) =>
              void onTaskStatusChange?.(task.id, value as TaskStatus)
            }
          >
            <SelectTrigger aria-label="Status">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {statuses.map((status) => (
                <SelectItem key={status} value={status}>
                  {status.replace("_", " ")}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        <div className="flex flex-col gap-2">
          <Label>Priority</Label>
          <Select
            value={priority}
            onValueChange={(value) => setPriority(value as TaskPriority)}
          >
            <SelectTrigger aria-label="Priority">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {priorities.map((item) => (
                <SelectItem key={item} value={item}>
                  {item}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </div>

      <div className="grid grid-cols-2 gap-4">
        <div className="flex flex-col gap-2">
          <Label htmlFor={startDateId}>Planned Start</Label>
          <Input
            id={startDateId}
            type="date"
            value={plannedStartDate}
            aria-invalid={planningError ? true : undefined}
            onChange={(event) => {
              setPlannedStartDate(event.target.value);
              setPlanningError(null);
            }}
          />
        </div>

        <div className="flex flex-col gap-2">
          <Label htmlFor={endDateId}>Planned End</Label>
          <Input
            id={endDateId}
            type="date"
            value={plannedEndDate}
            aria-invalid={planningError ? true : undefined}
            onChange={(event) => {
              setPlannedEndDate(event.target.value);
              setPlanningError(null);
            }}
          />
        </div>
      </div>

      {planningError ? (
        <div className="text-sm text-destructive">{planningError}</div>
      ) : null}

      <Separator />

      <div className="flex flex-wrap gap-2">
        <Badge variant="outline">
          Assignee: {task.assigneeName ?? "Unassigned"}
        </Badge>
        {task.cost != null ? (
          <Badge variant="secondary">Cost: ${task.cost.toFixed(2)}</Badge>
        ) : null}
        <Badge variant="secondary">
          {task.plannedStartAt && task.plannedEndAt
            ? `${task.plannedStartAt.slice(0, 10)} → ${task.plannedEndAt.slice(0, 10)}`
            : "Unscheduled"}
        </Badge>
        {formatProgressHealth(task) ? (
          <Badge variant="secondary">{formatProgressHealth(task)}</Badge>
        ) : null}
        {formatProgressReason(task) ? (
          <Badge variant="outline">{formatProgressReason(task)}</Badge>
        ) : null}
      </div>

      {task.progress ? (
        <div className="rounded-lg border border-border/60 bg-muted/20 p-3 text-sm">
          <div className="font-medium">Progress Signal</div>
          <div className="mt-2 text-muted-foreground">
            Last activity: {task.progress.lastActivityAt.slice(0, 16).replace("T", " ")}
          </div>
          <div className="text-muted-foreground">
            Source: {task.progress.lastActivitySource}
          </div>
          {formatProgressReason(task) ? (
            <div className="text-muted-foreground">
              Reason: {formatProgressReason(task)}
            </div>
          ) : null}
        </div>
      ) : null}

      <Button
        type="button"
        disabled={!onTaskSave}
        onClick={() => void handleSave()}
      >
        Save Changes
      </Button>
    </div>
  );
}
