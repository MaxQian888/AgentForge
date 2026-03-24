"use client";

import { useEffect } from "react";
import Link from "next/link";
import { Users } from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
import { useTeamStore } from "@/lib/stores/team-store";
import { TeamCard } from "@/components/team/team-card";

export default function TeamsPage() {
  const { teams, loading, fetchTeams } = useTeamStore();

  useEffect(() => {
    fetchTeams();
  }, [fetchTeams]);

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

      {loading ? (
        <p className="text-muted-foreground">Loading teams...</p>
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
