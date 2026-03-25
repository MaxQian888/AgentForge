"use client";

import { Badge } from "@/components/ui/badge";
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
  const costPct = agent.budget > 0 ? (agent.cost / agent.budget) * 100 : 0;

  return (
    <div className="flex flex-col gap-2 rounded-md border p-4">
      <div className="flex items-center justify-between">
        <span className="font-medium">{agent.roleName}</span>
        <Badge variant="secondary" className={cn(statusColors[agent.status])}>
          {agent.status}
        </Badge>
      </div>
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
      <div className="h-1.5 w-full overflow-hidden rounded-full bg-muted">
        <div
          className={cn(
            "h-full rounded-full transition-all",
            costPct > 80 ? "bg-destructive" : "bg-primary"
          )}
          style={{ width: `${Math.min(costPct, 100)}%` }}
        />
      </div>
    </div>
  );
}
