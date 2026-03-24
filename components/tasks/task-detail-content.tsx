"use client";

import { useEffect, useMemo, useState } from "react";
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
import { TaskReviewSection } from "@/components/review/task-review-section";
import { recommendTaskAssignees } from "@/lib/tasks/task-assignment";
import { getTaskDependencyState } from "@/lib/tasks/task-dependencies";
import { normalizePlanningInput } from "@/lib/tasks/task-planning";
import type { TeamMember } from "@/lib/dashboard/summary";
import type { Agent } from "@/lib/stores/agent-store";
import type { Sprint } from "@/lib/stores/sprint-store";
import type {
  Task,
  TaskDecompositionResult,
  TaskPriority,
  TaskStatus,
} from "@/lib/stores/task-store";

const statuses: TaskStatus[] = [
  "inbox",
  "triaged",
  "assigned",
  "in_progress",
  "blocked",
  "in_review",
  "changes_requested",
  "done",
  "cancelled",
  "budget_exceeded",
];

const priorities: TaskPriority[] = ["urgent", "high", "medium", "low"];

interface TaskDraft {
  title: string;
  description: string;
  priority: TaskPriority;
  sprintId: string;
  plannedStartDate: string;
  plannedEndDate: string;
  blockedBy: string[];
}

export interface TaskDetailContentProps {
  task: Task;
  tasks: Task[];
  members: TeamMember[];
  agents: Agent[];
  sprints?: Sprint[];
  onTaskSave?: (taskId: string, data: Partial<Task>) => Promise<void> | void;
  onTaskAssign?: (
    taskId: string,
    assigneeId: string,
    assigneeType: "human" | "agent"
  ) => Promise<void> | void;
  onTaskStatusChange?: (
    taskId: string,
    status: TaskStatus
  ) => Promise<void> | void;
  onTaskDecompose?: (
    taskId: string
  ) => Promise<TaskDecompositionResult | null> | TaskDecompositionResult | null | void;
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

function formatExecutionMode(mode: Task["executionMode"]): string | null {
  switch (mode) {
    case "agent":
      return "Agent-ready";
    case "human":
      return "Human-led";
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
    sprintId: task.sprintId ?? "",
    plannedStartDate: toDateInputValue(task.plannedStartAt),
    plannedEndDate: toDateInputValue(task.plannedEndAt),
    blockedBy: [...task.blockedBy],
  };
}

export function TaskDetailContent({
  task,
  tasks,
  members,
  agents,
  sprints = [],
  onTaskSave,
  onTaskAssign,
  onTaskStatusChange,
  onTaskDecompose,
}: TaskDetailContentProps) {
  const initialDraft = getTaskDraft(task);
  const [title, setTitle] = useState(initialDraft.title);
  const [description, setDescription] = useState(initialDraft.description);
  const [priority, setPriority] = useState<TaskPriority>(initialDraft.priority);
  const [sprintId, setSprintId] = useState(initialDraft.sprintId);
  const [plannedStartDate, setPlannedStartDate] = useState(initialDraft.plannedStartDate);
  const [plannedEndDate, setPlannedEndDate] = useState(initialDraft.plannedEndDate);
  const [blockedBy, setBlockedBy] = useState<string[]>(initialDraft.blockedBy);
  const [planningError, setPlanningError] = useState<string | null>(null);
  const [decompositionSummary, setDecompositionSummary] = useState<string | null>(null);
  const [decompositionError, setDecompositionError] = useState<string | null>(null);
  const [generatedSubtasks, setGeneratedSubtasks] = useState<Task[]>([]);
  const [isDecomposing, setIsDecomposing] = useState(false);
  const recommendations = useMemo(
    () => recommendTaskAssignees(task, members, tasks, agents),
    [agents, members, task, tasks]
  );
  const dependencyState = useMemo(
    () => getTaskDependencyState(task, tasks),
    [task, tasks]
  );
  const activeMembers = useMemo(
    () => members.filter((member) => member.isActive),
    [members]
  );
  const dependencyOptions = useMemo(
    () =>
      tasks
        .filter((candidate) => candidate.projectId === task.projectId && candidate.id !== task.id)
        .sort((left, right) => left.title.localeCompare(right.title)),
    [task.id, task.projectId, tasks]
  );
  const childTasks = useMemo(
    () =>
      tasks
        .filter((candidate) => candidate.projectId === task.projectId && candidate.parentId === task.id)
        .sort((left, right) => left.createdAt.localeCompare(right.createdAt)),
    [task.id, task.projectId, tasks]
  );
  const visibleChildTasks = useMemo(() => {
    if (generatedSubtasks.length === 0) {
      return childTasks;
    }

    const byId = new Map<string, Task>();
    for (const childTask of childTasks) {
      byId.set(childTask.id, childTask);
    }
    for (const childTask of generatedSubtasks) {
      byId.set(childTask.id, childTask);
    }
    return [...byId.values()].sort((left, right) => left.createdAt.localeCompare(right.createdAt));
  }, [childTasks, generatedSubtasks]);
  const [manualAssigneeId, setManualAssigneeId] = useState(
    task.assigneeId ?? recommendations[0]?.member.id ?? ""
  );
  const [assigningMemberId, setAssigningMemberId] = useState<string | null>(null);
  const budgetRatio =
    task.budgetUsd > 0 ? Math.round((task.spentUsd / task.budgetUsd) * 100) : null;

  useEffect(() => {
    setManualAssigneeId(task.assigneeId ?? recommendations[0]?.member.id ?? "");
  }, [recommendations, task.assigneeId, task.id]);

  useEffect(() => {
    setBlockedBy([...task.blockedBy]);
  }, [task.blockedBy, task.id]);

  useEffect(() => {
    setSprintId(task.sprintId ?? "");
  }, [task.id, task.sprintId]);

  useEffect(() => {
    setDecompositionSummary(null);
    setDecompositionError(null);
    setGeneratedSubtasks([]);
    setIsDecomposing(false);
  }, [task.id]);

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
      sprintId: sprintId || null,
      blockedBy,
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

  const handleAssign = async (memberId: string) => {
    const member = activeMembers.find((item) => item.id === memberId);
    if (!member || !onTaskAssign) {
      return;
    }

    setAssigningMemberId(member.id);
    try {
      await onTaskAssign(task.id, member.id, member.type);
      setManualAssigneeId(member.id);
    } finally {
      setAssigningMemberId(null);
    }
  };

  const handleDecompose = async () => {
    if (!onTaskDecompose) {
      return;
    }

    setIsDecomposing(true);
    setDecompositionError(null);
    try {
      const result = await onTaskDecompose(task.id);
      setDecompositionSummary(result?.summary ?? "AI decomposition completed.");
      setGeneratedSubtasks(result?.subtasks ?? []);
    } catch (error) {
      setDecompositionError(
        error instanceof Error ? error.message : "Unable to decompose this task."
      );
    } finally {
      setIsDecomposing(false);
    }
  };

  const titleId = `task-detail-title-${task.id}`;
  const descriptionId = `task-detail-description-${task.id}`;
  const startDateId = `task-detail-start-${task.id}`;
  const endDateId = `task-detail-end-${task.id}`;
  const manualAssignId = `task-detail-assign-${task.id}`;

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

      <div className="flex flex-col gap-2">
        <Label htmlFor={`task-detail-sprint-${task.id}`}>Sprint</Label>
        <select
          id={`task-detail-sprint-${task.id}`}
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

      <div className="flex flex-col gap-3 rounded-lg border border-border/60 bg-muted/20 p-3 text-sm">
        <div>
          <div className="font-medium">Dependencies</div>
          <div className="text-muted-foreground">
            Select tasks that must finish before this task can move forward.
          </div>
        </div>
        {dependencyOptions.length > 0 ? (
          <div className="space-y-2">
            {dependencyOptions.map((candidate) => {
              const inputId = `task-dependency-${task.id}-${candidate.id}`;
              const checked = blockedBy.includes(candidate.id);

              return (
                <label
                  key={candidate.id}
                  htmlFor={inputId}
                  className="flex cursor-pointer items-center justify-between gap-3 rounded-md border border-border/60 bg-background px-3 py-2"
                >
                  <div className="flex min-w-0 items-center gap-3">
                    <input
                      id={inputId}
                      type="checkbox"
                      checked={checked}
                      onChange={(event) => {
                        setBlockedBy((current) =>
                          event.target.checked
                            ? [...current, candidate.id]
                            : current.filter((value) => value !== candidate.id)
                        );
                      }}
                    />
                    <span className="truncate">{candidate.title}</span>
                  </div>
                  <Badge variant={candidate.status === "done" ? "secondary" : "outline"}>
                    {candidate.status === "done" ? "Done" : candidate.status}
                  </Badge>
                </label>
              );
            })}
          </div>
        ) : (
          <div className="text-muted-foreground">
            No other project tasks are available to use as blockers yet.
          </div>
        )}
      </div>

      <Separator />

      <div className="flex flex-col gap-3">
        <div>
          <div className="font-medium">Smart assignment</div>
          <div className="text-sm text-muted-foreground">
            Recommend assignees from member skills and current workload.
          </div>
        </div>

        {recommendations.length > 0 ? (
          <div className="grid gap-3">
            {recommendations.map((recommendation, index) => {
              const isCurrentAssignee = task.assigneeId === recommendation.member.id;
              const isAssigning = assigningMemberId === recommendation.member.id;

              return (
                <div
                  key={recommendation.member.id}
                  className="rounded-lg border border-border/60 bg-muted/20 p-3 text-sm"
                >
                  <div className="flex items-start justify-between gap-3">
                    <div className="space-y-2">
                      <div className="flex flex-wrap items-center gap-2">
                        <span className="font-medium">{recommendation.member.name}</span>
                        {index === 0 ? <Badge>Best match</Badge> : null}
                        <Badge variant="outline">{recommendation.member.typeLabel}</Badge>
                        <Badge variant="secondary">{recommendation.member.role}</Badge>
                      </div>
                      <div className="flex flex-wrap gap-2">
                        {recommendation.reasons.map((reason) => (
                          <Badge key={reason} variant="outline">
                            {reason}
                          </Badge>
                        ))}
                      </div>
                    </div>
                    <Button
                      type="button"
                      size="sm"
                      variant={isCurrentAssignee ? "secondary" : "outline"}
                      disabled={isCurrentAssignee || isAssigning || !onTaskAssign}
                      onClick={() => void handleAssign(recommendation.member.id)}
                    >
                      {isCurrentAssignee
                        ? "Assigned"
                        : isAssigning
                          ? "Assigning..."
                          : `Assign ${recommendation.member.name}`}
                    </Button>
                  </div>
                </div>
              );
            })}
          </div>
        ) : (
          <div className="rounded-lg border border-dashed border-border/60 px-3 py-4 text-sm text-muted-foreground">
            No active members are available for smart assignment yet.
          </div>
        )}

        {activeMembers.length > 0 ? (
          <div className="grid gap-2 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-end">
            <div className="flex flex-col gap-2">
              <Label htmlFor={manualAssignId}>Assign manually</Label>
              <select
                id={manualAssignId}
                className="h-10 rounded-md border bg-background px-3 text-sm"
                value={manualAssigneeId}
                onChange={(event) => setManualAssigneeId(event.target.value)}
              >
                {activeMembers.map((member) => (
                  <option key={member.id} value={member.id}>
                    {member.name} ({member.typeLabel})
                  </option>
                ))}
              </select>
            </div>
            <Button
              type="button"
              variant="outline"
              disabled={
                !manualAssigneeId ||
                manualAssigneeId === task.assigneeId ||
                Boolean(assigningMemberId) ||
                !onTaskAssign
              }
              onClick={() => void handleAssign(manualAssigneeId)}
            >
              Assign member
            </Button>
          </div>
        ) : null}
      </div>

      <Separator />

      <div className="flex flex-wrap gap-2">
        <Badge variant="outline">
          Assignee: {task.assigneeName ?? "Unassigned"}
        </Badge>
        <Badge variant="secondary">Spent: ${task.spentUsd.toFixed(2)}</Badge>
        {task.budgetUsd > 0 ? (
          <Badge variant="outline">Budget: ${task.budgetUsd.toFixed(2)}</Badge>
        ) : null}
        {task.sprintId ? (
          <Badge variant="outline">
            Sprint: {sprints.find((sprint) => sprint.id === task.sprintId)?.name ?? task.sprintId}
          </Badge>
        ) : (
          <Badge variant="outline">Backlog</Badge>
        )}
        {budgetRatio != null ? (
          <Badge variant={budgetRatio >= 100 ? "destructive" : "secondary"}>
            Usage: {budgetRatio}%
          </Badge>
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

      {dependencyState.blockers.length > 0 || dependencyState.blockedTasks.length > 0 ? (
        <div className="rounded-lg border border-border/60 bg-muted/20 p-3 text-sm">
          <div className="font-medium">Design blocker</div>
          {dependencyState.state === "ready_to_unblock" ? (
            <div className="mt-2 text-emerald-700 dark:text-emerald-300">
              All blockers are done. This task can move forward.
            </div>
          ) : dependencyState.state === "blocked" ? (
            <div className="mt-2 text-muted-foreground">
              Waiting on unresolved blockers before work can continue.
            </div>
          ) : null}
          {dependencyState.blockers.length > 0 ? (
            <div className="mt-3 space-y-2">
              {dependencyState.blockers.map((blocker) => (
                <div
                  key={blocker.id}
                  className="flex flex-wrap items-center gap-2 text-muted-foreground"
                >
                  <span>{blocker.title}</span>
                  <Badge variant={blocker.isComplete ? "secondary" : "outline"}>
                    {blocker.isComplete ? "Done" : blocker.status}
                  </Badge>
                </div>
              ))}
            </div>
          ) : null}
          {dependencyState.blockedTasks.length > 0 ? (
            <div className="mt-3 space-y-2">
              <div className="font-medium text-foreground">
                Blocks {dependencyState.blockedTasks.length} downstream task
                {dependencyState.blockedTasks.length === 1 ? "" : "s"}
              </div>
              {dependencyState.blockedTasks.map((blockedTask) => (
                <div
                  key={blockedTask.id}
                  className="flex flex-wrap items-center gap-2 text-muted-foreground"
                >
                  <span>{blockedTask.title}</span>
                  <Badge variant="outline">{blockedTask.status}</Badge>
                </div>
              ))}
            </div>
          ) : null}
        </div>
      ) : null}

      {task.agentBranch || task.agentWorktree || task.agentSessionId ? (
        <div className="rounded-lg border border-border/60 bg-muted/20 p-3 text-sm">
          <div className="font-medium">Runtime Context</div>
          {task.agentBranch ? (
            <div className="mt-2 text-muted-foreground">Branch: {task.agentBranch}</div>
          ) : null}
          {task.agentWorktree ? (
            <div className="text-muted-foreground">Worktree: {task.agentWorktree}</div>
          ) : null}
          {task.agentSessionId ? (
            <div className="text-muted-foreground">Session: {task.agentSessionId}</div>
          ) : null}
        </div>
      ) : null}

      <div className="rounded-lg border border-border/60 bg-muted/20 p-3 text-sm">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <div className="font-medium">AI task decomposition</div>
            <div className="text-muted-foreground">
              Break this task into child tasks through the Bridge decomposition flow.
            </div>
          </div>
          <Button
            type="button"
            variant="outline"
            disabled={!onTaskDecompose || isDecomposing || visibleChildTasks.length > 0}
            onClick={() => void handleDecompose()}
          >
            {visibleChildTasks.length > 0
              ? "Already decomposed"
              : isDecomposing
                ? "Decomposing..."
                : "AI Decompose Task"}
          </Button>
        </div>
        {decompositionError ? (
          <div className="mt-3 text-sm text-destructive">{decompositionError}</div>
        ) : null}
        {decompositionSummary ? (
          <div className="mt-3 text-muted-foreground">{decompositionSummary}</div>
        ) : null}
        {visibleChildTasks.length > 0 ? (
          <div className="mt-3 space-y-2">
            <div className="font-medium text-foreground">Generated subtasks</div>
            {visibleChildTasks.map((childTask) => (
              <div
                key={childTask.id}
                className="flex flex-wrap items-center justify-between gap-2 rounded-md border border-border/60 bg-background px-3 py-2"
              >
                <div className="flex min-w-0 flex-col gap-1">
                  <span>{childTask.title}</span>
                  {childTask.description ? (
                    <span className="text-xs text-muted-foreground">
                      {childTask.description}
                    </span>
                  ) : null}
                </div>
                <div className="flex flex-wrap items-center gap-2">
                  {formatExecutionMode(childTask.executionMode) ? (
                    <Badge variant="secondary">
                      {formatExecutionMode(childTask.executionMode)}
                    </Badge>
                  ) : null}
                  <Badge variant="outline">{childTask.status}</Badge>
                </div>
              </div>
            ))}
          </div>
        ) : null}
      </div>

      <Separator />

      <TaskReviewSection taskId={task.id} />

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
