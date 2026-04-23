"use client";

import { Pause, Play, Skull } from "lucide-react";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Progress } from "@/components/ui/progress";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";
import type { Agent } from "@/lib/stores/agent-store";
import { statusDotColors } from "./agent-status-colors";

interface AgentSidebarItemProps {
  agent: Agent;
  selected: boolean;
  onSelect: (id: string) => void;
  onPause: (id: string) => void;
  onResume: (id: string) => void;
  onKill: (id: string) => void;
  bridgeDegraded: boolean;
}

export function AgentSidebarItem({
  agent,
  selected,
  onSelect,
  onPause,
  onResume,
  onKill,
  bridgeDegraded,
}: AgentSidebarItemProps) {
  const t = useTranslations("agents");
  const costPct =
    agent.budget > 0 ? Math.min((agent.cost / agent.budget) * 100, 100) : 0;
  const isActive = agent.status === "running" || agent.status === "paused";
  const handleSelect = () => onSelect(agent.id);

  return (
    <div
      role="button"
      tabIndex={0}
      aria-pressed={selected}
      className={cn(
        "group flex w-full flex-col gap-1 rounded-md px-2.5 py-2 text-left text-sm transition-colors hover:bg-accent/50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/50",
        selected && "bg-accent",
      )}
      onClick={handleSelect}
      onKeyDown={(event) => {
        if (event.key === "Enter" || event.key === " ") {
          event.preventDefault();
          handleSelect();
        }
      }}
    >
      <div className="flex items-center gap-2 min-w-0">
        <span
          className={cn(
            "size-2 shrink-0 rounded-full",
            statusDotColors[agent.status],
            agent.status === "running" && "animate-pulse",
          )}
        />
        <span className="truncate font-medium">{agent.taskTitle}</span>
      </div>
      <div className="flex items-center justify-between gap-2 pl-4">
        <span className="truncate text-xs text-muted-foreground">
          {agent.roleName}
        </span>
        <div className="flex shrink-0 items-center gap-1">
          {agent.budget > 0 && (
            <Progress
              value={costPct}
              aria-label={t("card.budgetUsage")}
              className="h-1 w-10"
              indicatorClassName={costPct > 80 ? "bg-destructive" : undefined}
            />
          )}
          <span className="text-[10px] text-muted-foreground">
            ${agent.cost.toFixed(2)}
          </span>
        </div>
      </div>
      {/* Quick actions on hover */}
      {isActive && (
        <div className="flex gap-0.5 pl-4 opacity-0 transition-opacity group-hover:opacity-100">
          {agent.status === "running" && (
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  variant="ghost"
                  size="icon"
                  className="size-6"
                  aria-label={t("workspace.quickPause")}
                  disabled={bridgeDegraded}
                  onClick={(e) => {
                    e.stopPropagation();
                    onPause(agent.id);
                  }}
                >
                  <Pause className="size-3" />
                </Button>
              </TooltipTrigger>
              <TooltipContent side="right">
                {t("workspace.quickPause")}
              </TooltipContent>
            </Tooltip>
          )}
          {agent.status === "paused" && (
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  variant="ghost"
                  size="icon"
                  className="size-6"
                  aria-label={t("workspace.quickResume")}
                  disabled={bridgeDegraded}
                  onClick={(e) => {
                    e.stopPropagation();
                    onResume(agent.id);
                  }}
                >
                  <Play className="size-3" />
                </Button>
              </TooltipTrigger>
              <TooltipContent side="right">
                {t("workspace.quickResume")}
              </TooltipContent>
            </Tooltip>
          )}
          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                className="size-6 text-destructive hover:text-destructive"
                aria-label={t("workspace.quickKill")}
                onClick={(e) => {
                  e.stopPropagation();
                  onKill(agent.id);
                }}
              >
                <Skull className="size-3" />
              </Button>
            </TooltipTrigger>
            <TooltipContent side="right">
              {t("workspace.quickKill")}
            </TooltipContent>
          </Tooltip>
        </div>
      )}
    </div>
  );
}
