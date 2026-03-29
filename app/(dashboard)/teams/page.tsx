"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useTranslations } from "next-intl";
import { Users } from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { useDashboardStore } from "@/lib/stores/dashboard-store";
import { useTeamStore } from "@/lib/stores/team-store";
import { TeamCard } from "@/components/team/team-card";

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

  useEffect(() => {
    if (!activeProjectId) return;
    const queryStatus = statusFilterToQuery(statusFilter);
    void fetchTeams(activeProjectId, queryStatus);
  }, [activeProjectId, statusFilter, fetchTeams]);

  const filteredTeams =
    statusFilter === "active"
      ? teams.filter(
          (t) =>
            t.status === "planning" ||
            t.status === "executing" ||
            t.status === "reviewing"
        )
      : teams;

  const activeTeams = teams.filter(
    (t) =>
      t.status === "planning" ||
      t.status === "executing" ||
      t.status === "reviewing"
  );
  const completedTeams = teams.filter((t) => t.status === "completed");
  const totalSpent = teams.reduce((sum, t) => sum + t.totalSpent, 0);

  const handleDelete = async (id: string) => {
    await deleteTeam(id);
  };

  return (
    <div className="flex flex-col gap-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">{t("title")}</h1>
        <Link
          href="/agents"
          className="text-sm text-muted-foreground hover:underline"
        >
          {t("viewAgents")}
        </Link>
      </div>

      {projects.length > 0 && (
        <div className="flex flex-wrap items-end gap-4">
          <div className="flex flex-col gap-2">
            <Label>{t("projectLabel")}</Label>
            <Select
              value={activeProjectId ?? ""}
              onValueChange={(value) =>
                router.replace(`${pathname}?project=${value}`)
              }
            >
              <SelectTrigger className="w-[220px]">
                <SelectValue placeholder={t("selectProject")} />
              </SelectTrigger>
              <SelectContent>
                {projects.map((project) => (
                  <SelectItem key={project.id} value={project.id}>
                    {project.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="flex flex-col gap-2">
            <Label>{t("statusLabel")}</Label>
            <Select value={statusFilter} onValueChange={setStatusFilter}>
              <SelectTrigger className="w-[160px]">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {STATUS_OPTION_KEYS.map((key) => (
                  <SelectItem key={key} value={key}>
                    {t(STATUS_LABEL_KEYS[key])}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </div>
      )}

      <div className="grid gap-4 sm:grid-cols-3">
        <Card>
          <CardContent className="py-4">
            <p className="text-sm text-muted-foreground">{t("stats.activeTeams")}</p>
            <p className="text-2xl font-bold">{activeTeams.length}</p>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="py-4">
            <p className="text-sm text-muted-foreground">{t("stats.completed")}</p>
            <p className="text-2xl font-bold">{completedTeams.length}</p>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="py-4">
            <p className="text-sm text-muted-foreground">{t("stats.totalSpent")}</p>
            <p className="text-2xl font-bold">${totalSpent.toFixed(2)}</p>
          </CardContent>
        </Card>
      </div>

      {!activeProjectId ? (
        <Card>
          <CardContent className="py-12 text-center">
            <Users className="mx-auto mb-4 size-12 text-muted-foreground" />
            <p className="text-muted-foreground">{t("empty.selectProject")}</p>
          </CardContent>
        </Card>
      ) : loading ? (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {Array.from({ length: 3 }).map((_, i) => (
            <Card key={i}>
              <CardContent className="flex flex-col gap-3 py-4">
                <div className="flex items-center justify-between">
                  <Skeleton className="h-5 w-32" />
                  <Skeleton className="h-5 w-16 rounded-full" />
                </div>
                <Skeleton className="h-4 w-40" />
                <div className="flex items-center gap-2">
                  <Skeleton className="size-2.5 rounded-full" />
                  <Skeleton className="size-2.5 rounded-full" />
                  <Skeleton className="size-2.5 rounded-full" />
                </div>
                <Skeleton className="h-1.5 w-full rounded-full" />
                <div className="flex justify-between">
                  <Skeleton className="h-3 w-16" />
                  <Skeleton className="h-3 w-24" />
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      ) : error ? (
        <Card>
          <CardContent className="flex flex-col items-center gap-4 py-12 text-center">
            <Users className="size-10 text-muted-foreground" />
            <p className="text-muted-foreground">{error}</p>
            <Button type="button" onClick={() => void fetchTeams(activeProjectId)}>
              {t("error.retry")}
            </Button>
          </CardContent>
        </Card>
      ) : filteredTeams.length === 0 ? (
        <Card>
          <CardContent className="py-12 text-center">
            <Users className="mx-auto mb-4 size-12 text-muted-foreground" />
            <p className="text-muted-foreground">
              {statusFilter !== "all"
                ? t("empty.noFilteredTeams", { status: statusFilter })
                : t("empty.noTeams")}
            </p>
          </CardContent>
        </Card>
      ) : (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {filteredTeams.map((team) => (
            <TeamCard
              key={team.id}
              team={team}
              onDelete={handleDelete}
            />
          ))}
        </div>
      )}
    </div>
  );
}
