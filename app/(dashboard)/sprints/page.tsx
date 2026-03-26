"use client";

import { useEffect, useMemo, useState } from "react";
import { Plus } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { BurndownChart } from "@/components/sprint/burndown-chart";
import {
  useSprintStore,
  type Sprint,
  type SprintStatus,
} from "@/lib/stores/sprint-store";
import { useDashboardStore } from "@/lib/stores/dashboard-store";
import { useMilestoneStore } from "@/lib/stores/milestone-store";
import { MilestoneEditor } from "@/components/milestones/milestone-editor";

const EMPTY_SPRINTS: Sprint[] = [];

function statusVariant(status: SprintStatus): "default" | "secondary" | "outline" {
  switch (status) {
    case "active":
      return "default";
    case "planning":
      return "secondary";
    case "closed":
      return "outline";
  }
}

function SprintCard({
  sprint,
  onSelect,
  onEdit,
}: {
  sprint: Sprint;
  onSelect: () => void;
  onEdit: () => void;
}) {
  const budgetRatio =
    sprint.totalBudgetUsd > 0
      ? Math.round((sprint.spentUsd / sprint.totalBudgetUsd) * 100)
      : null;

  return (
    <Card className="cursor-pointer hover:ring-1 hover:ring-ring" onClick={onSelect}>
      <CardHeader className="flex flex-row items-center justify-between pb-2">
        <CardTitle className="text-base">{sprint.name}</CardTitle>
        <Badge variant={statusVariant(sprint.status)}>{sprint.status}</Badge>
      </CardHeader>
      <CardContent className="space-y-2">
        <div className="text-sm text-muted-foreground">
          {sprint.startDate.slice(0, 10)} &mdash; {sprint.endDate.slice(0, 10)}
        </div>
        {sprint.totalBudgetUsd > 0 && (
          <div className="space-y-1">
            <div className="flex justify-between text-sm">
              <span>Budget</span>
              <span>
                ${sprint.spentUsd.toFixed(2)} / ${sprint.totalBudgetUsd.toFixed(2)}
              </span>
            </div>
            <div className="h-2 rounded-full bg-muted">
              <div
                className="h-2 rounded-full bg-primary transition-all"
                style={{ width: `${Math.min(budgetRatio ?? 0, 100)}%` }}
              />
            </div>
          </div>
        )}
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={(e) => {
            e.stopPropagation();
            onEdit();
          }}
        >
          Edit
        </Button>
      </CardContent>
    </Card>
  );
}

export default function SprintsPage() {
  const { selectedProjectId } = useDashboardStore();
  const {
    sprintsByProject,
    loadingByProject,
    metricsBySprintId,
    fetchSprints,
    fetchSprintMetrics,
    createSprint,
    updateSprint,
  } = useSprintStore();

  const projectId = selectedProjectId ?? "";
  const sprints = useMemo(
    () => sprintsByProject[projectId] ?? EMPTY_SPRINTS,
    [projectId, sprintsByProject]
  );
  const loading = loadingByProject[projectId] ?? false;
  const [selectedSprintId, setSelectedSprintId] = useState<string | null>(null);
  const [createOpen, setCreateOpen] = useState(false);
  const [editSprint, setEditSprint] = useState<Sprint | null>(null);
  const [milestoneOpen, setMilestoneOpen] = useState(false);

  const [formName, setFormName] = useState("");
  const [formStart, setFormStart] = useState("");
  const [formEnd, setFormEnd] = useState("");
  const [formBudget, setFormBudget] = useState("");
  const [formStatus, setFormStatus] = useState<SprintStatus>("planning");
  const [formMilestoneId, setFormMilestoneId] = useState("");
  const milestonesByProject = useMilestoneStore((state) => state.milestonesByProject);
  const fetchMilestones = useMilestoneStore((state) => state.fetchMilestones);
  const milestones = milestonesByProject[projectId] ?? [];

  useEffect(() => {
    if (projectId) {
      void fetchSprints(projectId);
      void fetchMilestones(projectId);
    }
  }, [fetchMilestones, fetchSprints, projectId]);

  const activeSprint = useMemo(() => sprints.find((s) => s.status === "active"), [sprints]);
  const effectiveSelectedSprintId = selectedSprintId ?? activeSprint?.id ?? null;
  const selectedMetrics = effectiveSelectedSprintId
    ? metricsBySprintId[effectiveSelectedSprintId]
    : null;

  useEffect(() => {
    if (effectiveSelectedSprintId && projectId) {
      void fetchSprintMetrics(projectId, effectiveSelectedSprintId);
    }
  }, [effectiveSelectedSprintId, fetchSprintMetrics, projectId]);

  const openCreate = () => {
    setFormName("");
    setFormStart("");
    setFormEnd("");
    setFormBudget("");
    setFormMilestoneId("");
    setCreateOpen(true);
  };

  const openEdit = (sprint: Sprint) => {
    setFormName(sprint.name);
    setFormStart(sprint.startDate.slice(0, 10));
    setFormEnd(sprint.endDate.slice(0, 10));
    setFormBudget(sprint.totalBudgetUsd.toString());
    setFormMilestoneId(sprint.milestoneId ?? "");
    setFormStatus(sprint.status);
    setEditSprint(sprint);
  };

  const handleCreate = async () => {
    if (!projectId || !formName || !formStart || !formEnd) return;
    await createSprint(projectId, {
      name: formName,
      startDate: formStart,
      endDate: formEnd,
      totalBudgetUsd: parseFloat(formBudget) || 0,
    });
    setCreateOpen(false);
  };

  const handleUpdate = async () => {
    if (!projectId || !editSprint) return;
    await updateSprint(projectId, editSprint.id, {
      name: formName,
      startDate: formStart,
      endDate: formEnd,
      status: formStatus,
      totalBudgetUsd: parseFloat(formBudget) || 0,
      milestoneId: formMilestoneId || null,
    });
    setEditSprint(null);
  };

  if (!projectId) {
    return (
      <div className="flex flex-col gap-6">
        <h1 className="text-2xl font-bold">Sprint Management</h1>
        <p className="text-sm text-muted-foreground">
          Select a project from the Dashboard to manage sprints.
        </p>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Sprint Management</h1>
        <div className="flex items-center gap-2">
          <Button type="button" variant="outline" onClick={() => setMilestoneOpen(true)}>
            New Milestone
          </Button>
          <Button type="button" onClick={openCreate}>
            <Plus className="mr-2 size-4" />
            New Sprint
          </Button>
        </div>
      </div>

      {loading && (
        <p className="text-sm text-muted-foreground">Loading sprints...</p>
      )}

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {sprints.map((sprint) => (
          <SprintCard
            key={sprint.id}
            sprint={sprint}
            onSelect={() => setSelectedSprintId(sprint.id)}
            onEdit={() => openEdit(sprint)}
          />
        ))}
        {sprints.length === 0 && !loading && (
          <p className="col-span-full text-sm text-muted-foreground">
            No sprints yet. Create one to get started.
          </p>
        )}
      </div>

      {selectedMetrics && (
        <Card>
          <CardHeader>
            <CardTitle>
              Burndown &mdash; {selectedMetrics.sprint.name}
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="flex flex-wrap gap-4 text-sm">
              <Badge variant="secondary">
                Completed: {selectedMetrics.completedTasks}/{selectedMetrics.plannedTasks}
              </Badge>
              <Badge variant="outline">
                Completion: {Math.round(selectedMetrics.completionRate * 100)}%
              </Badge>
              <Badge variant="outline">
                Velocity: {selectedMetrics.velocityPerWeek.toFixed(1)}/week
              </Badge>
              <Badge variant="secondary">
                Cost: ${selectedMetrics.taskSpentUsd.toFixed(2)} / ${selectedMetrics.taskBudgetUsd.toFixed(2)}
              </Badge>
            </div>
            <BurndownChart
              burndown={selectedMetrics.burndown}
              plannedTasks={selectedMetrics.plannedTasks}
            />
          </CardContent>
        </Card>
      )}

      {/* Create Dialog */}
      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Create Sprint</DialogTitle>
          </DialogHeader>
          <div className="flex flex-col gap-4">
            <div className="flex flex-col gap-2">
              <Label>Name</Label>
              <Input value={formName} onChange={(e) => setFormName(e.target.value)} />
            </div>
            <div className="grid grid-cols-2 gap-4">
              <div className="flex flex-col gap-2">
                <Label>Start Date</Label>
                <Input type="date" value={formStart} onChange={(e) => setFormStart(e.target.value)} />
              </div>
              <div className="flex flex-col gap-2">
                <Label>End Date</Label>
                <Input type="date" value={formEnd} onChange={(e) => setFormEnd(e.target.value)} />
              </div>
            </div>
            <div className="flex flex-col gap-2">
              <Label>Budget (USD)</Label>
              <Input type="number" step="0.01" value={formBudget} onChange={(e) => setFormBudget(e.target.value)} />
            </div>
            <div className="flex flex-col gap-2">
              <Label>Milestone</Label>
              <select
                className="h-10 rounded-md border bg-background px-3 text-sm"
                value={formMilestoneId}
                onChange={(e) => setFormMilestoneId(e.target.value)}
              >
                <option value="">No milestone</option>
                {milestones.map((milestone) => (
                  <option key={milestone.id} value={milestone.id}>
                    {milestone.name}
                  </option>
                ))}
              </select>
            </div>
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setCreateOpen(false)}>
              Cancel
            </Button>
            <Button type="button" onClick={() => void handleCreate()}>
              Create
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Edit Dialog */}
      <Dialog open={!!editSprint} onOpenChange={(open) => !open && setEditSprint(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Edit Sprint</DialogTitle>
          </DialogHeader>
          <div className="flex flex-col gap-4">
            <div className="flex flex-col gap-2">
              <Label>Name</Label>
              <Input value={formName} onChange={(e) => setFormName(e.target.value)} />
            </div>
            <div className="grid grid-cols-2 gap-4">
              <div className="flex flex-col gap-2">
                <Label>Start Date</Label>
                <Input type="date" value={formStart} onChange={(e) => setFormStart(e.target.value)} />
              </div>
              <div className="flex flex-col gap-2">
                <Label>End Date</Label>
                <Input type="date" value={formEnd} onChange={(e) => setFormEnd(e.target.value)} />
              </div>
            </div>
            <div className="flex flex-col gap-2">
              <Label>Budget (USD)</Label>
              <Input type="number" step="0.01" value={formBudget} onChange={(e) => setFormBudget(e.target.value)} />
            </div>
            <div className="flex flex-col gap-2">
              <Label>Milestone</Label>
              <select
                className="h-10 rounded-md border bg-background px-3 text-sm"
                value={formMilestoneId}
                onChange={(e) => setFormMilestoneId(e.target.value)}
              >
                <option value="">No milestone</option>
                {milestones.map((milestone) => (
                  <option key={milestone.id} value={milestone.id}>
                    {milestone.name}
                  </option>
                ))}
              </select>
            </div>
            <div className="flex flex-col gap-2">
              <Label>Status</Label>
              <select
                className="h-10 rounded-md border bg-background px-3 text-sm"
                value={formStatus}
                onChange={(e) => setFormStatus(e.target.value as SprintStatus)}
              >
                <option value="planning">Planning</option>
                <option value="active">Active</option>
                <option value="closed">Closed</option>
              </select>
            </div>
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setEditSprint(null)}>
              Cancel
            </Button>
            <Button type="button" onClick={() => void handleUpdate()}>
              Save
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <MilestoneEditor
        open={milestoneOpen}
        onOpenChange={setMilestoneOpen}
        projectId={projectId}
      />
    </div>
  );
}
