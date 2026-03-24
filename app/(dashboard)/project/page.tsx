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
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { ProjectTaskWorkspace } from "@/components/tasks/project-task-workspace";
import { useAgentStore } from "@/lib/stores/agent-store";
import { useMemberStore } from "@/lib/stores/member-store";
import { useProjectStore } from "@/lib/stores/project-store";
import { useNotificationStore } from "@/lib/stores/notification-store";
import { useSprintStore } from "@/lib/stores/sprint-store";
import {
  useTaskStore,
  type TaskPriority,
} from "@/lib/stores/task-store";
import { useTaskWorkspaceStore } from "@/lib/stores/task-workspace-store";
import { useWSStore } from "@/lib/stores/ws-store";

const EMPTY_PROJECT_MEMBERS: ReturnType<typeof useMemberStore.getState>["membersByProject"][string] = [];

function CreateTaskDialog({
  projectId,
  sprints,
}: {
  projectId: string;
  sprints: ReturnType<typeof useSprintStore.getState>["sprintsByProject"][string];
}) {
  const createTask = useTaskStore((state) => state.createTask);
  const [open, setOpen] = useState(false);
  const [title, setTitle] = useState("");
  const [priority, setPriority] = useState<TaskPriority>("medium");
  const [sprintId, setSprintId] = useState("");

  const handleSubmit = async (event: React.FormEvent) => {
    event.preventDefault();
    await createTask({
      projectId,
      title,
      priority,
      description: "",
      sprintId: sprintId || null,
    });
    setTitle("");
    setPriority("medium");
    setSprintId("");
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
          <DialogDescription>
            Capture the task goal and initial priority before the workspace fills in the rest.
          </DialogDescription>
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
          <div className="flex flex-col gap-2">
            <Label htmlFor="create-task-sprint">Sprint</Label>
            <select
              id="create-task-sprint"
              className="h-10 rounded-md border bg-background px-3 text-sm"
              value={sprintId}
              onChange={(event) => setSprintId(event.target.value)}
            >
              <option value="">Backlog / no sprint</option>
              {sprints.map((sprint) => (
                <option key={sprint.id} value={sprint.id}>
                  {sprint.name}
                </option>
              ))}
            </select>
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
  const error = useTaskStore((state) => state.error);
  const tasks = useTaskStore((state) => state.tasks);
  const fetchTasks = useTaskStore((state) => state.fetchTasks);
  const updateTask = useTaskStore((state) => state.updateTask);
  const transitionTask = useTaskStore((state) => state.transitionTask);
  const assignTask = useTaskStore((state) => state.assignTask);
  const decomposeTask = useTaskStore((state) => state.decomposeTask);
  const agents = useAgentStore((state) => state.agents);
  const fetchAgents = useAgentStore((state) => state.fetchAgents);
  const membersByProject = useMemberStore((state) => state.membersByProject);
  const fetchMembers = useMemberStore((state) => state.fetchMembers);
  const notifications = useNotificationStore((state) => state.notifications);
  const realtimeConnected = useWSStore((state) => state.connected);
  const sprintFilterId = useTaskWorkspaceStore((state) => state.filters.sprintId);
  const sprintsByProject = useSprintStore((state) => state.sprintsByProject);
  const metricsBySprintId = useSprintStore((state) => state.metricsBySprintId);
  const metricsLoadingBySprintId = useSprintStore(
    (state) => state.metricsLoadingBySprintId
  );
  const fetchSprints = useSprintStore((state) => state.fetchSprints);
  const fetchSprintMetrics = useSprintStore((state) => state.fetchSprintMetrics);
  const project = useProjectStore((state) =>
    state.projects.find((item) => item.id === projectId)
  );

  useEffect(() => {
    if (!projectId) {
      router.replace("/projects");
      return;
    }
    void fetchTasks(projectId);
    void fetchMembers(projectId);
    void fetchAgents();
    void fetchSprints(projectId);
  }, [fetchAgents, fetchMembers, fetchSprints, fetchTasks, projectId, router]);

  const projectTasks = useMemo(
    () => tasks.filter((task) => task.projectId === projectId),
    [projectId, tasks]
  );
  const members = useMemo(
    () => (projectId ? membersByProject[projectId] ?? EMPTY_PROJECT_MEMBERS : EMPTY_PROJECT_MEMBERS),
    [membersByProject, projectId]
  );
  const sprints = useMemo(
    () => (projectId ? sprintsByProject[projectId] ?? [] : []),
    [projectId, sprintsByProject]
  );
  const projectTaskIds = useMemo(
    () => new Set(projectTasks.map((task) => task.id)),
    [projectTasks]
  );
  const projectAgents = useMemo(
    () => agents.filter((agent) => projectTaskIds.has(agent.taskId)),
    [agents, projectTaskIds]
  );
  const handleTaskAssign = async (
    taskId: string,
    assigneeId: string,
    assigneeType: "human" | "agent"
  ) => {
    const member = members.find((item) => item.id === assigneeId);
    await assignTask(taskId, assigneeId, assigneeType, member?.name);
  };
  const metricsSprintId = useMemo(() => {
    if (sprintFilterId !== "all" && sprints.some((sprint) => sprint.id === sprintFilterId)) {
      return sprintFilterId;
    }
    return sprints.find((sprint) => sprint.status === "active")?.id ?? sprints[0]?.id ?? null;
  }, [sprintFilterId, sprints]);
  const sprintMetrics = metricsSprintId ? metricsBySprintId[metricsSprintId] ?? null : null;
  const sprintMetricsLoading = metricsSprintId
    ? metricsLoadingBySprintId[metricsSprintId] ?? false
    : false;

  useEffect(() => {
    if (!projectId || !metricsSprintId) {
      return;
    }
    void fetchSprintMetrics(projectId, metricsSprintId);
  }, [fetchSprintMetrics, metricsSprintId, projectId]);

  if (!projectId) return null;

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">
          {project?.name ?? "Project"} — Task Workspace
        </h1>
        <CreateTaskDialog projectId={projectId} sprints={sprints} />
      </div>

      <ProjectTaskWorkspace
        projectId={projectId}
        tasks={projectTasks}
        sprints={sprints}
        sprintMetrics={sprintMetrics}
        sprintMetricsLoading={sprintMetricsLoading}
        loading={loading}
        error={error}
        realtimeConnected={realtimeConnected}
        notifications={notifications}
        members={members}
        agents={projectAgents}
        onRetry={() => void fetchTasks(projectId)}
        onTaskOpen={() => undefined}
        onTaskStatusChange={transitionTask}
        onTaskScheduleChange={(taskId, changes) => updateTask(taskId, changes)}
        onTaskSave={updateTask}
        onTaskAssign={handleTaskAssign}
        onTaskDecompose={decomposeTask}
        onSprintFilterChange={(nextSprintId) => {
          if (nextSprintId === "all") {
            return;
          }
          void fetchSprintMetrics(projectId, nextSprintId);
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
