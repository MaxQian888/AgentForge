"use client";

import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Progress } from "@/components/ui/progress";
import { cn } from "@/lib/utils";
import type { Agent, AgentStatus } from "@/lib/stores/agent-store";

const statusColors: Record<AgentStatus, string> = {
  starting: "bg-blue-500/15 text-blue-700 dark:text-blue-400",
  running: "bg-green-500/15 text-green-700 dark:text-green-400",
  paused: "bg-yellow-500/15 text-yellow-700 dark:text-yellow-400",
  completed: "bg-blue-500/15 text-blue-700 dark:text-blue-400",
  failed: "bg-red-500/15 text-red-700 dark:text-red-400",
  cancelled: "bg-zinc-500/15 text-zinc-600 dark:text-zinc-400",
  budget_exceeded: "bg-amber-500/15 text-amber-700 dark:text-amber-400",
};

interface AgentCardProps {
  agent: Agent;
}

export function AgentCard({ agent }: AgentCardProps) {
  const costPct = agent.budget > 0 ? Math.min((agent.cost / agent.budget) * 100, 100) : 0;

  return (
    <Card>
      <CardHeader className="pb-3">
        <div className="flex items-center justify-between gap-3">
          <CardTitle className="text-base">{agent.roleName}</CardTitle>
          <Badge variant="secondary" className={cn(statusColors[agent.status])}>
            {agent.status}
          </Badge>
        </div>
      </CardHeader>
      <CardContent className="flex flex-col gap-3">
        <p className="text-sm text-muted-foreground">{agent.taskTitle}</p>
        <p className="text-xs text-muted-foreground">
          Runtime: {agent.runtime || "-"} / {agent.provider || "-"} / {agent.model || "-"}
        </p>
        <div className="flex items-center gap-4 text-xs text-muted-foreground">
          <span>Turns: {agent.turns}</span>
          <span>
            Cost: ${agent.cost.toFixed(2)} / ${agent.budget.toFixed(2)}
          </span>
        </div>
        <Progress
          value={costPct}
          aria-label="Budget usage"
          indicatorClassName={costPct > 80 ? "bg-destructive" : undefined}
        />
      </CardContent>
    </Card>
  );
}
