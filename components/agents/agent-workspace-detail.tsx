"use client";

import { useEffect, useCallback } from "react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import {
  ArrowLeft,
  Pause,
  Play,
  Skull,
  Wrench,
  FileCode,
  Brain,
  CheckSquare,
  ShieldAlert,
  ScrollText,
} from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Progress } from "@/components/ui/progress";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Separator } from "@/components/ui/separator";
import { OutputStream } from "@/components/agent/output-stream";
import { DispatchHistoryPanel } from "@/components/tasks/dispatch-history-panel";
import { cn } from "@/lib/utils";
import { useAgentStore } from "@/lib/stores/agent-store";
import { useAuthStore } from "@/lib/stores/auth-store";
import { statusColors } from "./agent-status-colors";

interface AgentWorkspaceDetailProps {
  agentId: string;
  onBack: () => void;
}

export function AgentWorkspaceDetail({
  agentId,
  onBack,
}: AgentWorkspaceDetailProps) {
  const t = useTranslations("agents");
  const agent = useAgentStore((s) =>
    s.agents.find((a) => a.id === agentId),
  );
  const outputs = useAgentStore((s) => s.agentOutputs.get(agentId) ?? []);
  const pool = useAgentStore((s) => s.pool);
  const dispatchHistory = useAgentStore(
    (s) => s.dispatchHistoryByTask[agent?.taskId ?? ""] ?? [],
  );
  const toolCalls = useAgentStore((s) => s.agentToolCalls.get(agentId) ?? []);
  const toolResults = useAgentStore((s) => s.agentToolResults.get(agentId) ?? []);
  const reasoning = useAgentStore((s) => s.agentReasoning.get(agentId));
  const fileChanges = useAgentStore((s) => s.agentFileChanges.get(agentId) ?? []);
  const todos = useAgentStore((s) => s.agentTodos.get(agentId) ?? []);
  const partialMessage = useAgentStore((s) => s.agentPartialMessages.get(agentId));
  const permissionRequests = useAgentStore(
    (s) => s.agentPermissionRequests.get(agentId) ?? [],
  );
  const agentLogs = useAgentStore((s) => s.agentLogs.get(agentId) ?? []);
  const fetchAgent = useAgentStore((s) => s.fetchAgent);
  const fetchDispatchHistory = useAgentStore((s) => s.fetchDispatchHistory);
  const fetchAgentLogs = useAgentStore((s) => s.fetchAgentLogs);
  const pauseAgent = useAgentStore((s) => s.pauseAgent);
  const resumeAgent = useAgentStore((s) => s.resumeAgent);
  const killAgent = useAgentStore((s) => s.killAgent);

  const handlePermissionResponse = useCallback(
    async (requestId: string, approved: boolean) => {
      const backendUrl =
        process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";
      const token = useAuthStore.getState().accessToken;
      try {
        await fetch(
          `${backendUrl}/api/v1/bridge/permission-response/${requestId}`,
          {
            method: "POST",
            headers: {
              "Content-Type": "application/json",
              ...(token ? { Authorization: `Bearer ${token}` } : {}),
            },
            body: JSON.stringify({ approved }),
          },
        );
        useAgentStore.getState().removePermissionRequest(agentId, requestId);
      } catch {
        // Permission response delivery is best-effort
      }
    },
    [agentId],
  );

  useEffect(() => {
    void fetchAgent(agentId);
  }, [agentId, fetchAgent]);

  useEffect(() => {
    if (agent?.taskId) {
      void fetchDispatchHistory(agent.taskId);
    }
  }, [agent?.taskId, fetchDispatchHistory]);

  useEffect(() => {
    void fetchAgentLogs(agentId);
  }, [agentId, fetchAgentLogs]);

  if (!agent) {
    return (
      <div className="flex items-center justify-center py-20">
        <p className="text-muted-foreground">Agent not found</p>
      </div>
    );
  }

  const costPct =
    agent.budget > 0 ? Math.min((agent.cost / agent.budget) * 100, 100) : 0;

  return (
    <div className="flex flex-col gap-6 p-6">
      {/* Header */}
      <div className="flex items-center justify-between gap-4">
        <div className="flex items-center gap-3 min-w-0">
          <Button
            variant="ghost"
            size="icon"
            className="size-8 shrink-0"
            onClick={onBack}
          >
            <ArrowLeft className="size-4" />
          </Button>
          <div className="min-w-0">
            <h2 className="truncate text-xl font-bold">{agent.roleName}</h2>
            <p className="truncate text-sm text-muted-foreground">
              {agent.taskTitle}
            </p>
          </div>
        </div>
        <div className="flex shrink-0 gap-2">
          {agent.status === "running" && (
            <Button
              variant="outline"
              size="sm"
              onClick={() => pauseAgent(agent.id)}
            >
              <Pause className="mr-1 size-4" />
              {t("workspace.quickPause")}
            </Button>
          )}
          {agent.status === "paused" && (
            <Button
              variant="outline"
              size="sm"
              onClick={() => resumeAgent(agent.id)}
            >
              <Play className="mr-1 size-4" />
              {t("workspace.quickResume")}
            </Button>
          )}
          {(agent.status === "running" || agent.status === "paused") && (
            <Button
              variant="destructive"
              size="sm"
              onClick={() => killAgent(agent.id)}
            >
              <Skull className="mr-1 size-4" />
              {t("workspace.quickKill")}
            </Button>
          )}
        </div>
      </div>

      {/* Stats grid */}
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              {t("table.status")}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <Badge
              variant="secondary"
              className={cn(statusColors[agent.status])}
            >
              {t(`status.${agent.status}`)}
            </Badge>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              {t("table.runtime")}
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-1 text-sm">
            <p className="font-medium">{agent.runtime || "-"}</p>
            <p className="text-muted-foreground">
              {agent.provider || "-"} / {agent.model || "-"}
            </p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              {t("table.turns")}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <span className="text-2xl font-bold">{agent.turns}</span>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              {t("table.costBudget")}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <span className="text-2xl font-bold">
              ${agent.cost.toFixed(2)}
            </span>
            <span className="text-sm text-muted-foreground">
              {" "}
              / ${agent.budget.toFixed(2)}
            </span>
            <Progress
              value={costPct}
              aria-label="Budget usage"
              className="mt-2"
              indicatorClassName={costPct > 80 ? "bg-destructive" : undefined}
            />
          </CardContent>
        </Card>
      </div>

      {/* Permission requests banner */}
      {permissionRequests.length > 0 && (
        <Card className="border-amber-500/50 bg-amber-500/5">
          <CardHeader className="pb-2">
            <CardTitle className="flex items-center gap-2 text-sm font-medium text-amber-600 dark:text-amber-400">
              <ShieldAlert className="size-4" />
              {t("workspace.permissionRequests")} ({permissionRequests.length})
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-2">
            {permissionRequests.map((req) => (
              <div
                key={req.requestId}
                className="flex items-center justify-between rounded-md border p-3 text-sm"
              >
                <div className="min-w-0">
                  <p className="font-medium">{req.toolName ?? "Unknown tool"}</p>
                  {req.mcpServerId && (
                    <p className="text-xs text-muted-foreground">
                      MCP: {req.mcpServerId}
                    </p>
                  )}
                </div>
                <div className="flex shrink-0 gap-2">
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() =>
                      handlePermissionResponse(req.requestId, true)
                    }
                  >
                    {t("workspace.permissionRequests.approve")}
                  </Button>
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={() =>
                      handlePermissionResponse(req.requestId, false)
                    }
                  >
                    {t("workspace.permissionRequests.deny")}
                  </Button>
                </div>
              </div>
            ))}
          </CardContent>
        </Card>
      )}

      {/* Reasoning */}
      {reasoning && (
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="flex items-center gap-2 text-sm font-medium text-muted-foreground">
              <Brain className="size-4" />
              {t("workspace.reasoning")}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <pre className="max-h-48 overflow-auto whitespace-pre-wrap rounded-md bg-muted/50 p-3 text-xs">
              {reasoning}
            </pre>
          </CardContent>
        </Card>
      )}

      {/* Partial message (live streaming output) */}
      {partialMessage && (
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="flex items-center gap-2 text-sm font-medium text-muted-foreground">
              {t("workspace.partialMessage")}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <pre className="max-h-48 overflow-auto whitespace-pre-wrap rounded-md bg-muted/50 p-3 text-xs">
              {partialMessage}
            </pre>
          </CardContent>
        </Card>
      )}

      {/* Pool snapshot */}
      {pool && (
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              Pool Snapshot
            </CardTitle>
          </CardHeader>
          <CardContent className="grid gap-4 text-sm sm:grid-cols-4">
            <div>
              <p className="text-muted-foreground">Active</p>
              <p className="text-xl font-semibold">{pool.active}</p>
            </div>
            <div>
              <p className="text-muted-foreground">Available</p>
              <p className="text-xl font-semibold">{pool.available}</p>
            </div>
            <div>
              <p className="text-muted-foreground">Queued</p>
              <p className="text-xl font-semibold">{pool.queued ?? 0}</p>
            </div>
            <div>
              <p className="text-muted-foreground">Warm</p>
              <p className="text-xl font-semibold">{pool.warm ?? 0}</p>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Tool calls */}
      {toolCalls.length > 0 && (
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="flex items-center gap-2 text-sm font-medium text-muted-foreground">
              <Wrench className="size-4" />
              {t("workspace.toolCalls")} ({toolCalls.length})
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="max-h-64 space-y-2 overflow-auto">
              {toolCalls.map((call, i) => {
                const result = toolResults.find(
                  (r) => r.toolCallId === call.toolCallId,
                );
                return (
                  <div
                    key={call.toolCallId ?? i}
                    className="rounded-md border p-3 text-xs"
                  >
                    <div className="flex items-center justify-between">
                      <span className="font-mono font-medium">
                        {call.toolName}
                      </span>
                      {result && (
                        <Badge
                          variant="secondary"
                          className={cn(
                            result.isError
                              ? "bg-destructive/10 text-destructive"
                              : "bg-green-500/10 text-green-600",
                          )}
                        >
                          {result.isError ? "error" : "ok"}
                        </Badge>
                      )}
                    </div>
                    {call.input != null && (
                      <pre className="mt-1 max-h-20 overflow-auto whitespace-pre-wrap text-muted-foreground">
                        {typeof call.input === "string"
                          ? call.input
                          : JSON.stringify(call.input, null, 2)}
                      </pre>
                    )}
                  </div>
                );
              })}
            </div>
          </CardContent>
        </Card>
      )}

      {/* File changes */}
      {fileChanges.length > 0 && (
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="flex items-center gap-2 text-sm font-medium text-muted-foreground">
              <FileCode className="size-4" />
              {t("workspace.fileChanges")} ({fileChanges.length})
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="max-h-48 space-y-1 overflow-auto text-xs">
              {fileChanges.map((file, i) => (
                <div key={i} className="flex items-center gap-2">
                  <Badge variant="outline" className="shrink-0 text-[10px]">
                    {file.changeType ?? "modified"}
                  </Badge>
                  <span className="truncate font-mono">{file.path}</span>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      )}

      {/* Todos */}
      {todos.length > 0 && (
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="flex items-center gap-2 text-sm font-medium text-muted-foreground">
              <CheckSquare className="size-4" />
              {t("workspace.todos")} ({todos.length})
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="max-h-48 space-y-1 overflow-auto text-sm">
              {todos.map((todo, i) => (
                <div key={todo.id ?? i} className="flex items-center gap-2">
                  <span
                    className={cn(
                      "size-2 shrink-0 rounded-full",
                      todo.status === "completed"
                        ? "bg-green-500"
                        : todo.status === "in_progress"
                          ? "bg-blue-500"
                          : "bg-muted-foreground/40",
                    )}
                  />
                  <span
                    className={cn(
                      todo.status === "completed" && "line-through text-muted-foreground",
                    )}
                  >
                    {todo.content ?? todo.id}
                  </span>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      )}

      {/* Dispatch context */}
      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-sm font-medium text-muted-foreground">
            Dispatch Context
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-3 text-sm">
          {agent.dispatchStatus ? (
            <>
              <div className="flex items-center gap-2">
                <span className="text-muted-foreground">Outcome:</span>
                <Badge variant="secondary">{agent.dispatchStatus}</Badge>
                {agent.guardrailType && (
                  <Badge variant="outline">{agent.guardrailType}</Badge>
                )}
              </div>
              {agent.budget > 0 && (
                <div className="text-muted-foreground">
                  Budget: ${agent.budget.toFixed(2)}
                </div>
              )}
            </>
          ) : (
            <div className="text-muted-foreground">
              Manual spawn
              {agent.lastActivity && (
                <span>
                  {" "}
                  · {new Date(agent.lastActivity).toLocaleString()}
                </span>
              )}
            </div>
          )}
          {dispatchHistory.length > 0 && (
            <DispatchHistoryPanel attempts={dispatchHistory} />
          )}
          <div className="flex flex-wrap gap-2 pt-2">
            <Button asChild type="button" size="sm" variant="outline">
              <Link href={`/project?taskId=${agent.taskId}`}>Current Task</Link>
            </Button>
            <Button asChild type="button" size="sm" variant="outline">
              <Link href={`/reviews?taskId=${agent.taskId}`}>Review History</Link>
            </Button>
          </div>
        </CardContent>
      </Card>

      <Separator />

      {/* Output stream */}
      <div>
        <h2 className="mb-3 text-lg font-semibold">Output Stream</h2>
        <OutputStream lines={outputs} />
      </div>

      {/* Agent logs */}
      {agentLogs.length > 0 && (
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="flex items-center gap-2 text-sm font-medium text-muted-foreground">
              <ScrollText className="size-4" />
              {t("workspace.logs")} ({agentLogs.length})
            </CardTitle>
          </CardHeader>
          <CardContent>
            <ScrollArea className="h-64">
              <div className="space-y-1 font-mono text-xs">
                {agentLogs.map((log, i) => (
                  <div
                    key={i}
                    className={cn(
                      "flex gap-2 px-2 py-0.5 rounded",
                      log.type === "error" && "bg-destructive/10 text-destructive",
                      log.type === "status" && "text-muted-foreground",
                    )}
                  >
                    <span className="shrink-0 text-muted-foreground">
                      {new Date(log.timestamp).toLocaleTimeString()}
                    </span>
                    <Badge variant="outline" className="shrink-0 text-[10px] h-4">
                      {log.type}
                    </Badge>
                    <span className="whitespace-pre-wrap break-all">
                      {log.content}
                    </span>
                  </div>
                ))}
              </div>
            </ScrollArea>
          </CardContent>
        </Card>
      )}
    </div>
  );
}
