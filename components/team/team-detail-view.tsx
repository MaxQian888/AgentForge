"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { XCircle, RotateCw, Clock, DollarSign, Hash, Users, Pencil, Trash2 } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { PageHeader } from "@/components/shared/page-header";
import { Separator } from "@/components/ui/separator";
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@/components/ui/tabs";
import { cn } from "@/lib/utils";
import { getTeamStrategyLabel, useTeamStore, type TeamStatus } from "@/lib/stores/team-store";
import { useAgentStore, type Agent } from "@/lib/stores/agent-store";
import { TeamPipeline } from "./team-pipeline";
import { TeamTimeline } from "./team-timeline";
import { TeamArtifactsPanel } from "./team-artifacts-panel";
import { OutputStream } from "@/components/agent/output-stream";
import { Skeleton } from "@/components/ui/skeleton";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { EditTeamDialog } from "./edit-team-dialog";

const statusColors: Record<TeamStatus, string> = {
  pending: "bg-zinc-500/15 text-zinc-600 dark:text-zinc-400",
  planning: "bg-blue-500/15 text-blue-700 dark:text-blue-400",
  executing: "bg-emerald-500/15 text-emerald-700 dark:text-emerald-400",
  reviewing: "bg-amber-500/15 text-amber-700 dark:text-amber-400",
  completed: "bg-green-500/15 text-green-700 dark:text-green-400",
  failed: "bg-red-500/15 text-red-700 dark:text-red-400",
  cancelled: "bg-zinc-500/15 text-zinc-600 dark:text-zinc-400",
};

interface TeamDetailViewProps {
  teamId: string;
}

export function TeamDetailView({ teamId }: TeamDetailViewProps) {
  const router = useRouter();
  const team = useTeamStore((s) => s.teams.find((t) => t.id === teamId));
  const fetchTeam = useTeamStore((s) => s.fetchTeam);
  const cancelTeam = useTeamStore((s) => s.cancelTeam);
  const retryTeam = useTeamStore((s) => s.retryTeam);
  const deleteTeam = useTeamStore((s) => s.deleteTeam);
  const updateTeam = useTeamStore((s) => s.updateTeam);
  const loading = useTeamStore((s) => Boolean(s.loadingById[teamId]));
  const error = useTeamStore((s) => s.errorById[teamId] ?? null);

  const agents = useAgentStore((s) => s.agents);
  const agentOutputs = useAgentStore((s) => s.agentOutputs);
  const fetchAgent = useAgentStore((s) => s.fetchAgent);

  const [duration, setDuration] = useState(0);
  const [confirmCancel, setConfirmCancel] = useState(false);
  const [confirmRetry, setConfirmRetry] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [showEditDialog, setShowEditDialog] = useState(false);

  useEffect(() => {
    void fetchTeam(teamId);
  }, [teamId, fetchTeam]);

  useEffect(() => {
    if (!team) return;
    const runIds = [
      team.plannerRunId,
      ...team.coderRunIds,
      team.reviewerRunId,
    ].filter((id): id is string => Boolean(id));
    runIds.forEach((id) => void fetchAgent(id));
  }, [team, fetchAgent]);

  useEffect(() => {
    if (!team?.createdAt) return;
    const createdAt = team.createdAt;
    const update = () =>
      setDuration(
        Math.round(
          (Date.now() - new Date(createdAt).getTime()) / 1000 / 60
        )
      );
    update();
    const interval = setInterval(update, 60_000);
    return () => clearInterval(interval);
  }, [team?.createdAt]);

  if (!team && loading) {
    return (
      <div className="flex flex-col gap-6">
        <div className="flex items-center justify-between">
          <div>
            <Skeleton className="h-8 w-48" />
            <Skeleton className="mt-2 h-4 w-64" />
          </div>
          <Skeleton className="h-8 w-20" />
        </div>
        <Skeleton className="h-16 w-full rounded-lg" />
        <div className="grid gap-4 sm:grid-cols-4">
          {Array.from({ length: 4 }).map((_, i) => (
            <Card key={i}>
              <CardContent className="py-4">
                <Skeleton className="mb-2 h-4 w-20" />
                <Skeleton className="h-8 w-16" />
              </CardContent>
            </Card>
          ))}
        </div>
      </div>
    );
  }

  if (!team && error) {
    return (
      <div className="flex items-center justify-center py-20">
        <p className="text-muted-foreground">{error}</p>
      </div>
    );
  }

  if (!team) {
    return (
      <div className="flex items-center justify-center py-20">
        <p className="text-muted-foreground">Team not found</p>
      </div>
    );
  }

  const plannerAgent = team.plannerRunId
    ? agents.find((a) => a.id === team.plannerRunId)
    : undefined;
  const coderAgents = team.coderRunIds
    .map((id) => agents.find((a) => a.id === id))
    .filter((a): a is Agent => a !== undefined);
  const reviewerAgent = team.reviewerRunId
    ? agents.find((a) => a.id === team.reviewerRunId)
    : undefined;

  const totalTurns =
    (plannerAgent?.turns ?? 0) +
    coderAgents.reduce((sum, a) => sum + a.turns, 0) +
    (reviewerAgent?.turns ?? 0);

  const isActive =
    team.status === "planning" ||
    team.status === "executing" ||
    team.status === "reviewing";

  const isTerminal =
    team.status === "completed" ||
    team.status === "failed" ||
    team.status === "cancelled";

  const plannerOutputs = team.plannerRunId
    ? agentOutputs.get(team.plannerRunId) ?? []
    : [];
  const reviewerOutputs = team.reviewerRunId
    ? agentOutputs.get(team.reviewerRunId) ?? []
    : [];

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        title={team.name || team.taskTitle || "Agent Team"}
        description={team.taskTitle}
        actions={
          <>
            <Button
              variant="ghost"
              size="icon"
              className="size-8"
              onClick={() => setShowEditDialog(true)}
            >
              <Pencil className="size-4" />
            </Button>
            {isActive && (
              <Button
                variant="destructive"
                size="sm"
                onClick={() => setConfirmCancel(true)}
              >
                <XCircle className="mr-1 size-4" />
                Cancel
              </Button>
            )}
            {(team.status === "failed" || team.status === "cancelled") && (
              <Button
                variant="outline"
                size="sm"
                onClick={() => setConfirmRetry(true)}
              >
                <RotateCw className="mr-1 size-4" />
                Retry
              </Button>
            )}
            {isTerminal && (
              <Button
                variant="ghost"
                size="sm"
                onClick={() => setConfirmDelete(true)}
              >
                <Trash2 className="mr-1 size-4" />
                Delete
              </Button>
            )}
          </>
        }
      />

      <ConfirmDialog
        open={confirmCancel}
        title="Cancel Team"
        description="This will stop all active agents in the team. This action cannot be undone."
        confirmLabel="Cancel Team"
        variant="destructive"
        onConfirm={() => {
          setConfirmCancel(false);
          void cancelTeam(team.id);
        }}
        onCancel={() => setConfirmCancel(false)}
      />
      <ConfirmDialog
        open={confirmRetry}
        title="Retry Team"
        description="This will restart the team from its last failed phase."
        confirmLabel="Retry"
        onConfirm={() => {
          setConfirmRetry(false);
          void retryTeam(team.id);
        }}
        onCancel={() => setConfirmRetry(false)}
      />
      <ConfirmDialog
        open={confirmDelete}
        title="Delete Team"
        description="This will permanently remove this team record. This action cannot be undone."
        confirmLabel="Delete"
        variant="destructive"
        onConfirm={() => {
          setConfirmDelete(false);
          void deleteTeam(team.id).then(() => router.push("/teams"));
        }}
        onCancel={() => setConfirmDelete(false)}
      />
      {showEditDialog && (
        <EditTeamDialog
          open={showEditDialog}
          team={team}
          onSave={(input) => updateTeam(team.id, input).then(() => void fetchTeam(team.id))}
          onClose={() => setShowEditDialog(false)}
        />
      )}

      <TeamPipeline team={team} />

      <TeamTimeline team={team} />

      <div className="grid gap-4 sm:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle className="text-sm font-medium">
              Strategy
            </CardTitle>
          </CardHeader>
          <CardContent className="text-sm text-muted-foreground">
            {getTeamStrategyLabel(team.strategy)}
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle className="text-sm font-medium">
              Resolved Runtime
            </CardTitle>
          </CardHeader>
          <CardContent className="text-sm text-muted-foreground">
            {team.runtime || "-"} / {team.provider || "-"} / {team.model || "-"}
          </CardContent>
        </Card>
      </div>

      <div className="grid gap-4 sm:grid-cols-4">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="flex items-center gap-1.5 text-sm font-medium text-muted-foreground">
              <DollarSign className="size-3.5" />
              Total Cost
            </CardTitle>
          </CardHeader>
          <CardContent>
            <span className="text-2xl font-bold">
              ${team.totalSpent.toFixed(2)}
            </span>
            <span className="text-sm text-muted-foreground">
              {" "}
              / ${team.totalBudget.toFixed(2)}
            </span>
            <div className="mt-2 h-1.5 w-full overflow-hidden rounded-full bg-muted">
              <div
                className={cn(
                  "h-full rounded-full",
                  team.totalBudget > 0 &&
                    team.totalSpent / team.totalBudget > 0.8
                    ? "bg-destructive"
                    : "bg-primary"
                )}
                style={{
                  width: `${Math.min(
                    team.totalBudget > 0
                      ? (team.totalSpent / team.totalBudget) * 100
                      : 0,
                    100
                  )}%`,
                }}
              />
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="flex items-center gap-1.5 text-sm font-medium text-muted-foreground">
              <Hash className="size-3.5" />
              Total Turns
            </CardTitle>
          </CardHeader>
          <CardContent>
            <span className="text-2xl font-bold">{totalTurns}</span>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="flex items-center gap-1.5 text-sm font-medium text-muted-foreground">
              <Clock className="size-3.5" />
              Duration
            </CardTitle>
          </CardHeader>
          <CardContent>
            <span className="text-2xl font-bold">{duration}m</span>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="flex items-center gap-1.5 text-sm font-medium text-muted-foreground">
              <Users className="size-3.5" />
              Coders
            </CardTitle>
          </CardHeader>
          <CardContent>
            <span className="text-2xl font-bold">{coderAgents.length}</span>
          </CardContent>
        </Card>
      </div>

      <Separator />

      <Tabs defaultValue="overview">
        <TabsList>
          <TabsTrigger value="overview">Overview</TabsTrigger>
          <TabsTrigger value="artifacts">Artifacts</TabsTrigger>
          <TabsTrigger value="planner">Planner</TabsTrigger>
          <TabsTrigger value="coders">Coders</TabsTrigger>
          <TabsTrigger value="reviewer">Reviewer</TabsTrigger>
        </TabsList>

        <TabsContent value="overview" className="mt-4">
          <div className="flex flex-col gap-4">
            <Card>
              <CardHeader>
                <CardTitle className="text-sm font-medium">
                  Cost Breakdown
                </CardTitle>
              </CardHeader>
              <CardContent>
                <div className="flex flex-col gap-2 text-sm">
                  <div className="flex justify-between">
                    <span className="text-muted-foreground">Planner</span>
                    <span>${(plannerAgent?.cost ?? 0).toFixed(2)}</span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-muted-foreground">
                      Coders ({coderAgents.length})
                    </span>
                    <span>
                      $
                      {coderAgents
                        .reduce((sum, a) => sum + a.cost, 0)
                        .toFixed(2)}
                    </span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-muted-foreground">Reviewer</span>
                    <span>${(reviewerAgent?.cost ?? 0).toFixed(2)}</span>
                  </div>
                  <Separator />
                  <div className="flex justify-between font-medium">
                    <span>Total</span>
                    <span>${team.totalSpent.toFixed(2)}</span>
                  </div>
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle className="text-sm font-medium">Status</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="flex items-center gap-2">
                  <Badge
                    variant="secondary"
                    className={cn(statusColors[team.status])}
                  >
                    {team.status}
                  </Badge>
                  {team.errorMessage && (
                    <span className="text-sm text-destructive">
                      {team.errorMessage}
                    </span>
                  )}
                </div>
              </CardContent>
            </Card>
          </div>
        </TabsContent>

        <TabsContent value="artifacts" className="mt-4">
          <TeamArtifactsPanel teamId={team.id} />
        </TabsContent>

        <TabsContent value="planner" className="mt-4">
          {plannerAgent ? (
            <div className="flex flex-col gap-3">
              <div className="flex items-center gap-2">
                <span className="font-medium">{plannerAgent.roleName}</span>
                <Badge
                  variant="secondary"
                  className={cn(
                    agentStatusColorsForTab[plannerAgent.status]
                  )}
                >
                  {plannerAgent.status}
                </Badge>
              </div>
              <OutputStream lines={plannerOutputs} />
            </div>
          ) : (
            <p className="text-muted-foreground">
              No planner agent assigned yet.
            </p>
          )}
        </TabsContent>

        <TabsContent value="coders" className="mt-4">
          {coderAgents.length > 0 ? (
            <div className="flex flex-col gap-6">
              {coderAgents.map((agent) => {
                const outputs = agentOutputs.get(agent.id) ?? [];
                return (
                  <div key={agent.id} className="flex flex-col gap-3">
                    <div className="flex items-center gap-2">
                      <span className="font-medium">{agent.roleName}</span>
                      <Badge
                        variant="secondary"
                        className={cn(
                          agentStatusColorsForTab[agent.status]
                        )}
                      >
                        {agent.status}
                      </Badge>
                      <span className="text-xs text-muted-foreground">
                        Turns: {agent.turns} | ${agent.cost.toFixed(2)}
                      </span>
                    </div>
                    <OutputStream lines={outputs} />
                  </div>
                );
              })}
            </div>
          ) : (
            <p className="text-muted-foreground">
              No coder agents assigned yet.
            </p>
          )}
        </TabsContent>

        <TabsContent value="reviewer" className="mt-4">
          {reviewerAgent ? (
            <div className="flex flex-col gap-3">
              <div className="flex items-center gap-2">
                <span className="font-medium">{reviewerAgent.roleName}</span>
                <Badge
                  variant="secondary"
                  className={cn(
                    agentStatusColorsForTab[reviewerAgent.status]
                  )}
                >
                  {reviewerAgent.status}
                </Badge>
              </div>
              <OutputStream lines={reviewerOutputs} />
            </div>
          ) : (
            <p className="text-muted-foreground">
              No reviewer agent assigned yet.
            </p>
          )}
        </TabsContent>
      </Tabs>
    </div>
  );
}

const agentStatusColorsForTab: Record<string, string> = {
  starting: "bg-blue-500/15 text-blue-700 dark:text-blue-400",
  running: "bg-green-500/15 text-green-700 dark:text-green-400",
  paused: "bg-yellow-500/15 text-yellow-700 dark:text-yellow-400",
  completed: "bg-blue-500/15 text-blue-700 dark:text-blue-400",
  failed: "bg-red-500/15 text-red-700 dark:text-red-400",
  cancelled: "bg-zinc-500/15 text-zinc-600 dark:text-zinc-400",
  budget_exceeded: "bg-amber-500/15 text-amber-700 dark:text-amber-400",
};
