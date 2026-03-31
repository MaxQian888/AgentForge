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
import { useBreadcrumbs } from "@/hooks/use-breadcrumbs";
import { OverviewLayout } from "@/components/layout/templates";
import { MetricCard } from "@/components/shared/metric-card";
import { ActivityFeed } from "@/components/dashboard/activity-feed";
import { AgentFleetWidget } from "@/components/dashboard/agent-fleet-widget";
import { TeamHealthWidget } from "@/components/dashboard/team-health-widget";
import { BudgetWidget } from "@/components/dashboard/budget-widget";
import { DashboardWidgetsSkeleton } from "@/components/dashboard/dashboard-widget-skeletons";
import { QuickActionShortcuts } from "@/components/dashboard/quick-action-shortcuts";
import { Skeleton } from "@/components/ui/skeleton";
import { useDashboardStore } from "@/lib/stores/dashboard-store";
import { useAgentStore } from "@/lib/stores/agent-store";
import { useCostStore } from "@/lib/stores/cost-store";

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
  const quickActions = [
    {
      id: "create-task",
      label: t("actions.createTask"),
      href:
        summary?.scope.projectId
          ? `/project?id=${summary.scope.projectId}`
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
      href: "/sprints",
      icon: Zap,
      shortcut: "S",
      variant: "ghost" as const,
    },
    {
      id: "create-team",
      label: t("actions.createTeam"),
      href: "/team",
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
        <WidgetsSkeleton />
      </OverviewLayout>
    );
  }

  return (
    <OverviewLayout
      title={t("pageTitle")}
      breadcrumbs={[{ label: t("breadcrumb.home") }]}
      metrics={
        <>
          <MetricCard
            label={t("cards.taskProgress")}
            value={`${summary?.progress.inProgress ?? 0}/${summary?.progress.total ?? 0}`}
            icon={Activity}
          />
          <MetricCard
            label={t("cards.activeAgents")}
            value={String(summary?.headline.activeAgents ?? 0)}
            icon={Bot}
          />
          <MetricCard
            label={t("cards.pendingReviews")}
            value={String(summary?.headline.pendingReviews ?? 0)}
            icon={ClipboardCheck}
          />
          <MetricCard
            label={t("cards.weeklyCost")}
            value={formatCurrency(summary?.headline.weeklyCost ?? 0)}
            icon={DollarSign}
          />
          <MetricCard
            label={t("cards.teamCapacity")}
            value={t("cards.members", { count: summary?.team.totalMembers ?? 0 })}
            icon={Users}
          />
        </>
      }
    >
      {projects.length > 0 ? (
        <div className="flex flex-wrap items-end gap-4">
          <label className="flex min-w-[220px] flex-col gap-2 text-sm font-medium">
            <span>{t("projectFilterLabel")}</span>
            <select
              aria-label={t("projectFilterLabel")}
              className="h-9 rounded-md border border-input bg-background px-3 text-sm font-normal"
              value={projectId ?? ""}
              onChange={(event) => handleProjectChange(event.target.value)}
            >
              <option value="">{t("allProjects")}</option>
              {projects.map((project) => (
                <option key={project.id} value={project.id}>
                  {project.name}
                </option>
              ))}
            </select>
          </label>
        </div>
      ) : null}

      <div className="grid gap-4 lg:grid-cols-2">
        <ActivityFeed events={activityEvents} />
        <AgentFleetWidget agents={fleetAgents} />
        <TeamHealthWidget members={teamMembers} />
        <BudgetWidget
          totalBudget={budgetTotal}
          spent={budgetSpent}
          remaining={budgetRemaining}
        />
      </div>

      <QuickActionShortcuts actions={quickActions} />
    </OverviewLayout>
  );
}
