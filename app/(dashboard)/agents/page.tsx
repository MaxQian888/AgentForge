"use client";

import { Suspense } from "react";
import { useEffect } from "react";
import { useSearchParams } from "next/navigation";
import { useAgentStore } from "@/lib/stores/agent-store";
import { useDashboardStore } from "@/lib/stores/dashboard-store";
import { AgentWorkspace } from "@/components/agents/agent-workspace";
import { useBreadcrumbs } from "@/hooks/use-breadcrumbs";

function AgentsPageInner() {
  useBreadcrumbs([{ label: "Project", href: "/" }, { label: "Agents" }]);
  const searchParams = useSearchParams();
  const selectedProjectId = useDashboardStore((s) => s.selectedProjectId);
  const {
    agents,
    pool,
    runtimeCatalog,
    bridgeHealth,
    dispatchStats,
    loading,
    fetchAgents,
    fetchPool,
    fetchRuntimeCatalog,
    fetchBridgeHealth,
    fetchDispatchStats,
    pauseAgent,
    resumeAgent,
    killAgent,
  } = useAgentStore();

  const requestedMemberId = searchParams.get("member");

  useEffect(() => {
    fetchAgents();
    fetchPool();
    void fetchRuntimeCatalog();
    void fetchBridgeHealth();
    if (selectedProjectId) {
      void fetchDispatchStats(selectedProjectId);
    }
  }, [
    fetchAgents,
    fetchBridgeHealth,
    fetchDispatchStats,
    fetchPool,
    fetchRuntimeCatalog,
    selectedProjectId,
  ]);

  return (
    <AgentWorkspace
      agents={agents}
      pool={pool}
      runtimeCatalog={runtimeCatalog}
      bridgeHealth={bridgeHealth}
      dispatchStats={dispatchStats}
      loading={loading}
      requestedMemberId={requestedMemberId}
      onPause={(id) => void pauseAgent(id)}
      onResume={(id) => void resumeAgent(id)}
      onKill={(id) => void killAgent(id)}
    />
  );
}

export default function AgentsPage() {
  return (
    <Suspense>
      <AgentsPageInner />
    </Suspense>
  );
}
