"use client";

import { Sheet, SheetContent, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { TaskDetailContent } from "@/components/tasks/task-detail-content";
import {
  useTaskStore,
  type Task,
} from "@/lib/stores/task-store";

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
  const updateTask = useTaskStore((state) => state.updateTask);
  const transitionTask = useTaskStore((state) => state.transitionTask);

  if (!task) return null;

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="w-full sm:max-w-md">
        <SheetHeader>
          <SheetTitle>Task Details</SheetTitle>
        </SheetHeader>
        <TaskDetailContent
          key={task.id}
          task={task}
          onTaskSave={async (taskId, data) => {
            await updateTask(taskId, data);
            onOpenChange(false);
          }}
          onTaskStatusChange={transitionTask}
        />
      </SheetContent>
    </Sheet>
  );
}
