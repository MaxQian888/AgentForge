"use client";

import { useEffect, useMemo } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useTranslations } from "next-intl";
import {
  Activity,
  Bot,
  ClipboardCheck,
  DollarSign,
  Plus,
  Rocket,
  Users,
  Zap,
} from "lucide-react";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useBreadcrumbs } from "@/hooks/use-breadcrumbs";
import { OverviewLayout } from "@/components/layout/templates";
import { MetricCard } from "@/components/shared/metric-card";
import { ActivityFeed } from "@/components/dashboard/activity-feed";
import { AgentFleetWidget } from "@/components/dashboard/agent-fleet-widget";
import { TeamHealthWidget } from "@/components/dashboard/team-health-widget";
import { BudgetWidget } from "@/components/dashboard/budget-widget";
import { DashboardWidgetsSkeleton } from "@/components/dashboard/dashboard-widget-skeletons";
import { ProjectBootstrapPanel } from "@/components/dashboard/project-bootstrap-panel";
import { QuickActionShortcuts } from "@/components/dashboard/quick-action-shortcuts";
import { Skeleton } from "@/components/ui/skeleton";
import {
  buildRecentCountSparkline,
  buildRecentSumSparkline,
  buildSparklineTrend,
} from "@/lib/dashboard/metric-sparkline";
import { useDashboardStore } from "@/lib/stores/dashboard-store";
import { useAgentStore } from "@/lib/stores/agent-store";
import { useCostStore } from "@/lib/stores/cost-store";
import { buildProjectScopedHref } from "@/lib/route-hrefs";

function formatCurrency(value: number) {
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency: "USD",
  }).format(value);
}

function MetricsSkeleton() {
  return (
    <>
      {Array.from({ length: 5 }).map((_, i) => (
        <Skeleton key={i} className="h-[88px] rounded-lg" />
      ))}
    </>
  );
}

function WidgetsSkeleton() {
  return <DashboardWidgetsSkeleton />;
}

export default function DashboardPage() {
  const t = useTranslations("dashboard");
  const pathname = usePathname();
  const router = useRouter();
  const searchParams = useSearchParams();
  const projectId = searchParams.get("project");

  const summary = useDashboardStore((s) => s.summary);
  const projects = useDashboardStore((s) => s.projects);
  const loading = useDashboardStore((s) => s.loading);
  const dashboardTasks = useDashboardStore((s) => s.tasks ?? []);
  const fetchSummary = useDashboardStore((s) => s.fetchSummary);
  const dashboardActivity = useDashboardStore((s) => s.activity);
  const dashboardMembers = useDashboardStore((s) => s.members);
  const dashboardAgents = useDashboardStore((s) => s.agents);

  const agents = useAgentStore((s) => s.agents);
  const fetchAgents = useAgentStore((s) => s.fetchAgents);

  const projectCost = useCostStore((s) => s.projectCost);

  useBreadcrumbs([{ label: t("breadcrumb.home") }]);

  useEffect(() => {
    void fetchSummary({ projectId });
    void fetchAgents();
  }, [fetchSummary, fetchAgents, projectId]);

  const activityEvents = useMemo(
    () =>
      dashboardActivity.map((item) => ({
        id: item.id,
        type: item.type,
        title: item.title,
        timestamp: item.createdAt,
        status: item.type.includes("fail")
          ? "failed"
          : item.type.includes("complete")
            ? "completed"
            : item.type.includes("start")
              ? "running"
              : "pending",
      })),
    [dashboardActivity]
  );

  const fleetAgents = useMemo(
    () =>
      agents.filter(
        (a) => a.status === "running" || a.status === "starting" || a.status === "paused"
      ),
    [agents]
  );

  const teamMembers = useMemo(() => {
    if (!dashboardMembers.length) return [];
    const tasksByMember = new Map<string, number>();
    const totalTasks = summary?.progress.total ?? 1;

    for (const agent of dashboardAgents) {
      tasksByMember.set(
        agent.memberId,
        (tasksByMember.get(agent.memberId) ?? 0) + 1
      );
    }

    return dashboardMembers.slice(0, 8).map((member) => {
      const assigned = tasksByMember.get(member.id) ?? 0;
      const workloadPercent =
        totalTasks > 0 ? Math.min(Math.round((assigned / Math.max(totalTasks * 0.2, 1)) * 100), 100) : 0;
      return {
        id: member.id,
        name: member.name,
        role: member.role || (member.type === "human" ? "Contributor" : "Agent"),
        workloadPercent,
        status: member.isActive ? t("teamHealth.active") : t("teamHealth.idle"),
      };
    });
  }, [dashboardMembers, dashboardAgents, summary, t]);

  const budgetTotal = projectCost?.budgetSummary?.allocated ?? 0;
  const budgetSpent = projectCost?.budgetSummary?.spent ?? summary?.headline.weeklyCost ?? 0;
  const budgetRemaining = projectCost?.budgetSummary?.remaining ?? Math.max(budgetTotal - budgetSpent, 0);
  const taskProgressSparkline = useMemo(
    () =>
      buildRecentCountSparkline(
        dashboardTasks
          .filter((task) => task.status === "in_progress")
          .map((task) => ({
            timestamp: task.updatedAt || task.createdAt,
          })),
      ),
    [dashboardTasks],
  );
  const activeAgentsSparkline = useMemo(
    () =>
      buildRecentCountSparkline(
        agents
          .filter(
            (agent) =>
              agent.status === "running" ||
              agent.status === "starting" ||
              agent.status === "paused",
          )
          .map((agent) => ({
            timestamp: agent.lastActivity || agent.startedAt || agent.createdAt,
          })),
      ),
    [agents],
  );
  const pendingReviewsSparkline = useMemo(
    () =>
      buildRecentCountSparkline(
        dashboardTasks
          .filter((task) => task.status === "in_review")
          .map((task) => ({
            timestamp: task.updatedAt || task.createdAt,
          })),
      ),
    [dashboardTasks],
  );
  const weeklyCostSparkline = useMemo(
    () =>
      buildRecentSumSparkline([
        ...dashboardTasks.map((task) => ({
          timestamp: task.updatedAt || task.createdAt,
          amount: task.spentUsd ?? 0,
        })),
        ...agents.map((agent) => ({
          timestamp: agent.lastActivity || agent.startedAt || agent.createdAt,
          amount: agent.cost ?? 0,
        })),
      ]),
    [agents, dashboardTasks],
  );
  const quickActions = [
    {
      id: "create-task",
      label: t("actions.createTask"),
      href:
        summary?.scope.projectId
          ? buildProjectScopedHref("/project", {
              projectId: summary.scope.projectId,
              projectParam: "id",
              params: { action: "create-task" },
            })
          : "/projects",
      icon: Plus,
      shortcut: "N",
      variant: "ghost" as const,
    },
    {
      id: "spawn-agent",
      label: t("actions.spawnAgent"),
      href: summary?.links.agents ?? "/agents",
      icon: Rocket,
      shortcut: "A",
      variant: "ghost" as const,
    },
    {
      id: "new-sprint",
      label: t("actions.newSprint"),
      href: summary?.scope.projectId
        ? buildProjectScopedHref("/sprints", {
            projectId: summary.scope.projectId,
            params: { action: "create-sprint" },
          })
        : "/sprints",
      icon: Zap,
      shortcut: "S",
      variant: "ghost" as const,
    },
    {
      id: "create-team",
      label: t("actions.createTeam"),
      href: summary?.scope.projectId
        ? buildProjectScopedHref("/team", {
            projectId: summary.scope.projectId,
            params: { focus: "add-member" },
          })
        : "/team",
      icon: Users,
      shortcut: "T",
      variant: "ghost" as const,
    },
  ];

  const handleProjectChange = (nextProjectId: string) => {
    if (!nextProjectId) {
      router.replace(pathname);
      return;
    }

    router.replace(`${pathname}?project=${nextProjectId}`);
  };

  if (loading) {
    return (
      <OverviewLayout
        title={t("pageTitle")}
        breadcrumbs={[{ label: t("breadcrumb.home") }]}
        metrics={<MetricsSkeleton />}
      >
        <div className="lg:col-span-2">
          <WidgetsSkeleton />
        </div>
      </OverviewLayout>
    );
  }

  const metrics = (
    <>
      <MetricCard
        label={t("cards.taskProgress")}
        value={`${summary?.progress.inProgress ?? 0}/${summary?.progress.total ?? 0}`}
        icon={Activity}
        sparkline={taskProgressSparkline}
        trend={buildSparklineTrend(taskProgressSparkline)}
      />
      <MetricCard
        label={t("cards.activeAgents")}
        value={String(summary?.headline.activeAgents ?? 0)}
        icon={Bot}
        sparkline={activeAgentsSparkline}
        trend={buildSparklineTrend(activeAgentsSparkline)}
      />
      <MetricCard
        label={t("cards.pendingReviews")}
        value={String(summary?.headline.pendingReviews ?? 0)}
        icon={ClipboardCheck}
        sparkline={pendingReviewsSparkline}
        trend={buildSparklineTrend(pendingReviewsSparkline)}
      />
      <MetricCard
        label={t("cards.weeklyCost")}
        value={formatCurrency(summary?.headline.weeklyCost ?? 0)}
        icon={DollarSign}
        sparkline={weeklyCostSparkline}
        trend={buildSparklineTrend(weeklyCostSparkline)}
      />
      <MetricCard
        label={t("cards.teamCapacity")}
        value={t("cards.members", { count: summary?.team.totalMembers ?? 0 })}
        icon={Users}
      />
    </>
  );

  return (
    <div className="flex flex-col gap-[var(--space-section-gap)]">
      <OverviewLayout
        title={t("pageTitle")}
        breadcrumbs={[{ label: t("breadcrumb.home") }]}
        metrics={metrics}
      >
        <div className="flex flex-col gap-[var(--space-grid-gap)]">
          <ActivityFeed events={activityEvents} />
          <TeamHealthWidget members={teamMembers} />
        </div>
        <div className="flex flex-col gap-[var(--space-grid-gap)]">
          <AgentFleetWidget agents={fleetAgents} />
          <BudgetWidget
            totalBudget={budgetTotal}
            spent={budgetSpent}
            remaining={budgetRemaining}
          />
        </div>
      </OverviewLayout>

      {projects.length > 0 && (
        <div className="flex flex-wrap items-end gap-4">
          <div className="flex min-w-[220px] flex-col gap-2 text-sm font-medium">
            <span>{t("projectFilterLabel")}</span>
            <Select value={projectId ?? "__all__"} onValueChange={(v) => handleProjectChange(v === "__all__" ? "" : v)}>
              <SelectTrigger className="h-9 font-normal" aria-label={t("projectFilterLabel")}>
                <SelectValue placeholder={t("allProjects")} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="__all__">{t("allProjects")}</SelectItem>
                {projects.map((project) => (
                  <SelectItem key={project.id} value={project.id}>
                    {project.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </div>
      )}

      <ProjectBootstrapPanel bootstrap={summary?.bootstrap} />

      <QuickActionShortcuts actions={quickActions} />
    </div>
  );
}
