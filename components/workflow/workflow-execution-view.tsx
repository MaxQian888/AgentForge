"use client";

import { useCallback, useEffect, useState } from "react";
import {
  Play,
  CheckCircle,
  XCircle,
  Clock,
  Loader2,
  AlertCircle,
  SkipForward,
} from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import { createApiClient } from "@/lib/api-client";
import { useAuthStore } from "@/lib/stores/auth-store";

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";

interface WorkflowExecution {
  id: string;
  workflowId: string;
  projectId: string;
  taskId?: string;
  status: string;
  currentNodes: string[];
  errorMessage?: string;
  startedAt?: string;
  completedAt?: string;
  createdAt: string;
  updatedAt: string;
}

interface WorkflowNodeExecution {
  id: string;
  executionId: string;
  nodeId: string;
  status: string;
  result?: unknown;
  errorMessage?: string;
  startedAt?: string;
  completedAt?: string;
  createdAt: string;
}

interface WorkflowNodeData {
  id: string;
  type: string;
  label: string;
  position: { x: number; y: number };
  config?: Record<string, unknown>;
}

interface WorkflowEdgeData {
  id: string;
  source: string;
  target: string;
  condition?: string;
  label?: string;
}

const nodeStatusIcons: Record<string, React.ElementType> = {
  pending: Clock,
  running: Loader2,
  completed: CheckCircle,
  failed: XCircle,
  skipped: SkipForward,
};

const nodeStatusColors: Record<string, string> = {
  pending: "border-zinc-300 bg-zinc-50 dark:border-zinc-700 dark:bg-zinc-900",
  running: "border-blue-400 bg-blue-50 dark:border-blue-600 dark:bg-blue-950",
  completed:
    "border-green-400 bg-green-50 dark:border-green-600 dark:bg-green-950",
  failed: "border-red-400 bg-red-50 dark:border-red-600 dark:bg-red-950",
  skipped:
    "border-zinc-300 bg-zinc-50/50 dark:border-zinc-700 dark:bg-zinc-900/50",
};

const nodeTypeColors: Record<string, string> = {
  trigger: "text-green-600 dark:text-green-400",
  condition: "text-purple-600 dark:text-purple-400",
  agent_dispatch: "text-blue-600 dark:text-blue-400",
  notification: "text-yellow-600 dark:text-yellow-400",
  status_transition: "text-indigo-600 dark:text-indigo-400",
  gate: "text-red-600 dark:text-red-400",
  parallel_split: "text-orange-600 dark:text-orange-400",
  parallel_join: "text-orange-600 dark:text-orange-400",
};

function ExecutionStatusBadge({ status }: { status: string }) {
  const variant =
    status === "completed"
      ? "default"
      : status === "failed"
        ? "destructive"
        : "secondary";

  return (
    <Badge
      variant={variant}
      className={cn(
        status === "running" && "animate-pulse",
        status === "completed" &&
          "bg-green-500/15 text-green-700 dark:text-green-400",
        status === "cancelled" &&
          "bg-zinc-500/15 text-zinc-600 dark:text-zinc-400"
      )}
    >
      {status}
    </Badge>
  );
}

function NodeCard({
  node,
  nodeExec,
  isActive,
}: {
  node: WorkflowNodeData;
  nodeExec?: WorkflowNodeExecution;
  isActive: boolean;
}) {
  const status = nodeExec?.status ?? "pending";
  const StatusIcon = nodeStatusIcons[status] ?? Clock;

  return (
    <div
      className={cn(
        "flex items-center gap-3 rounded-lg border-2 p-3 transition-all",
        nodeStatusColors[status],
        isActive && "ring-2 ring-blue-400 ring-offset-2 dark:ring-offset-zinc-900"
      )}
    >
      <StatusIcon
        className={cn(
          "size-5 shrink-0",
          status === "running" && "animate-spin text-blue-500",
          status === "completed" && "text-green-500",
          status === "failed" && "text-red-500",
          status === "pending" && "text-zinc-400",
          status === "skipped" && "text-zinc-400"
        )}
      />
      <div className="flex flex-col gap-0.5 min-w-0">
        <div className="flex items-center gap-2">
          <span className="text-sm font-medium truncate">{node.label}</span>
          <Badge
            variant="outline"
            className={cn("text-[10px] shrink-0", nodeTypeColors[node.type])}
          >
            {node.type.replace(/_/g, " ")}
          </Badge>
        </div>
        {nodeExec?.errorMessage && (
          <span className="text-xs text-destructive truncate">
            {nodeExec.errorMessage}
          </span>
        )}
        {nodeExec?.startedAt && (
          <span className="text-xs text-muted-foreground">
            {new Date(nodeExec.startedAt).toLocaleTimeString()}
            {nodeExec.completedAt &&
              ` - ${new Date(nodeExec.completedAt).toLocaleTimeString()}`}
          </span>
        )}
      </div>
    </div>
  );
}

interface WorkflowExecutionViewProps {
  executionId: string;
  nodes: WorkflowNodeData[];
  edges?: WorkflowEdgeData[];
}

export function WorkflowExecutionView({
  executionId,
  nodes,
}: WorkflowExecutionViewProps) {
  const [execution, setExecution] = useState<WorkflowExecution | null>(null);
  const [nodeExecs, setNodeExecs] = useState<WorkflowNodeExecution[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchExecution = useCallback(async () => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return;

    try {
      const api = createApiClient(API_URL);
      const { data } = await api.get<
        WorkflowExecution & { nodeExecutions?: WorkflowNodeExecution[] }
      >(`/api/v1/executions/${executionId}`, { token });
      setExecution(data);
      setNodeExecs(data?.nodeExecutions ?? []);
      setError(null);
    } catch {
      setError("Unable to load execution");
    } finally {
      setLoading(false);
    }
  }, [executionId]);

  useEffect(() => {
    void fetchExecution();
    // Poll while running
    const interval = setInterval(() => {
      if (execution?.status === "running" || execution?.status === "pending") {
        void fetchExecution();
      }
    }, 3000);
    return () => clearInterval(interval);
  }, [fetchExecution, execution?.status]);

  const handleCancel = useCallback(async () => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return;
    try {
      const api = createApiClient(API_URL);
      await api.post(`/api/v1/executions/${executionId}/cancel`, {}, { token });
      void fetchExecution();
    } catch {
      // ignore
    }
  }, [executionId, fetchExecution]);

  if (loading) {
    return (
      <div className="flex flex-col gap-4">
        <Skeleton className="h-8 w-48" />
        <Skeleton className="h-64 w-full rounded-lg" />
      </div>
    );
  }

  if (error || !execution) {
    return (
      <div className="flex items-center gap-2 rounded-lg border border-dashed p-6 text-sm text-muted-foreground">
        <AlertCircle className="size-4" />
        {error ?? "Execution not found"}
      </div>
    );
  }

  // Build a map of nodeId → execution
  const nodeExecMap = new Map(nodeExecs.map((ne) => [ne.nodeId, ne]));
  const activeNodes = new Set(execution.currentNodes ?? []);

  // Topological sort: organize nodes by their position in the graph
  const sortedNodes = [...nodes].sort((a, b) => {
    const aY = a.position?.y ?? 0;
    const bY = b.position?.y ?? 0;
    if (aY !== bY) return aY - bY;
    return (a.position?.x ?? 0) - (b.position?.x ?? 0);
  });

  return (
    <div className="flex flex-col gap-4">
      {/* Execution header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <Play className="size-5 text-muted-foreground" />
          <div>
            <div className="flex items-center gap-2">
              <span className="font-medium">Execution</span>
              <ExecutionStatusBadge status={execution.status} />
            </div>
            <span className="text-xs text-muted-foreground">
              Started{" "}
              {execution.startedAt
                ? new Date(execution.startedAt).toLocaleString()
                : "pending"}
            </span>
          </div>
        </div>
        {(execution.status === "running" ||
          execution.status === "pending") && (
          <Button variant="destructive" size="sm" onClick={handleCancel}>
            <XCircle className="mr-1 size-4" />
            Cancel
          </Button>
        )}
      </div>

      {execution.errorMessage && (
        <div className="rounded-lg border border-destructive/50 bg-destructive/10 p-3 text-sm text-destructive">
          {execution.errorMessage}
        </div>
      )}

      {/* Node execution flow */}
      <Card>
        <CardHeader>
          <CardTitle className="text-sm font-medium">Node Execution</CardTitle>
        </CardHeader>
        <CardContent>
          <ScrollArea className="max-h-[400px]">
            <div className="flex flex-col gap-2">
              {sortedNodes.map((node) => (
                <NodeCard
                  key={node.id}
                  node={node}
                  nodeExec={nodeExecMap.get(node.id)}
                  isActive={activeNodes.has(node.id)}
                />
              ))}
            </div>
          </ScrollArea>
        </CardContent>
      </Card>

      {/* Execution log */}
      {nodeExecs.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle className="text-sm font-medium">
              Execution Log
            </CardTitle>
          </CardHeader>
          <CardContent>
            <ScrollArea className="max-h-64">
              <div className="flex flex-col gap-1 text-xs font-mono">
                {nodeExecs
                  .sort(
                    (a, b) =>
                      new Date(a.createdAt).getTime() -
                      new Date(b.createdAt).getTime()
                  )
                  .map((ne) => {
                    const node = nodes.find((n) => n.id === ne.nodeId);
                    return (
                      <div
                        key={ne.id}
                        className="flex items-center gap-2 text-muted-foreground"
                      >
                        <span className="shrink-0 w-20">
                          {new Date(ne.createdAt).toLocaleTimeString()}
                        </span>
                        <span
                          className={cn(
                            "shrink-0 w-20",
                            ne.status === "completed" && "text-green-600",
                            ne.status === "failed" && "text-red-600",
                            ne.status === "running" && "text-blue-600"
                          )}
                        >
                          [{ne.status}]
                        </span>
                        <span>{node?.label ?? ne.nodeId}</span>
                        {ne.errorMessage && (
                          <span className="text-destructive">
                            - {ne.errorMessage}
                          </span>
                        )}
                      </div>
                    );
                  })}
              </div>
            </ScrollArea>
          </CardContent>
        </Card>
      )}
    </div>
  );
}
