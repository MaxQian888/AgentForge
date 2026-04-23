"use client";

import type { KeyboardEvent, MouseEvent } from "react";
import { useMemo, useState } from "react";
import { Bot } from "lucide-react";
import { useTranslations } from "next-intl";
import { Pause, Play, Skull } from "lucide-react";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { EmptyState } from "@/components/shared/empty-state";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Progress } from "@/components/ui/progress";
import { useLayoutStore } from "@/lib/stores/layout-store";
import { cn } from "@/lib/utils";
import type { Agent } from "@/lib/stores/agent-store";
import { statusColors, statusDotColors } from "./agent-status-colors";

interface AgentGridViewProps {
  agents: Agent[];
  bridgeDegraded?: boolean;
  selectedAgentId?: string | null;
  onSelectAgent?: (id: string) => void;
  onPause?: (id: string) => void;
  onResume?: (id: string) => void;
  onKill?: (id: string) => void;
}

function formatRuntimeIdentity(agent: Agent) {
  return [agent.runtime, agent.provider, agent.model].filter(Boolean).join(" / ") || "-";
}

function clampPercent(value: number) {
  return Math.max(0, Math.min(100, Math.round(value)));
}

function buildSparklinePoints(values: number[]) {
  if (values.length === 0) {
    return "";
  }

  return values
    .map((value, index) => {
      const x = values.length === 1 ? 100 : (index / (values.length - 1)) * 100;
      const y = 100 - clampPercent(value);
      return `${x},${y}`;
    })
    .join(" ");
}

function AgentResourceSparkline({
  label,
  history,
  value,
  testId,
  pendingLabel,
}: {
  label: string;
  history?: number[];
  value?: number;
  testId: string;
  pendingLabel: string;
}) {
  const resolvedValue = typeof value === "number" ? clampPercent(value) : null;
  const resolvedHistory = Array.isArray(history) ? history.map(clampPercent) : [];
  const warning = resolvedValue != null && resolvedValue >= 80;

  return (
    <div
      data-testid={testId}
      className={cn("space-y-1", warning && "text-amber-600")}
      title={
        resolvedValue != null
          ? `${label}: ${resolvedValue}% (warning threshold 80%)`
          : `${label}: ${pendingLabel}`
      }
    >
      <div className="flex items-center justify-between gap-2 text-xs font-medium">
        <span>{label}</span>
        <span>{resolvedValue != null ? `${resolvedValue}%` : pendingLabel}</span>
      </div>
      <div className="h-10 rounded-md border bg-muted/30 px-2 py-1">
        {resolvedHistory.length > 0 ? (
          <svg viewBox="0 0 100 100" className="h-full w-full overflow-visible">
            <polyline
              fill="none"
              stroke="currentColor"
              strokeWidth="6"
              strokeLinecap="round"
              strokeLinejoin="round"
              points={buildSparklinePoints(resolvedHistory)}
            />
          </svg>
        ) : (
          <div className="flex h-full items-center text-[11px] text-muted-foreground">
            {pendingLabel}
          </div>
        )}
      </div>
    </div>
  );
}

export function AgentGridView({
  agents,
  bridgeDegraded = false,
  selectedAgentId,
  onSelectAgent,
  onPause,
  onResume,
  onKill,
}: AgentGridViewProps) {
  const t = useTranslations("agents");
  const openCommandPalette = useLayoutStore((state) => state.openCommandPalette);
  const [selectedAgentIds, setSelectedAgentIds] = useState<string[]>([]);
  const [pendingKillAgentIds, setPendingKillAgentIds] = useState<string[] | null>(null);

  const selectedAgents = useMemo(
    () => agents.filter((agent) => selectedAgentIds.includes(agent.id)),
    [agents, selectedAgentIds],
  );
  const runningSelectedAgents = selectedAgents.filter((agent) => agent.status === "running");
  const pausedSelectedAgents = selectedAgents.filter((agent) => agent.status === "paused");

  const clearSelection = () => {
    setSelectedAgentIds([]);
  };

  const toggleSelectedAgent = (id: string) => {
    setSelectedAgentIds((current) =>
      current.includes(id) ? current.filter((value) => value !== id) : [...current, id],
    );
  };

  const handleCardActivation = (
    id: string,
    event?: Pick<MouseEvent | KeyboardEvent, "shiftKey">,
  ) => {
    if (event?.shiftKey) {
      toggleSelectedAgent(id);
      return;
    }

    clearSelection();
    onSelectAgent?.(id);
  };

  const confirmKill = (ids: string[]) => {
    setPendingKillAgentIds(ids);
  };

  if (agents.length === 0) {
    return (
      <Card>
        <CardContent className="py-6">
          <EmptyState
            icon={Bot}
            title={t("workspace.emptyTitle")}
            description={t("workspace.emptyDescription")}
            action={{
              label: t("workspace.emptyAction"),
              onClick: openCommandPalette,
            }}
            className="py-10"
          />
        </CardContent>
      </Card>
    );
  }

  return (
    <section className="space-y-4">
      <div>
        <h2 className="text-base font-semibold">{t("workspace.agentGridTitle")}</h2>
        <p className="text-sm text-muted-foreground">
          {t("workspace.agentGridDescription")}
        </p>
      </div>

      {selectedAgentIds.length > 0 ? (
        <div
          data-testid="agent-bulk-toolbar"
          className="flex flex-wrap items-center gap-2 rounded-lg border bg-muted/50 px-4 py-2"
        >
          <span className="text-sm font-medium">
            {t("workspace.bulkSelected", { count: selectedAgentIds.length })}
          </span>
          <Button
            type="button"
            size="sm"
            variant="outline"
            disabled={runningSelectedAgents.length === 0 || bridgeDegraded || !onPause}
            onClick={() => {
              runningSelectedAgents.forEach((agent) => onPause?.(agent.id));
              clearSelection();
            }}
          >
            {t("workspace.bulkPause")}
          </Button>
          <Button
            type="button"
            size="sm"
            variant="outline"
            disabled={pausedSelectedAgents.length === 0 || bridgeDegraded || !onResume}
            onClick={() => {
              pausedSelectedAgents.forEach((agent) => onResume?.(agent.id));
              clearSelection();
            }}
          >
            {t("workspace.bulkResume")}
          </Button>
          <Button
            type="button"
            size="sm"
            variant="destructive"
            disabled={!onKill}
            onClick={() => confirmKill(selectedAgentIds)}
          >
            {t("workspace.bulkKill")}
          </Button>
          <Button type="button" size="sm" variant="ghost" onClick={clearSelection}>
            {t("workspace.bulkClear")}
          </Button>
        </div>
      ) : null}

      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
        {agents.map((agent) => {
          const budgetPct =
            agent.budget > 0 ? Math.min((agent.cost / agent.budget) * 100, 100) : 0;
          const isBulkSelected = selectedAgentIds.includes(agent.id);
          const isDetailSelected = selectedAgentId === agent.id;

          return (
            <Card
              key={agent.id}
              data-testid={`agent-card-${agent.id}`}
              role="button"
              tabIndex={0}
              className={cn(
                "cursor-pointer transition-colors hover:border-primary/40",
                (isBulkSelected || isDetailSelected) && "border-primary ring-2 ring-primary/20",
              )}
              onClick={(event) => handleCardActivation(agent.id, event)}
              onKeyDown={(event) => {
                if (event.key === "Enter" || event.key === " ") {
                  event.preventDefault();
                  handleCardActivation(agent.id, event);
                }
              }}
            >
              <CardHeader className="gap-3 pb-3">
                <div className="flex items-start justify-between gap-3">
                  <div className="min-w-0">
                    <CardTitle className="truncate text-base">
                      {agent.taskTitle}
                    </CardTitle>
                    <CardDescription className="truncate">
                      {agent.roleName}
                    </CardDescription>
                  </div>
                  <Badge
                    variant="secondary"
                    className={cn("shrink-0 capitalize", statusColors[agent.status])}
                  >
                    <span
                      className={cn(
                        "mr-1.5 inline-block size-2 rounded-full",
                        statusDotColors[agent.status],
                        agent.status === "running" && "animate-pulse-dot",
                      )}
                    />
                    {t(`status.${agent.status}`)}
                  </Badge>
                </div>
                <div className="text-sm text-muted-foreground">
                  {formatRuntimeIdentity(agent)}
                </div>
              </CardHeader>
              <CardContent className="space-y-4">
                <dl className="grid gap-3 sm:grid-cols-2">
                  <div className="space-y-1">
                    <dt className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                      {t("workspace.cardRuntime")}
                    </dt>
                    <dd className="text-sm">{formatRuntimeIdentity(agent)}</dd>
                  </div>
                  <div className="space-y-1">
                    <dt className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                      {t("workspace.cardMemory")}
                    </dt>
                    <dd className="text-sm">{agent.memoryStatus}</dd>
                  </div>
                  <div className="space-y-1">
                    <dt className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                      {t("workspace.cardTurns")}
                    </dt>
                    <dd className="text-sm">{agent.turns}</dd>
                  </div>
                  <div className="space-y-1">
                    <dt className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                      {t("workspace.cardBudget")}
                    </dt>
                    <dd className="text-sm">
                      {t("workspace.cardBudgetValue", {
                        cost: agent.cost,
                        budget: agent.budget,
                      })}
                    </dd>
                  </div>
                </dl>

                {agent.budget > 0 ? (
                  <Progress
                    value={budgetPct}
                    aria-label={t("workspace.cardBudget")}
                    className="h-2"
                    indicatorClassName={budgetPct > 80 ? "bg-destructive" : undefined}
                  />
                ) : null}

                {agent.status === "running" ? (
                  <div className="grid gap-3 sm:grid-cols-2">
                    <AgentResourceSparkline
                      label={t("workspace.cardCpu")}
                      history={agent.resourceUtilization?.cpuHistory}
                      value={agent.resourceUtilization?.cpuPercent}
                      testId={`agent-resource-cpu-${agent.id}`}
                      pendingLabel={t("workspace.telemetryPending")}
                    />
                    <AgentResourceSparkline
                      label={t("workspace.cardMemoryUsage")}
                      history={agent.resourceUtilization?.memoryHistory}
                      value={agent.resourceUtilization?.memoryPercent}
                      testId={`agent-resource-memory-${agent.id}`}
                      pendingLabel={t("workspace.telemetryPending")}
                    />
                  </div>
                ) : null}

                <div className="flex flex-wrap gap-2">
                  {agent.status === "running" ? (
                    <Button
                      type="button"
                      size="sm"
                      variant="outline"
                      disabled={!onPause || bridgeDegraded}
                      onClick={(event) => {
                        event.stopPropagation();
                        onPause?.(agent.id);
                      }}
                    >
                      <Pause className="mr-1 size-3.5" />
                      {t("workspace.quickPause")}
                    </Button>
                  ) : null}
                  {agent.status === "paused" ? (
                    <Button
                      type="button"
                      size="sm"
                      variant="outline"
                      disabled={!onResume || bridgeDegraded}
                      onClick={(event) => {
                        event.stopPropagation();
                        onResume?.(agent.id);
                      }}
                    >
                      <Play className="mr-1 size-3.5" />
                      {t("workspace.quickResume")}
                    </Button>
                  ) : null}
                  <Button
                    type="button"
                    size="sm"
                    variant="destructive"
                    disabled={!onKill}
                    onClick={(event) => {
                      event.stopPropagation();
                      confirmKill([agent.id]);
                    }}
                  >
                    <Skull className="mr-1 size-3.5" />
                    {t("workspace.quickKill")}
                  </Button>
                </div>
              </CardContent>
            </Card>
          );
        })}
      </div>
      <ConfirmDialog
        open={pendingKillAgentIds !== null}
        title={
          (pendingKillAgentIds?.length ?? 0) > 1
            ? t("workspace.confirmKillTitlePlural")
            : t("workspace.confirmKillTitle")
        }
        description={
          (pendingKillAgentIds?.length ?? 0) > 1
            ? t("workspace.confirmKillDescriptionPlural")
            : t("workspace.confirmKillDescription")
        }
        confirmLabel={t("workspace.confirmKillAction")}
        variant="destructive"
        onConfirm={() => {
          if (pendingKillAgentIds) {
            pendingKillAgentIds.forEach((id) => onKill?.(id));
          }
          setPendingKillAgentIds(null);
          clearSelection();
        }}
        onCancel={() => setPendingKillAgentIds(null)}
      />
    </section>
  );
}
