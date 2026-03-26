"use client";

import { useEffect, useMemo } from "react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
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
import { cn } from "@/lib/utils";
import { useAgentStore, type AgentStatus } from "@/lib/stores/agent-store";

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
        <CardTitle className="text-base">Pool Diagnostics</CardTitle>
        <CardDescription>
          Runtime health, warm reuse, and queue analysis.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="grid gap-4 sm:grid-cols-3">
          <div className="rounded-md border p-3">
            <p className="text-xs text-muted-foreground">Warm Reuse Ratio</p>
            <p className="text-lg font-semibold">{warmRatio}%</p>
            <p className="text-xs text-muted-foreground">
              {pool.warm ?? 0} warm / {pool.active} active
            </p>
          </div>
          <div className="rounded-md border p-3">
            <p className="text-xs text-muted-foreground">Pool Health</p>
            <p className="text-lg font-semibold">
              {pool.degraded ? (
                <span className="text-amber-600">Degraded</span>
              ) : (
                <span className="text-emerald-600">Healthy</span>
              )}
            </p>
            <p className="text-xs text-muted-foreground">
              {pool.available} slots available
            </p>
          </div>
          <div className="rounded-md border p-3">
            <p className="text-xs text-muted-foreground">Agent Distribution</p>
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
            <p className="text-sm font-medium mb-2">Blocked / Queued Reasons</p>
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
  const searchParams = useSearchParams();
  const { agents, fetchAgents, fetchPool, pool, loading } = useAgentStore();
  const requestedMemberId = searchParams.get("member");

  useEffect(() => {
    fetchAgents();
    fetchPool();
  }, [fetchAgents, fetchPool]);

  const visibleAgents = useMemo(
    () =>
      requestedMemberId
        ? agents.filter((agent) => agent.memberId === requestedMemberId)
        : agents,
    [agents, requestedMemberId]
  );

  return (
    <div className="flex flex-col gap-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Agent Monitor</h1>
        <Link
          href="/teams"
          className="inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground"
        >
          <Network className="size-4" />
          Agent Teams
        </Link>
      </div>

      {pool ? (
        <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-5">
          <Card>
            <CardContent className="py-4">
              <p className="text-sm text-muted-foreground">Active Slots</p>
              <p className="text-2xl font-bold">
                {pool.active} / {pool.max}
              </p>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="py-4">
              <p className="text-sm text-muted-foreground">Available Slots</p>
              <p className="text-2xl font-bold">{pool.available}</p>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="py-4">
              <p className="text-sm text-muted-foreground">Paused Sessions</p>
              <p className="text-2xl font-bold">{pool.pausedResumable}</p>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="py-4">
              <p className="text-sm text-muted-foreground">Warm Slots</p>
              <p className="text-2xl font-bold">{pool.warm ?? 0}</p>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="py-4">
              <p className="text-sm text-muted-foreground">Queued Admissions</p>
              <p className="text-2xl font-bold">{pool.queued ?? 0}</p>
            </CardContent>
          </Card>
        </div>
      ) : null}

      {pool ? (
        <PoolDiagnostics pool={pool} agents={agents} />
      ) : null}

      {pool?.queue?.length ? (
        <div className="rounded-md border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Queued Task</TableHead>
                <TableHead>Runtime</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Reason</TableHead>
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

      {loading ? (
        <p className="text-muted-foreground">Loading agents...</p>
      ) : visibleAgents.length === 0 ? (
        <Card>
          <CardContent className="py-12 text-center">
            <Bot className="mx-auto mb-4 size-12 text-muted-foreground" />
            <p className="text-muted-foreground">
              {requestedMemberId
                ? "No agents match the selected team member."
                : "No agents running. Spawn an agent from a task to get started."}
            </p>
          </CardContent>
        </Card>
      ) : (
        <div className="rounded-md border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Task</TableHead>
                <TableHead>Role</TableHead>
                <TableHead>Runtime</TableHead>
                <TableHead>Status</TableHead>
                <TableHead className="text-right">Turns</TableHead>
                <TableHead>Cost / Budget</TableHead>
                <TableHead>Last Activity</TableHead>
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
                    <TableCell>
                      <Badge
                        variant="secondary"
                        className={cn(statusColors[agent.status])}
                      >
                        {agent.status}
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
