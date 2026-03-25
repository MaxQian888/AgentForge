"use client";

import { useEffect } from "react";
import Link from "next/link";
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

export default function AgentsPage() {
  const { agents, fetchAgents, fetchPool, pool, loading } = useAgentStore();

  useEffect(() => {
    fetchAgents();
    fetchPool();
  }, [fetchAgents, fetchPool]);

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
      ) : agents.length === 0 ? (
        <Card>
          <CardContent className="py-12 text-center">
            <Bot className="mx-auto mb-4 size-12 text-muted-foreground" />
            <p className="text-muted-foreground">
              No agents running. Spawn an agent from a task to get started.
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
              {agents.map((agent) => {
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
