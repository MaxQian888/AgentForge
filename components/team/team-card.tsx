"use client";

import { useState } from "react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { Trash2 } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { cn } from "@/lib/utils";
import { getTeamStrategyLabel, type AgentTeam, type TeamStatus } from "@/lib/stores/team-store";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";

const statusColors: Record<TeamStatus, string> = {
  pending: "bg-zinc-500/15 text-zinc-600 dark:text-zinc-400",
  planning: "bg-blue-500/15 text-blue-700 dark:text-blue-400",
  executing: "bg-emerald-500/15 text-emerald-700 dark:text-emerald-400",
  reviewing: "bg-amber-500/15 text-amber-700 dark:text-amber-400",
  completed: "bg-green-500/15 text-green-700 dark:text-green-400",
  failed: "bg-red-500/15 text-red-700 dark:text-red-400",
  cancelled: "bg-zinc-500/15 text-zinc-600 dark:text-zinc-400",
};

const phaseDotColor: Record<string, string> = {
  pending: "bg-zinc-300 dark:bg-zinc-600",
  active: "bg-blue-500",
  completed: "bg-green-500",
  failed: "bg-red-500",
  cancelled: "bg-zinc-500 dark:bg-zinc-400",
};

function getPhaseDotStatus(
  teamStatus: TeamStatus,
  phase: "plan" | "execute" | "review"
): string {
  const phaseOrder: TeamStatus[] = ["planning", "executing", "reviewing"];
  const currentIndex = phaseOrder.indexOf(teamStatus);
  const phaseIndex = { plan: 0, execute: 1, review: 2 }[phase];

  if (teamStatus === "completed") return "completed";
  if (teamStatus === "failed" || teamStatus === "cancelled") {
    return teamStatus;
  }
  if (teamStatus === "pending") return "pending";
  if (currentIndex > phaseIndex) return "completed";
  if (currentIndex === phaseIndex) return "active";
  return "pending";
}

interface TeamCardProps {
  team: AgentTeam;
  onDelete?: (id: string) => void;
}

export function TeamCard({ team, onDelete }: TeamCardProps) {
  const t = useTranslations("teams");
  const tc = useTranslations("common");
  const [confirmDelete, setConfirmDelete] = useState(false);

  const costPct =
    team.totalBudget > 0 ? (team.totalSpent / team.totalBudget) * 100 : 0;

  const completedCoders = team.coderRunIds.length;

  const isTerminal =
    team.status === "completed" ||
    team.status === "failed" ||
    team.status === "cancelled";

  const teamName = team.name || team.taskTitle || t("card.untitled");

  return (
    <>
      <Link href={`/teams/detail?id=${team.id}`} className="block">
        <Card className="transition-colors hover:bg-accent/50">
          <CardContent className="flex flex-col gap-3 py-4">
            <div className="flex items-center justify-between">
              <h3 className="truncate font-medium">{teamName}</h3>
              <div className="flex items-center gap-1.5">
                <Badge
                  variant="secondary"
                  className={cn(statusColors[team.status])}
                >
                  {team.status}
                </Badge>
                {isTerminal && onDelete && (
                  <Button
                    variant="ghost"
                    size="icon"
                    className="size-6 text-muted-foreground hover:text-destructive"
                    onClick={(e) => {
                      e.preventDefault();
                      e.stopPropagation();
                      setConfirmDelete(true);
                    }}
                  >
                    <Trash2 className="size-3.5" />
                  </Button>
                )}
              </div>
            </div>

            <div className="flex items-center gap-2">
              <Badge variant="outline" className="text-xs">
                {getTeamStrategyLabel(team.strategy)}
              </Badge>
            </div>

            <div className="flex items-center gap-2">
              <span className="text-xs text-muted-foreground">{t("card.pipeline")}:</span>
              {(["plan", "execute", "review"] as const).map((phase) => {
                const dotStatus = getPhaseDotStatus(team.status, phase);
                return (
                  <div
                    key={phase}
                    className={cn(
                      "size-2.5 rounded-full",
                      phaseDotColor[dotStatus] ?? "bg-zinc-300"
                    )}
                    title={`${phase}: ${dotStatus}`}
                  />
                );
              })}
            </div>

            <div className="flex items-center gap-2">
              <div className="h-1.5 w-20 overflow-hidden rounded-full bg-muted">
                <div
                  className={cn(
                    "h-full rounded-full",
                    costPct > 80 ? "bg-destructive" : "bg-primary"
                  )}
                  style={{ width: `${Math.min(costPct, 100)}%` }}
                />
              </div>
              <span className="text-xs text-muted-foreground">
                ${team.totalSpent.toFixed(2)} / ${team.totalBudget.toFixed(2)}
              </span>
            </div>

            <div className="flex items-center justify-between text-xs text-muted-foreground">
              <span>{t("card.coders", { count: completedCoders })}</span>
              <span>{new Date(team.createdAt).toLocaleString()}</span>
            </div>
          </CardContent>
        </Card>
      </Link>

      <ConfirmDialog
        open={confirmDelete}
        title={t("card.deleteTitle")}
        description={t("card.deleteDescription", { name: teamName })}
        confirmLabel={tc("action.delete")}
        variant="destructive"
        onConfirm={() => {
          setConfirmDelete(false);
          onDelete?.(team.id);
        }}
        onCancel={() => setConfirmDelete(false)}
      />
    </>
  );
}
