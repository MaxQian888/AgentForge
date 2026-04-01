"use client";

import { useMemo, useState } from "react";
import { ActivityIcon } from "lucide-react";
import { useTranslations } from "next-intl";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import type {
  Agent,
  AgentPoolSummary,
  BridgeHealthSummary,
  DispatchStatsSummary,
} from "@/lib/stores/agent-store";
import type {
  CodingAgentCatalog,
  CodingAgentRuntimeOption,
} from "@/lib/stores/project-store";
import { AgentGridView } from "./agent-grid-view";
import { AgentPoolQueueTable } from "./agent-pool-queue-table";

interface AgentWorkspaceOverviewProps {
  activeTab: "monitor" | "dispatch";
  agents: Agent[];
  pool: AgentPoolSummary | null;
  runtimeCatalog: CodingAgentCatalog | null;
  bridgeHealth: BridgeHealthSummary | null;
  dispatchStats: DispatchStatsSummary | null;
  selectedAgentId?: string | null;
  onSelectAgent?: (id: string) => void;
  onPause?: (id: string) => void;
  onResume?: (id: string) => void;
  onKill?: (id: string) => void;
}

type AgentStatusFilter = "all" | "running" | "paused" | "error";

function matchesStatusFilter(agent: Agent, filter: AgentStatusFilter) {
  switch (filter) {
    case "running":
      return agent.status === "running";
    case "paused":
      return agent.status === "paused";
    case "error":
      return agent.status === "failed" || agent.status === "budget_exceeded";
    case "all":
    default:
      return true;
  }
}

function PoolDiagnostics({
  pool,
  agents,
}: {
  pool: AgentPoolSummary;
  agents: Agent[];
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
        <CardDescription>{t("diagnostics.description")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="grid gap-4 sm:grid-cols-3">
          <div className="rounded-md border p-3">
            <p className="text-xs text-muted-foreground">
              {t("diagnostics.warmReuseRatio")}
            </p>
            <p className="text-lg font-semibold">{warmRatio}%</p>
            <p className="text-xs text-muted-foreground">
              {t("diagnostics.warmActive", {
                warm: pool.warm ?? 0,
                active: pool.active,
              })}
            </p>
          </div>
          <div className="rounded-md border p-3">
            <p className="text-xs text-muted-foreground">
              {t("diagnostics.poolHealth")}
            </p>
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
            <p className="text-xs text-muted-foreground">
              {t("diagnostics.agentDistribution")}
            </p>
            <div className="mt-1 flex flex-wrap gap-1">
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
            <p className="mb-2 text-sm font-medium">
              {t("diagnostics.blockedQueuedReasons")}
            </p>
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

function DispatchStatsView({ stats }: { stats: DispatchStatsSummary }) {
  const t = useTranslations("agents");
  return (
    <div className="grid gap-4 md:grid-cols-3">
      <Card>
        <CardContent className="py-4">
          <p className="text-sm text-muted-foreground">{t("stats.outcomes")}</p>
          <div className="mt-2 flex flex-wrap gap-1">
            {Object.entries(stats.outcomes).map(([status, count]) => (
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
          <p className="text-2xl font-bold">{stats.queueDepth}</p>
          <p className="text-xs text-muted-foreground">
            {stats.medianWaitSeconds != null
              ? t("stats.medianWait", {
                  seconds: stats.medianWaitSeconds.toFixed(0),
                })
              : t("stats.medianWaitEmpty")}
          </p>
        </CardContent>
      </Card>
      <Card>
        <CardContent className="py-4">
          <p className="text-sm text-muted-foreground">
            {t("stats.blockedReasons")}
          </p>
          <div className="mt-2 flex flex-wrap gap-1">
            {Object.entries(stats.blockedReasons).length > 0 ? (
              Object.entries(stats.blockedReasons).map(([reason, count]) => (
                <Badge key={reason} variant="outline" className="text-xs">
                  {t(`guardrail.${reason}`)}: {count}
                </Badge>
              ))
            ) : (
              <span className="text-xs text-muted-foreground">
                {t("stats.none")}
              </span>
            )}
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

export function AgentWorkspaceOverview({
  activeTab,
  agents,
  pool,
  runtimeCatalog,
  bridgeHealth,
  dispatchStats,
  selectedAgentId,
  onSelectAgent,
  onPause,
  onResume,
  onKill,
}: AgentWorkspaceOverviewProps) {
  const t = useTranslations("agents");
  const bridgeDegraded = bridgeHealth?.status === "degraded";
  const [statusFilter, setStatusFilter] = useState<AgentStatusFilter>("all");
  const filteredAgents = useMemo(
    () => agents.filter((agent) => matchesStatusFilter(agent, statusFilter)),
    [agents, statusFilter],
  );
  const filterCounts = useMemo(
    () => ({
      all: agents.length,
      running: agents.filter((agent) => matchesStatusFilter(agent, "running")).length,
      paused: agents.filter((agent) => matchesStatusFilter(agent, "paused")).length,
      error: agents.filter((agent) => matchesStatusFilter(agent, "error")).length,
    }),
    [agents],
  );

  return (
    <div className="flex flex-col gap-6 p-6">
      {activeTab === "monitor" && (
        <>
          <div className="flex flex-wrap gap-2">
            {(
              [
                ["all", t("workspace.filterAll")],
                ["running", t("workspace.filterRunning")],
                ["paused", t("workspace.filterPaused")],
                ["error", t("workspace.filterError")],
              ] satisfies Array<[AgentStatusFilter, string]>
            ).map(([filterKey, label]) => (
              <Button
                key={filterKey}
                type="button"
                size="sm"
                variant={statusFilter === filterKey ? "secondary" : "outline"}
                onClick={() => setStatusFilter(filterKey)}
              >
                {label} {filterCounts[filterKey]}
              </Button>
            ))}
          </div>

          {bridgeHealth && (
            <Alert
              className={
                bridgeDegraded
                  ? "border-amber-500/40 bg-amber-500/5"
                  : "border-emerald-500/30 bg-emerald-500/5"
              }
            >
              <ActivityIcon />
              <AlertTitle>Bridge Health</AlertTitle>
              <AlertDescription>
                <p>
                  Status: {bridgeHealth.status}
                  {bridgeHealth.lastCheck
                    ? `, last check ${new Date(bridgeHealth.lastCheck).toLocaleString()}`
                    : ""}
                </p>
                <div className="flex gap-3 text-xs text-muted-foreground">
                  <span>Active {bridgeHealth.pool.active}</span>
                  <span>Available {bridgeHealth.pool.available}</span>
                  <span>Warm {bridgeHealth.pool.warm}</span>
                </div>
              </AlertDescription>
            </Alert>
          )}

          {pool && (
            <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-5">
              <Card>
                <CardContent className="py-4">
                  <p className="text-sm text-muted-foreground">
                    {t("pool.activeSlots")}
                  </p>
                  <p className="text-2xl font-bold">
                    {pool.active} / {pool.max}
                  </p>
                </CardContent>
              </Card>
              <Card>
                <CardContent className="py-4">
                  <p className="text-sm text-muted-foreground">
                    {t("pool.availableSlots")}
                  </p>
                  <p className="text-2xl font-bold">{pool.available}</p>
                </CardContent>
              </Card>
              <Card>
                <CardContent className="py-4">
                  <p className="text-sm text-muted-foreground">
                    {t("pool.pausedSessions")}
                  </p>
                  <p className="text-2xl font-bold">{pool.pausedResumable}</p>
                </CardContent>
              </Card>
              <Card>
                <CardContent className="py-4">
                  <p className="text-sm text-muted-foreground">
                    {t("pool.warmSlots")}
                  </p>
                  <p className="text-2xl font-bold">{pool.warm ?? 0}</p>
                </CardContent>
              </Card>
              <Card>
                <CardContent className="py-4">
                  <p className="text-sm text-muted-foreground">
                    {t("pool.queuedAdmissions")}
                  </p>
                  <p className="text-2xl font-bold">{pool.queued ?? 0}</p>
                </CardContent>
              </Card>
            </div>
          )}

          <AgentGridView
            agents={filteredAgents}
            bridgeDegraded={bridgeDegraded}
            selectedAgentId={selectedAgentId}
            onSelectAgent={onSelectAgent}
            onPause={onPause}
            onResume={onResume}
            onKill={onKill}
          />

          {pool && <PoolDiagnostics pool={pool} agents={agents} />}

          {runtimeCatalog?.runtimes?.length ? (
            <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
              {runtimeCatalog.runtimes.map((runtime: CodingAgentRuntimeOption) => (
                <Card key={runtime.runtime}>
                  <CardHeader className="pb-3">
                    <CardTitle className="text-base">{runtime.label}</CardTitle>
                    <CardDescription>
                      {runtime.defaultProvider} / {runtime.defaultModel}
                    </CardDescription>
                  </CardHeader>
                  <CardContent className="space-y-2 text-sm">
                    <Badge
                      variant={
                        runtime.available && !bridgeDegraded
                          ? "secondary"
                          : "outline"
                      }
                    >
                      {runtime.available && !bridgeDegraded
                        ? "Available"
                        : "Unavailable"}
                    </Badge>
                    <div className="text-muted-foreground">
                      Providers: {runtime.compatibleProviders.join(", ") || "-"}
                    </div>
                    {runtime.diagnostics.length > 0 && (
                      <div className="rounded-md border bg-muted/30 p-2 text-xs text-muted-foreground">
                        {runtime.diagnostics.map((diagnostic) => (
                          <p key={`${runtime.runtime}-${diagnostic.code}`}>
                            {diagnostic.message}
                          </p>
                        ))}
                      </div>
                    )}
                  </CardContent>
                </Card>
              ))}
            </div>
          ) : null}

          {pool?.queue?.length ? (
            <AgentPoolQueueTable queue={pool.queue} />
          ) : null}

          {dispatchStats && <DispatchStatsView stats={dispatchStats} />}
        </>
      )}

      {activeTab === "dispatch" && (
        <>
          {dispatchStats ? (
            <DispatchStatsView stats={dispatchStats} />
          ) : (
            <p className="text-muted-foreground">{t("stats.none")}</p>
          )}

          {bridgeDegraded && bridgeHealth && (
            <Alert className="border-amber-500/40 bg-amber-500/5">
              <ActivityIcon />
              <AlertTitle>Bridge Health</AlertTitle>
              <AlertDescription>
                <p>
                  Status: {bridgeHealth.status}
                  {bridgeHealth.lastCheck
                    ? `, last check ${new Date(bridgeHealth.lastCheck).toLocaleString()}`
                    : ""}
                </p>
                <div className="flex gap-3 text-xs text-muted-foreground">
                  <span>Active {bridgeHealth.pool.active}</span>
                  <span>Available {bridgeHealth.pool.available}</span>
                  <span>Warm {bridgeHealth.pool.warm}</span>
                </div>
              </AlertDescription>
            </Alert>
          )}

          {pool && (
            <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
              <Card>
                <CardContent className="py-4">
                  <p className="text-sm text-muted-foreground">
                    {t("pool.activeSlots")}
                  </p>
                  <p className="text-2xl font-bold">
                    {pool.active} / {pool.max}
                  </p>
                </CardContent>
              </Card>
              <Card>
                <CardContent className="py-4">
                  <p className="text-sm text-muted-foreground">
                    {t("pool.availableSlots")}
                  </p>
                  <p className="text-2xl font-bold">{pool.available}</p>
                </CardContent>
              </Card>
              <Card>
                <CardContent className="py-4">
                  <p className="text-sm text-muted-foreground">
                    {t("pool.warmSlots")}
                  </p>
                  <p className="text-2xl font-bold">{pool.warm ?? 0}</p>
                </CardContent>
              </Card>
              <Card>
                <CardContent className="py-4">
                  <p className="text-sm text-muted-foreground">
                    {t("pool.queuedAdmissions")}
                  </p>
                  <p className="text-2xl font-bold">{pool.queued ?? 0}</p>
                </CardContent>
              </Card>
            </div>
          )}

          {pool?.queue?.length ? (
            <AgentPoolQueueTable queue={pool.queue} />
          ) : null}
        </>
      )}
    </div>
  );
}
