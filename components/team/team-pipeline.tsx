"use client";

import { ArrowRight } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { cn } from "@/lib/utils";
import { useAgentStore, type Agent, type AgentStatus } from "@/lib/stores/agent-store";
import type { AgentTeam, TeamStatus } from "@/lib/stores/team-store";

const agentStatusColors: Record<AgentStatus, string> = {
  starting: "bg-blue-500/15 text-blue-700 dark:text-blue-400",
  running: "bg-green-500/15 text-green-700 dark:text-green-400",
  paused: "bg-yellow-500/15 text-yellow-700 dark:text-yellow-400",
  completed: "bg-blue-500/15 text-blue-700 dark:text-blue-400",
  failed: "bg-red-500/15 text-red-700 dark:text-red-400",
  cancelled: "bg-zinc-500/15 text-zinc-600 dark:text-zinc-400",
  budget_exceeded: "bg-amber-500/15 text-amber-700 dark:text-amber-400",
};

const phaseStatusColors: Record<string, string> = {
  pending: "bg-zinc-400",
  starting: "bg-zinc-400",
  planning: "bg-blue-500",
  executing: "bg-emerald-500",
  reviewing: "bg-amber-500",
  completed: "bg-green-500",
  failed: "bg-red-500",
  cancelled: "bg-zinc-400",
};

function getPhaseStatus(teamStatus: TeamStatus, phase: "plan" | "execute" | "review"): string {
  const phaseOrder = ["planning", "executing", "reviewing", "completed"];
  const phaseIndex = { plan: 0, execute: 1, review: 2 };
  const statusIndex = phaseOrder.indexOf(teamStatus);
  const target = phaseIndex[phase];

  if (teamStatus === "failed" || teamStatus === "cancelled") return teamStatus;
  if (teamStatus === "pending") return "pending";
  if (statusIndex > target) return "completed";
  if (statusIndex === target) return teamStatus;
  return "pending";
}

function AgentPhaseCard({
  title,
  agent,
  phaseStatus,
}: {
  title: string;
  agent: Agent | undefined;
  phaseStatus: string;
}) {
  return (
    <Card className="flex-1">
      <CardHeader className="pb-2">
        <div className="flex items-center justify-between">
          <CardTitle className="text-sm font-medium">{title}</CardTitle>
          <div
            className={cn(
              "size-2.5 rounded-full",
              phaseStatusColors[phaseStatus] ?? "bg-zinc-400"
            )}
          />
        </div>
      </CardHeader>
      <CardContent>
        {agent ? (
          <div className="flex flex-col gap-2">
            <div className="flex items-center justify-between">
              <span className="text-sm font-medium">{agent.roleName}</span>
              <Badge
                variant="secondary"
                className={cn(agentStatusColors[agent.status])}
              >
                {agent.status}
              </Badge>
            </div>
            <div className="flex items-center gap-4 text-xs text-muted-foreground">
              <span>Turns: {agent.turns}</span>
              <span>${agent.cost.toFixed(2)}</span>
              <span>{agent.runtime || "-"}</span>
            </div>
          </div>
        ) : (
          <p className="text-sm text-muted-foreground">
            {phaseStatus === "pending" ? "Waiting..." : "No agent assigned"}
          </p>
        )}
      </CardContent>
    </Card>
  );
}

interface TeamPipelineProps {
  team: AgentTeam;
}

export function TeamPipeline({ team }: TeamPipelineProps) {
  const agents = useAgentStore((s) => s.agents);

  const plannerAgent = team.plannerRunId
    ? agents.find((a) => a.id === team.plannerRunId)
    : undefined;
  const coderAgents = team.coderRunIds
    .map((id) => agents.find((a) => a.id === id))
    .filter((a): a is Agent => a !== undefined);
  const reviewerAgent = team.reviewerRunId
    ? agents.find((a) => a.id === team.reviewerRunId)
    : undefined;

  const planStatus = getPhaseStatus(team.status, "plan");
  const executeStatus = getPhaseStatus(team.status, "execute");
  const reviewStatus = getPhaseStatus(team.status, "review");

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center gap-3">
        <AgentPhaseCard
          title="Plan"
          agent={plannerAgent}
          phaseStatus={planStatus}
        />
        <ArrowRight className="size-5 shrink-0 text-muted-foreground" />
        <div className="flex flex-1 flex-col gap-2">
          <Card>
            <CardHeader className="pb-2">
              <div className="flex items-center justify-between">
                <CardTitle className="text-sm font-medium">Execute</CardTitle>
                <div
                  className={cn(
                    "size-2.5 rounded-full",
                    phaseStatusColors[executeStatus] ?? "bg-zinc-400"
                  )}
                />
              </div>
            </CardHeader>
            <CardContent>
              {coderAgents.length > 0 ? (
                <div className="grid gap-2 sm:grid-cols-2">
                  {coderAgents.map((agent) => (
                    <div
                      key={agent.id}
                      className="flex flex-col gap-1 rounded-md border p-3"
                    >
                      <div className="flex items-center justify-between">
                        <span className="text-xs font-medium">
                          {agent.roleName}
                        </span>
                        <Badge
                          variant="secondary"
                          className={cn(
                            "text-[10px]",
                            agentStatusColors[agent.status]
                          )}
                        >
                          {agent.status}
                        </Badge>
                      </div>
                      <div className="flex items-center gap-3 text-xs text-muted-foreground">
                        <span>Turns: {agent.turns}</span>
                        <span>${agent.cost.toFixed(2)}</span>
                        <span>{agent.runtime || "-"}</span>
                      </div>
                    </div>
                  ))}
                </div>
              ) : (
                <p className="text-sm text-muted-foreground">
                  {executeStatus === "pending"
                    ? "Waiting for plan..."
                    : "No coders assigned"}
                </p>
              )}
            </CardContent>
          </Card>
        </div>
        <ArrowRight className="size-5 shrink-0 text-muted-foreground" />
        <AgentPhaseCard
          title="Review"
          agent={reviewerAgent}
          phaseStatus={reviewStatus}
        />
      </div>
    </div>
  );
}
