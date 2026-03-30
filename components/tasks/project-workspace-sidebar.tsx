"use client";

import { useMemo } from "react";
import { useTranslations } from "next-intl";
import {
  LayoutGrid,
  List,
  GitBranch,
  Map,
  CalendarDays,
  Clock,
  Plus,
  Search,
  RotateCcw,
  X,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { ViewSwitcher } from "@/components/views/view-switcher";
import type { TaskViewMode, TaskDependencyFilter } from "@/lib/tasks/task-workspace";
import type { Task, TaskPriority, TaskStatus } from "@/lib/stores/task-store";
import type { Sprint } from "@/lib/stores/sprint-store";
import { useTaskWorkspaceStore } from "@/lib/stores/task-workspace-store";
import { cn } from "@/lib/utils";

const VIEW_MODES: Array<{ value: TaskViewMode; icon: typeof LayoutGrid; labelKey: string }> = [
  { value: "board", icon: LayoutGrid, labelKey: "viewMode.board" },
  { value: "list", icon: List, labelKey: "viewMode.list" },
  { value: "timeline", icon: Clock, labelKey: "viewMode.timeline" },
  { value: "calendar", icon: CalendarDays, labelKey: "viewMode.calendar" },
  { value: "dependencies", icon: GitBranch, labelKey: "viewMode.dependencies" },
  { value: "roadmap", icon: Map, labelKey: "viewMode.roadmap" },
];

function statusOptions(): Array<"all" | TaskStatus> {
  return ["all", "inbox", "triaged", "assigned", "in_progress", "in_review", "done"];
}

function priorityOptions(): Array<"all" | TaskPriority> {
  return ["all", "urgent", "high", "medium", "low"];
}

function dependencyOptions(): Array<{ value: TaskDependencyFilter; labelKey: string }> {
  return [
    { value: "all", labelKey: "filter.all" },
    { value: "blocked", labelKey: "filter.blocked" },
    { value: "ready_to_unblock", labelKey: "filter.readyToUnblock" },
  ];
}

function assigneeOptions(tasks: Task[]): Array<{ value: string; label: string }> {
  const seen: Record<string, string> = {};
  for (const task of tasks) {
    if (task.assigneeId && !(task.assigneeId in seen)) {
      seen[task.assigneeId] = task.assigneeName ?? task.assigneeId;
    }
  }
  return Object.entries(seen).map(([value, label]) => ({ value, label }));
}

function sprintOptions(sprints: Sprint[]): Array<{ value: string; label: string }> {
  return sprints.map((sprint) => ({
    value: sprint.id,
    label: sprint.name,
  }));
}

interface ProjectWorkspaceSidebarProps {
  projectId: string;
  projectName: string;
  tasks: Task[];
  sprints: Sprint[];
  filteredCount: number;
  realtimeConnected: boolean;
  onCreateTask?: () => void;
  onSprintFilterChange?: (sprintId: string | "all") => void;
}

export function ProjectWorkspaceSidebar({
  projectId,
  projectName,
  tasks,
  sprints,
  filteredCount,
  realtimeConnected,
  onCreateTask,
  onSprintFilterChange,
}: ProjectWorkspaceSidebarProps) {
  const t = useTranslations("tasks");
  const viewMode = useTaskWorkspaceStore((s) => s.viewMode);
  const filters = useTaskWorkspaceStore((s) => s.filters);
  const displayOptions = useTaskWorkspaceStore((s) => s.displayOptions);
  const setViewMode = useTaskWorkspaceStore((s) => s.setViewMode);
  const setSearch = useTaskWorkspaceStore((s) => s.setSearch);
  const setStatus = useTaskWorkspaceStore((s) => s.setStatus);
  const setPriority = useTaskWorkspaceStore((s) => s.setPriority);
  const setAssigneeId = useTaskWorkspaceStore((s) => s.setAssigneeId);
  const setSprintId = useTaskWorkspaceStore((s) => s.setSprintId);
  const setPlanning = useTaskWorkspaceStore((s) => s.setPlanning);
  const setDependency = useTaskWorkspaceStore((s) => s.setDependency);
  const setDensity = useTaskWorkspaceStore((s) => s.setDensity);
  const setShowDescriptions = useTaskWorkspaceStore((s) => s.setShowDescriptions);
  const setShowLinkedDocs = useTaskWorkspaceStore((s) => s.setShowLinkedDocs);
  const resetFilters = useTaskWorkspaceStore((s) => s.resetFilters);

  const assignees = useMemo(() => assigneeOptions(tasks), [tasks]);
  const sprintOpts = useMemo(() => sprintOptions(sprints), [sprints]);

  const hasActiveFilters =
    filters.search.trim() !== "" ||
    filters.status !== "all" ||
    filters.priority !== "all" ||
    filters.assigneeId !== "all" ||
    filters.sprintId !== "all" ||
    filters.planning !== "all" ||
    filters.dependency !== "all";

  return (
    <div className="flex h-full flex-col">
      {/* Header */}
      <div className="flex flex-col gap-2 border-b px-4 py-3">
        <div className="flex items-center justify-between gap-2">
          <h2 className="truncate text-sm font-semibold">{projectName}</h2>
          <Badge
            variant="secondary"
            className={cn(
              "shrink-0 text-[10px]",
              realtimeConnected
                ? "bg-emerald-500/15 text-emerald-700 dark:text-emerald-300"
                : "bg-amber-500/15 text-amber-700 dark:text-amber-300"
            )}
          >
            {realtimeConnected ? t("workspace.realtimeLive") : t("workspace.realtimeDegraded")}
          </Badge>
        </div>
        {onCreateTask ? (
          <Button size="sm" className="w-full" onClick={onCreateTask}>
            <Plus className="mr-1.5 size-3.5" />
            {t("empty.createTaskAction")}
          </Button>
        ) : null}
      </div>

      {/* Scrollable content */}
      <div className="flex-1 overflow-y-auto">
        {/* View mode selector */}
        <div className="border-b px-3 py-3">
          <div className="mb-2 text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
            {t("sidebar.views")}
          </div>
          <nav className="flex flex-col gap-0.5">
            {VIEW_MODES.map((mode) => {
              const Icon = mode.icon;
              const isActive = viewMode === mode.value;
              return (
                <button
                  key={mode.value}
                  type="button"
                  className={cn(
                    "flex items-center gap-2.5 rounded-md px-2.5 py-1.5 text-sm transition-colors",
                    isActive
                      ? "bg-accent font-medium text-accent-foreground"
                      : "text-muted-foreground hover:bg-accent/50 hover:text-foreground"
                  )}
                  onClick={() => setViewMode(mode.value)}
                >
                  <Icon className="size-4 shrink-0" />
                  {t(mode.labelKey)}
                </button>
              );
            })}
          </nav>
        </div>

        {/* Saved views */}
        <div className="border-b px-3 py-3">
          <ViewSwitcher projectId={projectId} />
        </div>

        {/* Filters */}
        <div className="border-b px-3 py-3">
          <div className="mb-2 flex items-center justify-between">
            <span className="text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
              {t("sidebar.filters")}
            </span>
            {hasActiveFilters ? (
              <Button
                type="button"
                variant="ghost"
                size="sm"
                className="h-5 px-1.5 text-[10px]"
                onClick={resetFilters}
              >
                <RotateCcw className="mr-1 size-3" />
                {t("workspace.resetFilters")}
              </Button>
            ) : null}
          </div>

          <div className="flex flex-col gap-2.5">
            <div className="relative">
              <Search className="absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
              <Input
                aria-label={t("filter.searchTasks")}
                className="h-8 pl-8 text-xs"
                value={filters.search}
                onChange={(e) => setSearch(e.target.value)}
                placeholder={t("filter.searchPlaceholder")}
              />
              {filters.search ? (
                <button
                  type="button"
                  className="absolute right-2 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
                  onClick={() => setSearch("")}
                >
                  <X className="size-3" />
                </button>
              ) : null}
            </div>

            <label className="flex flex-col gap-1 text-xs font-medium">
              {t("filter.status")}
              <select
                className="h-8 rounded-md border bg-background px-2 text-xs"
                value={filters.status}
                onChange={(e) => setStatus(e.target.value as "all" | TaskStatus)}
              >
                {statusOptions().map((s) => (
                  <option key={s} value={s}>{s}</option>
                ))}
              </select>
            </label>

            <label className="flex flex-col gap-1 text-xs font-medium">
              {t("filter.priority")}
              <select
                className="h-8 rounded-md border bg-background px-2 text-xs"
                value={filters.priority}
                onChange={(e) => setPriority(e.target.value as "all" | TaskPriority)}
              >
                {priorityOptions().map((p) => (
                  <option key={p} value={p}>{p}</option>
                ))}
              </select>
            </label>

            <label className="flex flex-col gap-1 text-xs font-medium">
              {t("filter.assignee")}
              <select
                className="h-8 rounded-md border bg-background px-2 text-xs"
                value={filters.assigneeId}
                onChange={(e) => setAssigneeId(e.target.value)}
              >
                <option value="all">{t("filter.all")}</option>
                {assignees.map((a) => (
                  <option key={a.value} value={a.value}>{a.label}</option>
                ))}
              </select>
            </label>

            <label className="flex flex-col gap-1 text-xs font-medium">
              {t("filter.sprint")}
              <select
                aria-label={t("filter.sprint")}
                className="h-8 rounded-md border bg-background px-2 text-xs"
                value={filters.sprintId}
                onChange={(e) => {
                  const v = e.target.value as string | "all";
                  setSprintId(v);
                  onSprintFilterChange?.(v);
                }}
              >
                <option value="all">{t("filter.all")}</option>
                {sprintOpts.map((s) => (
                  <option key={s.value} value={s.value}>{s.label}</option>
                ))}
              </select>
            </label>

            <label className="flex flex-col gap-1 text-xs font-medium">
              {t("filter.planning")}
              <select
                className="h-8 rounded-md border bg-background px-2 text-xs"
                value={filters.planning}
                onChange={(e) =>
                  setPlanning(e.target.value as "all" | "scheduled" | "unscheduled")
                }
              >
                <option value="all">{t("filter.all")}</option>
                <option value="scheduled">{t("filter.scheduled")}</option>
                <option value="unscheduled">{t("filter.unscheduled")}</option>
              </select>
            </label>

            <label className="flex flex-col gap-1 text-xs font-medium">
              {t("filter.dependencies")}
              <select
                aria-label={t("filter.dependencies")}
                className="h-8 rounded-md border bg-background px-2 text-xs"
                value={filters.dependency}
                onChange={(e) =>
                  setDependency(e.target.value as TaskDependencyFilter)
                }
              >
                {dependencyOptions().map((o) => (
                  <option key={o.value} value={o.value}>{t(o.labelKey)}</option>
                ))}
              </select>
            </label>
          </div>

          {/* Visible task count */}
          <div className="mt-2 text-[11px] text-muted-foreground">
            {t("workspace.visibleTasks", { count: filteredCount })}
          </div>
        </div>

        {/* Display options */}
        <div className="px-3 py-3">
          <div className="mb-2 text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
            {t("sidebar.displayOptions")}
          </div>
          <div className="flex flex-col gap-1.5">
            <div className="flex gap-1">
              <Button
                type="button"
                size="sm"
                variant={displayOptions.density === "comfortable" ? "secondary" : "outline"}
                className="h-7 flex-1 text-xs"
                onClick={() => setDensity("comfortable")}
              >
                {t("workspace.comfortable")}
              </Button>
              <Button
                type="button"
                size="sm"
                variant={displayOptions.density === "compact" ? "secondary" : "outline"}
                className="h-7 flex-1 text-xs"
                onClick={() => setDensity("compact")}
              >
                {t("workspace.compact")}
              </Button>
            </div>
            <Button
              type="button"
              size="sm"
              variant="outline"
              className="h-7 justify-start text-xs"
              onClick={() => setShowDescriptions(!displayOptions.showDescriptions)}
            >
              {displayOptions.showDescriptions
                ? t("workspace.hideDescriptions")
                : t("workspace.showDescriptions")}
            </Button>
            <Button
              type="button"
              size="sm"
              variant="outline"
              className="h-7 justify-start text-xs"
              onClick={() => setShowLinkedDocs(!displayOptions.showLinkedDocs)}
            >
              {displayOptions.showLinkedDocs
                ? t("workspace.hideLinkedDocs")
                : t("workspace.showLinkedDocs")}
            </Button>
          </div>
        </div>
      </div>
    </div>
  );
}
