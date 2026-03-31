"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";
import { Activity, AlertTriangle, Bot, ClipboardCheck, DollarSign, Users } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { QuickActionShortcuts } from "./quick-action-shortcuts";
import type { DashboardSummary } from "@/lib/dashboard/summary";

interface DashboardOverviewProps {
  summary: DashboardSummary | null;
  loading: boolean;
  error: string | null;
  sectionErrors: Record<string, string>;
  onRetry: (section?: string) => void;
}

function formatCurrency(value: number) {
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency: "USD",
  }).format(value);
}

function isEmptySummary(summary: DashboardSummary) {
  return (
    summary.progress.total === 0 &&
    summary.headline.activeAgents === 0 &&
    summary.headline.pendingReviews === 0 &&
    summary.team.totalMembers === 0 &&
    summary.activity.length === 0 &&
    summary.risks.length === 0
  );
}

export function DashboardOverview({
  summary,
  loading,
  error,
  sectionErrors,
  onRetry,
}: DashboardOverviewProps) {
  const t = useTranslations("dashboard");

  if (loading) {
    return <p className="text-sm text-muted-foreground">{t("loadingInsights")}</p>;
  }

  if (error) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>{t("error.title")}</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          <p className="text-sm text-muted-foreground">{error}</p>
          <div>
            <Button type="button" onClick={() => onRetry()}>
              {t("error.retry")}
            </Button>
          </div>
        </CardContent>
      </Card>
    );
  }

  if (!summary) {
    return null;
  }

  if (isEmptySummary(summary)) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>{summary.scope.projectName}</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          <p className="text-sm text-muted-foreground">
            {t("empty.noSignals")}
          </p>
          <div>
            <Button asChild>
              <Link href={summary.links.projects}>{t("empty.createProject")}</Link>
            </Button>
          </div>
        </CardContent>
      </Card>
    );
  }

  const insightCards = [
    {
      key: "progress",
      title: t("cards.taskProgress"),
      icon: Activity,
      value: `${summary.progress.inProgress}/${summary.progress.total}`,
      detail: t("cards.inReview", { count: summary.progress.inReview }),
      href: summary.scope.projectId
        ? `/project?id=${summary.scope.projectId}`
        : summary.links.projects,
    },
    {
      key: "agents",
      title: t("cards.activeAgents"),
      icon: Bot,
      value: String(summary.headline.activeAgents),
      detail: t("cards.activeRuns", { count: summary.team.activeAgentRuns }),
      href: summary.links.agents,
    },
    {
      key: "reviews",
      title: t("cards.pendingReviews"),
      icon: ClipboardCheck,
      value: String(summary.headline.pendingReviews),
      detail: t("cards.assigned", { count: summary.progress.assigned }),
      href: summary.links.reviews,
    },
    {
      key: "cost",
      title: t("cards.weeklyCost"),
      icon: DollarSign,
      value: formatCurrency(summary.headline.weeklyCost),
      detail: t("cards.trackedProjects", { count: summary.scope.projectsCount }),
      href: "/cost",
    },
    {
      key: "team",
      title: t("cards.teamCapacity"),
      icon: Users,
      value: t("cards.members", { count: summary.team.totalMembers }),
      detail: t("cards.overloaded", { count: summary.team.overloadedMembers }),
      href: summary.links.team,
    },
  ];
  const quickActions = [
    {
      id: "create-task",
      label: t("actions.createTask"),
      href: summary.scope.projectId
        ? `/project?id=${summary.scope.projectId}`
        : summary.links.projects,
      icon: Activity,
      shortcut: "N",
    },
    {
      id: "spawn-agent",
      label: t("actions.spawnAgent"),
      href: summary.links.agents,
      icon: Bot,
      shortcut: "A",
      variant: "outline" as const,
    },
    {
      id: "manage-sprints",
      label: t("actions.manageSprints"),
      href: "/sprints",
      icon: ClipboardCheck,
      shortcut: "S",
      variant: "outline" as const,
    },
    {
      id: "configure-roles",
      label: t("actions.configureRoles"),
      href: "/roles",
      icon: Users,
      shortcut: "R",
      variant: "outline" as const,
    },
  ];

  return (
    <div className="flex flex-col gap-6">
      <div className="flex items-center justify-between">
        <div className="space-y-1">
          <h1 className="text-2xl font-bold">{summary.scope.projectName}</h1>
          <p className="text-sm text-muted-foreground">
            {t("header.description")}
          </p>
        </div>
        <Button asChild variant="outline">
          <Link href={summary.links.team}>{t("header.openTeam")}</Link>
        </Button>
      </div>

      <div className="grid gap-4 lg:grid-cols-5">
        {insightCards.map((card) => {
          const Icon = card.icon;
          return (
            <Card key={card.key}>
              <CardHeader className="pb-2">
                <div className="flex items-center justify-between gap-2">
                  <CardTitle className="text-sm font-medium text-muted-foreground">
                    {card.title}
                  </CardTitle>
                  <Icon className="size-4 text-muted-foreground" />
                </div>
              </CardHeader>
              <CardContent className="space-y-3">
                <div className="text-2xl font-semibold">{card.value}</div>
                <p className="text-xs text-muted-foreground">{card.detail}</p>
                <Button asChild size="sm" variant="ghost" className="px-0">
                  <Link href={card.href}>{t("cards.drillDown")}</Link>
                </Button>
              </CardContent>
            </Card>
          );
        })}
      </div>

      <QuickActionShortcuts actions={quickActions} />

      <div className="grid gap-4 xl:grid-cols-[1.6fr_1fr]">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between gap-2">
            <div>
              <CardTitle>{t("activity.title")}</CardTitle>
            </div>
            {sectionErrors.activity ? (
              <div className="flex items-center gap-2">
                <span className="text-xs text-destructive">{sectionErrors.activity}</span>
                <Button
                  type="button"
                  size="sm"
                  variant="outline"
                  onClick={() => onRetry("activity")}
                >
                  {t("activity.retrySection")}
                </Button>
              </div>
            ) : null}
          </CardHeader>
          <CardContent className="space-y-3">
            {summary.activity.length === 0 ? (
              <p className="text-sm text-muted-foreground">
                {t("activity.empty")}
              </p>
            ) : (
              summary.activity.map((item) => (
                <div
                  key={item.id}
                  className="rounded-lg border border-border/60 p-3"
                >
                  <div className="flex items-center justify-between gap-3">
                    <div>
                      <p className="font-medium">{item.title}</p>
                      <p className="text-sm text-muted-foreground">{item.message}</p>
                    </div>
                    <Button asChild size="sm" variant="ghost">
                      <Link href={item.href}>{t("activity.open")}</Link>
                    </Button>
                  </div>
                </div>
              ))
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <AlertTriangle className="size-4 text-amber-600" />
              {t("risks.title")}
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            {summary.risks.length === 0 ? (
              <p className="text-sm text-muted-foreground">
                {t("risks.empty")}
              </p>
            ) : (
              summary.risks.map((risk) => (
                <div
                  key={risk.id}
                  className="rounded-lg border border-amber-200 bg-amber-50/50 p-3 dark:border-amber-900 dark:bg-amber-950/20"
                >
                  <p className="font-medium">{risk.title}</p>
                  <p className="mt-1 text-sm text-muted-foreground">
                    {risk.description}
                  </p>
                  <Button asChild size="sm" variant="ghost" className="mt-2 px-0">
                    <Link href={risk.href}>{t("risks.investigate")}</Link>
                  </Button>
                </div>
              ))
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
