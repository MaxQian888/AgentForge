"use client";

import { useState, useMemo } from "react";
import { DragDropContext, type DropResult } from "@hello-pangea/dnd";
import { Column } from "./column";
import { TaskDetailPanel } from "./task-detail-panel";
import { useTaskStore, type Task, type TaskStatus } from "@/lib/stores/task-store";

const columns: TaskStatus[] = [
  "inbox",
  "triaged",
  "assigned",
  "in_progress",
  "in_review",
  "done",
];

interface BoardProps {
  projectId: string;
}

export function Board({ projectId }: BoardProps) {
  const tasks = useTaskStore((s) => s.tasks);
  const transitionTask = useTaskStore((s) => s.transitionTask);
  const [selectedTask, setSelectedTask] = useState<Task | null>(null);
  const [panelOpen, setPanelOpen] = useState(false);

  const projectTasks = useMemo(
    () => tasks.filter((t) => t.projectId === projectId),
    [tasks, projectId]
  );

  const grouped = useMemo(() => {
    const map: Record<TaskStatus, Task[]> = {
      inbox: [],
      triaged: [],
      assigned: [],
      in_progress: [],
      in_review: [],
      done: [],
    };
    for (const task of projectTasks) {
      map[task.status]?.push(task);
    }
    return map;
  }, [projectTasks]);

  const onDragEnd = (result: DropResult) => {
    if (!result.destination) return;
    const newStatus = result.destination.droppableId as TaskStatus;
    if (newStatus !== result.source.droppableId) {
      transitionTask(result.draggableId, newStatus);
    }
  };

  const handleTaskClick = (task: Task) => {
    setSelectedTask(task);
    setPanelOpen(true);
  };

  return (
    <>
      <DragDropContext onDragEnd={onDragEnd}>
        <div className="flex gap-4 overflow-x-auto pb-4">
          {columns.map((status) => (
            <Column
              key={status}
              status={status}
              tasks={grouped[status]}
              onTaskClick={handleTaskClick}
            />
          ))}
        </div>
      </DragDropContext>
      <TaskDetailPanel
        task={selectedTask}
        open={panelOpen}
        onOpenChange={setPanelOpen}
      />
    </>
  );
}
