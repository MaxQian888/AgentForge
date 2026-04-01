"use client";

import { useEffect } from "react";
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

  useEffect(() => {
    if (selectedProjectId) {
      void fetchProjectCost(selectedProjectId);
      void fetchVelocity(selectedProjectId);
      void fetchAgentPerformance(selectedProjectId);
    }
  }, [selectedProjectId, fetchProjectCost, fetchVelocity, fetchAgentPerformance]);

  if (!selectedProjectId) {
    return (
      <div className="flex flex-col gap-6">
        <PageHeader title={t("title")} />
        <EmptyState
          icon={FolderOpen}
          title={t("selectProjectPrompt")}
        />
      </div>
    );
  }

  const hasSummary = projectCost !== null;
  const sprintCosts = projectCost?.sprintCosts ?? [];
  const taskCosts = projectCost?.taskCosts ?? [];
  const chartData = projectCost?.dailyCosts?.map((d) => ({ date: d.date, cost: d.costUsd })) ?? [];
  const costCoverage = projectCost?.costCoverage;
  const runtimeBreakdown = projectCost?.runtimeBreakdown ?? [];

  return (
    <div className="flex flex-col gap-6">
      <PageHeader title={t("title")} />

      {costError ? (
        <ErrorBanner message={costError} />
      ) : null}

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
