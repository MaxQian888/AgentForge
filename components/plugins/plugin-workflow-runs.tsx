"use client";

import { useCallback, useEffect, useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import {
  usePluginStore,
  type PluginRecord,
  type WorkflowRunStatus,
} from "@/lib/stores/plugin-store";
import { PluginWorkflowRunDetail } from "./plugin-workflow-run-detail";
import { Play } from "lucide-react";

const runStatusColors: Record<WorkflowRunStatus, string> = {
  pending: "bg-blue-500/15 text-blue-700 dark:text-blue-400",
  running: "bg-cyan-500/15 text-cyan-700 dark:text-cyan-400",
  completed: "bg-emerald-500/15 text-emerald-700 dark:text-emerald-400",
  failed: "bg-red-500/15 text-red-700 dark:text-red-400",
  cancelled: "bg-zinc-500/15 text-zinc-600 dark:text-zinc-400",
};

export function PluginWorkflowRuns({ plugin }: { plugin: PluginRecord }) {
  const runs = usePluginStore(
    (s) => s.workflowRuns[plugin.metadata.id] ?? [],
  );
  const fetchWorkflowRuns = usePluginStore((s) => s.fetchWorkflowRuns);
  const startWorkflowRun = usePluginStore((s) => s.startWorkflowRun);

  const [showTriggerForm, setShowTriggerForm] = useState(false);
  const [triggerJson, setTriggerJson] = useState("{}");
  const [triggerError, setTriggerError] = useState<string | null>(null);
  const [starting, setStarting] = useState(false);
  const [expandedRunId, setExpandedRunId] = useState<string | null>(null);

  useEffect(() => {
    void fetchWorkflowRuns(plugin.metadata.id);
  }, [plugin.metadata.id, fetchWorkflowRuns]);

  const handleStartRun = useCallback(async () => {
    setTriggerError(null);
    try {
      const trigger = JSON.parse(triggerJson) as Record<string, unknown>;
      setStarting(true);
      await startWorkflowRun(plugin.metadata.id, trigger);
      setShowTriggerForm(false);
      setTriggerJson("{}");
    } catch {
      setTriggerError("Invalid JSON trigger payload");
    } finally {
      setStarting(false);
    }
  }, [plugin.metadata.id, triggerJson, startWorkflowRun]);

  if (plugin.kind !== "WorkflowPlugin") {
    return (
      <p className="text-sm text-muted-foreground">
        Workflow runs are only available for WorkflowPlugin plugins.
      </p>
    );
  }

  return (
    <div className="flex flex-col gap-3">
      {/* Header with start button */}
      <div className="flex items-center justify-between">
        <p className="text-sm font-medium">Workflow Runs</p>
        <Button
          variant="outline"
          size="sm"
          onClick={() => setShowTriggerForm(!showTriggerForm)}
        >
          <Play className="mr-1 size-3.5" />
          Start Run
        </Button>
      </div>

      {/* Trigger form */}
      {showTriggerForm ? (
        <div className="rounded-lg border border-border/60 p-3">
          <p className="mb-2 text-xs font-medium">Trigger Payload (JSON)</p>
          <textarea
            className="min-h-[80px] w-full rounded-md border bg-background px-2 py-1 font-mono text-xs"
            value={triggerJson}
            onChange={(e) => {
              setTriggerJson(e.target.value);
              setTriggerError(null);
            }}
          />
          {triggerError ? (
            <p className="mt-1 text-xs text-destructive">{triggerError}</p>
          ) : null}
          <div className="mt-2 flex gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={() => setShowTriggerForm(false)}
            >
              Cancel
            </Button>
            <Button
              size="sm"
              disabled={starting}
              onClick={() => void handleStartRun()}
            >
              {starting ? "Starting..." : "Start"}
            </Button>
          </div>
        </div>
      ) : null}

      {/* Runs list */}
      {runs.length === 0 ? (
        <div className="flex h-[80px] items-center justify-center rounded-md border border-dashed text-sm text-muted-foreground">
          No workflow runs yet.
        </div>
      ) : (
        <div className="flex flex-col gap-2">
          {runs.map((run) => (
            <div key={run.id}>
              <button
                type="button"
                className={cn(
                  "flex w-full items-center justify-between rounded-lg border border-border/60 p-3 text-left text-sm hover:bg-muted/30",
                  expandedRunId === run.id && "border-primary/40",
                )}
                onClick={() =>
                  setExpandedRunId(
                    expandedRunId === run.id ? null : run.id,
                  )
                }
              >
                <div className="flex items-center gap-2">
                  <span className="font-mono text-xs text-muted-foreground">
                    {run.id.slice(0, 8)}
                  </span>
                  <Badge
                    variant="secondary"
                    className={cn("text-xs", runStatusColors[run.status])}
                  >
                    {run.status}
                  </Badge>
                </div>
                <div className="flex items-center gap-3 text-xs text-muted-foreground">
                  {run.current_step_id ? (
                    <span>Step: {run.current_step_id}</span>
                  ) : null}
                  <span>{new Date(run.started_at).toLocaleString()}</span>
                </div>
              </button>

              {expandedRunId === run.id ? (
                <div className="mt-1 ml-2 border-l-2 border-border/40 pl-3">
                  <PluginWorkflowRunDetail run={run} />
                </div>
              ) : null}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
