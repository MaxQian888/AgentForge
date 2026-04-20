"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Clock, Loader2, Layers, Package2 } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { cn } from "@/lib/utils";
import { EmptyState } from "@/components/shared/empty-state";
import {
  useWorkflowRunStore,
  type UnifiedRunEngine,
  type UnifiedRunRow,
  type UnifiedRunStatus,
} from "@/lib/stores/workflow-run-store";
import {
  useWorkflowStore,
  type WorkflowDefinition,
} from "@/lib/stores/workflow-store";
import { WorkflowExecutionView } from "./workflow-execution-view";
import { WorkflowPluginRunBody } from "./workflow-plugin-run-body";

type EngineFilter = "all" | UnifiedRunEngine;

const ENGINE_FILTERS: { value: EngineFilter; label: string }[] = [
  { value: "all", label: "All" },
  { value: "dag", label: "DAG" },
  { value: "plugin", label: "Plugin" },
];

const statusBadgeClass: Record<UnifiedRunStatus, string> = {
  pending: "bg-zinc-500/15 text-zinc-700 dark:text-zinc-400",
  running: "bg-blue-500/15 text-blue-700 dark:text-blue-400 animate-pulse",
  paused: "bg-amber-500/15 text-amber-700 dark:text-amber-400",
  completed: "bg-green-500/15 text-green-700 dark:text-green-400",
  failed: "bg-red-500/15 text-red-700 dark:text-red-400",
  cancelled: "bg-zinc-500/15 text-zinc-600 dark:text-zinc-400",
  unknown: "bg-zinc-500/15 text-zinc-600 dark:text-zinc-400",
};

function formatRelativeTime(iso?: string): string {
  if (!iso) return "—";
  const then = new Date(iso).getTime();
  if (Number.isNaN(then)) return "—";
  const diff = Date.now() - then;
  const abs = Math.abs(diff);
  const minute = 60 * 1000;
  const hour = 60 * minute;
  const day = 24 * hour;
  if (abs < minute) return "just now";
  if (abs < hour) return `${Math.floor(abs / minute)}m ago`;
  if (abs < day) return `${Math.floor(abs / hour)}h ago`;
  return `${Math.floor(abs / day)}d ago`;
}

function triggerLabel(row: UnifiedRunRow): string {
  switch (row.triggeredBy.kind) {
    case "trigger":
      return "Triggered";
    case "manual":
      return "Manual";
    case "sub_workflow":
      return "Sub-workflow";
    case "task":
      return "Task";
    default:
      return row.triggeredBy.kind;
  }
}

export function WorkflowRunsTab({ projectId }: { projectId: string }) {
  const {
    rows,
    summary,
    nextCursor,
    loading,
    filter,
    setFilter,
    fetchUnifiedRuns,
    fetchRunDetail,
    selectedDetail,
    detailLoading,
    clearDetail,
  } = useWorkflowRunStore();

  const [selected, setSelected] = useState<{
    engine: UnifiedRunEngine;
    runId: string;
  } | null>(null);

  useEffect(() => {
    void fetchUnifiedRuns(projectId);
  }, [projectId, fetchUnifiedRuns, filter.engine, filter.status]);

  // Poll the list every 5 seconds while on the tab so WS drops do not
  // leave the operator with a stale view. Cheap for typical run volumes.
  useEffect(() => {
    const interval = setInterval(() => {
      if (!selected) {
        void fetchUnifiedRuns(projectId);
      }
    }, 5000);
    return () => clearInterval(interval);
  }, [projectId, fetchUnifiedRuns, selected]);

  const handleEngineFilter = useCallback(
    (next: EngineFilter) => {
      setFilter({ ...filter, engine: next });
    },
    [filter, setFilter]
  );

  const handleLoadMore = useCallback(() => {
    if (!nextCursor) return;
    void fetchUnifiedRuns(projectId, { append: true, cursor: nextCursor });
  }, [nextCursor, fetchUnifiedRuns, projectId]);

  const handleSelect = useCallback(
    (row: UnifiedRunRow) => {
      setSelected({ engine: row.engine, runId: row.runId });
      void fetchRunDetail(projectId, row.engine, row.runId);
    },
    [fetchRunDetail, projectId]
  );

  const handleBack = useCallback(() => {
    setSelected(null);
    clearDetail();
  }, [clearDetail]);

  if (selected) {
    return (
      <UnifiedRunDetailView
        engine={selected.engine}
        runId={selected.runId}
        loading={detailLoading}
        detail={selectedDetail}
        onBack={handleBack}
      />
    );
  }

  const activeEngine: EngineFilter = (filter.engine as EngineFilter) ?? "all";

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between gap-3 flex-wrap">
        <div className="flex items-center gap-2">
          {ENGINE_FILTERS.map((f) => (
            <Button
              key={f.value}
              variant={activeEngine === f.value ? "default" : "outline"}
              size="sm"
              onClick={() => handleEngineFilter(f.value)}
              className="text-xs h-7"
            >
              {f.label}
            </Button>
          ))}
        </div>
        <div className="flex items-center gap-2 text-xs">
          <Badge className="bg-blue-500/15 text-blue-700 dark:text-blue-400">
            Running {summary.running}
          </Badge>
          <Badge className="bg-amber-500/15 text-amber-700 dark:text-amber-400">
            Paused {summary.paused}
          </Badge>
          <Badge className="bg-red-500/15 text-red-700 dark:text-red-400">
            Failed {summary.failed}
          </Badge>
        </div>
      </div>

      {loading && rows.length === 0 && (
        <div className="flex items-center gap-2 text-sm text-muted-foreground">
          <Loader2 className="h-4 w-4 animate-spin" />
          Loading runs...
        </div>
      )}

      {!loading && rows.length === 0 && (
        <EmptyState
          icon={Clock}
          title="No runs"
          description="No workflow runs in this project yet."
        />
      )}

      {rows.length > 0 && (
        <div className="grid gap-2">
          {rows.map((row) => (
            <Card
              key={`${row.engine}:${row.runId}`}
              className="cursor-pointer hover:bg-muted/50 transition-colors"
              onClick={() => handleSelect(row)}
            >
              <CardContent className="p-3">
                <div className="flex items-center justify-between gap-3">
                  <div className="flex items-center gap-2 min-w-0">
                    <Badge
                      variant="outline"
                      className={cn(
                        "text-[10px] shrink-0",
                        row.engine === "dag"
                          ? "text-violet-600 dark:text-violet-400"
                          : "text-cyan-600 dark:text-cyan-400"
                      )}
                    >
                      {row.engine === "dag" ? (
                        <>
                          <Layers className="inline h-3 w-3 mr-1" />
                          DAG
                        </>
                      ) : (
                        <>
                          <Package2 className="inline h-3 w-3 mr-1" />
                          Plugin
                        </>
                      )}
                    </Badge>
                    <span className="text-sm font-medium truncate">
                      {row.workflowRef.name || row.workflowRef.id}
                    </span>
                    <Badge
                      className={cn(
                        "text-[10px] shrink-0",
                        statusBadgeClass[row.status]
                      )}
                    >
                      {row.status}
                    </Badge>
                  </div>
                  <div className="flex items-center gap-2 text-xs text-muted-foreground">
                    <span>{triggerLabel(row)}</span>
                    {row.actingEmployeeId && (
                      <Badge variant="outline" className="text-[10px]">
                        Employee
                      </Badge>
                    )}
                    <span>{formatRelativeTime(row.startedAt)}</span>
                  </div>
                </div>
              </CardContent>
            </Card>
          ))}
          {nextCursor && (
            <div className="flex justify-center">
              <Button
                variant="outline"
                size="sm"
                onClick={handleLoadMore}
                disabled={loading}
              >
                {loading ? "Loading..." : "Load more"}
              </Button>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

interface UnifiedRunDetailViewProps {
  engine: UnifiedRunEngine;
  runId: string;
  loading: boolean;
  detail: ReturnType<typeof useWorkflowRunStore.getState>["selectedDetail"];
  onBack: () => void;
}

function UnifiedRunDetailView({
  engine,
  runId,
  loading,
  detail,
  onBack,
}: UnifiedRunDetailViewProps) {
  // For DAG runs we need the workflow definition (nodes) so the engine-native
  // body component can render its node flow. Pull from the existing workflow
  // store — by the time a caller reaches a DAG detail, the workflow
  // definitions list has been fetched for the project.
  const definitions = useWorkflowStore((s) => s.definitions);
  const workflowDef: WorkflowDefinition | undefined = useMemo(() => {
    if (!detail || detail.row.engine !== "dag") return undefined;
    return definitions.find((d) => d.id === detail.row.workflowRef.id);
  }, [definitions, detail]);

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center gap-2">
        <Button variant="ghost" size="sm" onClick={onBack}>
          Back to runs
        </Button>
        <span className="text-sm text-muted-foreground">
          / {engine} / {runId.slice(0, 8)}
        </span>
      </div>
      {loading && (
        <div className="flex items-center gap-2 text-sm text-muted-foreground">
          <Loader2 className="h-4 w-4 animate-spin" />
          Loading run detail...
        </div>
      )}
      {detail && (
        <>
          <SharedRunHeader detail={detail} />
          {detail.row.engine === "dag" ? (
            <WorkflowExecutionView
              engine="dag"
              executionId={detail.row.runId}
              nodes={workflowDef?.nodes ?? []}
              edges={workflowDef?.edges}
            />
          ) : (
            <WorkflowPluginRunBody body={detail.body} />
          )}
        </>
      )}
    </div>
  );
}

function SharedRunHeader({
  detail,
}: {
  detail: NonNullable<
    ReturnType<typeof useWorkflowRunStore.getState>["selectedDetail"]
  >;
}) {
  const row = detail.row;
  return (
    <Card>
      <CardContent className="p-3">
        <div className="flex items-center gap-3 flex-wrap">
          <Badge
            variant="outline"
            className={cn(
              "text-[10px]",
              row.engine === "dag"
                ? "text-violet-600 dark:text-violet-400"
                : "text-cyan-600 dark:text-cyan-400"
            )}
          >
            {row.engine.toUpperCase()}
          </Badge>
          <span className="text-sm font-medium">
            {row.workflowRef.name || row.workflowRef.id}
          </span>
          <Badge className={cn("text-[10px]", statusBadgeClass[row.status])}>
            {row.status}
          </Badge>
          <span className="text-xs text-muted-foreground">
            Started {formatRelativeTime(row.startedAt)}
          </span>
          {row.actingEmployeeId && (
            <Badge variant="outline" className="text-[10px]">
              Employee {row.actingEmployeeId.slice(0, 8)}
            </Badge>
          )}
          <span className="text-xs text-muted-foreground">
            {triggerLabel(row)}
            {row.triggeredBy.ref ? ` · ${row.triggeredBy.ref.slice(0, 8)}` : ""}
          </span>
          {row.parentLink && (
            <Badge variant="outline" className="text-[10px]">
              Parent {row.parentLink.parentExecutionId.slice(0, 8)}
            </Badge>
          )}
        </div>
      </CardContent>
    </Card>
  );
}
