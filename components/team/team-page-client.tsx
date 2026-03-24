"use client";

import { useEffect, useMemo } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { summarizeMemberRoster } from "@/lib/dashboard/summary";
import { useDashboardStore } from "@/lib/stores/dashboard-store";
import { useMemberStore, type CreateMemberInput, type UpdateMemberInput } from "@/lib/stores/member-store";
import { TeamManagement } from "./team-management";

export function TeamPageClient() {
  const pathname = usePathname();
  const router = useRouter();
  const searchParams = useSearchParams();
  const requestedProjectId = searchParams.get("project");

  const dashboardProjects = useDashboardStore((state) => state.projects);
  const selectedProjectId = useDashboardStore((state) => state.selectedProjectId);
  const members = useDashboardStore((state) => state.members);
  const tasks = useDashboardStore((state) => state.tasks);
  const agents = useDashboardStore((state) => state.agents);
  const activity = useDashboardStore((state) => state.activity);
  const loading = useDashboardStore((state) => state.loading);
  const error = useDashboardStore((state) => state.error ?? state.sectionErrors.team ?? null);
  const fetchSummary = useDashboardStore((state) => state.fetchSummary);

  const createMember = useMemberStore((state) => state.createMember);
  const updateMember = useMemberStore((state) => state.updateMember);

  const activeProjectId = requestedProjectId ?? selectedProjectId;
  const projects = useMemo(
    () =>
      dashboardProjects.map((project) => ({
        id: project.id,
        name: project.name,
      })),
    [dashboardProjects]
  );

  useEffect(() => {
    void fetchSummary({ projectId: requestedProjectId });
  }, [fetchSummary, requestedProjectId]);

  const roster = useMemo(
    () =>
      summarizeMemberRoster({
        members,
        tasks,
        agents,
        activity,
      }),
    [activity, agents, members, tasks]
  );

  const handleProjectChange = (projectId: string) => {
    router.replace(`${pathname}?project=${projectId}`);
  };

  const handleCreateMember = async (input: CreateMemberInput) => {
    if (!activeProjectId) return;
    await createMember(activeProjectId, input);
    await fetchSummary({ projectId: activeProjectId });
  };

  const handleUpdateMember = async (memberId: string, input: UpdateMemberInput) => {
    if (!activeProjectId) return;
    await updateMember(memberId, activeProjectId, input);
    await fetchSummary({ projectId: activeProjectId });
  };

  return (
    <TeamManagement
      projects={projects}
      selectedProjectId={activeProjectId}
      members={roster}
      loading={loading}
      error={error}
      onRetry={() => {
        void fetchSummary({ projectId: activeProjectId });
      }}
      onProjectChange={handleProjectChange}
      onCreateMember={handleCreateMember}
      onUpdateMember={handleUpdateMember}
    />
  );
}
