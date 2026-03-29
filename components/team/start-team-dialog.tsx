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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type { CodingAgentSelection } from "@/lib/stores/project-store";
import { useDashboardStore } from "@/lib/stores/dashboard-store";
import { useProjectStore } from "@/lib/stores/project-store";
import { useTeamStore } from "@/lib/stores/team-store";
import { RuntimeSelector } from "@/components/shared/runtime-selector";

const DEFAULT_TEAM_STRATEGY = "plan-code-review";
const EMPTY_RUNTIME_OPTIONS: NonNullable<
  NonNullable<ReturnType<typeof useProjectStore.getState>["currentProject"]>["codingAgentCatalog"]
>["runtimes"] = [];
const EMPTY_SELECTION: CodingAgentSelection = {
  runtime: "",
  provider: "",
  model: "",
};

interface StartTeamDialogProps {
  taskId: string;
  taskTitle: string;
  memberId: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function StartTeamDialog({
  taskId,
  taskTitle,
  memberId,
  open,
  onOpenChange,
}: StartTeamDialogProps) {
  const [budget, setBudget] = useState("10.00");
  const selectedProjectId = useDashboardStore((s) => s.selectedProjectId);
  const project = useProjectStore((s) => {
    if (s.currentProject) {
      return s.currentProject;
    }
    if (selectedProjectId) {
      return s.projects.find((item) => item.id === selectedProjectId) ?? null;
    }
    return s.projects[0] ?? null;
  });
  const catalog = project?.codingAgentCatalog;
  const runtimeOptions = catalog?.runtimes ?? EMPTY_RUNTIME_OPTIONS;
  const defaultSelection = catalog?.defaultSelection ?? EMPTY_SELECTION;

  const [selection, setSelection] = useState<CodingAgentSelection>(defaultSelection);
  const [strategy] = useState(DEFAULT_TEAM_STRATEGY);
  const [submitting, setSubmitting] = useState(false);

  const startTeam = useTeamStore((s) => s.startTeam);
  const selectedRuntime = useMemo(
    () => runtimeOptions.find((option) => option.runtime === selection.runtime) ?? runtimeOptions[0],
    [runtimeOptions, selection.runtime]
  );
  const runtimeDiagnostics = selectedRuntime?.diagnostics ?? [];
  const hasBlockingDiagnostics = runtimeDiagnostics.some((item) => item.blocking);

  useEffect(() => {
    if (!catalog) {
      return;
    }
    setSelection(defaultSelection);
  }, [catalog, defaultSelection]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (hasBlockingDiagnostics || !selectedRuntime?.available) {
      return;
    }
    setSubmitting(true);
    try {
      await startTeam(taskId, memberId, {
        totalBudgetUsd: parseFloat(budget) || 10,
        runtime: selection.runtime,
        provider: selection.provider,
        model: selection.model,
        strategy,
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
          <DialogTitle>Start Agent Team</DialogTitle>
          <DialogDescription>
            Launch a Planner, Coder(s), Reviewer pipeline for: {taskTitle}
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSubmit} className="flex flex-col gap-4">
          <div className="flex flex-col gap-2">
            <Label htmlFor="budget">Budget (USD)</Label>
            <Input
              id="budget"
              type="number"
              step="0.01"
              min="0.01"
              value={budget}
              onChange={(e) => setBudget(e.target.value)}
            />
          </div>

          <RuntimeSelector
            catalog={catalog}
            value={selection}
            onChange={setSelection}
            idPrefix="start-team"
          />

          <div className="flex flex-col gap-2">
            <Label htmlFor="strategy">Strategy</Label>
            <Select value={strategy} disabled>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value={DEFAULT_TEAM_STRATEGY}>
                  Planner &rarr; Coder &rarr; Reviewer
                </SelectItem>
              </SelectContent>
            </Select>
          </div>

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
            >
              Cancel
            </Button>
            <Button
              type="submit"
              disabled={submitting || hasBlockingDiagnostics || !selectedRuntime?.available}
            >
              {submitting ? "Starting..." : "Start Team"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
