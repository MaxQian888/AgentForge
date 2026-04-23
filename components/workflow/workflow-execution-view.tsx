"use client";

import { useCallback, useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import {
  Play,
  CheckCircle,
  XCircle,
  Clock,
  Loader2,
  AlertCircle,
  SkipForward,
  Hourglass,
} from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Skeleton } from "@/components/ui/skeleton";
import { Textarea } from "@/components/ui/textarea";
import { cn } from "@/lib/utils";
import { createApiClient } from "@/lib/api-client";
import { useAuthStore } from "@/lib/stores/auth-store";
import { toast } from "sonner";
import {
  useWorkflowStore,
  type WorkflowExecution,
  type WorkflowNodeExecution,
  type WorkflowNodeData,
  type WorkflowEdgeData,
} from "@/lib/stores/workflow-store";

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";

// Parent↔child linkage envelope surfaced by GET /api/v1/executions/:id after
// bridge-sub-workflow-invocation. `childEngineKind` is "dag" or "plugin".
interface SubWorkflowLinkDTO {
  id: string;
  parentExecutionId: string;
  parentNodeId: string;
  childEngineKind: string;
  childRunId: string;
  status: string;
}

interface ExecutionDetailResponse {
  execution: WorkflowExecution;
  nodeExecutions?: WorkflowNodeExecution[];
  subInvocations?: SubWorkflowLinkDTO[];
  invokedByParent?: SubWorkflowLinkDTO | null;
}

const nodeStatusIcons: Record<string, React.ElementType> = {
  pending: Clock,
  running: Loader2,
  completed: CheckCircle,
  failed: XCircle,
  skipped: SkipForward,
  waiting: Hourglass,
};

const nodeStatusColors: Record<string, string> = {
  pending: "border-zinc-300 bg-zinc-50 dark:border-zinc-700 dark:bg-zinc-900",
  running: "border-blue-400 bg-blue-50 dark:border-blue-600 dark:bg-blue-950",
  completed:
    "border-green-400 bg-green-50 dark:border-green-600 dark:bg-green-950",
  failed: "border-red-400 bg-red-50 dark:border-red-600 dark:bg-red-950",
  skipped:
    "border-zinc-300 bg-zinc-50/50 dark:border-zinc-700 dark:bg-zinc-900/50",
  waiting:
    "border-amber-400 bg-amber-50 dark:border-amber-600 dark:bg-amber-950",
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
  human_review: "text-emerald-600 dark:text-emerald-400",
  wait_event: "text-slate-600 dark:text-slate-400",
  llm_agent: "text-indigo-600 dark:text-indigo-400",
  function: "text-cyan-600 dark:text-cyan-400",
  loop: "text-pink-600 dark:text-pink-400",
  sub_workflow: "text-violet-600 dark:text-violet-400",
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

interface NodeCardProps {
  node: WorkflowNodeData;
  nodeExec?: WorkflowNodeExecution;
  isActive: boolean;
  executionId: string;
  onRefresh: () => void;
}

function NodeCard({
  node,
  nodeExec,
  isActive,
  executionId,
  onRefresh,
}: NodeCardProps) {
  const t = useTranslations("workflow");
  const status = nodeExec?.status ?? "pending";
  const StatusIcon = nodeStatusIcons[status] ?? Clock;

  // Review form state
  const [showReviewForm, setShowReviewForm] = useState(false);
  const [reviewComment, setReviewComment] = useState("");
  const [reviewSubmitting, setReviewSubmitting] = useState(false);

  // Event form state
  const [showEventForm, setShowEventForm] = useState(false);
  const [eventPayload, setEventPayload] = useState("");
  const [eventError, setEventError] = useState<string | null>(null);
  const [eventSubmitting, setEventSubmitting] = useState(false);

  const handleReview = useCallback(
    async (decision: "approved" | "rejected") => {
      setReviewSubmitting(true);
      try {
        await useWorkflowStore
          .getState()
          .resolveReview(executionId, node.id, decision, reviewComment);
        toast.success(
          decision === "approved" ? t("review.approvedToast") : t("review.rejectedToast")
        );
        setShowReviewForm(false);
        setReviewComment("");
        onRefresh();
      } catch {
        toast.error(t("review.failedToast"));
      } finally {
        setReviewSubmitting(false);
      }
    },
    [executionId, node.id, reviewComment, onRefresh, t]
  );

  const handleSendEvent = useCallback(async () => {
    setEventError(null);
    let parsed: unknown;
    try {
      parsed = JSON.parse(eventPayload);
    } catch {
      setEventError(t("event.invalidJson"));
      return;
    }
    setEventSubmitting(true);
    try {
      await useWorkflowStore
        .getState()
        .sendExternalEvent(executionId, node.id, parsed);
      toast.success(t("event.sentToast"));
      setShowEventForm(false);
      setEventPayload("");
      onRefresh();
    } catch {
      toast.error(t("event.failedToast"));
    } finally {
      setEventSubmitting(false);
    }
  }, [executionId, node.id, eventPayload, onRefresh, t]);

  return (
    <div
      className={cn(
        "flex flex-col gap-2 rounded-lg border-2 p-3 transition-all",
        nodeStatusColors[status],
        isActive && "ring-2 ring-blue-400 ring-offset-2 dark:ring-offset-zinc-900"
      )}
    >
      <div className="flex items-center gap-3">
        <StatusIcon
          className={cn(
            "size-5 shrink-0",
            status === "running" && "animate-spin text-blue-500",
            status === "completed" && "text-green-500",
            status === "failed" && "text-red-500",
            status === "pending" && "text-zinc-400",
            status === "skipped" && "text-zinc-400",
            status === "waiting" && "text-amber-500"
          )}
        />
        <div className="flex flex-col gap-0.5 min-w-0">
          <div className="flex items-center gap-2">
            <span className="text-sm font-medium truncate">{node.label}</span>
            <Badge
              variant="outline"
              className={cn("text-[10px] shrink-0", nodeTypeColors[node.type])}
            >
              {t(`node.type.${node.type}` as const) ?? node.type.replace(/_/g, " ")}
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

      {/* Inline review form for human_review nodes */}
      {status === "waiting" && node.type === "human_review" && (
        <div className="mt-2 space-y-2">
          <Badge className="bg-amber-500/15 text-amber-700 dark:text-amber-400 text-[10px]">
            {t("node.awaitingReview")}
          </Badge>
          {showReviewForm && (
            <div className="space-y-2">
              <Textarea
                placeholder={t("node.optionalComment")}
                value={reviewComment}
                onChange={(e) => setReviewComment(e.target.value)}
                className="text-xs"
                rows={2}
              />
              <div className="flex gap-2">
                <Button
                  size="sm"
                  className="bg-green-600 hover:bg-green-700 text-white text-xs h-7"
                  disabled={reviewSubmitting}
                  onClick={() => handleReview("approved")}
                >
                  {t("review.approve")}
                </Button>
                <Button
                  variant="destructive"
                  size="sm"
                  className="text-xs h-7"
                  disabled={reviewSubmitting}
                  onClick={() => handleReview("rejected")}
                >
                  {t("review.reject")}
                </Button>
              </div>
            </div>
          )}
          {!showReviewForm && (
            <Button
              variant="outline"
              size="sm"
              className="text-xs h-7"
              onClick={() => setShowReviewForm(true)}
            >
              {t("node.reviewButton")}
            </Button>
          )}
        </div>
      )}

      {/* Inline event form for wait_event nodes */}
      {status === "waiting" && node.type === "wait_event" && (
        <div className="mt-2 space-y-2">
          <Badge className="bg-slate-500/15 text-slate-700 dark:text-slate-400 text-[10px]">
            {t("node.waitingForEvent")}
          </Badge>
          {showEventForm && (
            <div className="space-y-2">
              <Textarea
                placeholder={t("node.eventPayloadPlaceholder")}
                value={eventPayload}
                onChange={(e) => setEventPayload(e.target.value)}
                className="text-xs font-mono"
                rows={3}
              />
              {eventError && (
                <p className="text-xs text-destructive">{eventError}</p>
              )}
              <Button
                size="sm"
                className="text-xs h-7"
                disabled={eventSubmitting}
                onClick={handleSendEvent}
              >
                {t("event.sendEvent")}
              </Button>
            </div>
          )}
          {!showEventForm && (
            <Button
              variant="outline"
              size="sm"
              className="text-xs h-7"
              onClick={() => setShowEventForm(true)}
            >
              {t("node.sendEventButton")}
            </Button>
          )}
        </div>
      )}
    </div>
  );
}

interface WorkflowExecutionViewProps {
  executionId: string;
  nodes: WorkflowNodeData[];
  edges?: WorkflowEdgeData[];
  // engine discriminator from bridge-unified-run-view. When omitted or "dag"
  // the component renders the DAG execution content below. When "plugin" it
  // returns early — callers should use a plugin-body component (e.g.
  // WorkflowPluginRunBody) instead. Kept here so the unified detail route
  // can pass the engine without knowing which component to mount.
  engine?: "dag" | "plugin";
}

export function WorkflowExecutionView({
  executionId,
  nodes,
  engine = "dag",
}: WorkflowExecutionViewProps) {
  const t = useTranslations("workflow");
  const [execution, setExecution] = useState<WorkflowExecution | null>(null);
  const [nodeExecs, setNodeExecs] = useState<WorkflowNodeExecution[]>([]);
  const [subInvocations, setSubInvocations] = useState<SubWorkflowLinkDTO[]>([]);
  const [invokedByParent, setInvokedByParent] = useState<SubWorkflowLinkDTO | null>(null);
  const outboundDeliveryFailed = useWorkflowStore(
    (s) => s.outboundDeliveryFailedExecIds.has(executionId)
  );
  const vcsDeliveryFailed = useWorkflowStore(
    (s) => s.vcsDeliveryFailedReviewIds.has(executionId)
  );
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [prevExecId, setPrevExecId] = useState<string | symbol>(Symbol("init"));
  if (prevExecId !== executionId) {
    setPrevExecId(executionId);
    setLoading(true);
    setError(null);
  }

  useEffect(() => {
    if (!executionId) return;
    const token = useAuthStore.getState().accessToken;
    if (!token) return;

    let cancelled = false;
    const api = createApiClient(API_URL);
    api.get<
      ExecutionDetailResponse | (WorkflowExecution & { nodeExecutions?: WorkflowNodeExecution[] })
    >(`/api/v1/executions/${executionId}`, { token })
      .then(({ data }) => {
        if (cancelled) return;
        const wrapped = (data as ExecutionDetailResponse).execution !== undefined
          ? (data as ExecutionDetailResponse)
          : null;
        if (wrapped) {
          setExecution(wrapped.execution);
          setNodeExecs(wrapped.nodeExecutions ?? []);
          setSubInvocations(wrapped.subInvocations ?? []);
          setInvokedByParent(wrapped.invokedByParent ?? null);
        } else {
          const legacy = data as WorkflowExecution & { nodeExecutions?: WorkflowNodeExecution[] };
          setExecution(legacy);
          setNodeExecs(legacy.nodeExecutions ?? []);
          setSubInvocations([]);
          setInvokedByParent(null);
        }
        setError(null);
      })
      .catch(() => {
        if (!cancelled) setError(t("execution.unableToLoad"));
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });

    return () => { cancelled = true; };
  }, [executionId, t]);

  const fetchForPoll = useCallback(async () => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return;
    try {
      const api = createApiClient(API_URL);
      const { data } = await api.get<
        ExecutionDetailResponse | (WorkflowExecution & { nodeExecutions?: WorkflowNodeExecution[] })
      >(`/api/v1/executions/${executionId}`, { token });
      const wrapped = (data as ExecutionDetailResponse).execution !== undefined
        ? (data as ExecutionDetailResponse)
        : null;
      if (wrapped) {
        setExecution(wrapped.execution);
        setNodeExecs(wrapped.nodeExecutions ?? []);
        setSubInvocations(wrapped.subInvocations ?? []);
        setInvokedByParent(wrapped.invokedByParent ?? null);
      } else {
        const legacy = data as WorkflowExecution & { nodeExecutions?: WorkflowNodeExecution[] };
        setExecution(legacy);
        setNodeExecs(legacy.nodeExecutions ?? []);
        setSubInvocations([]);
        setInvokedByParent(null);
      }
    } catch { /* poll errors silently */ }
  }, [executionId]);

  useEffect(() => {
    const interval = setInterval(() => {
      if (execution?.status === "running" || execution?.status === "pending") {
        void fetchForPoll();
      }
    }, 3000);
    return () => clearInterval(interval);
  }, [fetchForPoll, execution?.status]);

  const handleCancel = useCallback(async () => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return;
    try {
      const api = createApiClient(API_URL);
      await api.post(`/api/v1/executions/${executionId}/cancel`, {}, { token });
      void fetchForPoll();
    } catch {
      // ignore
    }
  }, [executionId, fetchForPoll]);

  if (engine !== "dag") {
    return null;
  }

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
        {error ?? t("execution.notFound")}
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
              <span className="font-medium">{t("execution.title")}</span>
              <ExecutionStatusBadge status={execution.status} />
              {outboundDeliveryFailed && (
                <Badge
                  variant="destructive"
                  className="text-[10px]"
                  title={t("execution.deliveryFailed.imTitle")}
                >
                  {t("execution.deliveryFailed.im")}
                </Badge>
              )}
              {vcsDeliveryFailed && (
                <Badge
                  variant="destructive"
                  className="text-[10px]"
                  title={t("execution.deliveryFailed.vcsTitle")}
                >
                  {t("execution.deliveryFailed.vcs")}
                </Badge>
              )}
            </div>
            <span className="text-xs text-muted-foreground">
              {t("execution.started")}{" "}
              {execution.startedAt
                ? new Date(execution.startedAt).toLocaleString()
                : t("execution.pending")}
            </span>
          </div>
        </div>
        {(execution.status === "running" ||
          execution.status === "pending") && (
          <Button variant="destructive" size="sm" onClick={handleCancel}>
            <XCircle className="mr-1 size-4" />
            {t("execution.cancel")}
          </Button>
        )}
      </div>

      {execution.errorMessage && (
        <div className="rounded-lg border border-destructive/50 bg-destructive/10 p-3 text-sm text-destructive">
          {execution.errorMessage}
        </div>
      )}

      {/* Sub-workflow linkage strip. Shows "invoked by parent" when this
          execution is a child run, and "sub-invocations" when this execution
          started one or more children. Hidden entirely when neither applies. */}
      {(invokedByParent || subInvocations.length > 0) && (
        <div
          className="flex flex-wrap items-center gap-2 rounded-lg border bg-muted/30 p-3 text-xs"
          data-testid="sub-workflow-linkage-strip"
        >
          {invokedByParent && (
            <Badge variant="outline" className="gap-1">
              <span className="text-muted-foreground">{t("execution.subWorkflow.invokedByParent")}</span>
              <span className="font-mono">
                {invokedByParent.parentExecutionId.slice(0, 8)}
              </span>
              <span className="text-muted-foreground">·</span>
              <span>{invokedByParent.parentNodeId}</span>
            </Badge>
          )}
          {subInvocations.length > 0 && (
            <Badge variant="outline" className="gap-1">
              <span className="text-muted-foreground">{t("execution.subWorkflow.subInvocations")}</span>
              <span className="font-mono">{subInvocations.length}</span>
            </Badge>
          )}
          {subInvocations.map((inv) => (
            <Badge
              key={inv.id}
              variant="secondary"
              className="gap-1 font-normal"
              data-testid="sub-invocation-badge"
            >
              <span className="text-muted-foreground">{inv.parentNodeId}</span>
              <span className="text-muted-foreground">→</span>
              <span className="uppercase text-[10px] tracking-wide">
                {inv.childEngineKind}
              </span>
              <span className="font-mono">
                {inv.childRunId.slice(0, 8)}
              </span>
              <span className="text-muted-foreground">({inv.status})</span>
            </Badge>
          ))}
        </div>
      )}

      {/* Node execution flow */}
      <Card>
        <CardHeader>
          <CardTitle className="text-sm font-medium">{t("execution.nodeExecution")}</CardTitle>
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
                  executionId={execution.id}
                  onRefresh={fetchForPoll}
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
              {t("execution.executionLog")}
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
