"use client";

import { useEffect } from "react";
import Link from "next/link";
import { Bot } from "lucide-react";
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
  running: "bg-green-500/15 text-green-700 dark:text-green-400",
  paused: "bg-yellow-500/15 text-yellow-700 dark:text-yellow-400",
  completed: "bg-blue-500/15 text-blue-700 dark:text-blue-400",
  failed: "bg-red-500/15 text-red-700 dark:text-red-400",
  killed: "bg-zinc-500/15 text-zinc-600 dark:text-zinc-400",
};

export default function AgentsPage() {
  const { agents, fetchAgents, loading } = useAgentStore();

  useEffect(() => {
    fetchAgents();
  }, [fetchAgents]);

  return (
    <div className="flex flex-col gap-6">
      <h1 className="text-2xl font-bold">Agent Monitor</h1>

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
