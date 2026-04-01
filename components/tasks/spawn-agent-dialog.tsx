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
    let cancelled = false;
    void fetchDispatchPreflight(currentProjectId, tid, mid).then((result) => {
      if (!cancelled) setPreflight(result);
    });
    return () => { cancelled = true; };
  }, [open, currentProjectId, taskId, selectedTaskId, memberId, selectedMemberId, fetchDispatchPreflight]);

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
          <DialogTitle>Start Agent</DialogTitle>
          <DialogDescription>
            Launch a single agent for: {effectiveTaskTitle}
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSubmit} className="flex flex-col gap-4">
          {bridgeDegraded ? (
            <Alert variant="destructive">
              <AlertTriangle className="h-4 w-4" />
              <AlertTitle>Bridge Degraded</AlertTitle>
              <AlertDescription>
                Spawn is temporarily unavailable until health checks recover.
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
                  ? "Blocked"
                  : preflight.dispatchOutcomeHint === "queued"
                    ? "Will be queued"
                    : "Ready to dispatch"}
              </AlertTitle>
              <AlertDescription className="text-xs">
                {preflight.budgetBlocked?.message ??
                  preflight.budgetWarning?.message ??
                  (preflight.dispatchOutcomeHint === "queued"
                    ? `Pool has ${preflight.poolAvailable ?? 0} slots available, ${preflight.poolQueued ?? 0} already queued.`
                    : `Pool has ${preflight.poolAvailable ?? 0} slots available.`)}
              </AlertDescription>
            </Alert>
          )}

          {!taskId && taskOptions?.length ? (
            <div className="flex flex-col gap-2">
              <Label htmlFor="spawn-agent-task">Task</Label>
              <select
                id="spawn-agent-task"
                aria-label="Task"
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
              <Label htmlFor="spawn-agent-member">Member</Label>
              <select
                id="spawn-agent-member"
                aria-label="Member"
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
            <Label htmlFor="spawn-agent-budget">Budget (USD)</Label>
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
            <Label htmlFor="spawn-agent-role">Role</Label>
            <select
              id="spawn-agent-role"
              aria-label="Role"
              className="h-10 rounded-md border bg-background px-3 text-sm"
              value={selectedRoleId}
              onChange={(e) => setSelectedRoleId(e.target.value)}
            >
              <option value="">No role (default)</option>
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
              Cancel
            </Button>
            <Button type="submit" disabled={submitting || !canSubmit}>
              {submitting ? "Starting..." : "Start Agent"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
