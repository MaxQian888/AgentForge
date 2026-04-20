"use client";

import { Suspense, useEffect, useMemo, useRef, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { useTranslations } from "next-intl";
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
import { useBreadcrumbs } from "@/hooks/use-breadcrumbs";

const EMPTY_PROJECT_MEMBERS: ReturnType<typeof useMemberStore.getState>["membersByProject"][string] = [];

function CreateTaskDialog({
  projectId,
  sprints,
  tasks,
  open,
  onOpenChange,
}: {
  projectId: string;
  sprints: ReturnType<typeof useSprintStore.getState>["sprintsByProject"][string];
  tasks: import("@/lib/stores/task-store").Task[];
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
}) {
  const t = useTranslations("projects");
  const createTask = useTaskStore((state) => state.createTask);
  const [internalOpen, setInternalOpen] = useState(false);
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [priority, setPriority] = useState<TaskPriority>("medium");
  const [sprintId, setSprintId] = useState("");
  const [labelsInput, setLabelsInput] = useState("");
  const [budgetUsd, setBudgetUsd] = useState("");
  const [plannedStart, setPlannedStart] = useState("");
  const [plannedEnd, setPlannedEnd] = useState("");
  const [parentId, setParentId] = useState("");
  const dialogOpen = open ?? internalOpen;
  const setDialogOpen = onOpenChange ?? setInternalOpen;

  const resetForm = () => {
    setTitle("");
    setDescription("");
    setPriority("medium");
    setSprintId("");
    setLabelsInput("");
    setBudgetUsd("");
    setPlannedStart("");
    setPlannedEnd("");
    setParentId("");
  };

  const handleSubmit = async (event: React.FormEvent) => {
    event.preventDefault();
    const labels = labelsInput
      .split(",")
      .map((l) => l.trim())
      .filter(Boolean);
    await createTask({
      projectId,
      title,
      description,
      priority,
      sprintId: sprintId || null,
      parentId: parentId || null,
      labels,
      budgetUsd: budgetUsd ? parseFloat(budgetUsd) : 0,
      plannedStartAt: plannedStart ? `${plannedStart}T09:00:00.000Z` : null,
      plannedEndAt: plannedEnd ? `${plannedEnd}T18:00:00.000Z` : null,
    });
    resetForm();
    setDialogOpen(false);
  };

  const topLevelTasks = tasks.filter(
    (task) => task.projectId === projectId && !task.parentId
  );

  return (
    <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
      <DialogTrigger asChild>
        <Button size="sm">
          <Plus className="mr-1 size-4" />
          {t("createTask.button")}
        </Button>
      </DialogTrigger>
      <DialogContent className="max-h-[85vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{t("createTask.title")}</DialogTitle>
          <DialogDescription>
            {t("createTask.description")}
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="flex flex-col gap-4">
          <div className="flex flex-col gap-2">
            <Label>{t("createTask.titleLabel")}</Label>
            <Input
              value={title}
              onChange={(event) => setTitle(event.target.value)}
              required
            />
          </div>
          <div className="flex flex-col gap-2">
            <Label>{t("createTask.descriptionLabel")}</Label>
            <textarea
              className="min-h-[80px] rounded-md border bg-background px-3 py-2 text-sm"
              value={description}
              placeholder={t("createTask.descriptionPlaceholder")}
              onChange={(event) => setDescription(event.target.value)}
            />
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div className="flex flex-col gap-2">
              <Label>{t("createTask.priorityLabel")}</Label>
              <Select
                value={priority}
                onValueChange={(value) => setPriority(value as TaskPriority)}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="urgent">{t("createTask.priority.urgent")}</SelectItem>
                  <SelectItem value="high">{t("createTask.priority.high")}</SelectItem>
                  <SelectItem value="medium">{t("createTask.priority.medium")}</SelectItem>
                  <SelectItem value="low">{t("createTask.priority.low")}</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor="create-task-sprint">{t("createTask.sprintLabel")}</Label>
              <select
                id="create-task-sprint"
                className="h-10 rounded-md border bg-background px-3 text-sm"
                value={sprintId}
                onChange={(event) => setSprintId(event.target.value)}
              >
                <option value="">{t("createTask.backlog")}</option>
                {sprints.map((sprint) => (
                  <option key={sprint.id} value={sprint.id}>
                    {sprint.name}
                  </option>
                ))}
              </select>
            </div>
          </div>
          <div className="flex flex-col gap-2">
            <Label>{t("createTask.labelsLabel")}</Label>
            <Input
              value={labelsInput}
              placeholder={t("createTask.labelsPlaceholder")}
              onChange={(event) => setLabelsInput(event.target.value)}
            />
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div className="flex flex-col gap-2">
              <Label>{t("createTask.budgetLabel")}</Label>
              <Input
                type="number"
                min="0"
                step="0.01"
                value={budgetUsd}
                placeholder={t("createTask.budgetPlaceholder")}
                onChange={(event) => setBudgetUsd(event.target.value)}
              />
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor="create-task-parent">{t("createTask.parentTaskLabel")}</Label>
              <select
                id="create-task-parent"
                className="h-10 rounded-md border bg-background px-3 text-sm"
                value={parentId}
                onChange={(event) => setParentId(event.target.value)}
              >
                <option value="">{t("createTask.noParent")}</option>
                {topLevelTasks.map((task) => (
                  <option key={task.id} value={task.id}>
                    {task.title}
                  </option>
                ))}
              </select>
            </div>
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div className="flex flex-col gap-2">
              <Label>{t("createTask.plannedStartLabel")}</Label>
              <Input
                type="date"
                value={plannedStart}
                onChange={(event) => setPlannedStart(event.target.value)}
              />
            </div>
            <div className="flex flex-col gap-2">
              <Label>{t("createTask.plannedEndLabel")}</Label>
              <Input
                type="date"
                value={plannedEnd}
                onChange={(event) => setPlannedEnd(event.target.value)}
              />
            </div>
          </div>
          <Button type="submit">{t("createTask.submit")}</Button>
        </form>
      </DialogContent>
    </Dialog>
  );
}

function ProjectView() {
  useBreadcrumbs([{ label: "Projects", href: "/projects" }, { label: "Tasks" }]);
  const t = useTranslations("projects");
  const searchParams = useSearchParams();
  const router = useRouter();
  const projectId = searchParams.get("id");
  const requestedMemberId = searchParams.get("member");
  const requestedSprintId = searchParams.get("sprint");
  const requestedAction = searchParams.get("action");
  const loading = useTaskStore((state) => state.loading);
  const error = useTaskStore((state) => state.error);
  const tasks = useTaskStore((state) => state.tasks);
  const fetchTasks = useTaskStore((state) => state.fetchTasks);
  const updateTask = useTaskStore((state) => state.updateTask);
  const transitionTask = useTaskStore((state) => state.transitionTask);
  const assignTask = useTaskStore((state) => state.assignTask);
  const decomposeTask = useTaskStore((state) => state.decomposeTask);
  const deleteTask = useTaskStore((state) => state.deleteTask);
  const agents = useAgentStore((state) => state.agents);
  const spawnAgent = useAgentStore((state) => state.spawnAgent);
  const fetchAgents = useAgentStore((state) => state.fetchAgents);
  const membersByProject = useMemberStore((state) => state.membersByProject);
  const fetchMembers = useMemberStore((state) => state.fetchMembers);
  const notifications = useNotificationStore((state) => state.notifications);
  const realtimeConnected = useWSStore((state) => state.connected);
  const sprintFilterId = useTaskWorkspaceStore((state) => state.filters.sprintId);
  const setAssigneeId = useTaskWorkspaceStore((state) => state.setAssigneeId);
  const setSprintId = useTaskWorkspaceStore((state) => state.setSprintId);
  const sprintsByProject = useSprintStore((state) => state.sprintsByProject);
  const metricsBySprintId = useSprintStore((state) => state.metricsBySprintId);
  const metricsLoadingBySprintId = useSprintStore(
    (state) => state.metricsLoadingBySprintId
  );
  const fetchSprints = useSprintStore((state) => state.fetchSprints);
  const fetchSprintMetrics = useSprintStore((state) => state.fetchSprintMetrics);
  const createTaskActionSeed = requestedAction === "create-task" && projectId ? projectId : null;
  const [manualCreateTaskDialogOpen, setManualCreateTaskDialogOpen] = useState(false);
  const [dismissedCreateTaskActionSeed, setDismissedCreateTaskActionSeed] = useState<string | null>(
    null
  );
  const seededSprintScopeRef = useRef<string | null>(null);
  const project = useProjectStore((state) =>
    state.projects.find((item) => item.id === projectId)
  );
  const createTaskDialogOpen =
    manualCreateTaskDialogOpen ||
    (createTaskActionSeed !== null && dismissedCreateTaskActionSeed !== createTaskActionSeed);
  const handleCreateTaskDialogOpenChange = (open: boolean) => {
    setManualCreateTaskDialogOpen(open);
    if (!open && createTaskActionSeed) {
      setDismissedCreateTaskActionSeed(createTaskActionSeed);
    }
  };

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
  const hasLoadedProjectSprintScope = useMemo(
    () => (projectId ? Object.prototype.hasOwnProperty.call(sprintsByProject, projectId) : false),
    [projectId, sprintsByProject],
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

  useEffect(() => {
    setAssigneeId(requestedMemberId ?? "all");
  }, [requestedMemberId, setAssigneeId, projectId]);

  useEffect(() => {
    if (!projectId) {
      seededSprintScopeRef.current = null;
      return;
    }

    const seedKey = `${projectId}:${requestedSprintId ?? "__none__"}`;
    if (seededSprintScopeRef.current === seedKey) {
      return;
    }

    if (!requestedSprintId) {
      setSprintId("all");
      seededSprintScopeRef.current = seedKey;
      return;
    }

    if (!hasLoadedProjectSprintScope) {
      return;
    }

    if (sprints.some((sprint) => sprint.id === requestedSprintId)) {
      setSprintId(requestedSprintId);
    } else {
      setSprintId("all");
    }
    seededSprintScopeRef.current = seedKey;
  }, [
    hasLoadedProjectSprintScope,
    projectId,
    requestedSprintId,
    setSprintId,
    sprints,
  ]);

  if (!projectId) return null;

  return (
    <div className="-m-[var(--space-page-inline)] h-[calc(100vh-var(--header-height))]">
      <CreateTaskDialog
        projectId={projectId}
        sprints={sprints}
        tasks={projectTasks}
        open={createTaskDialogOpen}
        onOpenChange={handleCreateTaskDialogOpenChange}
      />
      <ProjectTaskWorkspace
        projectId={projectId}
        projectName={project?.name ?? t("taskWorkspace.defaultName")}
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
        onCreateTask={() => setManualCreateTaskDialogOpen(true)}
        onTaskAssign={handleTaskAssign}
        onBulkStatusChange={async (ids, status) => {
          const failed = (
            await Promise.all(
              ids.map(async (id) => {
                try {
                  await transitionTask(id, status);
                  return null;
                } catch (error) {
                  return {
                    taskId: id,
                    message:
                      error instanceof Error
                        ? error.message
                        : "Failed to change task status.",
                  };
                }
              })
            )
          ).filter((item): item is { taskId: string; message: string } => item !== null);

          return { failed };
        }}
        onBulkAssign={async (ids, assigneeId, assigneeType) => {
          const failed = (
            await Promise.all(
              ids.map(async (id) => {
                try {
                  await assignTask(id, assigneeId, assigneeType);
                  return null;
                } catch (error) {
                  return {
                    taskId: id,
                    message:
                      error instanceof Error
                        ? error.message
                        : "Failed to assign task.",
                  };
                }
              })
            )
          ).filter((item): item is { taskId: string; message: string } => item !== null);

          return { failed };
        }}
        onBulkDelete={async (ids) => {
          const failed = (
            await Promise.all(
              ids.map(async (id) => {
                try {
                  await deleteTask(id);
                  return null;
                } catch (error) {
                  return {
                    taskId: id,
                    message:
                      error instanceof Error
                        ? error.message
                        : "Failed to delete task.",
                  };
                }
              })
            )
          ).filter((item): item is { taskId: string; message: string } => item !== null);

          return { failed };
        }}
        onTaskDecompose={decomposeTask}
        onTaskDelete={async (taskId) => {
          await deleteTask(taskId);
          useTaskWorkspaceStore.getState().selectTask(null);
        }}
        onSpawnAgent={spawnAgent}
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
