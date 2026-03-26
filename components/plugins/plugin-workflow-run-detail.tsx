"use client";

import { useState } from "react";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import type {
  WorkflowPluginRun,
  WorkflowStepRunStatus,
} from "@/lib/stores/plugin-store";
import { ChevronDown, ChevronRight } from "lucide-react";

const stepStatusColors: Record<WorkflowStepRunStatus, string> = {
  pending: "bg-blue-500/15 text-blue-700 dark:text-blue-400",
  running: "bg-cyan-500/15 text-cyan-700 dark:text-cyan-400",
  completed: "bg-emerald-500/15 text-emerald-700 dark:text-emerald-400",
  failed: "bg-red-500/15 text-red-700 dark:text-red-400",
  skipped: "bg-zinc-500/15 text-zinc-600 dark:text-zinc-400",
};

function formatDuration(start?: string, end?: string): string {
  if (!start || !end) return "";
  const ms = new Date(end).getTime() - new Date(start).getTime();
  if (ms < 1000) return `${ms}ms`;
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`;
  return `${(ms / 60000).toFixed(1)}m`;
}

export function PluginWorkflowRunDetail({ run }: { run: WorkflowPluginRun }) {
  const [expandedSteps, setExpandedSteps] = useState<Set<string>>(new Set());

  const toggleStep = (stepId: string) => {
    setExpandedSteps((prev) => {
      const next = new Set(prev);
      if (next.has(stepId)) next.delete(stepId);
      else next.add(stepId);
      return next;
    });
  };

  return (
    <div className="flex flex-col gap-3 py-2">
      {/* Run header */}
      <div className="grid gap-1 text-xs text-muted-foreground">
        <p>
          Process: <span className="font-medium text-foreground">{run.process}</span>
        </p>
        <p>Started: {new Date(run.started_at).toLocaleString()}</p>
        {run.completed_at ? (
          <p>
            Completed: {new Date(run.completed_at).toLocaleString()}
            <span className="ml-1">
              ({formatDuration(run.started_at, run.completed_at)})
            </span>
          </p>
        ) : null}
      </div>

      {/* Step progress */}
      {run.steps && run.steps.length > 0 ? (
        <div className="flex flex-col gap-1">
          <p className="text-xs font-medium">Steps</p>
          {run.steps.map((step) => {
            const isCurrent = step.step_id === run.current_step_id;
            const isExpanded = expandedSteps.has(step.step_id);

            return (
              <div key={step.step_id}>
                <button
                  type="button"
                  className={cn(
                    "flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-xs hover:bg-muted/30",
                    isCurrent && "bg-primary/5 ring-1 ring-primary/20",
                  )}
                  onClick={() => toggleStep(step.step_id)}
                >
                  {isExpanded ? (
                    <ChevronDown className="size-3 shrink-0" />
                  ) : (
                    <ChevronRight className="size-3 shrink-0" />
                  )}
                  <span className="font-mono font-medium">{step.step_id}</span>
                  <Badge
                    variant="secondary"
                    className={cn("text-[10px]", stepStatusColors[step.status])}
                  >
                    {step.status}
                  </Badge>
                  <span className="text-muted-foreground">
                    {step.role_id} / {step.action}
                  </span>
                  {step.started_at && step.completed_at ? (
                    <span className="ml-auto text-muted-foreground">
                      {formatDuration(step.started_at, step.completed_at)}
                    </span>
                  ) : null}
                </button>

                {isExpanded ? (
                  <div className="ml-5 border-l border-border/40 pl-3 py-1">
                    {step.error ? (
                      <p className="text-xs text-destructive mb-1">
                        Error: {step.error}
                      </p>
                    ) : null}

                    {step.retry_count > 0 ? (
                      <p className="text-xs text-muted-foreground mb-1">
                        Retries: {step.retry_count}
                      </p>
                    ) : null}

                    {/* Attempts */}
                    {step.attempts && step.attempts.length > 0 ? (
                      <div className="flex flex-col gap-1">
                        <p className="text-[10px] font-medium text-muted-foreground">
                          Attempts
                        </p>
                        {step.attempts.map((attempt) => (
                          <div
                            key={attempt.attempt}
                            className="rounded border border-border/40 px-2 py-1 text-xs"
                          >
                            <div className="flex items-center gap-2">
                              <span>#{attempt.attempt}</span>
                              <Badge
                                variant="secondary"
                                className={cn(
                                  "text-[10px]",
                                  stepStatusColors[attempt.status],
                                )}
                              >
                                {attempt.status}
                              </Badge>
                              {attempt.completed_at ? (
                                <span className="text-muted-foreground">
                                  {formatDuration(
                                    attempt.started_at,
                                    attempt.completed_at,
                                  )}
                                </span>
                              ) : null}
                            </div>
                            {attempt.error ? (
                              <p className="mt-1 text-destructive">
                                {attempt.error}
                              </p>
                            ) : null}
                            {attempt.output &&
                            Object.keys(attempt.output).length > 0 ? (
                              <pre className="mt-1 max-h-[100px] overflow-auto text-[10px] text-muted-foreground">
                                {JSON.stringify(attempt.output, null, 2)}
                              </pre>
                            ) : null}
                          </div>
                        ))}
                      </div>
                    ) : null}
                  </div>
                ) : null}
              </div>
            );
          })}
        </div>
      ) : null}

      {/* Run error */}
      {run.error ? (
        <div className="rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-xs text-destructive">
          {run.error}
        </div>
      ) : null}
    </div>
  );
}
