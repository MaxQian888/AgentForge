"use client";

import { useEffect, useMemo, useState } from "react";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { AlertTriangle, CheckCircle, Clock } from "lucide-react";
import { RuntimeSelector } from "@/components/shared/runtime-selector";
import type { CodingAgentSelection } from "@/lib/stores/project-store";
import { useAgentStore, type DispatchPreflightSummary } from "@/lib/stores/agent-store";
import { useRoleStore } from "@/lib/stores/role-store";
import { useProjectStore } from "@/lib/stores/project-store";
import { useTranslations } from "next-intl";

interface SpawnAgentDialogProps {
  taskId?: string;
  taskTitle?: string;
  memberId?: string;
  defaultTaskId?: string;
  defaultMemberId?: string;
  taskOptions?: Array<{ id: string; title: string }>;
  memberOptions?: Array<{ id: string; label: string }>;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSpawnAgent?: (
    taskId: string,
    memberId: string,
    options?: {
      runtime?: string;
      provider?: string;
      model?: string;
      maxBudgetUsd?: number;
      roleId?: string;
    },
  ) => Promise<void> | void;
}

const EMPTY_SELECTION: CodingAgentSelection = {
  runtime: "",
  provider: "",
  model: "",
};

export function SpawnAgentDialog({
  taskId,
  taskTitle,
  memberId,
  defaultTaskId,
  defaultMemberId,
  taskOptions,
  memberOptions,
  open,
  onOpenChange,
  onSpawnAgent,
}: SpawnAgentDialogProps) {
  const runtimeCatalog = useAgentStore((state) => state.runtimeCatalog);
  const bridgeHealth = useAgentStore((state) => state.bridgeHealth);
  const fetchRuntimeCatalog = useAgentStore((state) => state.fetchRuntimeCatalog);
  const fetchBridgeHealth = useAgentStore((state) => state.fetchBridgeHealth);
  const fetchDispatchPreflight = useAgentStore((state) => state.fetchDispatchPreflight);
  const spawnAgent = useAgentStore((state) => state.spawnAgent);
  const roles = useRoleStore((state) => state.roles);
  const fetchRoles = useRoleStore((state) => state.fetchRoles);
  const currentProjectId = useProjectStore((state) => state.currentProject?.id ?? "");
  const t = useTranslations("tasks");
  const [selection, setSelection] = useState<CodingAgentSelection>(EMPTY_SELECTION);
  const [budget, setBudget] = useState("5.00");
  const [selectedRoleId, setSelectedRoleId] = useState("");
  const [selectedTaskId, setSelectedTaskId] = useState("");
  const [selectedMemberId, setSelectedMemberId] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [preflight, setPreflight] = useState<DispatchPreflightSummary | null>(null);

  useEffect(() => {
    if (!open) {
      return;
    }
    void fetchRuntimeCatalog();
    void fetchBridgeHealth();
    void fetchRoles();
  }, [fetchBridgeHealth, fetchRoles, fetchRuntimeCatalog, open]);

  useEffect(() => {
    if (!runtimeCatalog) {
      return;
    }
    setSelection(runtimeCatalog.defaultSelection);
  }, [runtimeCatalog]);

  useEffect(() => {
    if (!open || taskId) {
      return;
    }

    const nextTaskId =
      (defaultTaskId &&
      taskOptions?.some((option) => option.id === defaultTaskId)
        ? defaultTaskId
        : taskOptions?.[0]?.id) ?? "";

    setSelectedTaskId(nextTaskId);
  }, [defaultTaskId, open, taskId, taskOptions]);

  useEffect(() => {
    if (!open || memberId) {
      return;
    }

    const nextMemberId =
      (defaultMemberId &&
      memberOptions?.some((option) => option.id === defaultMemberId)
        ? defaultMemberId
        : memberOptions?.[0]?.id) ?? "";

    setSelectedMemberId(nextMemberId);
  }, [defaultMemberId, memberId, memberOptions, open]);

  useEffect(() => {
    if (!open || !currentProjectId) {
      setPreflight(null);
      return;
    }
    const tid = taskId ?? selectedTaskId;
    const mid = memberId ?? selectedMemberId;
    if (!tid || !mid) {
      setPreflight(null);
      return;
    }
    const budgetUsd = parseFloat(budget);
    let cancelled = false;
    void fetchDispatchPreflight(currentProjectId, tid, mid, {
      runtime: selection.runtime || undefined,
      provider: selection.provider || undefined,
      model: selection.model || undefined,
      roleId: selectedRoleId || undefined,
      budgetUsd: Number.isNaN(budgetUsd) ? undefined : budgetUsd,
    }).then((result) => {
      if (!cancelled) setPreflight(result);
    });
    return () => { cancelled = true; };
  }, [open, currentProjectId, taskId, selectedTaskId, memberId, selectedMemberId, fetchDispatchPreflight, selection.runtime, selection.provider, selection.model, selectedRoleId, budget]);

  const selectedRuntime = useMemo(
    () =>
      runtimeCatalog?.runtimes.find((option) => option.runtime === selection.runtime) ??
      runtimeCatalog?.runtimes[0],
    [runtimeCatalog, selection.runtime],
  );
  const bridgeDegraded = bridgeHealth?.status === "degraded";
  const runtimeBlocked = !selectedRuntime?.available || selectedRuntime.diagnostics.some((item) => item.blocking);
  const effectiveTaskId = taskId ?? selectedTaskId;
  const effectiveMemberId = memberId ?? selectedMemberId;
  const effectiveTaskTitle =
    taskTitle ??
    taskOptions?.find((option) => option.id === effectiveTaskId)?.title ??
    "";
  const canSubmit =
    Boolean(effectiveTaskId) &&
    Boolean(effectiveMemberId) &&
    !bridgeDegraded &&
    !runtimeBlocked;

  const handleSubmit = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!canSubmit) {
      return;
    }

    setSubmitting(true);
    try {
      const spawn = onSpawnAgent ?? spawnAgent;
      await spawn(effectiveTaskId, effectiveMemberId, {
        runtime: selection.runtime,
        provider: selection.provider,
        model: selection.model,
        maxBudgetUsd: parseFloat(budget) || 5,
        roleId: selectedRoleId || undefined,
      });
      onOpenChange(false);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("spawn.title")}</DialogTitle>
          <DialogDescription>
            {t("spawn.description", { task: effectiveTaskTitle })}
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSubmit} className="flex flex-col gap-4">
          {bridgeDegraded ? (
            <Alert variant="destructive">
              <AlertTriangle className="h-4 w-4" />
              <AlertTitle>{t("spawn.bridgeDegraded")}</AlertTitle>
              <AlertDescription>
                {t("spawn.bridgeDegradedDescription")}
              </AlertDescription>
            </Alert>
          ) : null}

          {preflight && !bridgeDegraded && (
            <Alert variant={preflight.budgetBlocked ? "destructive" : preflight.dispatchOutcomeHint === "queued" ? "default" : "default"}>
              {preflight.budgetBlocked ? (
                <AlertTriangle className="h-4 w-4" />
              ) : preflight.dispatchOutcomeHint === "queued" ? (
                <Clock className="h-4 w-4" />
              ) : (
                <CheckCircle className="h-4 w-4" />
              )}
              <AlertTitle>
                {preflight.budgetBlocked
                  ? t("spawn.blocked")
                  : preflight.dispatchOutcomeHint === "queued"
                    ? t("spawn.willBeQueued")
                    : t("spawn.readyToDispatch")}
              </AlertTitle>
              <AlertDescription className="text-xs">
                {preflight.budgetBlocked?.message ??
                  preflight.budgetWarning?.message ??
                  (preflight.dispatchOutcomeHint === "queued"
                    ? t("spawn.poolQueued", { available: preflight.poolAvailable ?? 0, queued: preflight.poolQueued ?? 0 })
                    : t("spawn.poolAvailable", { available: preflight.poolAvailable ?? 0 }))}
              </AlertDescription>
            </Alert>
          )}

          {!taskId && taskOptions?.length ? (
            <div className="flex flex-col gap-2">
              <Label htmlFor="spawn-agent-task">{t("spawn.taskLabel")}</Label>
              <select
                id="spawn-agent-task"
                aria-label={t("spawn.taskLabel")}
                className="h-10 rounded-md border bg-background px-3 text-sm"
                value={selectedTaskId}
                onChange={(event) => setSelectedTaskId(event.target.value)}
              >
                {taskOptions.map((option) => (
                  <option key={option.id} value={option.id}>
                    {option.title}
                  </option>
                ))}
              </select>
            </div>
          ) : null}

          {!memberId && memberOptions?.length ? (
            <div className="flex flex-col gap-2">
              <Label htmlFor="spawn-agent-member">{t("spawn.memberLabel")}</Label>
              <select
                id="spawn-agent-member"
                aria-label={t("spawn.memberLabel")}
                className="h-10 rounded-md border bg-background px-3 text-sm"
                value={selectedMemberId}
                onChange={(event) => setSelectedMemberId(event.target.value)}
              >
                {memberOptions.map((option) => (
                  <option key={option.id} value={option.id}>
                    {option.label}
                  </option>
                ))}
              </select>
            </div>
          ) : null}

          <div className="flex flex-col gap-2">
            <Label htmlFor="spawn-agent-budget">{t("spawn.budgetLabel")}</Label>
            <Input
              id="spawn-agent-budget"
              type="number"
              step="0.01"
              min="0.01"
              value={budget}
              onChange={(event) => setBudget(event.target.value)}
            />
          </div>

          <div className="flex flex-col gap-2">
            <Label htmlFor="spawn-agent-role">{t("spawn.roleLabel")}</Label>
            <select
              id="spawn-agent-role"
              aria-label={t("spawn.roleLabel")}
              className="h-10 rounded-md border bg-background px-3 text-sm"
              value={selectedRoleId}
              onChange={(e) => setSelectedRoleId(e.target.value)}
            >
              <option value="">{t("spawn.noRole")}</option>
              {roles.map((role) => (
                <option key={role.metadata.id} value={role.metadata.id}>
                  {role.metadata.name}
                </option>
              ))}
            </select>
          </div>

          <RuntimeSelector
            catalog={runtimeCatalog}
            value={selection}
            onChange={setSelection}
            disabled={bridgeDegraded}
            idPrefix="spawn-agent"
          />

          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
              {t("spawn.cancel")}
            </Button>
            <Button type="submit" disabled={submitting || !canSubmit}>
              {submitting ? t("spawn.starting") : t("spawn.startAgent")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
