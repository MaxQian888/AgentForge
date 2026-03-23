"use client";

import { Suspense } from "react";
import { useSearchParams } from "next/navigation";
import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { Plus } from "lucide-react";
import { useState } from "react";
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
import { Board } from "@/components/kanban/board";
import {
  useTaskStore,
  type TaskPriority,
} from "@/lib/stores/task-store";
import { useProjectStore } from "@/lib/stores/project-store";

function CreateTaskDialog({ projectId }: { projectId: string }) {
  const createTask = useTaskStore((s) => s.createTask);
  const [open, setOpen] = useState(false);
  const [title, setTitle] = useState("");
  const [priority, setPriority] = useState<TaskPriority>("medium");

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    await createTask({
      projectId,
      title,
      priority,
      status: "inbox",
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
              onChange={(e) => setTitle(e.target.value)}
              required
            />
          </div>
          <div className="flex flex-col gap-2">
            <Label>Priority</Label>
            <Select
              value={priority}
              onValueChange={(v) => setPriority(v as TaskPriority)}
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
  const fetchTasks = useTaskStore((s) => s.fetchTasks);
  const project = useProjectStore((s) =>
    s.projects.find((p) => p.id === projectId)
  );

  useEffect(() => {
    if (!projectId) {
      router.replace("/projects");
      return;
    }
    fetchTasks(projectId);
  }, [projectId, fetchTasks, router]);

  if (!projectId) return null;

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">
          {project?.name ?? "Project"} — Board
        </h1>
        <CreateTaskDialog projectId={projectId} />
      </div>
      <Board projectId={projectId} />
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
