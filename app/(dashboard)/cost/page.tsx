"use client";

import { useEffect, useMemo } from "react";
import { useTranslations } from "next-intl";
import {
  Activity,
  Cpu,
  DollarSign,
  FolderOpen,
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
import { PageHeader } from "@/components/shared/page-header";
import { EmptyState } from "@/components/shared/empty-state";
import { MetricCard } from "@/components/shared/metric-card";
import { ErrorBanner } from "@/components/shared/error-banner";
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
import { SpendingTrendChart } from "@/components/cost/spending-trend-chart";
import { BudgetAllocationChart } from "@/components/cost/budget-allocation-chart";
import { AgentCostBarChart } from "@/components/cost/agent-cost-bar-chart";
import { BudgetForecastCard } from "@/components/cost/budget-forecast-card";
import { CostProjectFilter } from "@/components/cost/cost-project-filter";
import {
  CostBreakdownTable,
  type CostBreakdownEntry,
} from "@/components/cost/cost-breakdown-table";
import { CostCsvExport } from "@/components/cost/cost-csv-export";
import {
  OverspendingAlertBanner,
  deriveOverspendingAlerts,
} from "@/components/cost/overspending-alert";
import { useCostStore } from "@/lib/stores/cost-store";
import { useDashboardStore } from "@/lib/stores/dashboard-store";
import { useBreadcrumbs } from "@/hooks/use-breadcrumbs";

function formatTokens(count: number): string {
  if (count >= 1_000_000) return `${(count / 1_000_000).toFixed(1)}M`;
  if (count >= 1_000) return `${(count / 1_000).toFixed(1)}K`;
  return String(count);
}

function renderMetric(
  hasSummary: boolean,
  value: number | undefined,
  formatter: (next: number) => string,
): string {
  if (!hasSummary) {
    return "\u2014";
  }
  return formatter(value ?? 0);
}

function formatCoverageLabel(
  authoritativeRunCount: number,
  estimatedRunCount: number,
  unpricedRunCount: number,
  t: (key: string) => string,
): string {
  if (unpricedRunCount > 0) {
    return t("coverageUnpriced");
  }
  if (estimatedRunCount > 0) {
    return t("coverageEstimated");
  }
  if (authoritativeRunCount > 0) {
    return t("coverageAuthoritative");
  }
  return t("coverageUnpriced");
}

export default function CostPage() {
  useBreadcrumbs([{ label: "Operations", href: "/" }, { label: "Cost" }]);
  const t = useTranslations("cost");
  const projectCost = useCostStore((s) => s.projectCost);
  const costLoading = useCostStore((s) => s.loading);
  const costError = useCostStore((s) => s.error);
  const fetchProjectCost = useCostStore((s) => s.fetchProjectCost);
  const velocity = useCostStore((s) => s.velocity);
  const velocityLoading = useCostStore((s) => s.velocityLoading);
  const fetchVelocity = useCostStore((s) => s.fetchVelocity);
  const agentPerformance = useCostStore((s) => s.agentPerformance);
  const performanceLoading = useCostStore((s) => s.performanceLoading);
  const fetchAgentPerformance = useCostStore((s) => s.fetchAgentPerformance);
  const selectedProjectId = useDashboardStore((s) => s.selectedProjectId);
  const projects = useDashboardStore((s) => s.projects);

  useEffect(() => {
    if (selectedProjectId) {
      void fetchProjectCost(selectedProjectId);
      void fetchVelocity(selectedProjectId);
      void fetchAgentPerformance(selectedProjectId);
    }
  }, [selectedProjectId, fetchProjectCost, fetchVelocity, fetchAgentPerformance]);

  const hasSummary = projectCost !== null;
  const sprintCosts = useMemo(
    () => projectCost?.sprintCosts ?? [],
    [projectCost],
  );
  const taskCosts = useMemo(
    () => projectCost?.taskCosts ?? [],
    [projectCost],
  );
  const chartData = useMemo(
    () =>
      projectCost?.dailyCosts?.map((d) => ({
        date: d.date,
        cost: d.costUsd,
      })) ?? [],
    [projectCost],
  );
  const costCoverage = projectCost?.costCoverage;
  const runtimeBreakdown = useMemo(
    () => projectCost?.runtimeBreakdown ?? [],
    [projectCost],
  );

  const agentCostEntries = useMemo(
    () =>
      agentPerformance.map((entry) => ({
        label: entry.label,
        totalCostUsd: entry.totalCostUsd,
      })),
    [agentPerformance],
  );

  const allocationData = useMemo(() => {
    const byRuntime = new Map<string, number>();
    for (const entry of runtimeBreakdown) {
      byRuntime.set(
        entry.runtime,
        (byRuntime.get(entry.runtime) ?? 0) + entry.totalCostUsd,
      );
    }
    return Array.from(byRuntime.entries()).map(([category, amountUsd]) => ({
      category,
      amountUsd,
    }));
  }, [runtimeBreakdown]);

  const breakdownEntries = useMemo<CostBreakdownEntry[]>(() => {
    const entries: CostBreakdownEntry[] = [];
    const daily = projectCost?.dailyCosts ?? [];
    for (const point of daily) {
      if (point.costUsd <= 0) continue;
      entries.push({
        id: `daily-${point.date}`,
        date: point.date,
        category: t("costOverTime"),
        agent: "—",
        amountUsd: point.costUsd,
      });
    }
    for (const task of taskCosts) {
      if (task.costUsd <= 0) continue;
      entries.push({
        id: `task-${task.taskId}`,
        date: "—",
        category: t("perTaskCost"),
        agent: task.taskTitle,
        amountUsd: task.costUsd,
      });
    }
    return entries;
  }, [projectCost, taskCosts, t]);

  const overspendingAlerts = useMemo(() => {
    const scoped: Array<{
      id: string;
      scope: string;
      spentUsd: number;
      budgetUsd: number;
    }> = sprintCosts
      .filter((sprint) => sprint.budgetUsd > 0)
      .map((sprint) => ({
        id: `sprint-${sprint.sprintId}`,
        scope: sprint.sprintName,
        spentUsd: sprint.costUsd,
        budgetUsd: sprint.budgetUsd,
      }));
    const budgetSummary = projectCost?.budgetSummary;
    if (budgetSummary && budgetSummary.allocated > 0) {
      scoped.unshift({
        id: "project",
        scope: t("title"),
        spentUsd: budgetSummary.spent,
        budgetUsd: budgetSummary.allocated,
      });
    }
    return deriveOverspendingAlerts(scoped);
  }, [sprintCosts, projectCost, t]);

  const forecastInput = useMemo(() => {
    const daily = projectCost?.dailyCosts ?? [];
    const budgetSummary = projectCost?.budgetSummary;
    return {
      dailyCosts: daily,
      budgetUsd: budgetSummary?.allocated ?? null,
      spentUsd: budgetSummary?.spent ?? projectCost?.totalCostUsd ?? 0,
      daysRemaining: 7,
    };
  }, [projectCost]);

  const projectOptions = useMemo(
    () => projects.map((p) => ({ id: p.id, name: p.name })),
    [projects],
  );

  if (!selectedProjectId) {
    return (
      <div className="flex flex-col gap-6">
        <PageHeader title={t("title")} />
        {projectOptions.length > 0 ? (
          <div className="flex justify-end">
            <CostProjectFilter
              projects={projectOptions}
              selectedProjectId={null}
              onChange={(next) =>
                useDashboardStore.setState({ selectedProjectId: next })
              }
            />
          </div>
        ) : null}
        <EmptyState icon={FolderOpen} title={t("selectProjectPrompt")} />
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-6">
      <PageHeader title={t("title")} />

      <div className="flex justify-end">
        <CostProjectFilter
          projects={projectOptions}
          selectedProjectId={selectedProjectId}
          onChange={(next) =>
            useDashboardStore.setState({ selectedProjectId: next })
          }
        />
      </div>

      {overspendingAlerts.length > 0 ? (
        <OverspendingAlertBanner alerts={overspendingAlerts} />
      ) : null}

      {costError ? <ErrorBanner message={costError} /> : null}

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-6">
        <MetricCard
          label={t("totalSpend")}
          value={renderMetric(hasSummary, projectCost?.totalCostUsd, (value) => `$${value.toFixed(2)}`)}
          icon={DollarSign}
        />
        <MetricCard
          label={t("inputTokens")}
          value={renderMetric(hasSummary, projectCost?.totalInputTokens, formatTokens)}
          icon={TrendingUp}
        />
        <MetricCard
          label={t("outputTokens")}
          value={renderMetric(hasSummary, projectCost?.totalOutputTokens, formatTokens)}
          icon={TrendingUp}
        />
        <MetricCard
          label={t("cacheTokens")}
          value={renderMetric(hasSummary, projectCost?.totalCacheReadTokens, formatTokens)}
          icon={Hash}
        />
        <MetricCard
          label={t("totalTurns")}
          value={renderMetric(hasSummary, projectCost?.totalTurns, (value) => String(value))}
          icon={Activity}
        />
        <MetricCard
          label={t("activeAgents")}
          value={renderMetric(hasSummary, projectCost?.activeAgents, (value) => String(value))}
          icon={Cpu}
        />
      </div>

      <BudgetForecastCard input={forecastInput} />

      <Card>
        <CardHeader>
          <CardTitle>{t("spendingTrend")}</CardTitle>
          <CardDescription>{t("spendingTrendDesc")}</CardDescription>
        </CardHeader>
        <CardContent>
          <SpendingTrendChart data={chartData} />
        </CardContent>
      </Card>

      <div className="grid gap-6 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>{t("budgetAllocation")}</CardTitle>
            <CardDescription>{t("budgetAllocationDesc")}</CardDescription>
          </CardHeader>
          <CardContent>
            <BudgetAllocationChart data={allocationData} />
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>{t("agentCostComparison")}</CardTitle>
            <CardDescription>{t("agentCostComparisonDesc")}</CardDescription>
          </CardHeader>
          <CardContent>
            <AgentCostBarChart data={agentCostEntries} />
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>{t("externalRuntimeCoverage")}</CardTitle>
          <CardDescription>{t("externalRuntimeCoverageDesc")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid gap-4 md:grid-cols-3">
            <MetricCard
              label={t("authoritativeSpend")}
              value={`$${(costCoverage?.authoritativeCostUsd ?? 0).toFixed(2)}`}
              icon={DollarSign}
            />
            <MetricCard
              label={t("estimatedSpend")}
              value={`$${(costCoverage?.estimatedCostUsd ?? 0).toFixed(2)}`}
              icon={TrendingUp}
            />
            <MetricCard
              label={t("unpricedRuns")}
              value={String(
                (costCoverage?.unpricedRunCount ?? 0) +
                  (costCoverage?.planIncludedRunCount ?? 0),
              )}
              icon={Hash}
            />
          </div>

          {costCoverage?.hasCoverageGap ? (
            <ErrorBanner message={t("coverageGapWarning")} />
          ) : null}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t("runtimeCostBreakdown")}</CardTitle>
          <CardDescription>{t("runtimeCostBreakdownDesc")}</CardDescription>
        </CardHeader>
        <CardContent>
          {runtimeBreakdown.length > 0 ? (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t("colRuntime")}</TableHead>
                  <TableHead>{t("colProvider")}</TableHead>
                  <TableHead>{t("colModel")}</TableHead>
                  <TableHead>{t("colCoverage")}</TableHead>
                  <TableHead className="text-right">{t("colPricedRuns")}</TableHead>
                  <TableHead className="text-right">{t("colTotalCost")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {runtimeBreakdown.map((entry) => (
                  <TableRow
                    key={`${entry.runtime}:${entry.provider}:${entry.model}`}
                  >
                    <TableCell className="font-medium">{entry.runtime}</TableCell>
                    <TableCell>{entry.provider}</TableCell>
                    <TableCell>{entry.model}</TableCell>
                    <TableCell>
                      <Badge variant={entry.unpricedRunCount > 0 ? "outline" : "secondary"}>
                        {formatCoverageLabel(
                          entry.authoritativeRunCount,
                          entry.estimatedRunCount,
                          entry.unpricedRunCount + entry.planIncludedRunCount,
                          t,
                        )}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-right">
                      {entry.pricedRunCount}/{entry.runCount}
                    </TableCell>
                    <TableCell className="text-right">
                      ${entry.totalCostUsd.toFixed(2)}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          ) : (
            <p className="text-sm text-muted-foreground">{t("noRuntimeBreakdownData")}</p>
          )}
        </CardContent>
      </Card>

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

      <Card>
        <CardHeader className="flex flex-row items-start justify-between gap-2 space-y-0">
          <div>
            <CardTitle>{t("costBreakdown")}</CardTitle>
            <CardDescription>{t("costBreakdownDesc")}</CardDescription>
          </div>
          <CostCsvExport
            data={breakdownEntries}
            fileName={`cost-breakdown-${selectedProjectId}.csv`}
          />
        </CardHeader>
        <CardContent>
          <CostBreakdownTable data={breakdownEntries} />
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t("sprintCostComparison")}</CardTitle>
          <CardDescription>{t("sprintCostDesc")}</CardDescription>
        </CardHeader>
        <CardContent>
          {sprintCosts.length > 0 ? (
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
          ) : (
            <p className="text-sm text-muted-foreground">{t("noSprintCostData")}</p>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t("teamVelocity")}</CardTitle>
          <CardDescription>{t("teamVelocityDesc")}</CardDescription>
        </CardHeader>
        <CardContent>
          {velocityLoading && velocity.length === 0 ? (
            <div className="flex h-[200px] items-center justify-center text-sm text-muted-foreground">
              {t("loadingVelocity")}
            </div>
          ) : (
            <VelocityChart data={velocity} />
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t("agentPerformance")}</CardTitle>
          <CardDescription>{t("agentPerformanceDesc")}</CardDescription>
        </CardHeader>
        <CardContent>
          {performanceLoading && agentPerformance.length === 0 ? (
            <div className="flex h-[120px] items-center justify-center text-sm text-muted-foreground">
              {t("loadingPerformance")}
            </div>
          ) : (
            <AgentPerformanceTable data={agentPerformance} />
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t("perTaskCost")}</CardTitle>
          <CardDescription>{t("perTaskCostDesc")}</CardDescription>
        </CardHeader>
        <CardContent>
          {taskCosts.length > 0 ? (
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
          ) : (
            <p className="text-sm text-muted-foreground">{t("noTaskCostData")}</p>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
