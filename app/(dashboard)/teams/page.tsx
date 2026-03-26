"use client";

import { useEffect } from "react";
import Link from "next/link";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { Users } from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { useDashboardStore } from "@/lib/stores/dashboard-store";
import { useTeamStore } from "@/lib/stores/team-store";
import { TeamCard } from "@/components/team/team-card";

export default function TeamsPage() {
  const pathname = usePathname();
  const router = useRouter();
  const searchParams = useSearchParams();
  const requestedProjectId = searchParams.get("project");
  const projects = useDashboardStore((state) => state.projects);
  const selectedProjectId = useDashboardStore((state) => state.selectedProjectId);
  const activeProjectId = requestedProjectId ?? selectedProjectId;
  const { teams, loading, error, fetchTeams } = useTeamStore();

  useEffect(() => {
    if (!activeProjectId) return;
    void fetchTeams(activeProjectId);
  }, [activeProjectId, fetchTeams]);

  const activeTeams = teams.filter(
    (t) =>
      t.status === "planning" ||
      t.status === "executing" ||
      t.status === "reviewing"
  );
  const completedTeams = teams.filter((t) => t.status === "completed");
  const totalSpent = teams.reduce((sum, t) => sum + t.totalSpent, 0);

  return (
    <div className="flex flex-col gap-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Agent Teams</h1>
        <Link
          href="/agents"
          className="text-sm text-muted-foreground hover:underline"
        >
          View Agents
        </Link>
      </div>

      {projects.length > 0 ? (
        <div className="flex max-w-sm flex-col gap-2">
          <Label htmlFor="teams-project-selector">Project</Label>
          <select
            id="teams-project-selector"
            aria-label="Project"
            className="h-9 rounded-md border border-input bg-transparent px-3 text-sm"
            value={activeProjectId ?? ""}
            onChange={(event) => router.replace(`${pathname}?project=${event.target.value}`)}
          >
            {projects.map((project) => (
              <option key={project.id} value={project.id}>
                {project.name}
              </option>
            ))}
          </select>
        </div>
      ) : null}

      <div className="grid gap-4 sm:grid-cols-3">
        <Card>
          <CardContent className="py-4">
            <p className="text-sm text-muted-foreground">Active Teams</p>
            <p className="text-2xl font-bold">{activeTeams.length}</p>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="py-4">
            <p className="text-sm text-muted-foreground">Completed</p>
            <p className="text-2xl font-bold">{completedTeams.length}</p>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="py-4">
            <p className="text-sm text-muted-foreground">Total Spent</p>
            <p className="text-2xl font-bold">${totalSpent.toFixed(2)}</p>
          </CardContent>
        </Card>
      </div>

      {!activeProjectId ? (
        <Card>
          <CardContent className="py-12 text-center">
            <Users className="mx-auto mb-4 size-12 text-muted-foreground" />
            <p className="text-muted-foreground">Select a project to inspect team runs.</p>
          </CardContent>
        </Card>
      ) : loading ? (
        <p className="text-muted-foreground">Loading teams...</p>
      ) : error ? (
        <Card>
          <CardContent className="flex flex-col items-center gap-4 py-12 text-center">
            <Users className="size-10 text-muted-foreground" />
            <p className="text-muted-foreground">{error}</p>
            <Button type="button" onClick={() => void fetchTeams(activeProjectId)}>
              Retry
            </Button>
          </CardContent>
        </Card>
      ) : teams.length === 0 ? (
        <Card>
          <CardContent className="py-12 text-center">
            <Users className="mx-auto mb-4 size-12 text-muted-foreground" />
            <p className="text-muted-foreground">
              No agent teams yet. Start a team from a task detail page.
            </p>
          </CardContent>
        </Card>
      ) : (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {teams.map((team) => (
            <TeamCard key={team.id} team={team} />
          ))}
        </div>
      )}
    </div>
  );
}
