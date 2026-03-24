"use client";

import Link from "next/link";
import { Activity, AlertTriangle, Bot, ClipboardCheck, DollarSign, Users } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
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
  if (loading) {
    return <p className="text-sm text-muted-foreground">Loading dashboard insights...</p>;
  }

  if (error) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>Dashboard insights unavailable</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          <p className="text-sm text-muted-foreground">{error}</p>
          <div>
            <Button type="button" onClick={() => onRetry()}>
              Retry Dashboard
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
            No delivery signals yet for this scope.
          </p>
          <div>
            <Button asChild>
              <Link href={summary.links.projects}>Create or open a project</Link>
            </Button>
          </div>
        </CardContent>
      </Card>
    );
  }

  const insightCards = [
    {
      key: "progress",
      title: "Task Progress",
      icon: Activity,
      value: `${summary.progress.inProgress}/${summary.progress.total}`,
      detail: `${summary.progress.inReview} in review`,
      href: summary.scope.projectId
        ? `/project?id=${summary.scope.projectId}`
        : summary.links.projects,
    },
    {
      key: "agents",
      title: "Active Agents",
      icon: Bot,
      value: String(summary.headline.activeAgents),
      detail: `${summary.team.activeAgentRuns} active runs`,
      href: summary.links.agents,
    },
    {
      key: "reviews",
      title: "Pending Reviews",
      icon: ClipboardCheck,
      value: String(summary.headline.pendingReviews),
      detail: `${summary.progress.assigned} assigned`,
      href: summary.links.reviews,
    },
    {
      key: "cost",
      title: "Weekly Cost",
      icon: DollarSign,
      value: formatCurrency(summary.headline.weeklyCost),
      detail: `${summary.scope.projectsCount} tracked projects`,
      href: "/cost",
    },
    {
      key: "team",
      title: "Team Capacity",
      icon: Users,
      value: `${summary.team.totalMembers} members`,
      detail: `${summary.team.overloadedMembers} overloaded`,
      href: summary.links.team,
    },
  ];

  return (
    <div className="flex flex-col gap-6">
      <div className="flex items-center justify-between">
        <div className="space-y-1">
          <h1 className="text-2xl font-bold">{summary.scope.projectName}</h1>
          <p className="text-sm text-muted-foreground">
            Cross-project visibility for delivery flow, reviews, spend, and team coverage.
          </p>
        </div>
        <Button asChild variant="outline">
          <Link href={summary.links.team}>Open Team</Link>
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
                  <Link href={card.href}>Drill Down</Link>
                </Button>
              </CardContent>
            </Card>
          );
        })}
      </div>

      <div className="flex flex-wrap gap-3">
        <Button asChild>
          <Link href={summary.scope.projectId ? `/project?id=${summary.scope.projectId}` : summary.links.projects}>
            Create Task
          </Link>
        </Button>
        <Button asChild variant="outline">
          <Link href={summary.links.agents}>Spawn Agent</Link>
        </Button>
        <Button asChild variant="outline">
          <Link href="/sprints">Manage Sprints</Link>
        </Button>
        <Button asChild variant="outline">
          <Link href="/roles">Configure Roles</Link>
        </Button>
      </div>

      <div className="grid gap-4 xl:grid-cols-[1.6fr_1fr]">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between gap-2">
            <div>
              <CardTitle>Recent Activity</CardTitle>
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
                  Retry Section
                </Button>
              </div>
            ) : null}
          </CardHeader>
          <CardContent className="space-y-3">
            {summary.activity.length === 0 ? (
              <p className="text-sm text-muted-foreground">
                No recent activity for the selected scope.
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
                      <Link href={item.href}>Open</Link>
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
              Risk Signals
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            {summary.risks.length === 0 ? (
              <p className="text-sm text-muted-foreground">
                No immediate operational risks detected.
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
                    <Link href={risk.href}>Investigate</Link>
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
