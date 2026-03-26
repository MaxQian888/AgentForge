"use client";

import { useEffect, useMemo } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { enrichTeamMembers } from "@/lib/dashboard/summary";
import { useDashboardStore } from "@/lib/stores/dashboard-store";
import { useMemberStore, type CreateMemberInput, type UpdateMemberInput } from "@/lib/stores/member-store";
import { useRoleStore } from "@/lib/stores/role-store";
import { TeamManagement } from "./team-management";

export function TeamPageClient() {
  const pathname = usePathname();
  const router = useRouter();
  const searchParams = useSearchParams();
  const requestedProjectId = searchParams.get("project");
  const dashboardProjects = useDashboardStore((state) => state.projects);
  const selectedProjectId = useDashboardStore((state) => state.selectedProjectId);
  const tasks = useDashboardStore((state) => state.tasks);
  const agents = useDashboardStore((state) => state.agents);
  const activity = useDashboardStore((state) => state.activity);
  const dashboardLoading = useDashboardStore((state) => state.loading);
  const fetchSummary = useDashboardStore((state) => state.fetchSummary);

  const membersByProject = useMemberStore((state) => state.membersByProject);
  const loadingByProject = useMemberStore((state) => state.loadingByProject);
  const errorByProject = useMemberStore((state) => state.errorByProject);
  const fetchMembers = useMemberStore((state) => state.fetchMembers);
  const createMember = useMemberStore((state) => state.createMember);
  const updateMember = useMemberStore((state) => state.updateMember);
  const roles = useRoleStore((state) => state.roles);
  const fetchRoles = useRoleStore((state) => state.fetchRoles);

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
    void fetchSummary({ projectId: activeProjectId ?? requestedProjectId });
  }, [activeProjectId, fetchSummary, requestedProjectId]);

  useEffect(() => {
    if (!activeProjectId) return;
    void fetchMembers(activeProjectId);
  }, [activeProjectId, fetchMembers]);

  useEffect(() => {
    void fetchRoles();
  }, [fetchRoles]);

  const projectMembers = activeProjectId ? membersByProject[activeProjectId] ?? [] : [];
  const loading = activeProjectId
    ? Boolean(loadingByProject[activeProjectId]) || (dashboardLoading && projectMembers.length === 0)
    : dashboardLoading;
  const error = activeProjectId ? errorByProject[activeProjectId] ?? null : null;
  const roster = useMemo(
    () =>
      enrichTeamMembers({
        members: projectMembers,
        tasks,
        agents,
        activity,
      }),
    [activity, agents, projectMembers, tasks]
  );

  const handleProjectChange = (projectId: string) => {
    router.replace(`${pathname}?project=${projectId}`);
  };

  const handleCreateMember = async (input: CreateMemberInput) => {
    if (!activeProjectId) return;
    await createMember(activeProjectId, input);
    await fetchMembers(activeProjectId);
    await fetchSummary({ projectId: activeProjectId });
  };

  const handleUpdateMember = async (memberId: string, input: UpdateMemberInput) => {
    if (!activeProjectId) return;
    await updateMember(memberId, activeProjectId, input);
    await fetchMembers(activeProjectId);
    await fetchSummary({ projectId: activeProjectId });
  };

  return (
    <TeamManagement
      projects={projects}
      selectedProjectId={activeProjectId}
      members={roster}
      loading={loading}
      error={error}
      availableRoles={roles}
      onRetry={() => {
        if (activeProjectId) {
          void fetchMembers(activeProjectId);
        }
        void fetchSummary({ projectId: activeProjectId });
      }}
      onProjectChange={handleProjectChange}
      onCreateMember={handleCreateMember}
      onUpdateMember={handleUpdateMember}
    />
  );
}
