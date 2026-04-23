"use client";

import type { ReactNode } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Progress } from "@/components/ui/progress";
import { DispatchHistoryPanel } from "@/components/tasks/dispatch-history-panel";
import type { DispatchAttemptRecord, AgentStatus } from "@/lib/stores/agent-store";
import { statusColors } from "./agent-status-colors";
import { useTranslations } from "next-intl";
import type { AgentVisualizationFocus } from "./agent-visualization-model";

interface AgentVisualizationFocusPanelProps {
  focus: AgentVisualizationFocus | null;
  dispatchHistory: DispatchAttemptRecord[];
  dispatchHistoryLoading: boolean;
  onClearFocus: () => void;
}

function FocusSection({
  title,
  children,
}: {
  title: string;
  children: ReactNode;
}) {
  return (
    <div className="space-y-2">
      <h3 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
        {title}
      </h3>
      {children}
    </div>
  );
}

export function AgentVisualizationFocusPanel({
  focus,
  dispatchHistory,
  dispatchHistoryLoading,
  onClearFocus,
}: AgentVisualizationFocusPanelProps) {
  const t = useTranslations("agents");

  if (!focus) {
    return null;
  }

  return (
    <aside className="flex flex-col gap-4 rounded-xl border bg-card/70 p-4">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <p className="text-sm font-semibold">
            {t(`visualization.focus.title.${focus.kind}`)}
          </p>
          <p className="truncate text-sm text-muted-foreground">
            {"taskTitle" in focus
              ? focus.taskTitle
              : "label" in focus
                ? focus.label
                : ""}
          </p>
        </div>
        <Button type="button" size="sm" variant="outline" onClick={onClearFocus}>
          {t("visualization.focus.clear")}
        </Button>
      </div>

      {focus.kind === "task" ? (
        <>
          <Badge variant="outline">{focus.taskId}</Badge>
          <p className="text-sm text-muted-foreground">
            {t("visualization.focus.task.summary", {
              agentCount: focus.agentCount,
              queueCount: focus.queueCount,
            })}
          </p>
        </>
      ) : null}

      {focus.kind === "dispatch" ? (
        <>
          <div className="flex flex-wrap gap-2">
            <Badge variant="secondary" className="capitalize">
              {focus.status}
            </Badge>
            <Badge variant="outline">{focus.taskId}</Badge>
            <Badge variant="outline">{focus.priorityLabel}</Badge>
          </div>
          <div className="space-y-1 text-sm text-muted-foreground">
            <p>{focus.reason || t("visualization.focus.empty")}</p>
            <p>
              {[focus.runtime, focus.provider, focus.model]
                .filter(Boolean)
                .join(" / ")}
            </p>
          </div>
        </>
      ) : null}

      {focus.kind === "agent" ? (
        <>
          <div className="flex flex-wrap gap-2">
            <Badge
              className={statusColors[focus.status as AgentStatus] ?? ""}
            >
              {t(`status.${focus.status}`)}
            </Badge>
            <Badge variant="outline">{focus.taskId}</Badge>
          </div>
          <FocusSection title={t("visualization.focus.section.cost")}>
            <div className="space-y-1.5">
              <div className="flex items-center justify-between text-sm text-muted-foreground">
                <span>${focus.costUsd.toFixed(4)} / ${focus.budgetUsd.toFixed(2)}</span>
                <span>{Math.round(focus.budgetPct)}%</span>
              </div>
              <Progress value={focus.budgetPct} className="h-1.5" />
            </div>
          </FocusSection>
          <FocusSection title={t("visualization.focus.section.runtime")}>
            <div className="flex flex-wrap gap-2">
              <Badge variant="outline">{focus.runtime || "—"}</Badge>
              <Badge variant="outline">{focus.provider || "—"}</Badge>
              <Badge variant="outline">{focus.model || "—"}</Badge>
            </div>
          </FocusSection>
          <div className="space-y-1 text-sm text-muted-foreground">
            <p>{t("visualization.focus.agent.turnsLabel", { turnCount: focus.turnCount })}</p>
            <p>{t("visualization.focus.agent.taskLabel", { taskTitle: focus.taskTitle })}</p>
            {focus.worktreePath ? <p>{t("visualization.focus.agent.worktreeLabel", { worktreePath: focus.worktreePath })}</p> : null}
            {focus.branchName ? <p>{t("visualization.focus.agent.branchLabel", { branchName: focus.branchName })}</p> : null}
            {focus.canResume ? (
              <Badge variant="secondary" className="mt-1">
                {t("visualization.focus.agent.resumable")}
              </Badge>
            ) : null}
          </div>
        </>
      ) : null}

      {focus.kind === "runtime" ? (
        <>
          <div className="flex flex-wrap gap-2">
            <Badge variant={focus.available ? "secondary" : "destructive"}>
              {t(
                `visualization.focus.runtime.availability.${
                  focus.available ? "available" : "unavailable"
                }`,
              )}
            </Badge>
            <Badge variant="outline">{focus.runtime}</Badge>
            <Badge variant="outline">{focus.provider}</Badge>
            <Badge variant="outline">{focus.model}</Badge>
          </div>
          <p className="text-sm text-muted-foreground">
            {t("visualization.focus.runtime.connected", {
              agentCount: focus.agentCount,
              dispatchCount: focus.dispatchCount,
            })}
          </p>
          {focus.diagnostics.length > 0 ? (
            <FocusSection title={t("visualization.focus.section.diagnostics")}>
              <div className="space-y-2">
                {focus.diagnostics.map((diagnostic) => (
                  <div
                    key={`${diagnostic.code}-${diagnostic.message}`}
                    className="rounded-lg border border-border/60 bg-background px-3 py-2 text-sm"
                  >
                    <div className="font-medium">{diagnostic.message}</div>
                    <div className="text-xs text-muted-foreground">
                      {diagnostic.code}
                    </div>
                  </div>
                ))}
              </div>
            </FocusSection>
          ) : null}
          {focus.supportedFeatures.length > 0 ? (
            <FocusSection title={t("visualization.focus.section.features")}>
              <div className="flex flex-wrap gap-2">
                {focus.supportedFeatures.map((feature) => (
                  <Badge key={feature} variant="outline">
                    {feature}
                  </Badge>
                ))}
              </div>
            </FocusSection>
          ) : null}
          {focus.diagnostics.length === 0 && focus.supportedFeatures.length === 0 ? (
            <p className="text-sm text-muted-foreground">
              {t("visualization.focus.empty")}
            </p>
          ) : null}
        </>
      ) : null}

      {focus.kind !== "runtime" ? (
        dispatchHistoryLoading ? (
          <p className="text-sm text-muted-foreground">
            {t("visualization.focus.loading")}
          </p>
        ) : dispatchHistory.length > 0 ? (
          <DispatchHistoryPanel attempts={dispatchHistory} />
        ) : (
          <p className="text-sm text-muted-foreground">
            {t("visualization.focus.empty")}
          </p>
        )
      ) : null}
    </aside>
  );
}
