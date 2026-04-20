"use client";

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";

// WorkflowPluginRunBody renders the plugin-native detail section used by the
// unified workflow-run detail route (bridge-unified-run-view). The shape is
// the raw WorkflowPluginRun JSON returned by the backend; the component
// extracts the fields a reviewer typically needs — status, plugin id, step
// list with attempt history — without forcing the caller to thread a typed
// plugin-run interface through the unified surface.

type WorkflowStepAttempt = {
  attempt: number;
  status: string;
  error?: string;
  startedAt?: string;
  completedAt?: string;
};

type WorkflowStepRun = {
  stepId: string;
  roleId?: string;
  action?: string;
  status: string;
  retryCount?: number;
  error?: string;
  attempts?: WorkflowStepAttempt[];
  startedAt?: string;
  completedAt?: string;
};

type PluginRunBodyShape = {
  id: string;
  plugin_id?: string;
  pluginId?: string;
  status: string;
  current_step_id?: string;
  currentStepID?: string;
  steps?: WorkflowStepRun[];
  error?: string;
  started_at?: string;
  startedAt?: string;
  completed_at?: string;
  completedAt?: string;
};

const stepStatusColors: Record<string, string> = {
  pending: "border-zinc-300 bg-zinc-50 dark:border-zinc-700 dark:bg-zinc-900",
  running: "border-blue-400 bg-blue-50 dark:border-blue-600 dark:bg-blue-950",
  completed: "border-green-400 bg-green-50 dark:border-green-600 dark:bg-green-950",
  failed: "border-red-400 bg-red-50 dark:border-red-600 dark:bg-red-950",
  skipped: "border-zinc-300 bg-zinc-50/50 dark:border-zinc-700 dark:bg-zinc-900/50",
};

export function WorkflowPluginRunBody({ body }: { body: unknown }) {
  if (!body || typeof body !== "object") {
    return (
      <div className="rounded-lg border border-dashed p-6 text-sm text-muted-foreground">
        Plugin run body unavailable
      </div>
    );
  }
  const run = body as PluginRunBodyShape;
  const pluginID = run.plugin_id ?? run.pluginId ?? "";
  const steps = run.steps ?? [];

  return (
    <div className="flex flex-col gap-4">
      <Card>
        <CardHeader>
          <CardTitle className="text-sm font-medium">Plugin Run</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="flex items-center gap-3 text-xs">
            <Badge variant="outline">{pluginID}</Badge>
            <span className="text-muted-foreground">Status:</span>
            <Badge variant="secondary">{run.status}</Badge>
          </div>
          {run.error && (
            <p className="mt-2 text-xs text-destructive">{run.error}</p>
          )}
        </CardContent>
      </Card>
      <Card>
        <CardHeader>
          <CardTitle className="text-sm font-medium">
            Steps ({steps.length})
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="flex flex-col gap-2">
            {steps.length === 0 && (
              <p className="text-xs text-muted-foreground">No steps recorded</p>
            )}
            {steps.map((step) => (
              <div
                key={step.stepId}
                className={cn(
                  "flex flex-col gap-1 rounded-md border-2 p-2",
                  stepStatusColors[step.status] ?? stepStatusColors.pending
                )}
              >
                <div className="flex items-center gap-2 text-xs">
                  <span className="font-medium">{step.stepId}</span>
                  <Badge variant="secondary" className="text-[10px]">
                    {step.status}
                  </Badge>
                  {step.roleId && (
                    <span className="text-muted-foreground">
                      role: {step.roleId}
                    </span>
                  )}
                  {step.retryCount ? (
                    <span className="text-muted-foreground">
                      retries: {step.retryCount}
                    </span>
                  ) : null}
                </div>
                {step.error && (
                  <p className="text-xs text-destructive">{step.error}</p>
                )}
                {step.attempts && step.attempts.length > 1 && (
                  <div className="text-[10px] text-muted-foreground">
                    {step.attempts.length} attempts
                  </div>
                )}
              </div>
            ))}
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
