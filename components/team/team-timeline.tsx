"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { cn } from "@/lib/utils";
import { useAgentStore, type Agent } from "@/lib/stores/agent-store";
import type { AgentTeam } from "@/lib/stores/team-store";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";

interface PhaseSegment {
  label: string;
  role: "planner" | "coder" | "reviewer";
  status: string;
  agent?: Agent;
  agents?: Agent[];
  startMs: number;
  endMs: number;
  durationMs: number;
  cost: number;
  turns: number;
}

const phaseColors: Record<string, { bg: string; fill: string }> = {
  planning: {
    bg: "bg-blue-100 dark:bg-blue-950",
    fill: "bg-blue-500",
  },
  executing: {
    bg: "bg-emerald-100 dark:bg-emerald-950",
    fill: "bg-emerald-500",
  },
  reviewing: {
    bg: "bg-amber-100 dark:bg-amber-950",
    fill: "bg-amber-500",
  },
  completed: {
    bg: "bg-green-100 dark:bg-green-950",
    fill: "bg-green-500",
  },
  failed: {
    bg: "bg-red-100 dark:bg-red-950",
    fill: "bg-red-500",
  },
  cancelled: {
    bg: "bg-zinc-100 dark:bg-zinc-900",
    fill: "bg-zinc-400",
  },
  pending: {
    bg: "bg-zinc-100 dark:bg-zinc-900",
    fill: "bg-zinc-300 dark:bg-zinc-700",
  },
};

function getAgentTimeRange(
  agent: Agent,
  fallbackNow: number,
): { start: number; end: number } {
  const start = agent.startedAt
    ? new Date(agent.startedAt).getTime()
    : new Date(agent.createdAt).getTime();
  const end = agent.completedAt
    ? new Date(agent.completedAt).getTime()
    : fallbackNow;
  return { start, end };
}

function formatDuration(ms: number): string {
  if (ms < 1000) return "<1s";
  const seconds = Math.floor(ms / 1000);
  if (seconds < 60) return `${seconds}s`;
  const minutes = Math.floor(seconds / 60);
  const remSeconds = seconds % 60;
  if (minutes < 60) return `${minutes}m ${remSeconds}s`;
  const hours = Math.floor(minutes / 60);
  const remMinutes = minutes % 60;
  return `${hours}h ${remMinutes}m`;
}

function statusIcon(status: string): string {
  switch (status) {
    case "completed":
      return "✓";
    case "running":
    case "starting":
      return "●";
    case "failed":
      return "✗";
    case "cancelled":
      return "○";
    case "paused":
      return "‖";
    case "budget_exceeded":
      return "$";
    default:
      return "·";
  }
}

interface TeamTimelineProps {
  team: AgentTeam;
}

export function TeamTimeline({ team }: TeamTimelineProps) {
  const t = useTranslations("teams");
  const agents = useAgentStore((s) => s.agents);
  const [now] = useState(() => Date.now());

  const segments = (() => {
    const result: PhaseSegment[] = [];
    const teamStart = new Date(team.createdAt).getTime();

    const plannerAgent = team.plannerRunId
      ? agents.find((a) => a.id === team.plannerRunId)
      : undefined;
    const coderAgents = team.coderRunIds
      .map((id) => agents.find((a) => a.id === id))
      .filter((a): a is Agent => a !== undefined);
    const reviewerAgent = team.reviewerRunId
      ? agents.find((a) => a.id === team.reviewerRunId)
      : undefined;

    // Planner segment
    if (plannerAgent) {
      const { start, end } = getAgentTimeRange(plannerAgent, now);
      result.push({
        label: t("pipeline.plan"),
        role: "planner",
        status: plannerAgent.status,
        agent: plannerAgent,
        startMs: start,
        endMs: end,
        durationMs: end - start,
        cost: plannerAgent.cost,
        turns: plannerAgent.turns,
      });
    } else if (team.status === "planning" || team.status === "pending") {
      result.push({
        label: t("pipeline.plan"),
        role: "planner",
        status: team.status === "planning" ? "running" : "pending",
        startMs: teamStart,
        endMs: now,
        durationMs: now - teamStart,
        cost: 0,
        turns: 0,
      });
    }

    // Coder segments
    if (coderAgents.length > 0) {
      const coderStart = Math.min(
        ...coderAgents.map((a) => getAgentTimeRange(a, now).start)
      );
      const coderEnd = Math.max(
        ...coderAgents.map((a) => getAgentTimeRange(a, now).end)
      );
      const totalCost = coderAgents.reduce((sum, a) => sum + a.cost, 0);
      const totalTurns = coderAgents.reduce((sum, a) => sum + a.turns, 0);
      const allCompleted = coderAgents.every(
        (a) =>
          a.status === "completed" ||
          a.status === "failed" ||
          a.status === "cancelled"
      );
      const anyFailed = coderAgents.some((a) => a.status === "failed");

      result.push({
        label: `${t("pipeline.execute")} (${coderAgents.length})`,
        role: "coder",
        status: allCompleted
          ? anyFailed
            ? "failed"
            : "completed"
          : "running",
        agents: coderAgents,
        startMs: coderStart,
        endMs: coderEnd,
        durationMs: coderEnd - coderStart,
        cost: totalCost,
        turns: totalTurns,
      });
    } else if (
      team.status === "executing" ||
      (team.status !== "pending" &&
        team.status !== "planning" &&
        team.coderRunIds.length === 0 &&
        team.status !== "completed" &&
        team.status !== "failed" &&
        team.status !== "cancelled")
    ) {
      const plannerEnd = plannerAgent
        ? getAgentTimeRange(plannerAgent, now).end
        : teamStart;
      result.push({
        label: t("pipeline.execute"),
        role: "coder",
        status: team.status === "executing" ? "running" : "pending",
        startMs: plannerEnd,
        endMs: now,
        durationMs: now - plannerEnd,
        cost: 0,
        turns: 0,
      });
    }

    // Reviewer segment
    if (reviewerAgent) {
      const { start, end } = getAgentTimeRange(reviewerAgent, now);
      result.push({
        label: t("pipeline.review"),
        role: "reviewer",
        status: reviewerAgent.status,
        agent: reviewerAgent,
        startMs: start,
        endMs: end,
        durationMs: end - start,
        cost: reviewerAgent.cost,
        turns: reviewerAgent.turns,
      });
    } else if (team.status === "reviewing") {
      const coderEnd =
        coderAgents.length > 0
          ? Math.max(...coderAgents.map((a) => getAgentTimeRange(a, now).end))
          : now;
      result.push({
        label: t("pipeline.review"),
        role: "reviewer",
        status: "running",
        startMs: coderEnd,
        endMs: now,
        durationMs: now - coderEnd,
        cost: 0,
        turns: 0,
      });
    }

    return result;
  })();

  const totalDuration = segments.reduce((sum, s) => sum + s.durationMs, 0);

  if (segments.length === 0) {
    return (
      <div className="rounded-lg border p-4 text-center text-sm text-muted-foreground">
        {t("timeline.noData")}
      </div>
    );
  }

  return (
    <TooltipProvider delayDuration={200}>
      <div className="flex flex-col gap-2">
        <div className="flex items-center gap-1 text-xs text-muted-foreground">
          <span>{t("timeline.label")}</span>
          <span className="ml-auto">{formatDuration(totalDuration)}</span>
        </div>

        <div className="flex h-10 w-full gap-0.5 overflow-hidden rounded-lg">
          {segments.map((segment, i) => {
            const widthPercent =
              totalDuration > 0
                ? Math.max((segment.durationMs / totalDuration) * 100, 3)
                : 100 / segments.length;

            const phase =
              segment.role === "planner"
                ? "planning"
                : segment.role === "coder"
                  ? "executing"
                  : "reviewing";
            const colors =
              segment.status === "failed"
                ? phaseColors.failed
                : segment.status === "cancelled"
                  ? phaseColors.cancelled
                  : segment.status === "pending"
                    ? phaseColors.pending
                    : phaseColors[phase] ?? phaseColors.pending;

            return (
              <Tooltip key={i}>
                <TooltipTrigger asChild>
                  <div
                    className={cn(
                      "relative flex items-center justify-center overflow-hidden transition-all",
                      colors.bg,
                      i === 0 && "rounded-l-lg",
                      i === segments.length - 1 && "rounded-r-lg"
                    )}
                    style={{ width: `${widthPercent}%` }}
                  >
                    <div
                      className={cn(
                        "absolute inset-0 opacity-60",
                        colors.fill,
                        segment.status === "running" && "animate-pulse"
                      )}
                    />
                    <span className="relative z-10 text-xs font-medium text-white drop-shadow-sm">
                      {segment.label}
                    </span>
                  </div>
                </TooltipTrigger>
                <TooltipContent side="bottom" className="max-w-xs">
                  <div className="flex flex-col gap-1 text-xs">
                    <div className="flex items-center gap-1.5 font-medium">
                      <span>{statusIcon(segment.status)}</span>
                      <span>{segment.label}</span>
                      <span className="text-muted-foreground">
                        {segment.status}
                      </span>
                    </div>
                    <div className="text-muted-foreground">
                      Duration: {formatDuration(segment.durationMs)}
                    </div>
                    <div className="text-muted-foreground">
                      Cost: ${segment.cost.toFixed(2)} | Turns:{" "}
                      {segment.turns}
                    </div>
                    {segment.agents && segment.agents.length > 1 && (
                      <div className="mt-1 border-t pt-1 text-muted-foreground">
                        {segment.agents.map((a) => (
                          <div key={a.id} className="flex justify-between">
                            <span>
                              {statusIcon(a.status)} {a.roleName}
                            </span>
                            <span>
                              {a.turns}t / ${a.cost.toFixed(2)}
                            </span>
                          </div>
                        ))}
                      </div>
                    )}
                  </div>
                </TooltipContent>
              </Tooltip>
            );
          })}
        </div>

        {/* Phase labels below the bar */}
        <div className="flex items-center gap-4 text-xs text-muted-foreground">
          {segments.map((segment, i) => (
            <div key={i} className="flex items-center gap-1">
              <div
                className={cn(
                  "size-2 rounded-full",
                  segment.status === "failed"
                    ? "bg-red-500"
                    : segment.status === "completed"
                      ? "bg-green-500"
                      : segment.status === "running"
                        ? "bg-emerald-500 animate-pulse"
                        : "bg-zinc-400"
                )}
              />
              <span>{segment.label}</span>
              <span>
                {formatDuration(segment.durationMs)} | $
                {segment.cost.toFixed(2)}
              </span>
            </div>
          ))}
        </div>
      </div>
    </TooltipProvider>
  );
}
