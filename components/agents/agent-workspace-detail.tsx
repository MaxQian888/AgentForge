"use client";

import { useEffect } from "react";
import { useTranslations } from "next-intl";
import { ArrowLeft, Pause, Play, Skull } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Progress } from "@/components/ui/progress";
import { Separator } from "@/components/ui/separator";
import { OutputStream } from "@/components/agent/output-stream";
import { DispatchHistoryPanel } from "@/components/tasks/dispatch-history-panel";
import { cn } from "@/lib/utils";
import { useAgentStore } from "@/lib/stores/agent-store";
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
  const fetchAgent = useAgentStore((s) => s.fetchAgent);
  const fetchDispatchHistory = useAgentStore((s) => s.fetchDispatchHistory);
  const pauseAgent = useAgentStore((s) => s.pauseAgent);
  const resumeAgent = useAgentStore((s) => s.resumeAgent);
  const killAgent = useAgentStore((s) => s.killAgent);

  useEffect(() => {
    void fetchAgent(agentId);
  }, [agentId, fetchAgent]);

  useEffect(() => {
    if (agent?.taskId) {
      void fetchDispatchHistory(agent.taskId);
    }
  }, [agent?.taskId, fetchDispatchHistory]);

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
        </CardContent>
      </Card>

      <Separator />

      {/* Output stream */}
      <div>
        <h2 className="mb-3 text-lg font-semibold">Output Stream</h2>
        <OutputStream lines={outputs} />
      </div>
    </div>
  );
}
