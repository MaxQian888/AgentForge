"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useTranslations } from "next-intl";
import { Activity, CheckCircle2, DollarSign, Plus, Users } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useDashboardStore } from "@/lib/stores/dashboard-store";
import { useTeamStore } from "@/lib/stores/team-store";
import { TeamCard } from "@/components/team/team-card";
import { TeamCreationWizard } from "@/components/team/team-creation-wizard";
import { ListLayout } from "@/components/layout/templates/list-layout";
import { MetricCard } from "@/components/shared/metric-card";
import { EmptyState } from "@/components/shared/empty-state";
import { ErrorBanner } from "@/components/shared/error-banner";
import { FilterBar, type FilterConfig } from "@/components/shared/filter-bar";
import { MetricCardSkeleton } from "@/components/shared/skeleton-layouts/metric-card-skeleton";

const STATUS_OPTION_KEYS = ["all", "active", "completed", "failed", "cancelled"] as const;

const STATUS_LABEL_KEYS: Record<string, string> = {
  all: "filter.allStatuses",
  active: "filter.active",
  completed: "filter.completed",
  failed: "filter.failed",
  cancelled: "filter.cancelled",
};

function statusFilterToQuery(filter: string): string | undefined {
  if (filter === "all") return undefined;
  if (filter === "active") return undefined; // handled client-side
  return filter;
}

export default function TeamsPage() {
  const t = useTranslations("teams");
  const pathname = usePathname();
  const router = useRouter();
  const searchParams = useSearchParams();
  const requestedProjectId = searchParams.get("project");
  const projects = useDashboardStore((state) => state.projects);
  const selectedProjectId = useDashboardStore((state) => state.selectedProjectId);
  const activeProjectId = requestedProjectId ?? selectedProjectId;
  const { teams, loading, error, fetchTeams, deleteTeam } = useTeamStore();

  const [statusFilter, setStatusFilter] = useState("all");
  const [wizardOpen, setWizardOpen] = useState(false);

  useEffect(() => {
    if (!activeProjectId) return;
    const queryStatus = statusFilterToQuery(statusFilter);
    void fetchTeams(activeProjectId, queryStatus);
  }, [activeProjectId, statusFilter, fetchTeams]);

  const filteredTeams =
    statusFilter === "active"
      ? teams.filter(
          (team) =>
            team.status === "planning" ||
            team.status === "executing" ||
            team.status === "reviewing"
        )
      : teams;

  const activeTeams = teams.filter(
    (team) =>
      team.status === "planning" ||
      team.status === "executing" ||
      team.status === "reviewing"
  );
  const completedTeams = teams.filter((team) => team.status === "completed");
  const totalSpent = teams.reduce((sum, team) => sum + team.totalSpent, 0);

  const handleDelete = async (id: string) => {
    await deleteTeam(id);
  };

  const filterConfigs: FilterConfig[] = [];
  if (projects.length > 0) {
    filterConfigs.push({
      key: "project",
      label: t("projectLabel"),
      placeholder: t("selectProject"),
      value: activeProjectId ?? "all",
      onChange: (value) => {
        if (value === "all") {
          router.replace(pathname);
          return;
        }
        router.replace(`${pathname}?project=${value}`);
      },
      options: projects.map((project) => ({
        value: project.id,
        label: project.name,
      })),
    });
    filterConfigs.push({
      key: "status",
      label: t("statusLabel"),
      value: statusFilter,
      onChange: setStatusFilter,
      options: STATUS_OPTION_KEYS.filter((k) => k !== "all").map((key) => ({
        value: key,
        label: t(STATUS_LABEL_KEYS[key]),
      })),
    });
  }

  const actions = (
    <>
      <Link
        href="/agents"
        className="text-sm text-muted-foreground hover:underline"
      >
        {t("viewAgents")}
      </Link>
      <Button onClick={() => setWizardOpen(true)} size="sm">
        <Plus className="mr-1.5 size-4" />
        {t("wizard.createTeam")}
      </Button>
    </>
  );

  const toolbar = filterConfigs.length > 0 ? (
    <FilterBar
      filters={filterConfigs}
      onReset={() => {
        setStatusFilter("all");
        router.replace(pathname);
      }}
    />
  ) : null;

  return (
    <ListLayout title={t("title")} actions={actions} toolbar={toolbar}>
      <div className="flex flex-col gap-[var(--space-section-gap)]">
        <TeamCreationWizard open={wizardOpen} onOpenChange={setWizardOpen} />

        <div className="grid grid-cols-1 gap-[var(--space-grid-gap)] sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-3 xl:grid-cols-3">
          <MetricCard
            label={t("stats.activeTeams")}
            value={activeTeams.length}
            icon={Activity}
          />
          <MetricCard
            label={t("stats.completed")}
            value={completedTeams.length}
            icon={CheckCircle2}
          />
          <MetricCard
            label={t("stats.totalSpent")}
            value={`$${totalSpent.toFixed(2)}`}
            icon={DollarSign}
          />
        </div>

        {!activeProjectId ? (
          <EmptyState icon={Users} title={t("empty.selectProject")} />
        ) : loading ? (
          <div className="grid grid-cols-1 gap-[var(--space-grid-gap)] sm:grid-cols-2 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-3">
            {Array.from({ length: 3 }).map((_, i) => (
              <MetricCardSkeleton key={i} />
            ))}
          </div>
        ) : error ? (
          <ErrorBanner
            message={error}
            onRetry={() => void fetchTeams(activeProjectId)}
          />
        ) : filteredTeams.length === 0 ? (
          <EmptyState
            icon={Users}
            title={
              statusFilter !== "all"
                ? t("empty.noFilteredTeams", { status: statusFilter })
                : t("empty.noTeams")
            }
          />
        ) : (
          <div className="grid grid-cols-1 gap-[var(--space-grid-gap)] sm:grid-cols-2 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-3">
            {filteredTeams.map((team) => (
              <TeamCard key={team.id} team={team} onDelete={handleDelete} />
            ))}
          </div>
        )}
      </div>
    </ListLayout>
  );
}
