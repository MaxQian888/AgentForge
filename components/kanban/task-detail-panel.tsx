"use client";

import { useState } from "react";
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
import { useTaskStore, type Task, type TaskPriority, type TaskStatus } from "@/lib/stores/task-store";

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

export function TaskDetailPanel({
  task,
  open,
  onOpenChange,
}: TaskDetailPanelProps) {
  const updateTask = useTaskStore((s) => s.updateTask);
  const [title, setTitle] = useState(task?.title ?? "");
  const [description, setDescription] = useState(task?.description ?? "");

  if (!task) return null;

  const handleSave = () => {
    updateTask(task.id, { title, description });
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
              onChange={(e) => setTitle(e.target.value)}
            />
          </div>
          <div className="flex flex-col gap-2">
            <Label>Description</Label>
            <Input
              value={description}
              onChange={(e) => setDescription(e.target.value)}
            />
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div className="flex flex-col gap-2">
              <Label>Status</Label>
              <Select
                value={task.status}
                onValueChange={(v) =>
                  updateTask(task.id, { status: v as TaskStatus })
                }
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {statuses.map((s) => (
                    <SelectItem key={s} value={s}>
                      {s.replace("_", " ")}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="flex flex-col gap-2">
              <Label>Priority</Label>
              <Select
                value={task.priority}
                onValueChange={(v) =>
                  updateTask(task.id, { priority: v as TaskPriority })
                }
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {priorities.map((p) => (
                    <SelectItem key={p} value={p}>
                      {p}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>

          <Separator />

          <div className="flex gap-2">
            {task.assigneeName && (
              <Badge variant="outline">Assignee: {task.assigneeName}</Badge>
            )}
            {task.cost != null && (
              <Badge variant="secondary">Cost: ${task.cost.toFixed(2)}</Badge>
            )}
          </div>

          <Button onClick={handleSave}>Save Changes</Button>
        </div>
      </SheetContent>
    </Sheet>
  );
}
