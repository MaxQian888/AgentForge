"use client";

import { useEffect, useState } from "react";
import { Sheet, SheetContent, SheetHeader, SheetTitle } from "@/components/ui/sheet";
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
import {
  useTaskStore,
  type Task,
  type TaskPriority,
  type TaskStatus,
} from "@/lib/stores/task-store";

const statuses: TaskStatus[] = [
  "inbox",
  "triaged",
  "assigned",
  "in_progress",
  "in_review",
  "done",
];

const priorities: TaskPriority[] = ["urgent", "high", "medium", "low"];

interface TaskDetailPanelProps {
  task: Task | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
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

function toPlanningWindow(startDate: string, endDate: string) {
  const normalizedStart = startDate || endDate;
  const normalizedEnd = endDate || startDate;

  return {
    plannedStartAt: normalizedStart
      ? `${normalizedStart}T09:00:00.000Z`
      : "",
    plannedEndAt: normalizedEnd ? `${normalizedEnd}T18:00:00.000Z` : "",
  };
}

export function TaskDetailPanel({
  task,
  open,
  onOpenChange,
}: TaskDetailPanelProps) {
  const updateTask = useTaskStore((state) => state.updateTask);
  const transitionTask = useTaskStore((state) => state.transitionTask);
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [priority, setPriority] = useState<TaskPriority>("medium");
  const [plannedStartDate, setPlannedStartDate] = useState("");
  const [plannedEndDate, setPlannedEndDate] = useState("");

  useEffect(() => {
    setTitle(task?.title ?? "");
    setDescription(task?.description ?? "");
    setPriority(task?.priority ?? "medium");
    setPlannedStartDate(toDateInputValue(task?.plannedStartAt ?? null));
    setPlannedEndDate(toDateInputValue(task?.plannedEndAt ?? null));
  }, [task]);

  if (!task) return null;

  const handleSave = async () => {
    await updateTask(task.id, {
      title,
      description,
      priority,
      ...toPlanningWindow(plannedStartDate, plannedEndDate),
    });
    onOpenChange(false);
  };

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="w-full sm:max-w-md">
        <SheetHeader>
          <SheetTitle>Task Details</SheetTitle>
        </SheetHeader>
        <div className="flex flex-col gap-4 py-4">
          <div className="flex flex-col gap-2">
            <Label>Title</Label>
            <Input
              value={title}
              onChange={(event) => setTitle(event.target.value)}
            />
          </div>

          <div className="flex flex-col gap-2">
            <Label>Description</Label>
            <Input
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
                  void transitionTask(task.id, value as TaskStatus)
                }
              >
                <SelectTrigger>
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
                <SelectTrigger>
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
              <Label>Planned Start</Label>
              <Input
                type="date"
                value={plannedStartDate}
                onChange={(event) => setPlannedStartDate(event.target.value)}
              />
            </div>

            <div className="flex flex-col gap-2">
              <Label>Planned End</Label>
              <Input
                type="date"
                value={plannedEndDate}
                onChange={(event) => setPlannedEndDate(event.target.value)}
              />
            </div>
          </div>

          <Separator />

          <div className="flex flex-wrap gap-2">
            {task.assigneeName ? (
              <Badge variant="outline">Assignee: {task.assigneeName}</Badge>
            ) : null}
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

          <Button onClick={() => void handleSave()}>Save Changes</Button>
        </div>
      </SheetContent>
    </Sheet>
  );
}
