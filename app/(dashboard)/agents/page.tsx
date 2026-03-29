"use client";

import { useEffect, useMemo } from "react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { useTranslations } from "next-intl";
import { Bot, Network } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent } from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { EventBadgeList } from "@/components/shared/event-badge-list";
import { cn } from "@/lib/utils";
import { useAgentStore, type AgentStatus } from "@/lib/stores/agent-store";
import { useDashboardStore } from "@/lib/stores/dashboard-store";

const statusColors: Record<AgentStatus, string> = {
  starting: "bg-blue-500/15 text-blue-700 dark:text-blue-400",
  running: "bg-green-500/15 text-green-700 dark:text-green-400",
  paused: "bg-yellow-500/15 text-yellow-700 dark:text-yellow-400",
  completed: "bg-blue-500/15 text-blue-700 dark:text-blue-400",
  failed: "bg-red-500/15 text-red-700 dark:text-red-400",
  cancelled: "bg-zinc-500/15 text-zinc-600 dark:text-zinc-400",
  budget_exceeded: "bg-amber-500/15 text-amber-700 dark:text-amber-400",
};

function PoolDiagnostics({
  pool,
  agents,
}: {
  pool: import("@/lib/stores/agent-store").AgentPoolSummary;
  agents: import("@/lib/stores/agent-store").Agent[];
}) {
  const t = useTranslations("agents");
  const reasonCounts = useMemo(() => {
    const counts: Record<string, number> = {};
    for (const entry of pool.queue ?? []) {
      const reason = entry.reason || "unspecified";
      counts[reason] = (counts[reason] ?? 0) + 1;
    }
    return counts;
  }, [pool.queue]);

  const statusCounts = useMemo(() => {
    const counts: Record<string, number> = {};
    for (const agent of agents) {
      counts[agent.status] = (counts[agent.status] ?? 0) + 1;
    }
    return counts;
  }, [agents]);

  const warmRatio =
    pool.active > 0 ? Math.round(((pool.warm ?? 0) / pool.active) * 100) : 0;

  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="text-base">{t("diagnostics.title")}</CardTitle>
        <CardDescription>
          {t("diagnostics.description")}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="grid gap-4 sm:grid-cols-3">
          <div className="rounded-md border p-3">
            <p className="text-xs text-muted-foreground">{t("diagnostics.warmReuseRatio")}</p>
            <p className="text-lg font-semibold">{warmRatio}%</p>
            <p className="text-xs text-muted-foreground">
              {t("diagnostics.warmActive", { warm: pool.warm ?? 0, active: pool.active })}
            </p>
          </div>
          <div className="rounded-md border p-3">
            <p className="text-xs text-muted-foreground">{t("diagnostics.poolHealth")}</p>
            <p className="text-lg font-semibold">
              {pool.degraded ? (
                <span className="text-amber-600">{t("diagnostics.degraded")}</span>
              ) : (
                <span className="text-emerald-600">{t("diagnostics.healthy")}</span>
              )}
            </p>
            <p className="text-xs text-muted-foreground">
              {t("diagnostics.slotsAvailable", { count: pool.available })}
            </p>
          </div>
          <div className="rounded-md border p-3">
            <p className="text-xs text-muted-foreground">{t("diagnostics.agentDistribution")}</p>
            <div className="flex flex-wrap gap-1 mt-1">
              {Object.entries(statusCounts).map(([status, count]) => (
                <Badge key={status} variant="secondary" className="text-xs">
                  {status}: {count}
                </Badge>
              ))}
            </div>
          </div>
        </div>
        {Object.keys(reasonCounts).length > 0 && (
          <div>
            <p className="text-sm font-medium mb-2">{t("diagnostics.blockedQueuedReasons")}</p>
            <div className="flex flex-wrap gap-2">
              {Object.entries(reasonCounts).map(([reason, count]) => (
                <Badge key={reason} variant="outline">
                  {reason}: {count}
                </Badge>
              ))}
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  );
}

export default function AgentsPage() {
  const t = useTranslations("agents");
  const searchParams = useSearchParams();
  const selectedProjectId = useDashboardStore((state) => state.selectedProjectId);
  const {
    agents,
    fetchAgents,
    fetchPool,
    fetchRuntimeCatalog,
    fetchBridgeHealth,
    fetchDispatchStats,
    resumeAgent,
    runtimeCatalog,
    bridgeHealth,
    dispatchStats,
    pool,
    loading,
  } = useAgentStore();
  const requestedMemberId = searchParams.get("member");

  useEffect(() => {
    fetchAgents();
    fetchPool();
    void fetchRuntimeCatalog();
    void fetchBridgeHealth();
    if (selectedProjectId) {
      void fetchDispatchStats(selectedProjectId);
    }
  }, [fetchAgents, fetchBridgeHealth, fetchDispatchStats, fetchPool, fetchRuntimeCatalog, selectedProjectId]);

  const visibleAgents = useMemo(
    () =>
      requestedMemberId
        ? agents.filter((agent) => agent.memberId === requestedMemberId)
        : agents,
    [agents, requestedMemberId]
  );
  const pausedAgents = useMemo(
    () => visibleAgents.filter((agent) => agent.status === "paused"),
    [visibleAgents],
  );
  const bridgeDegraded = bridgeHealth?.status === "degraded";

  return (
    <div className="flex flex-col gap-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">{t("monitor.title")}</h1>
        <Link
          href="/teams"
          className="inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground"
        >
          <Network className="size-4" />
          {t("monitor.teamsLink")}
        </Link>
      </div>

      {bridgeHealth ? (
        <Card className={bridgeDegraded ? "border-amber-500/40" : "border-emerald-500/30"}>
          <CardContent className="flex flex-col gap-2 py-4 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <div className="text-sm font-medium">Bridge Health</div>
              <div className="text-sm text-muted-foreground">
                Status: {bridgeHealth.status}
                {bridgeHealth.lastCheck ? `, last check ${new Date(bridgeHealth.lastCheck).toLocaleString()}` : ""}
              </div>
            </div>
            <div className="flex gap-3 text-xs text-muted-foreground">
              <span>Active {bridgeHealth.pool.active}</span>
              <span>Available {bridgeHealth.pool.available}</span>
              <span>Warm {bridgeHealth.pool.warm}</span>
            </div>
          </CardContent>
        </Card>
      ) : null}

      {pool ? (
        <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-5">
          <Card>
            <CardContent className="py-4">
              <p className="text-sm text-muted-foreground">{t("pool.activeSlots")}</p>
              <p className="text-2xl font-bold">
                {pool.active} / {pool.max}
              </p>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="py-4">
              <p className="text-sm text-muted-foreground">{t("pool.availableSlots")}</p>
              <p className="text-2xl font-bold">{pool.available}</p>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="py-4">
              <p className="text-sm text-muted-foreground">{t("pool.pausedSessions")}</p>
              <p className="text-2xl font-bold">{pool.pausedResumable}</p>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="py-4">
              <p className="text-sm text-muted-foreground">{t("pool.warmSlots")}</p>
              <p className="text-2xl font-bold">{pool.warm ?? 0}</p>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="py-4">
              <p className="text-sm text-muted-foreground">{t("pool.queuedAdmissions")}</p>
              <p className="text-2xl font-bold">{pool.queued ?? 0}</p>
            </CardContent>
          </Card>
        </div>
      ) : null}

      {pool ? (
        <PoolDiagnostics pool={pool} agents={agents} />
      ) : null}

      {dispatchStats ? (
        <div className="grid gap-4 md:grid-cols-3">
          <Card>
            <CardContent className="py-4">
              <p className="text-sm text-muted-foreground">{t("stats.outcomes")}</p>
              <div className="mt-2 flex flex-wrap gap-1">
                {Object.entries(dispatchStats.outcomes).map(([status, count]) => (
                  <Badge key={status} variant="secondary" className="text-xs">
                    {t(`dispatchStatus.${status}`)}: {count}
                  </Badge>
                ))}
              </div>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="py-4">
              <p className="text-sm text-muted-foreground">{t("stats.queueDepth")}</p>
              <p className="text-2xl font-bold">{dispatchStats.queueDepth}</p>
              <p className="text-xs text-muted-foreground">
                {dispatchStats.medianWaitSeconds != null
                  ? t("stats.medianWait", { seconds: dispatchStats.medianWaitSeconds.toFixed(0) })
                  : t("stats.medianWaitEmpty")}
              </p>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="py-4">
              <p className="text-sm text-muted-foreground">{t("stats.blockedReasons")}</p>
              <div className="mt-2 flex flex-wrap gap-1">
                {Object.entries(dispatchStats.blockedReasons).length > 0 ? (
                  Object.entries(dispatchStats.blockedReasons).map(([reason, count]) => (
                    <Badge key={reason} variant="outline" className="text-xs">
                      {t(`guardrail.${reason}`)}: {count}
                    </Badge>
                  ))
                ) : (
                  <span className="text-xs text-muted-foreground">{t("stats.none")}</span>
                )}
              </div>
            </CardContent>
          </Card>
        </div>
      ) : null}

      {runtimeCatalog?.runtimes?.length ? (
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
          {runtimeCatalog.runtimes.map((runtime) => (
            <Card key={runtime.runtime}>
              <CardHeader className="pb-3">
                <CardTitle className="text-base">{runtime.label}</CardTitle>
                <CardDescription>
                  {runtime.defaultProvider} / {runtime.defaultModel}
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-2 text-sm">
                <Badge variant={runtime.available && !bridgeDegraded ? "secondary" : "outline"}>
                  {runtime.available && !bridgeDegraded ? "Available" : "Unavailable"}
                </Badge>
                <div className="text-muted-foreground">
                  Providers: {runtime.compatibleProviders.join(", ") || "-"}
                </div>
                {runtime.diagnostics.length > 0 ? (
                  <div className="rounded-md border bg-muted/30 p-2 text-xs text-muted-foreground">
                    {runtime.diagnostics.map((diagnostic) => (
                      <p key={`${runtime.runtime}-${diagnostic.code}`}>{diagnostic.message}</p>
                    ))}
                  </div>
                ) : null}
              </CardContent>
            </Card>
          ))}
        </div>
      ) : null}

      {pool?.queue?.length ? (
        <div className="rounded-md border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("queue.task")}</TableHead>
                <TableHead>{t("queue.runtime")}</TableHead>
                <TableHead>{t("queue.priority")}</TableHead>
                <TableHead>{t("queue.status")}</TableHead>
                <TableHead>{t("queue.reason")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {pool.queue.map((entry) => (
                <TableRow key={entry.entryId}>
                  <TableCell className="font-medium">{entry.taskId}</TableCell>
                  <TableCell className="text-xs text-muted-foreground">
                    {entry.runtime || "-"}
                    <div>{entry.provider || "-"}</div>
                  </TableCell>
                  <TableCell className="text-xs text-muted-foreground">
                    {t(`priority.${priorityLabel(entry.priority)}`)}
                  </TableCell>
                  <TableCell>
                    <Badge variant="secondary">{entry.status}</Badge>
                  </TableCell>
                  <TableCell className="text-xs text-muted-foreground">
                    {entry.reason || "-"}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      ) : null}

      {pausedAgents.length > 0 ? (
        <div className="rounded-md border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Paused Task</TableHead>
                <TableHead>Runtime</TableHead>
                <TableHead>Last Activity</TableHead>
                <TableHead className="text-right">Action</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {pausedAgents.map((agent) => (
                <TableRow key={`paused-${agent.id}`}>
                  <TableCell className="font-medium">{agent.taskTitle}</TableCell>
                  <TableCell className="text-xs text-muted-foreground">
                    {agent.runtime || "-"}
                    <div>{agent.provider || "-"}</div>
                  </TableCell>
                  <TableCell className="text-xs text-muted-foreground">
                    {agent.lastActivity ? new Date(agent.lastActivity).toLocaleString() : "-"}
                  </TableCell>
                  <TableCell className="text-right">
                    <button
                      type="button"
                      className="text-sm text-primary hover:underline disabled:text-muted-foreground"
                      disabled={bridgeDegraded}
                      onClick={() => void resumeAgent(agent.id)}
                    >
                      Resume
                    </button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      ) : null}

      {loading ? (
        <p className="text-muted-foreground">{t("monitor.loading")}</p>
      ) : visibleAgents.length === 0 ? (
        <Card>
          <CardContent className="py-12 text-center">
            <Bot className="mx-auto mb-4 size-12 text-muted-foreground" />
            <p className="text-muted-foreground">
              {requestedMemberId
                ? t("empty.noMatch")
                : t("empty.noAgents")}
            </p>
          </CardContent>
        </Card>
      ) : (
        <div className="rounded-md border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("table.task")}</TableHead>
                <TableHead>{t("table.role")}</TableHead>
                <TableHead>{t("table.runtime")}</TableHead>
                <TableHead>{t("table.dispatch")}</TableHead>
                <TableHead>{t("table.status")}</TableHead>
                <TableHead className="text-right">{t("table.turns")}</TableHead>
                <TableHead>{t("table.costBudget")}</TableHead>
                <TableHead>{t("table.lastActivity")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {visibleAgents.map((agent) => {
                const costPct =
                  agent.budget > 0
                    ? (agent.cost / agent.budget) * 100
                    : 0;
                return (
                  <TableRow key={agent.id} className="cursor-pointer">
                    <TableCell>
                      <Link
                        href={`/agent?id=${agent.id}`}
                        className="font-medium hover:underline"
                      >
                        {agent.taskTitle}
                      </Link>
                    </TableCell>
                    <TableCell>{agent.roleName}</TableCell>
                    <TableCell className="text-xs text-muted-foreground">
                      {agent.runtime || "-"}
                      <div>{agent.provider || "-"}</div>
                    </TableCell>
                    <TableCell title={agent.guardrailType ? t(`guardrail.${agent.guardrailType}`) : undefined}>
                      <EventBadgeList
                        events={[t(`dispatchStatus.${agent.dispatchStatus ?? "started"}`)]}
                        emptyLabel={t("stats.none")}
                      />
                    </TableCell>
                    <TableCell>
                      <Badge
                        variant="secondary"
                        className={cn(statusColors[agent.status])}
                      >
                        {t(`status.${agent.status}`)}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-right">{agent.turns}</TableCell>
                    <TableCell>
                      <div className="flex items-center gap-2">
                        <div className="h-1.5 w-20 overflow-hidden rounded-full bg-muted">
                          <div
                            className={cn(
                              "h-full rounded-full",
                              costPct > 80 ? "bg-destructive" : "bg-primary"
                            )}
                            style={{
                              width: `${Math.min(costPct, 100)}%`,
                            }}
                          />
                        </div>
                        <span className="text-xs text-muted-foreground">
                          ${agent.cost.toFixed(2)}
                        </span>
                      </div>
                    </TableCell>
                    <TableCell className="text-xs text-muted-foreground">
                      {agent.lastActivity
                        ? new Date(agent.lastActivity).toLocaleString()
                        : "-"}
                    </TableCell>
                  </TableRow>
                );
              })}
            </TableBody>
          </Table>
        </div>
      )}
    </div>
  );
}

function priorityLabel(priority?: number): "low" | "normal" | "high" | "critical" {
  switch (priority) {
    case 30:
      return "critical";
    case 20:
      return "high";
    case 10:
      return "normal";
    default:
      return "low";
  }
}
