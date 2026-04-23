"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { useSearchParams, useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { Network, PanelLeftIcon, Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { cn } from "@/lib/utils";
import { useBreakpoint } from "@/hooks/use-breakpoint";
import type {
  Agent,
  AgentPoolSummary,
  BridgeHealthSummary,
  DispatchAttemptRecord,
  DispatchStatsSummary,
} from "@/lib/stores/agent-store";
import type { CodingAgentCatalog } from "@/lib/stores/project-store";
import type { TeamMember } from "@/lib/dashboard/summary";
import type { Task } from "@/lib/stores/task-store";
import { SpawnAgentDialog } from "@/components/tasks/spawn-agent-dialog";
import { AgentWorkspaceSidebar } from "./agent-workspace-sidebar";
import { AgentWorkspaceOverview } from "./agent-workspace-overview";
import { AgentWorkspaceDetail } from "./agent-workspace-detail";
import { buildAgentVisualizationModel } from "./agent-visualization-model";
import { AgentVisualizationCanvas } from "./agent-visualization-canvas";
import { AgentVisualizationFocusPanel } from "./agent-visualization-focus-panel";

type AgentWorkspaceView = "monitor" | "visualization" | "dispatch";

function parseAgentWorkspaceView(view: string | null): AgentWorkspaceView {
  if (view === "visualization" || view === "dispatch") {
    return view;
  }

  return "monitor";
}

interface AgentWorkspaceProps {
  agents: Agent[];
  pool: AgentPoolSummary | null;
  runtimeCatalog: CodingAgentCatalog | null;
  bridgeHealth: BridgeHealthSummary | null;
  dispatchStats: DispatchStatsSummary | null;
  loading: boolean;
  requestedMemberId: string | null;
  dispatchHistoryByTask: Record<string, DispatchAttemptRecord[]>;
  fetchDispatchHistory: (taskId: string) => Promise<DispatchAttemptRecord[]>;
  fetchAgent?: (id: string) => Promise<Agent | null>;
  selectedProjectId?: string | null;
  tasks?: Task[];
  members?: TeamMember[];
  onSpawnAgent?: (
    taskId: string,
    memberId: string,
    options?: {
      runtime?: string;
      provider?: string;
      model?: string;
      maxBudgetUsd?: number;
      roleId?: string;
    },
  ) => void | Promise<void>;
  onPause: (id: string) => void;
  onResume: (id: string) => void;
  onKill: (id: string) => void;
}

export function AgentWorkspace({
  agents,
  pool,
  runtimeCatalog,
  bridgeHealth,
  dispatchStats,
  loading,
  requestedMemberId,
  dispatchHistoryByTask,
  fetchDispatchHistory,
  fetchAgent,
  selectedProjectId,
  tasks = [],
  members = [],
  onSpawnAgent,
  onPause,
  onResume,
  onKill,
}: AgentWorkspaceProps) {
  const t = useTranslations("agents");
  const { isDesktop, isMobile } = useBreakpoint();
  const router = useRouter();
  const searchParams = useSearchParams();

  const [sidebarOpen, setSidebarOpen] = useState(true);
  const [sidebarSheetOpen, setSidebarSheetOpen] = useState(false);
  const [spawnDialogOpen, setSpawnDialogOpen] = useState(false);
  const [visualizationHistoryTaskId, setVisualizationHistoryTaskId] = useState<
    string | null
  >(null);

  const activeView = parseAgentWorkspaceView(searchParams.get("view"));
  const selectedAgentId = searchParams.get("agent");
  const visualizationFocusId = searchParams.get("vizNode");
  const visibleAgents = useMemo(
    () =>
      requestedMemberId
        ? agents.filter((agent) => agent.memberId === requestedMemberId)
        : agents,
    [agents, requestedMemberId],
  );
  const bridgeDegraded = bridgeHealth?.status === "degraded";
  const spawnTaskOptions = useMemo(
    () =>
      tasks
        .filter(
          (task) =>
            task.projectId === selectedProjectId &&
            task.status !== "done" &&
            task.status !== "cancelled",
        )
        .map((task) => ({
          id: task.id,
          title: task.title,
        })),
    [selectedProjectId, tasks],
  );
  const spawnMemberOptions = useMemo(
    () =>
      members.map((member) => ({
        id: member.id,
        label: `${member.name} (${member.typeLabel})`,
      })),
    [members],
  );
  const canOpenSpawnDialog =
    Boolean(selectedProjectId) &&
    Boolean(onSpawnAgent) &&
    spawnTaskOptions.length > 0 &&
    spawnMemberOptions.length > 0;
  const visualizationModel = useMemo(
    () =>
      buildAgentVisualizationModel({
        agents,
        pool,
        runtimeCatalog,
        bridgeHealth,
        requestedMemberId,
      }),
    [agents, bridgeHealth, pool, requestedMemberId, runtimeCatalog],
  );
  const visualizationFocus = useMemo(() => {
    if (selectedAgentId || activeView !== "visualization" || !visualizationFocusId) {
      return null;
    }

    return visualizationModel.focusByNodeId[visualizationFocusId] ?? null;
  }, [
    activeView,
    selectedAgentId,
    visualizationFocusId,
    visualizationModel,
  ]);
  const visualizationFocusTaskId =
    visualizationFocus && "taskId" in visualizationFocus
      ? visualizationFocus.taskId
      : null;
  const visualizationDispatchHistory = visualizationFocusTaskId
    ? dispatchHistoryByTask[visualizationFocusTaskId] ?? []
    : [];

  const [prevIsDesktop, setPrevIsDesktop] = useState<boolean | symbol>(Symbol("init"));
  if (prevIsDesktop !== isDesktop) {
    setPrevIsDesktop(isDesktop);
    setSidebarOpen(isDesktop);
  }

  const [prevFocusId, setPrevFocusId] = useState<string | null | symbol>(Symbol("init"));
  if (prevFocusId !== visualizationFocusTaskId) {
    setPrevFocusId(visualizationFocusTaskId);
    if (!visualizationFocusTaskId || dispatchHistoryByTask[visualizationFocusTaskId] !== undefined) {
      setVisualizationHistoryTaskId(null);
    } else {
      setVisualizationHistoryTaskId(visualizationFocusTaskId);
    }
  }

  useEffect(() => {
    if (!visualizationFocusTaskId) return;
    if (dispatchHistoryByTask[visualizationFocusTaskId] !== undefined) return;

    let cancelled = false;
    void fetchDispatchHistory(visualizationFocusTaskId).finally(() => {
      if (!cancelled) {
        setVisualizationHistoryTaskId((current) =>
          current === visualizationFocusTaskId ? null : current,
        );
      }
    });

    return () => {
      cancelled = true;
    };
  }, [
    dispatchHistoryByTask,
    fetchDispatchHistory,
    visualizationFocusTaskId,
  ]);

  useEffect(() => {
    if (!fetchAgent || activeView !== "monitor") {
      return;
    }

    const runningAgentIds = visibleAgents
      .filter((agent) => agent.status === "running")
      .map((agent) => agent.id);

    if (runningAgentIds.length === 0) {
      return;
    }

    const intervalId = window.setInterval(() => {
      runningAgentIds.forEach((id) => {
        void fetchAgent(id);
      });
    }, 5000);

    return () => {
      window.clearInterval(intervalId);
    };
  }, [activeView, fetchAgent, visibleAgents]);

  const buildWorkspaceHref = (
    mutate: (params: URLSearchParams) => void,
  ) => {
    const params = new URLSearchParams(searchParams.toString());
    mutate(params);
    const query = params.toString();

    return query ? `/agents?${query}` : "/agents?";
  };

  const setSelectedAgentId = (id: string | null) => {
    const href = buildWorkspaceHref((params) => {
      if (id) {
        params.set("agent", id);
        params.delete("vizNode");
      } else {
        params.delete("agent");
      }
    });

    router.replace(href, { scroll: false });
    if (isMobile) {
      setSidebarSheetOpen(false);
    }
  };

  const setActiveView = (view: AgentWorkspaceView) => {
    const href = buildWorkspaceHref((params) => {
      params.set("view", view);
      if (view !== "visualization") {
        params.delete("vizNode");
      }
    });

    router.replace(href, { scroll: false });
  };

  const setVisualizationFocusId = (id: string | null) => {
    const href = buildWorkspaceHref((params) => {
      params.set("view", "visualization");
      params.delete("agent");
      if (id) {
        params.set("vizNode", id);
      } else {
        params.delete("vizNode");
      }
    });

    router.replace(href, { scroll: false });
  };

  const toggleSidebar = () => {
    if (isMobile) {
      setSidebarSheetOpen((open) => !open);
      return;
    }

    setSidebarOpen((open) => !open);
  };

  const sidebarContent = (
    <AgentWorkspaceSidebar
      agents={visibleAgents}
      pool={pool}
      selectedAgentId={selectedAgentId}
      onSelectAgent={setSelectedAgentId}
      onPause={onPause}
      onResume={onResume}
      onKill={onKill}
      bridgeDegraded={bridgeDegraded}
    />
  );

  return (
    <TooltipProvider>
      <div className="flex h-[calc(100vh-3.5rem)]">
        {isMobile ? (
          <Sheet open={sidebarSheetOpen} onOpenChange={setSidebarSheetOpen}>
            <SheetContent
              side="left"
              className="w-72 p-0"
              showCloseButton={false}
            >
              <SheetHeader className="sr-only">
                <SheetTitle>{t("monitor.title")}</SheetTitle>
                <SheetDescription>{t("workspace.searchPlaceholder")}</SheetDescription>
              </SheetHeader>
              {sidebarContent}
            </SheetContent>
          </Sheet>
        ) : (
          <div
            className={cn(
              "shrink-0 overflow-hidden border-r bg-sidebar transition-[width] duration-200 ease-linear",
              sidebarOpen ? "w-[280px]" : "w-0",
            )}
          >
            <div className="h-full w-[280px] overflow-y-auto">
              {sidebarContent}
            </div>
          </div>
        )}

        <Tabs
          value={activeView}
          onValueChange={(value) =>
            setActiveView(value as AgentWorkspaceView)
          }
          className="flex min-w-0 flex-1 flex-col gap-0 overflow-hidden"
        >
          <div className="flex h-10 shrink-0 items-center gap-1 border-b bg-background px-2">
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  variant="ghost"
                  size="icon"
                  className="size-7"
                  onClick={toggleSidebar}
                  aria-label={t("workspace.toggleSidebar")}
                >
                  <PanelLeftIcon className="size-4" />
                </Button>
              </TooltipTrigger>
              <TooltipContent side="bottom">
                {t("workspace.toggleSidebar")}
              </TooltipContent>
            </Tooltip>

            <TabsList variant="line" className="ml-2 h-8 gap-1 p-0">
              <TabsTrigger
                value="monitor"
                aria-label={t("monitor.title")}
                className="h-7 px-3 text-xs"
              >
                {t("monitor.title")}
              </TabsTrigger>
              <TabsTrigger
                value="visualization"
                aria-label={t("visualization.title")}
                className="h-7 px-3 text-xs"
              >
                {t("visualization.title")}
              </TabsTrigger>
              <TabsTrigger
                value="dispatch"
                aria-label={t("stats.dispatch")}
                className="h-7 px-3 text-xs"
              >
                {t("stats.dispatch")}
                {dispatchStats ? (
                  <span className="rounded-full bg-muted px-1.5 py-0.5 text-[10px] font-medium text-muted-foreground">
                    {Object.values(dispatchStats.outcomes).reduce(
                      (count, value) => count + value,
                      0,
                    )}
                  </span>
                ) : null}
              </TabsTrigger>
            </TabsList>

            <div className="flex-1" />

            {onSpawnAgent ? (
              <Button
                type="button"
                variant="outline"
                size="sm"
                disabled={!canOpenSpawnDialog}
                onClick={() => setSpawnDialogOpen(true)}
              >
                <Plus className="mr-1 size-3.5" />
                {t("workspace.spawnAgent")}
              </Button>
            ) : null}

            <Link
              href="/teams"
              className="inline-flex items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground"
            >
              <Network className="size-3.5" />
              {t("monitor.teamsLink")}
            </Link>
          </div>

          <div className="flex-1 overflow-y-auto">
            {loading && !agents.length ? (
              <div className="flex items-center justify-center py-20">
                <p className="text-muted-foreground">{t("monitor.loading")}</p>
              </div>
            ) : (
              <>
                <TabsContent value="monitor" className="mt-0">
                  <AgentWorkspaceOverview
                    activeTab="monitor"
                    agents={visibleAgents}
                    pool={pool}
                    runtimeCatalog={runtimeCatalog}
                    bridgeHealth={bridgeHealth}
                    dispatchStats={dispatchStats}
                    selectedAgentId={selectedAgentId}
                    onSelectAgent={setSelectedAgentId}
                    onPause={onPause}
                    onResume={onResume}
                    onKill={onKill}
                  />
                </TabsContent>
                <TabsContent value="dispatch" className="mt-0">
                  <AgentWorkspaceOverview
                    activeTab="dispatch"
                    agents={visibleAgents}
                    pool={pool}
                    runtimeCatalog={runtimeCatalog}
                    bridgeHealth={bridgeHealth}
                    dispatchStats={dispatchStats}
                    selectedAgentId={selectedAgentId}
                    onSelectAgent={setSelectedAgentId}
                    onPause={onPause}
                    onResume={onResume}
                    onKill={onKill}
                  />
                </TabsContent>
                <TabsContent value="visualization" className="mt-0">
                  <div className="flex flex-col gap-4 xl:flex-row xl:items-start">
                    <div className="min-w-0 flex-1">
                      <AgentVisualizationCanvas
                        model={visualizationModel}
                        loading={loading}
                        requestedMemberId={requestedMemberId}
                        selectedAgentId={selectedAgentId}
                        selectedVisualizationNodeId={visualizationFocus?.nodeId ?? null}
                        onSelectAgent={setSelectedAgentId}
                        onSelectVisualizationNode={setVisualizationFocusId}
                      />
                    </div>
                    {visualizationFocus ? (
                      <div className="px-6 pb-6 xl:w-[360px] xl:px-0 xl:pr-6 xl:pt-6">
                        <AgentVisualizationFocusPanel
                          focus={visualizationFocus}
                          dispatchHistory={visualizationDispatchHistory}
                          dispatchHistoryLoading={
                            visualizationFocusTaskId != null &&
                            visualizationHistoryTaskId === visualizationFocusTaskId
                          }
                          onClearFocus={() => setVisualizationFocusId(null)}
                        />
                      </div>
                    ) : null}
                  </div>
                </TabsContent>
              </>
            )}
          </div>
        </Tabs>
        <Sheet
          open={Boolean(selectedAgentId)}
          onOpenChange={(open) => {
            if (!open) {
              setSelectedAgentId(null);
            }
          }}
        >
          <SheetContent
            side="right"
            className="w-full max-w-none overflow-y-auto p-0 sm:max-w-2xl"
            showCloseButton={false}
          >
            <SheetHeader className="sr-only">
              <SheetTitle>{t("monitor.title")}</SheetTitle>
            </SheetHeader>
            {selectedAgentId ? (
              <AgentWorkspaceDetail
                agentId={selectedAgentId}
                onBack={() => setSelectedAgentId(null)}
              />
            ) : null}
          </SheetContent>
        </Sheet>
        {onSpawnAgent ? (
          <SpawnAgentDialog
            open={spawnDialogOpen}
            onOpenChange={setSpawnDialogOpen}
            taskOptions={spawnTaskOptions}
            memberOptions={spawnMemberOptions}
            defaultMemberId={requestedMemberId ?? undefined}
            onSpawnAgent={onSpawnAgent}
          />
        ) : null}
      </div>
    </TooltipProvider>
  );
}
