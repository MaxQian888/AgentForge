"use client";

import { Suspense, useEffect, useMemo, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { Plus } from "lucide-react";
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
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { TaskDetailPanel } from "@/components/kanban/task-detail-panel";
import { ProjectTaskWorkspace } from "@/components/tasks/project-task-workspace";
import { useProjectStore } from "@/lib/stores/project-store";
import {
  useTaskStore,
  type TaskPriority,
} from "@/lib/stores/task-store";
import { useTaskWorkspaceStore } from "@/lib/stores/task-workspace-store";

function CreateTaskDialog({ projectId }: { projectId: string }) {
  const createTask = useTaskStore((state) => state.createTask);
  const [open, setOpen] = useState(false);
  const [title, setTitle] = useState("");
  const [priority, setPriority] = useState<TaskPriority>("medium");

  const handleSubmit = async (event: React.FormEvent) => {
    event.preventDefault();
    await createTask({
      projectId,
      title,
      priority,
      description: "",
    });
    setTitle("");
    setPriority("medium");
    setOpen(false);
  };

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button size="sm">
          <Plus className="mr-1 size-4" />
          New Task
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Create Task</DialogTitle>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="flex flex-col gap-4">
          <div className="flex flex-col gap-2">
            <Label>Title</Label>
            <Input
              value={title}
              onChange={(event) => setTitle(event.target.value)}
              required
            />
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
                <SelectItem value="urgent">Urgent</SelectItem>
                <SelectItem value="high">High</SelectItem>
                <SelectItem value="medium">Medium</SelectItem>
                <SelectItem value="low">Low</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <Button type="submit">Create</Button>
        </form>
      </DialogContent>
    </Dialog>
  );
}

function ProjectView() {
  const searchParams = useSearchParams();
  const router = useRouter();
  const projectId = searchParams.get("id");
  const loading = useTaskStore((state) => state.loading);
  const tasks = useTaskStore((state) => state.tasks);
  const fetchTasks = useTaskStore((state) => state.fetchTasks);
  const updateTask = useTaskStore((state) => state.updateTask);
  const transitionTask = useTaskStore((state) => state.transitionTask);
  const selectedTaskId = useTaskWorkspaceStore((state) => state.selectedTaskId);
  const selectTask = useTaskWorkspaceStore((state) => state.selectTask);
  const project = useProjectStore((state) =>
    state.projects.find((item) => item.id === projectId)
  );
  const [panelOpen, setPanelOpen] = useState(false);

  useEffect(() => {
    if (!projectId) {
      router.replace("/projects");
      return;
    }
    void fetchTasks(projectId);
  }, [fetchTasks, projectId, router]);

  const projectTasks = useMemo(
    () => tasks.filter((task) => task.projectId === projectId),
    [projectId, tasks]
  );

  const selectedTask = useMemo(
    () => projectTasks.find((task) => task.id === selectedTaskId) ?? null,
    [projectTasks, selectedTaskId]
  );

  if (!projectId) return null;

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">
          {project?.name ?? "Project"} — Task Workspace
        </h1>
        <CreateTaskDialog projectId={projectId} />
      </div>

      <ProjectTaskWorkspace
        tasks={projectTasks}
        loading={loading}
        error={null}
        onRetry={() => void fetchTasks(projectId)}
        onTaskOpen={(taskId) => {
          selectTask(taskId);
          setPanelOpen(true);
        }}
        onTaskStatusChange={transitionTask}
        onTaskScheduleChange={(taskId, changes) => updateTask(taskId, changes)}
      />

      <TaskDetailPanel
        key={selectedTask?.id ?? "task-detail-panel"}
        task={selectedTask}
        open={panelOpen}
        onOpenChange={(open) => {
          setPanelOpen(open);
          if (!open) {
            selectTask(null);
          }
        }}
      />
    </div>
  );
}

export default function ProjectPage() {
  return (
    <Suspense>
      <ProjectView />
    </Suspense>
  );
}
