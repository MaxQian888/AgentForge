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
import { useDashboardStore } from "@/lib/stores/dashboard-store";
import { useProjectStore } from "@/lib/stores/project-store";
import { useTeamStore } from "@/lib/stores/team-store";

const DEFAULT_TEAM_STRATEGY = "plan-code-review";

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
  const runtimeOptions = catalog?.runtimes ?? [];
  const defaultSelection = catalog?.defaultSelection ?? {
    runtime: "",
    provider: "",
    model: "",
  };

  const [runtime, setRuntime] = useState(defaultSelection.runtime);
  const [provider, setProvider] = useState(defaultSelection.provider);
  const [model, setModel] = useState(defaultSelection.model);
  const [strategy] = useState(DEFAULT_TEAM_STRATEGY);
  const [submitting, setSubmitting] = useState(false);

  const startTeam = useTeamStore((s) => s.startTeam);
  const selectedRuntime = useMemo(
    () => runtimeOptions.find((option) => option.runtime === runtime) ?? runtimeOptions[0],
    [runtime, runtimeOptions]
  );
  const runtimeDiagnostics = selectedRuntime?.diagnostics ?? [];
  const catalogDiagnostics = useMemo(
    () =>
      runtimeOptions.flatMap((option) =>
        option.diagnostics.map((diagnostic) => ({
          runtime: option.label,
          code: diagnostic.code,
          message: diagnostic.message,
          blocking: diagnostic.blocking,
        }))
      ),
    [runtimeOptions]
  );
  const hasBlockingDiagnostics = runtimeDiagnostics.some((item) => item.blocking);

  useEffect(() => {
    if (!catalog) {
      return;
    }
    setRuntime(defaultSelection.runtime);
    setProvider(defaultSelection.provider);
    setModel(defaultSelection.model);
  }, [catalog, defaultSelection.model, defaultSelection.provider, defaultSelection.runtime]);

  const handleRuntimeChange = (nextRuntime: string) => {
    setRuntime(nextRuntime);
    const nextOption = runtimeOptions.find((option) => option.runtime === nextRuntime);
    if (!nextOption) {
      return;
    }
    setProvider(nextOption.defaultProvider);
    setModel(nextOption.defaultModel);
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (hasBlockingDiagnostics || !selectedRuntime?.available) {
      return;
    }
    setSubmitting(true);
    try {
      await startTeam(taskId, memberId, {
        totalBudgetUsd: parseFloat(budget) || 10,
        runtime,
        provider,
        model,
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

          <div className="flex flex-col gap-2">
            <Label htmlFor="runtime">Runtime</Label>
            <Select value={runtime} onValueChange={handleRuntimeChange}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {runtimeOptions.map((option) => (
                  <SelectItem key={option.runtime} value={option.runtime}>
                    {option.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="flex flex-col gap-2">
            <Label htmlFor="provider">Provider</Label>
            <Select value={provider} onValueChange={setProvider}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {(selectedRuntime?.compatibleProviders ?? []).map((option) => (
                  <SelectItem key={option} value={option}>
                    {option}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="flex flex-col gap-2">
            <Label htmlFor="model">Model</Label>
            <Select value={model} onValueChange={setModel}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {selectedRuntime && (
                  <SelectItem value={selectedRuntime.defaultModel}>
                    {selectedRuntime.defaultModel}
                  </SelectItem>
                )}
                {model && selectedRuntime?.defaultModel !== model && (
                  <SelectItem value={model}>{model}</SelectItem>
                )}
              </SelectContent>
            </Select>
          </div>

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

          {runtimeDiagnostics.length > 0 && (
            <div className="rounded-md border border-amber-500/40 bg-amber-500/10 p-3 text-sm">
              {runtimeDiagnostics.map((diagnostic) => (
                <p key={`${diagnostic.code}-${diagnostic.message}`}>
                  {diagnostic.message}
                </p>
              ))}
            </div>
          )}

          {catalogDiagnostics.length > 0 && (
            <div className="rounded-md border p-3 text-sm text-muted-foreground">
              {catalogDiagnostics.map((diagnostic) => (
                <p key={`${diagnostic.runtime}-${diagnostic.code}`}>
                  {diagnostic.runtime}: {diagnostic.message}
                </p>
              ))}
            </div>
          )}

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
