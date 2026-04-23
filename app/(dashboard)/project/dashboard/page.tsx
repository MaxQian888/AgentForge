"use client";

import { Suspense, useEffect, useMemo, useState } from "react";
import { useTranslations } from "next-intl";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { useDashboardStore, type DashboardConfig } from "@/lib/stores/dashboard-store";
import { DashboardGrid } from "@/components/dashboard/dashboard-grid";
import { useBreadcrumbs } from "@/hooks/use-breadcrumbs";
import { PageHeader } from "@/components/shared/page-header";
import { EmptyState } from "@/components/shared/empty-state";
import { ErrorBanner } from "@/components/shared/error-banner";
import { SectionCard } from "@/components/shared/section-card";
import { FolderOpen, LayoutDashboard } from "lucide-react";

const EMPTY_DASHBOARDS: DashboardConfig[] = [];

function ProjectDashboardView() {
  const tc = useTranslations("common");
  useBreadcrumbs([{ label: tc("nav.projects"), href: "/projects" }, { label: tc("nav.projectDashboard") }]);
  const t = useTranslations("dashboard");
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const requestedProjectId = searchParams.get("project");
  const selectedProjectId = useDashboardStore((state) => state.selectedProjectId);
  const projectId = requestedProjectId ?? selectedProjectId;
  const activeDashboardIdByProject = useDashboardStore(
    (state) => state.activeDashboardIdByProject
  );
  const dashboardsLoadingByProject = useDashboardStore(
    (state) => state.dashboardsLoadingByProject
  );
  const dashboardsErrorByProject = useDashboardStore(
    (state) => state.dashboardsErrorByProject
  );
  const dashboardsByProject = useDashboardStore((state) => state.dashboardsByProject);
  const fetchDashboards = useDashboardStore((state) => state.fetchDashboards);
  const setActiveDashboard = useDashboardStore((state) => state.setActiveDashboard);
  const createDashboard = useDashboardStore((state) => state.createDashboard);
  const updateDashboard = useDashboardStore((state) => state.updateDashboard);
  const deleteDashboard = useDashboardStore((state) => state.deleteDashboard);
  const [isEditingName, setIsEditingName] = useState(false);
  const [draftName, setDraftName] = useState("");

  const dashboards = useMemo(
    () => (projectId ? dashboardsByProject[projectId] ?? EMPTY_DASHBOARDS : EMPTY_DASHBOARDS),
    [dashboardsByProject, projectId]
  );
  const routeDashboardId = searchParams.get("dashboard");
  const activeDashboardId = projectId
    ? activeDashboardIdByProject[projectId] ?? null
    : null;
  const dashboardsLoading = projectId
    ? dashboardsLoadingByProject[projectId] ?? false
    : false;
  const dashboardsError = projectId
    ? dashboardsErrorByProject[projectId] ?? null
    : null;

  useEffect(() => {
    if (projectId) {
      void fetchDashboards(projectId);
    }
  }, [fetchDashboards, projectId]);

  const selectedDashboard = useMemo(() => {
    if (routeDashboardId) {
      const matched = dashboards.find((dashboard) => dashboard.id === routeDashboardId);
      if (matched) {
        return matched;
      }
    }

    if (activeDashboardId) {
      const matched = dashboards.find((dashboard) => dashboard.id === activeDashboardId);
      if (matched) {
        return matched;
      }
    }

    return dashboards[0] ?? null;
  }, [activeDashboardId, dashboards, routeDashboardId]);

  useEffect(() => {
    if (!projectId || !selectedDashboard) {
      return;
    }

    if (activeDashboardId !== selectedDashboard.id) {
      setActiveDashboard(projectId, selectedDashboard.id);
    }

    if (routeDashboardId === selectedDashboard.id) {
      return;
    }

    const nextParams = new URLSearchParams(searchParams.toString());
    nextParams.set("dashboard", selectedDashboard.id);
    router.replace(`${pathname}?${nextParams.toString()}`);
  }, [
    activeDashboardId,
    pathname,
    projectId,
    routeDashboardId,
    router,
    searchParams,
    selectedDashboard,
    setActiveDashboard,
  ]);

  if (!projectId) {
    return (
      <EmptyState
        icon={FolderOpen}
        title={t("projectDashboard.selectProject")}
      />
    );
  }

  if (dashboardsLoading && dashboards.length === 0) {
    return (
      <div className="text-sm text-muted-foreground">
        {t("projectDashboard.loading")}
      </div>
    );
  }

  if (dashboardsError && dashboards.length === 0) {
    return (
      <ErrorBanner
        message={`${t("projectDashboard.error")}: ${dashboardsError}`}
        onRetry={() => void fetchDashboards(projectId)}
      />
    );
  }

  return (
    <div className="flex flex-col gap-[var(--space-section-gap)]">
      <PageHeader title={t("projectDashboard.title")} />
      {!selectedDashboard ? (
        <EmptyState
          icon={LayoutDashboard}
          title={t("projectDashboard.createDashboard")}
          action={{
            label: t("projectDashboard.createDashboard"),
            onClick: () => void createDashboard(projectId, { name: t("projectDashboard.sprintOverview"), layout: [] }),
          }}
        />
      ) : (
        <>
          <SectionCard
            bodyClassName="flex flex-col gap-[var(--space-stack-sm)] md:flex-row md:items-end md:justify-between"
          >
            <div className="flex flex-col gap-[var(--space-stack-sm)] md:flex-row md:items-end">
              <div className="flex min-w-[220px] flex-col gap-[var(--space-stack-xs)]">
                <label
                  htmlFor="project-dashboard-selector"
                  className="text-sm font-medium"
                >
                  {t("projectDashboard.selectorLabel")}
                </label>
                <select
                  id="project-dashboard-selector"
                  className="h-9 rounded-md border border-input bg-background px-3 text-sm"
                  value={selectedDashboard.id}
                  onChange={(event) =>
                    setActiveDashboard(projectId, event.target.value)
                  }
                >
                  {dashboards.map((dashboard) => (
                    <option key={dashboard.id} value={dashboard.id}>
                      {dashboard.name}
                    </option>
                  ))}
                </select>
              </div>
              {isEditingName ? (
                <div className="flex flex-col gap-[var(--space-stack-xs)]">
                  <label
                    htmlFor="project-dashboard-name"
                    className="text-sm font-medium"
                  >
                    {t("projectDashboard.nameLabel")}
                  </label>
                  <Input
                    id="project-dashboard-name"
                    value={draftName}
                    onChange={(event) => setDraftName(event.target.value)}
                  />
                </div>
              ) : null}
            </div>
            <div className="flex flex-wrap gap-[var(--space-stack-sm)]">
              {isEditingName ? (
                <>
                  <Button
                    type="button"
                    size="sm"
                    onClick={async () => {
                      const trimmedName = draftName.trim();
                      if (!trimmedName) {
                        return;
                      }
                      await updateDashboard(projectId, selectedDashboard.id, {
                        name: trimmedName,
                      });
                      setIsEditingName(false);
                    }}
                  >
                    {t("projectDashboard.saveName")}
                  </Button>
                  <Button
                    type="button"
                    size="sm"
                    variant="outline"
                    onClick={() => {
                      setDraftName(selectedDashboard.name);
                      setIsEditingName(false);
                    }}
                  >
                    {t("projectDashboard.cancelEdit")}
                  </Button>
                </>
              ) : (
                <Button
                  type="button"
                  size="sm"
                  variant="outline"
                  onClick={() => {
                    setDraftName(selectedDashboard.name);
                    setIsEditingName(true);
                  }}
                >
                  {t("projectDashboard.rename")}
                </Button>
              )}
              <Button
                type="button"
                size="sm"
                variant="outline"
                onClick={async () => {
                  await deleteDashboard(projectId, selectedDashboard.id);
                }}
              >
                {t("projectDashboard.delete")}
              </Button>
            </div>
          </SectionCard>
          <DashboardGrid projectId={projectId} dashboard={selectedDashboard} />
        </>
      )}
    </div>
  );
}

export default function ProjectDashboardPage() {
  return (
    <Suspense fallback={null}>
      <ProjectDashboardView />
    </Suspense>
  );
}
