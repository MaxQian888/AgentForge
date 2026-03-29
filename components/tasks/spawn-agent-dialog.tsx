"use client";

import { useEffect, useMemo, useState } from "react";
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
import { RuntimeSelector } from "@/components/shared/runtime-selector";
import type { CodingAgentSelection } from "@/lib/stores/project-store";
import { useAgentStore } from "@/lib/stores/agent-store";

interface SpawnAgentDialogProps {
  taskId: string;
  taskTitle: string;
  memberId: string;
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
  open,
  onOpenChange,
  onSpawnAgent,
}: SpawnAgentDialogProps) {
  const runtimeCatalog = useAgentStore((state) => state.runtimeCatalog);
  const bridgeHealth = useAgentStore((state) => state.bridgeHealth);
  const fetchRuntimeCatalog = useAgentStore((state) => state.fetchRuntimeCatalog);
  const fetchBridgeHealth = useAgentStore((state) => state.fetchBridgeHealth);
  const spawnAgent = useAgentStore((state) => state.spawnAgent);
  const [selection, setSelection] = useState<CodingAgentSelection>(EMPTY_SELECTION);
  const [budget, setBudget] = useState("5.00");
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    if (!open) {
      return;
    }
    void fetchRuntimeCatalog();
    void fetchBridgeHealth();
  }, [fetchBridgeHealth, fetchRuntimeCatalog, open]);

  useEffect(() => {
    if (!runtimeCatalog) {
      return;
    }
    setSelection(runtimeCatalog.defaultSelection);
  }, [runtimeCatalog]);

  const selectedRuntime = useMemo(
    () =>
      runtimeCatalog?.runtimes.find((option) => option.runtime === selection.runtime) ??
      runtimeCatalog?.runtimes[0],
    [runtimeCatalog, selection.runtime],
  );
  const bridgeDegraded = bridgeHealth?.status === "degraded";
  const runtimeBlocked = !selectedRuntime?.available || selectedRuntime.diagnostics.some((item) => item.blocking);

  const handleSubmit = async (event: React.FormEvent) => {
    event.preventDefault();
    if (bridgeDegraded || runtimeBlocked) {
      return;
    }

    setSubmitting(true);
    try {
      const spawn = onSpawnAgent ?? spawnAgent;
      await spawn(taskId, memberId, {
        runtime: selection.runtime,
        provider: selection.provider,
        model: selection.model,
        maxBudgetUsd: parseFloat(budget) || 5,
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
            Launch a single agent for: {taskTitle}
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSubmit} className="flex flex-col gap-4">
          {bridgeDegraded ? (
            <div className="rounded-md border border-amber-500/40 bg-amber-500/10 p-3 text-sm text-amber-900 dark:text-amber-100">
              Bridge is degraded. Spawn is temporarily unavailable until health checks recover.
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
            <Button type="submit" disabled={submitting || bridgeDegraded || runtimeBlocked}>
              {submitting ? "Starting..." : "Start Agent"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
