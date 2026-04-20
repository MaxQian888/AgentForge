"use client";

import { Suspense } from "react";
import { useEffect } from "react";
import { useSearchParams } from "next/navigation";
import { useAgentStore } from "@/lib/stores/agent-store";
import { useDashboardStore } from "@/lib/stores/dashboard-store";
import { useMemberStore } from "@/lib/stores/member-store";
import { useTaskStore } from "@/lib/stores/task-store";
import { AgentWorkspace } from "@/components/agents/agent-workspace";
import { EmployeesSection } from "@/components/employees/employees-section";
import { useBreadcrumbs } from "@/hooks/use-breadcrumbs";

function AgentsPageInner() {
  useBreadcrumbs([{ label: "Project", href: "/" }, { label: "Agents" }]);
  const searchParams = useSearchParams();
  const selectedProjectId = useDashboardStore((s) => s.selectedProjectId);
  const tasks = useTaskStore((state) => state.tasks);
  const fetchTasks = useTaskStore((state) => state.fetchTasks);
  const membersByProject = useMemberStore((state) => state.membersByProject);
  const fetchMembers = useMemberStore((state) => state.fetchMembers);
  const {
    agents,
    pool,
    runtimeCatalog,
    bridgeHealth,
    dispatchStats,
    dispatchHistoryByTask,
    loading,
    fetchAgents,
    fetchAgent,
    fetchPool,
    fetchRuntimeCatalog,
    fetchBridgeHealth,
    fetchDispatchHistory,
    fetchDispatchStats,
    spawnAgent,
    pauseAgent,
    resumeAgent,
    killAgent,
  } = useAgentStore();

  const requestedMemberId = searchParams.get("member");
  const projectTasks = selectedProjectId
    ? tasks.filter((task) => task.projectId === selectedProjectId)
    : [];
  const projectMembers = selectedProjectId
    ? membersByProject[selectedProjectId] ?? []
    : [];

  useEffect(() => {
    fetchAgents();
    fetchPool();
    void fetchRuntimeCatalog();
    void fetchBridgeHealth();
    if (selectedProjectId) {
      void fetchDispatchStats(selectedProjectId);
      void fetchTasks(selectedProjectId);
      void fetchMembers(selectedProjectId);
    }
  }, [
    fetchAgents,
    fetchBridgeHealth,
    fetchDispatchStats,
    fetchMembers,
    fetchPool,
    fetchRuntimeCatalog,
    fetchTasks,
    selectedProjectId,
  ]);

  return (
    <div className="flex flex-col gap-[var(--space-section-gap)]">
      <EmployeesSection projectId={selectedProjectId} />
      <AgentWorkspace
        agents={agents}
        pool={pool}
        runtimeCatalog={runtimeCatalog}
        bridgeHealth={bridgeHealth}
        dispatchStats={dispatchStats}
        loading={loading}
        requestedMemberId={requestedMemberId}
        dispatchHistoryByTask={dispatchHistoryByTask}
        fetchDispatchHistory={fetchDispatchHistory}
        fetchAgent={fetchAgent}
        selectedProjectId={selectedProjectId}
        tasks={projectTasks}
        members={projectMembers}
        onSpawnAgent={(taskId, memberId, options) =>
          void spawnAgent(taskId, memberId, options)
        }
        onPause={(id) => void pauseAgent(id)}
        onResume={(id) => void resumeAgent(id)}
        onKill={(id) => void killAgent(id)}
      />
    </div>
  );
}

export default function AgentsPage() {
  return (
    <Suspense>
      <AgentsPageInner />
    </Suspense>
  );
}
