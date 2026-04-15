"use client";

import { Suspense, useCallback, useEffect, useMemo, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { useTranslations } from "next-intl";
import { Plus, CalendarRange } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { PageHeader } from "@/components/shared/page-header";
import { EmptyState } from "@/components/shared/empty-state";
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
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { BurndownChart } from "@/components/sprint/burndown-chart";
import {
  normalizeSprintDateInput,
  useSprintStore,
  type Sprint,
  type SprintBudgetDetail,
  type SprintStatus,
} from "@/lib/stores/sprint-store";
import { useDashboardStore } from "@/lib/stores/dashboard-store";
import { useMilestoneStore } from "@/lib/stores/milestone-store";
import { MilestoneEditor } from "@/components/milestones/milestone-editor";
import { useBreadcrumbs } from "@/hooks/use-breadcrumbs";
import { buildProjectTaskWorkspaceHref } from "@/lib/route-hrefs";

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
  const t = useTranslations("sprints");
  const budgetRatio =
    sprint.totalBudgetUsd > 0
      ? Math.round((sprint.spentUsd / sprint.totalBudgetUsd) * 100)
      : null;

  return (
    <Card className="cursor-pointer hover:ring-1 hover:ring-ring" onClick={onSelect}>
      <CardHeader className="flex flex-row items-center justify-between pb-2">
        <CardTitle className="text-base">{sprint.name}</CardTitle>
        <Badge variant={statusVariant(sprint.status)}>{t(`status.${sprint.status}`)}</Badge>
      </CardHeader>
      <CardContent className="space-y-2">
        <div className="text-sm text-muted-foreground">
          {sprint.startDate.slice(0, 10)} &mdash; {sprint.endDate.slice(0, 10)}
        </div>
        {sprint.totalBudgetUsd > 0 && (
          <div className="space-y-1">
            <div className="flex justify-between text-sm">
              <span>{t("card.budget")}</span>
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
          {t("card.edit")}
        </Button>
      </CardContent>
    </Card>
  );
}

function budgetThresholdVariant(
  thresholdStatus: SprintBudgetDetail["thresholdStatus"],
): "default" | "secondary" | "outline" {
  switch (thresholdStatus) {
    case "exceeded":
      return "default";
    case "warning":
      return "secondary";
    default:
      return "outline";
  }
}

function formatCurrency(value: number): string {
  return `$${value.toFixed(2)}`;
}

function SprintsPageContent() {
  useBreadcrumbs([{ label: "Project", href: "/" }, { label: "Sprints" }]);
  const t = useTranslations("sprints");
  const searchParams = useSearchParams();
  const router = useRouter();
  const requestedProjectId = searchParams.get("project");
  const requestedAction = searchParams.get("action");
  const { selectedProjectId, fetchSummary } = useDashboardStore();
  const {
    sprintsByProject,
    loadingByProject,
    metricsBySprintId,
    budgetDetailBySprintId,
    fetchSprints,
    fetchSprintMetrics,
    fetchSprintBudgetDetail,
    createSprint,
    updateSprint,
  } = useSprintStore();
  const budgetLoadingBySprintId = useSprintStore((state) => state.budgetLoadingBySprintId);
  const budgetErrorBySprintId = useSprintStore((state) => state.budgetErrorBySprintId);

  const projectId = requestedProjectId ?? selectedProjectId ?? "";
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
  const [formError, setFormError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState<"create" | "update" | null>(null);
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
  const effectiveSelectedSprintId = selectedSprintId ?? activeSprint?.id ?? sprints[0]?.id ?? null;
  const selectedSprint = useMemo(
    () => sprints.find((sprint) => sprint.id === effectiveSelectedSprintId) ?? null,
    [effectiveSelectedSprintId, sprints],
  );
  const selectedMetrics = effectiveSelectedSprintId
    ? metricsBySprintId[effectiveSelectedSprintId]
    : null;
  const selectedBudgetDetail = effectiveSelectedSprintId
    ? budgetDetailBySprintId[effectiveSelectedSprintId] ?? null
    : null;
  const selectedBudgetLoading = effectiveSelectedSprintId
    ? budgetLoadingBySprintId[effectiveSelectedSprintId] ?? false
    : false;
  const selectedBudgetError = effectiveSelectedSprintId
    ? budgetErrorBySprintId[effectiveSelectedSprintId] ?? null
    : null;
  const selectedSprintRefreshKey = selectedSprint
    ? [
        selectedSprint.id,
        selectedSprint.status,
        selectedSprint.startDate,
        selectedSprint.endDate,
        selectedSprint.totalBudgetUsd,
        selectedSprint.spentUsd,
        selectedSprint.milestoneId ?? "",
      ].join("|")
    : "";

  useEffect(() => {
    if (selectedSprintRefreshKey && selectedSprint && projectId) {
      void fetchSprintMetrics(projectId, selectedSprint.id);
      void fetchSprintBudgetDetail(selectedSprint.id);
    }
  }, [
    fetchSprintBudgetDetail,
    fetchSprintMetrics,
    projectId,
    selectedSprint,
    selectedSprintRefreshKey,
  ]);

  useEffect(() => {
    if (selectedSprintId && !sprints.some((sprint) => sprint.id === selectedSprintId)) {
      setSelectedSprintId(null);
    }
  }, [selectedSprintId, sprints]);

  const resetForm = useCallback(() => {
    setFormName("");
    setFormStart("");
    setFormEnd("");
    setFormBudget("");
    setFormStatus("planning");
    setFormMilestoneId("");
    setFormError(null);
  }, []);

  const openCreate = useCallback(() => {
    resetForm();
    setCreateOpen(true);
  }, [resetForm]);

  useEffect(() => {
    if (requestedAction === "create-sprint" && projectId) {
      openCreate();
    }
  }, [openCreate, projectId, requestedAction]);

  const openEdit = (sprint: Sprint) => {
    setFormError(null);
    setFormName(sprint.name);
    setFormStart(sprint.startDate.slice(0, 10));
    setFormEnd(sprint.endDate.slice(0, 10));
    setFormBudget(sprint.totalBudgetUsd.toString());
    setFormMilestoneId(sprint.milestoneId ?? "");
    setFormStatus(sprint.status);
    setEditSprint(sprint);
  };

  const handleCreate = async () => {
    if (!projectId || !formName || !formStart || !formEnd) {
      setFormError(t("dialog.error.requiredFields"));
      return;
    }

    setSubmitting("create");
    setFormError(null);

    try {
      const createdSprint = await createSprint(projectId, {
        name: formName.trim(),
        startDate: normalizeSprintDateInput(formStart),
        endDate: normalizeSprintDateInput(formEnd),
        totalBudgetUsd: parseFloat(formBudget) || 0,
        milestoneId: formMilestoneId || undefined,
      });
      await fetchSummary({ projectId });
      setSelectedSprintId(createdSprint.id);
      resetForm();
      setCreateOpen(false);
    } catch (error) {
      setFormError(error instanceof Error ? error.message : t("dialog.error.saveFailed"));
    } finally {
      setSubmitting(null);
    }
  };

  const handleUpdate = async () => {
    if (!projectId || !editSprint || !formName || !formStart || !formEnd) {
      setFormError(t("dialog.error.requiredFields"));
      return;
    }

    setSubmitting("update");
    setFormError(null);

    try {
      const updatedSprint = await updateSprint(projectId, editSprint.id, {
        name: formName.trim(),
        startDate: normalizeSprintDateInput(formStart),
        endDate: normalizeSprintDateInput(formEnd),
        status: formStatus,
        totalBudgetUsd: parseFloat(formBudget) || 0,
        milestoneId: formMilestoneId,
      });
      await fetchSummary({ projectId });
      setSelectedSprintId(updatedSprint.id);
      setEditSprint(null);
      setFormError(null);
    } catch (error) {
      setFormError(error instanceof Error ? error.message : t("dialog.error.saveFailed"));
    } finally {
      setSubmitting(null);
    }
  };

  if (!projectId) {
    return (
      <div className="flex flex-col gap-6">
        <PageHeader title={t("title")} />
        <p className="text-sm text-muted-foreground">
          {t("selectProjectPrompt")}
        </p>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        title={t("title")}
        actions={
          <>
            <Button type="button" variant="outline" onClick={() => setMilestoneOpen(true)}>
              {t("newMilestone")}
            </Button>
            <Button type="button" onClick={openCreate}>
              <Plus className="mr-2 size-4" />
              {t("newSprint")}
            </Button>
          </>
        }
      />

      {loading && (
        <p className="text-sm text-muted-foreground">{t("loading")}</p>
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
          <div className="col-span-full">
            <EmptyState
              icon={CalendarRange}
              title={t("empty.noSprints")}
            />
          </div>
        )}
      </div>

      {selectedSprint && (
        <Card>
          <CardHeader className="flex flex-row items-center justify-between gap-4">
            <CardTitle>
              {t("burndown.title")} &mdash; {selectedSprint.name}
            </CardTitle>
            <Button
              type="button"
              variant="outline"
              onClick={() =>
                router.push(
                  buildProjectTaskWorkspaceHref({
                    projectId,
                    sprintId: selectedSprint.id,
                  }),
                )
              }
            >
              {t("actions.openTasks")}
            </Button>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="flex flex-wrap gap-4 text-sm">
              <Badge variant="secondary">
                {t("burndown.completed", {
                  completed: selectedMetrics?.completedTasks ?? 0,
                  planned: selectedMetrics?.plannedTasks ?? 0,
                })}
              </Badge>
              <Badge variant="outline">
                {t("burndown.completion", {
                  rate: Math.round(selectedMetrics?.completionRate ?? 0),
                })}
              </Badge>
              <Badge variant="outline">
                {t("burndown.velocity", {
                  velocity: (selectedMetrics?.velocityPerWeek ?? 0).toFixed(1),
                })}
              </Badge>
              <Badge variant="secondary">
                {t("burndown.cost", {
                  spent: (selectedMetrics?.taskSpentUsd ?? 0).toFixed(2),
                  budget: (selectedMetrics?.taskBudgetUsd ?? 0).toFixed(2),
                })}
              </Badge>
            </div>
            <BurndownChart
              burndown={selectedMetrics?.burndown ?? []}
              plannedTasks={selectedMetrics?.plannedTasks ?? 0}
            />

            <div className="space-y-3 border-t pt-4">
              <div className="flex flex-wrap items-center justify-between gap-3">
                <div className="text-sm font-medium">{t("detail.budgetTitle")}</div>
                {selectedBudgetDetail ? (
                  <Badge variant={budgetThresholdVariant(selectedBudgetDetail.thresholdStatus)}>
                    {selectedBudgetDetail.thresholdStatus}
                  </Badge>
                ) : null}
              </div>

              {selectedBudgetLoading ? (
                <p className="text-sm text-muted-foreground">{t("detail.loadingBudget")}</p>
              ) : selectedBudgetError ? (
                <p className="text-sm text-destructive">{selectedBudgetError}</p>
              ) : selectedBudgetDetail ? (
                <div className="space-y-3">
                  <div className="grid gap-3 sm:grid-cols-3">
                    <div className="rounded-md border px-3 py-2 text-sm">
                      <div className="text-muted-foreground">{t("detail.allocated")}</div>
                      <div className="font-medium">{formatCurrency(selectedBudgetDetail.allocated)}</div>
                    </div>
                    <div className="rounded-md border px-3 py-2 text-sm">
                      <div className="text-muted-foreground">{t("detail.spent")}</div>
                      <div className="font-medium">{formatCurrency(selectedBudgetDetail.spent)}</div>
                    </div>
                    <div className="rounded-md border px-3 py-2 text-sm">
                      <div className="text-muted-foreground">{t("detail.remaining")}</div>
                      <div className="font-medium">{formatCurrency(selectedBudgetDetail.remaining)}</div>
                    </div>
                  </div>

                  {selectedBudgetDetail.tasks.length > 0 ? (
                    <div className="space-y-2">
                      {selectedBudgetDetail.tasks.map((task) => (
                        <div
                          key={task.taskId}
                          className="flex items-center justify-between gap-3 rounded-md border px-3 py-2 text-sm"
                        >
                          <div className="font-medium">{task.title}</div>
                          <div className="text-muted-foreground">
                            {formatCurrency(task.spent)} / {formatCurrency(task.allocated)}
                          </div>
                        </div>
                      ))}
                    </div>
                  ) : (
                    <p className="text-sm text-muted-foreground">{t("detail.budgetEmpty")}</p>
                  )}
                </div>
              ) : (
                <p className="text-sm text-muted-foreground">{t("detail.budgetEmpty")}</p>
              )}
            </div>
          </CardContent>
        </Card>
      )}

      {/* Create Dialog */}
      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("dialog.createTitle")}</DialogTitle>
          </DialogHeader>
          <div className="flex flex-col gap-4">
            <div className="flex flex-col gap-2">
              <Label>{t("dialog.name")}</Label>
              <Input value={formName} onChange={(e) => setFormName(e.target.value)} />
            </div>
            <div className="grid grid-cols-2 gap-4">
              <div className="flex flex-col gap-2">
                <Label>{t("dialog.startDate")}</Label>
                <Input type="date" value={formStart} onChange={(e) => setFormStart(e.target.value)} />
              </div>
              <div className="flex flex-col gap-2">
                <Label>{t("dialog.endDate")}</Label>
                <Input type="date" value={formEnd} onChange={(e) => setFormEnd(e.target.value)} />
              </div>
            </div>
            <div className="flex flex-col gap-2">
              <Label>{t("dialog.budgetUsd")}</Label>
              <Input type="number" step="0.01" value={formBudget} onChange={(e) => setFormBudget(e.target.value)} />
            </div>
            <div className="flex flex-col gap-2">
              <Label>{t("dialog.milestone")}</Label>
              <Select value={formMilestoneId || "__none__"} onValueChange={(v) => setFormMilestoneId(v === "__none__" ? "" : v)}>
                <SelectTrigger>
                  <SelectValue placeholder={t("dialog.noMilestone")} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="__none__">{t("dialog.noMilestone")}</SelectItem>
                  {milestones.map((milestone) => (
                    <SelectItem key={milestone.id} value={milestone.id}>
                      {milestone.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            {formError ? <p className="text-sm text-destructive">{formError}</p> : null}
          </div>
          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => {
                resetForm();
                setCreateOpen(false);
              }}
            >
              {t("dialog.cancel")}
            </Button>
            <Button type="button" onClick={() => void handleCreate()} disabled={submitting === "create"}>
              {submitting === "create" ? t("dialog.saving") : t("dialog.create")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Edit Dialog */}
      <Dialog open={!!editSprint} onOpenChange={(open) => !open && setEditSprint(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("dialog.editTitle")}</DialogTitle>
          </DialogHeader>
          <div className="flex flex-col gap-4">
            <div className="flex flex-col gap-2">
              <Label>{t("dialog.name")}</Label>
              <Input value={formName} onChange={(e) => setFormName(e.target.value)} />
            </div>
            <div className="grid grid-cols-2 gap-4">
              <div className="flex flex-col gap-2">
                <Label>{t("dialog.startDate")}</Label>
                <Input type="date" value={formStart} onChange={(e) => setFormStart(e.target.value)} />
              </div>
              <div className="flex flex-col gap-2">
                <Label>{t("dialog.endDate")}</Label>
                <Input type="date" value={formEnd} onChange={(e) => setFormEnd(e.target.value)} />
              </div>
            </div>
            <div className="flex flex-col gap-2">
              <Label>{t("dialog.budgetUsd")}</Label>
              <Input type="number" step="0.01" value={formBudget} onChange={(e) => setFormBudget(e.target.value)} />
            </div>
            <div className="flex flex-col gap-2">
              <Label>{t("dialog.milestone")}</Label>
              <Select value={formMilestoneId || "__none__"} onValueChange={(v) => setFormMilestoneId(v === "__none__" ? "" : v)}>
                <SelectTrigger>
                  <SelectValue placeholder={t("dialog.noMilestone")} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="__none__">{t("dialog.noMilestone")}</SelectItem>
                  {milestones.map((milestone) => (
                    <SelectItem key={milestone.id} value={milestone.id}>
                      {milestone.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="flex flex-col gap-2">
              <Label>{t("dialog.status")}</Label>
              <Select value={formStatus} onValueChange={(v) => setFormStatus(v as SprintStatus)}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="planning">{t("status.planning")}</SelectItem>
                  <SelectItem value="active">{t("status.active")}</SelectItem>
                  <SelectItem value="closed">{t("status.closed")}</SelectItem>
                </SelectContent>
              </Select>
            </div>
            {formError ? <p className="text-sm text-destructive">{formError}</p> : null}
          </div>
          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => {
                setEditSprint(null);
                setFormError(null);
              }}
            >
              {t("dialog.cancel")}
            </Button>
            <Button type="button" onClick={() => void handleUpdate()} disabled={submitting === "update"}>
              {submitting === "update" ? t("dialog.saving") : t("dialog.save")}
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

export default function SprintsPage() {
  return (
    <Suspense fallback={null}>
      <SprintsPageContent />
    </Suspense>
  );
}
