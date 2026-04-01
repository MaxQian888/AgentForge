"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useTranslations } from "next-intl";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  ALL_TASK_STATUSES,
  TRIGGER_ACTIONS,
  useWorkflowStore,
  type WorkflowTrigger,
} from "@/lib/stores/workflow-store";
import { useWSStore } from "@/lib/stores/ws-store";
import type { TaskStatus } from "@/lib/stores/task-store";

interface WorkflowConfigPanelProps {
  projectId: string;
}

function statusLabel(status: string): string {
  return status.replace(/_/g, " ");
}

function triggerLabel(action: string): string {
  return (
    TRIGGER_ACTIONS.find((item) => item.value === action)?.label ??
    action.replace(/_/g, " ")
  );
}

function WorkflowGraph({
  transitions,
}: {
  transitions: Record<string, string[]>;
}) {
  const t = useTranslations("workflow");
  const activeStatuses = ALL_TASK_STATUSES.filter(
    (status) =>
      (transitions[status] ?? []).length > 0 ||
      Object.values(transitions).some((targets) => targets.includes(status))
  );

  if (activeStatuses.length === 0) {
    return (
      <div className="rounded-md border border-dashed px-4 py-6 text-sm text-muted-foreground">
        {t("noTransitions")}
      </div>
    );
  }

  return (
    <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
      {activeStatuses.map((status) => {
        const targets = transitions[status] ?? [];
        return (
          <div
            key={status}
            className="rounded-lg border border-border/60 bg-muted/20 p-4"
          >
            <div className="mb-3 flex items-center justify-between gap-2">
              <span className="text-sm font-semibold">{statusLabel(status)}</span>
              <Badge variant="outline" className="text-[11px]">
                {t("next", { count: targets.length })}
              </Badge>
            </div>
            {targets.length > 0 ? (
              <div className="flex flex-wrap gap-1.5">
                {targets.map((target) => (
                  <Badge key={`${status}-${target}`} variant="secondary">
                    {statusLabel(target)}
                  </Badge>
                ))}
              </div>
            ) : (
              <p className="text-xs text-muted-foreground">
                {t("noOutbound")}
              </p>
            )}
          </div>
        );
      })}
    </div>
  );
}

function TransitionEditor({
  transitions,
  onChange,
}: {
  transitions: Record<string, string[]>;
  onChange: (transitions: Record<string, string[]>) => void;
}) {
  const t = useTranslations("workflow");
  const toggleTransition = (from: TaskStatus, to: TaskStatus) => {
    const current = transitions[from] ?? [];
    const next = current.includes(to)
      ? current.filter((s) => s !== to)
      : [...current, to];
    onChange({ ...transitions, [from]: next });
  };

  return (
    <div className="overflow-auto">
      <Table className="w-full text-xs">
        <TableHeader>
          <TableRow>
            <TableHead className="px-2 py-1 text-left font-medium text-muted-foreground">
              {t("fromTo")}
            </TableHead>
            {ALL_TASK_STATUSES.map((status) => (
              <TableHead
                key={status}
                className="px-1 py-1 text-center font-medium text-muted-foreground"
              >
                {statusLabel(status)}
              </TableHead>
            ))}
          </TableRow>
        </TableHeader>
        <TableBody>
          {ALL_TASK_STATUSES.map((from) => (
            <TableRow key={from} className="border-t border-border/40">
              <TableCell className="px-2 py-1 font-medium">{statusLabel(from)}</TableCell>
              {ALL_TASK_STATUSES.map((to) => {
                const checked = (transitions[from] ?? []).includes(to);
                const isSame = from === to;
                return (
                  <TableCell key={to} className="px-1 py-1 text-center">
                    {isSame ? (
                      <span className="text-muted-foreground/40">-</span>
                    ) : (
                      <input
                        type="checkbox"
                        checked={checked}
                        onChange={() => toggleTransition(from, to)}
                        aria-label={`Allow ${statusLabel(from)} to ${statusLabel(to)}`}
                      />
                    )}
                  </TableCell>
                );
              })}
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}

function TriggerEditor({
  triggers,
  onChange,
}: {
  triggers: WorkflowTrigger[];
  onChange: (triggers: WorkflowTrigger[]) => void;
}) {
  const t = useTranslations("workflow");
  const addTrigger = () => {
    onChange([
      ...triggers,
      { fromStatus: "triaged", toStatus: "assigned", action: "auto_assign" },
    ]);
  };

  const removeTrigger = (index: number) => {
    onChange(triggers.filter((_, i) => i !== index));
  };

  const updateTrigger = (index: number, field: keyof WorkflowTrigger, value: string) => {
    const updated = triggers.map((trigger, i) =>
      i === index ? { ...trigger, [field]: value } : trigger
    );
    onChange(updated);
  };

  return (
    <div className="flex flex-col gap-3">
      {triggers.map((trigger, index) => (
        <div
          key={index}
          className="flex flex-wrap items-center gap-2 rounded-md border border-border/60 bg-muted/20 p-3 text-sm"
        >
          <span className="text-muted-foreground">{t("when")}</span>
          <Select
            value={trigger.fromStatus}
            onValueChange={(value) => updateTrigger(index, "fromStatus", value)}
          >
            <SelectTrigger className="h-8 px-2 text-sm" aria-label={`Trigger ${index + 1} from status`}>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {ALL_TASK_STATUSES.map((s) => (
                <SelectItem key={s} value={s}>
                  {statusLabel(s)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <span className="text-muted-foreground">{t("transitionsTo")}</span>
          <Select
            value={trigger.toStatus}
            onValueChange={(value) => updateTrigger(index, "toStatus", value)}
          >
            <SelectTrigger className="h-8 px-2 text-sm" aria-label={`Trigger ${index + 1} to status`}>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {ALL_TASK_STATUSES.map((s) => (
                <SelectItem key={s} value={s}>
                  {statusLabel(s)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <span className="text-muted-foreground">{t("then")}</span>
          <Select
            value={trigger.action}
            onValueChange={(value) => updateTrigger(index, "action", value)}
          >
            <SelectTrigger className="h-8 px-2 text-sm" aria-label={`Trigger ${index + 1} action`}>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {TRIGGER_ACTIONS.map((a) => (
                <SelectItem key={a.value} value={a.value}>
                  {a.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Button
            type="button"
            size="sm"
            variant="ghost"
            onClick={() => removeTrigger(index)}
            aria-label={`Remove trigger ${index + 1}`}
          >
            {t("remove")}
          </Button>
        </div>
      ))}
      <Button type="button" size="sm" variant="outline" onClick={addTrigger}>
        {t("addTriggerRule")}
      </Button>
    </div>
  );
}

export function WorkflowConfigPanel({ projectId }: WorkflowConfigPanelProps) {
  const config = useWorkflowStore((s) => s.config);
  const loading = useWorkflowStore((s) => s.loading);
  const saving = useWorkflowStore((s) => s.saving);
  const error = useWorkflowStore((s) => s.error);
  const fetchWorkflow = useWorkflowStore((s) => s.fetchWorkflow);
  const updateWorkflow = useWorkflowStore((s) => s.updateWorkflow);
  const recentActivity = useWorkflowStore(
    (s) => s.recentActivityByProject[projectId] ?? [],
  );
  const connected = useWSStore((s) => s.connected);

  useEffect(() => {
    void fetchWorkflow(projectId);
  }, [projectId, fetchWorkflow]);

  const configKey = useMemo(
    () =>
      JSON.stringify({
        transitions: config?.transitions ?? {},
        triggers: config?.triggers ?? [],
      }),
    [config],
  );

  return (
    <WorkflowDraftEditor
      key={`${projectId}:${configKey}`}
      projectId={projectId}
      loading={loading}
      saving={saving}
      error={error}
      recentActivity={recentActivity}
      connected={connected}
      initialTransitions={config?.transitions ?? {}}
      initialTriggers={config?.triggers ?? []}
      onSave={updateWorkflow}
    />
  );
}

interface WorkflowDraftEditorProps {
  projectId: string;
  loading: boolean;
  saving: boolean;
  error: string | null;
  connected: boolean;
  recentActivity: ReturnType<typeof useWorkflowStore.getState>["recentActivityByProject"][string];
  initialTransitions: Record<string, string[]>;
  initialTriggers: WorkflowTrigger[];
  onSave: ReturnType<typeof useWorkflowStore.getState>["updateWorkflow"];
}

function WorkflowDraftEditor({
  projectId,
  loading,
  saving,
  error,
  connected,
  recentActivity,
  initialTransitions,
  initialTriggers,
  onSave,
}: WorkflowDraftEditorProps) {
  const t = useTranslations("workflow");
  const [transitions, setTransitions] =
    useState<Record<string, string[]>>(initialTransitions);
  const [triggers, setTriggers] = useState<WorkflowTrigger[]>(initialTriggers);
  const [dirty, setDirty] = useState(false);

  const handleTransitionChange = useCallback((next: Record<string, string[]>) => {
    setTransitions(next);
    setDirty(true);
  }, []);

  const handleTriggerChange = useCallback((next: WorkflowTrigger[]) => {
    setTriggers(next);
    setDirty(true);
  }, []);

  const handleSave = async () => {
    const saved = await onSave(projectId, { transitions, triggers });
    if (saved) {
      setDirty(false);
    }
  };

  const activeTransitionCount = useMemo(
    () =>
      Object.values(transitions).reduce((sum, targets) => sum + targets.length, 0),
    [transitions]
  );

  return (
    <div className="flex flex-col gap-6">
      {error ? (
        <div className="rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {error}
        </div>
      ) : null}

      <Card>
        <CardHeader>
          <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
            <div>
              <CardTitle>{t("workflowGraph")}</CardTitle>
              <CardDescription>
                {t("workflowGraphDesc")}
              </CardDescription>
            </div>
            <div className="flex flex-wrap items-center gap-2">
              <Badge variant={connected ? "secondary" : "outline"}>
                {connected ? t("realtimeLive") : t("realtimeDegraded")}
              </Badge>
              <Badge variant={dirty ? "outline" : "secondary"}>
                {dirty ? t("draftChanges") : t("persistedConfig")}
              </Badge>
            </div>
          </div>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          <WorkflowGraph transitions={transitions} />
          <div className="rounded-lg border border-border/60 bg-muted/20 p-4">
            <div className="mb-3 flex items-center justify-between gap-2">
              <h3 className="text-sm font-semibold">{t("triggerSummary")}</h3>
              <Badge variant="secondary">
                {triggers.length === 1 ? t("triggerCount", { count: triggers.length }) : t("triggerCountPlural", { count: triggers.length })}
              </Badge>
            </div>
            {triggers.length > 0 ? (
              <div className="flex flex-col gap-2">
                {triggers.map((trigger, index) => (
                  <div
                    key={`${trigger.fromStatus}-${trigger.toStatus}-${trigger.action}-${index}`}
                    className="rounded-md border border-border/60 bg-background px-3 py-2 text-sm"
                  >
                    <span className="font-medium">
                      {statusLabel(trigger.fromStatus)}
                    </span>
                    {" -> "}
                    <span className="font-medium">
                      {statusLabel(trigger.toStatus)}
                    </span>
                    {" · "}
                    <span className="text-muted-foreground">
                      {triggerLabel(trigger.action)}
                    </span>
                  </div>
                ))}
              </div>
            ) : (
              <p className="text-sm text-muted-foreground">
                {t("noTriggers")}
              </p>
            )}
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <div className="flex items-center justify-between gap-3">
            <div>
              <CardTitle>{t("statusTransitions")}</CardTitle>
              <CardDescription>
                {t("statusTransitionsDesc")}
              </CardDescription>
            </div>
            <Badge variant="secondary">
              {activeTransitionCount === 1 ? t("ruleCount", { count: activeTransitionCount }) : t("ruleCountPlural", { count: activeTransitionCount })}
            </Badge>
          </div>
        </CardHeader>
        <CardContent>
          {loading ? (
            <div className="text-sm text-muted-foreground">{t("loadingWorkflow")}</div>
          ) : (
            <TransitionEditor
              transitions={transitions}
              onChange={handleTransitionChange}
            />
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t("automationTriggers")}</CardTitle>
          <CardDescription>
            {t("automationTriggersDesc")}
          </CardDescription>
        </CardHeader>
        <CardContent>
          {loading ? (
            <div className="text-sm text-muted-foreground">{t("loading")}</div>
          ) : (
            <TriggerEditor triggers={triggers} onChange={handleTriggerChange} />
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t("recentActivity")}</CardTitle>
          <CardDescription>
            {t("recentActivityDesc")}
          </CardDescription>
        </CardHeader>
        <CardContent>
          {recentActivity.length > 0 ? (
            <div className="flex flex-col gap-3">
              {recentActivity.map((entry) => (
                <div
                  key={`${entry.taskId}-${entry.timestamp}-${entry.action}`}
                  className="rounded-lg border border-border/60 bg-muted/20 p-4"
                >
                  <div className="flex flex-wrap items-center gap-2 text-sm">
                    <Badge variant="outline">{entry.taskId || "unknown task"}</Badge>
                    <span className="font-medium">{statusLabel(entry.from)}</span>
                    <span className="text-muted-foreground">{t("to")}</span>
                    <span className="font-medium">{statusLabel(entry.to)}</span>
                  </div>
                  <p className="mt-2 text-sm text-muted-foreground">
                    {triggerLabel(entry.action)}
                    {connected
                      ? t("recordedLive")
                      : t("recordedLast")}
                  </p>
                </div>
              ))}
            </div>
          ) : (
            <div className="rounded-md border border-dashed px-4 py-6 text-sm text-muted-foreground">
              {t("noActivity")}
            </div>
          )}
        </CardContent>
      </Card>

      <div className="flex items-center gap-3">
        <Button
          type="button"
          disabled={!dirty || saving}
          onClick={() => void handleSave()}
        >
          {saving ? t("saving") : t("saveWorkflowConfig")}
        </Button>
        {dirty ? (
          <span className="text-sm text-muted-foreground">{t("unsavedChanges")}</span>
        ) : null}
      </div>
    </div>
  );
}
