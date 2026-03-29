"use client";

import { useEffect } from "react";
import { useTranslations } from "next-intl";
import {
  Activity,
  Cpu,
  DollarSign,
  Hash,
  TrendingUp,
} from "lucide-react";
import { Badge } from "@/components/ui/badge";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { CostChart } from "@/components/cost/cost-chart";
import { VelocityChart } from "@/components/cost/velocity-chart";
import { AgentPerformanceTable } from "@/components/cost/agent-performance-table";
import { useAgentStore } from "@/lib/stores/agent-store";
import { useCostStore } from "@/lib/stores/cost-store";
import { useDashboardStore } from "@/lib/stores/dashboard-store";

function formatTokens(count: number): string {
  if (count >= 1_000_000) return `${(count / 1_000_000).toFixed(1)}M`;
  if (count >= 1_000) return `${(count / 1_000).toFixed(1)}K`;
  return String(count);
}

export default function CostPage() {
  const t = useTranslations("cost");
  const agents = useAgentStore((s) => s.agents);
  const projectCost = useCostStore((s) => s.projectCost);
  const costLoading = useCostStore((s) => s.loading);
  const costError = useCostStore((s) => s.error);
  const fetchProjectCost = useCostStore((s) => s.fetchProjectCost);
  const velocity = useCostStore((s) => s.velocity);
  const fetchVelocity = useCostStore((s) => s.fetchVelocity);
  const agentPerformance = useCostStore((s) => s.agentPerformance);
  const fetchAgentPerformance = useCostStore((s) => s.fetchAgentPerformance);
  const selectedProjectId = useDashboardStore((s) => s.selectedProjectId);

  useEffect(() => {
    if (selectedProjectId) {
      void fetchProjectCost(selectedProjectId);
      void fetchVelocity(selectedProjectId);
      void fetchAgentPerformance(selectedProjectId);
    }
  }, [selectedProjectId, fetchProjectCost, fetchVelocity, fetchAgentPerformance]);

  const totalCost = projectCost?.totalCostUsd ?? agents.reduce((sum, a) => sum + a.cost, 0);
  const totalInput = projectCost?.totalInputTokens ?? 0;
  const totalOutput = projectCost?.totalOutputTokens ?? 0;
  const totalCache = projectCost?.totalCacheReadTokens ?? 0;
  const totalTurns = projectCost?.totalTurns ?? 0;
  const activeAgents = projectCost?.activeAgents ?? agents.filter((a) => a.status === "running" || a.status === "starting").length;
  const sprintCosts = projectCost?.sprintCosts ?? [];
  const taskCosts = projectCost?.taskCosts ?? [];
  const chartData = projectCost?.dailyCosts?.map((d) => ({ date: d.date, cost: d.costUsd })) ?? [];

  return (
    <div className="flex flex-col gap-6">
      <h1 className="text-2xl font-bold">{t("title")}</h1>

      {costError ? (
        <div className="rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {costError}
        </div>
      ) : null}

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-6">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              {t("totalSpend")}
            </CardTitle>
            <DollarSign className="size-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">${totalCost.toFixed(2)}</div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              {t("inputTokens")}
            </CardTitle>
            <TrendingUp className="size-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{formatTokens(totalInput)}</div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              {t("outputTokens")}
            </CardTitle>
            <TrendingUp className="size-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{formatTokens(totalOutput)}</div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              {t("cacheTokens")}
            </CardTitle>
            <Hash className="size-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{formatTokens(totalCache)}</div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              {t("totalTurns")}
            </CardTitle>
            <Activity className="size-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{totalTurns}</div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              {t("activeAgents")}
            </CardTitle>
            <Cpu className="size-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{activeAgents}</div>
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>{t("costOverTime")}</CardTitle>
          <CardDescription>{t("costOverTimeDesc")}</CardDescription>
        </CardHeader>
        <CardContent>
          {chartData.length > 0 ? (
            <CostChart data={chartData} />
          ) : (
            <div className="flex h-[200px] items-center justify-center text-sm text-muted-foreground">
              {costLoading ? t("loadingChart") : t("noChartData")}
            </div>
          )}
        </CardContent>
      </Card>

      {sprintCosts.length > 0 ? (
        <Card>
          <CardHeader>
            <CardTitle>{t("sprintCostComparison")}</CardTitle>
            <CardDescription>{t("sprintCostDesc")}</CardDescription>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t("colSprint")}</TableHead>
                  <TableHead className="text-right">{t("colBudget")}</TableHead>
                  <TableHead className="text-right">{t("colSpent")}</TableHead>
                  <TableHead className="text-right">{t("colRemaining")}</TableHead>
                  <TableHead className="text-right">{t("colTokensInOut")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {sprintCosts.map((sprint) => {
                  const remaining = sprint.budgetUsd - sprint.costUsd;
                  return (
                    <TableRow key={sprint.sprintId}>
                      <TableCell className="font-medium">{sprint.sprintName}</TableCell>
                      <TableCell className="text-right">
                        ${sprint.budgetUsd.toFixed(2)}
                      </TableCell>
                      <TableCell className="text-right">
                        ${sprint.costUsd.toFixed(2)}
                      </TableCell>
                      <TableCell className="text-right">
                        <span className={remaining < 0 ? "text-destructive" : ""}>
                          ${remaining.toFixed(2)}
                        </span>
                      </TableCell>
                      <TableCell className="text-right">
                        {formatTokens(sprint.inputTokens)} / {formatTokens(sprint.outputTokens)}
                      </TableCell>
                    </TableRow>
                  );
                })}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      ) : null}

      {velocity.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle>{t("teamVelocity")}</CardTitle>
            <CardDescription>{t("teamVelocityDesc")}</CardDescription>
          </CardHeader>
          <CardContent>
            <VelocityChart data={velocity} />
          </CardContent>
        </Card>
      )}

      {agentPerformance.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle>{t("agentPerformance")}</CardTitle>
            <CardDescription>{t("agentPerformanceDesc")}</CardDescription>
          </CardHeader>
          <CardContent>
            <AgentPerformanceTable data={agentPerformance} />
          </CardContent>
        </Card>
      )}

      {taskCosts.length > 0 ? (
        <Card>
          <CardHeader>
            <CardTitle>{t("perTaskCost")}</CardTitle>
            <CardDescription>{t("perTaskCostDesc")}</CardDescription>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t("colTask")}</TableHead>
                  <TableHead className="text-right">{t("colAgentRuns")}</TableHead>
                  <TableHead className="text-right">{t("colCost")}</TableHead>
                  <TableHead className="text-right">{t("colTokensInOutCache")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {taskCosts.map((task) => (
                  <TableRow key={task.taskId}>
                    <TableCell className="max-w-[300px] truncate font-medium">
                      {task.taskTitle}
                    </TableCell>
                    <TableCell className="text-right">
                      <Badge variant="secondary">{task.agentRuns}</Badge>
                    </TableCell>
                    <TableCell className="text-right">
                      ${task.costUsd.toFixed(2)}
                    </TableCell>
                    <TableCell className="text-right">
                      {formatTokens(task.inputTokens)} / {formatTokens(task.outputTokens)} / {formatTokens(task.cacheReadTokens)}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      ) : null}
    </div>
  );
}
