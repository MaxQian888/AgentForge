"use client";

import { useEffect, useMemo } from "react";
import { useDashboardStore, type DashboardConfig } from "@/lib/stores/dashboard-store";
import { DashboardGrid } from "@/components/dashboard/dashboard-grid";

const EMPTY_DASHBOARDS: DashboardConfig[] = [];

export default function ProjectDashboardPage() {
  const projectId = useDashboardStore((state) => state.selectedProjectId);
  const dashboardsByProject = useDashboardStore((state) => state.dashboardsByProject);
  const fetchDashboards = useDashboardStore((state) => state.fetchDashboards);
  const createDashboard = useDashboardStore((state) => state.createDashboard);

  const dashboards = useMemo(
    () => (projectId ? dashboardsByProject[projectId] ?? EMPTY_DASHBOARDS : EMPTY_DASHBOARDS),
    [dashboardsByProject, projectId]
  );

  useEffect(() => {
    if (projectId) {
      void fetchDashboards(projectId);
    }
  }, [fetchDashboards, projectId]);

  const selectedDashboard = useMemo(() => dashboards[0] ?? null, [dashboards]);

  if (!projectId) {
    return <div className="text-sm text-muted-foreground">Select a project first.</div>;
  }

  return (
    <div className="space-y-4">
      {!selectedDashboard ? (
        <button
          type="button"
          className="rounded-md border px-3 py-2 text-sm"
          onClick={() => void createDashboard(projectId, { name: "Sprint Overview", layout: [] })}
        >
          Create Dashboard
        </button>
      ) : (
        <DashboardGrid projectId={projectId} dashboard={selectedDashboard} />
      )}
    </div>
  );
}
