"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { useSearchParams, useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { Network, PanelLeftIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Sheet,
  SheetContent,
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
  DispatchStatsSummary,
} from "@/lib/stores/agent-store";
import type { CodingAgentCatalog } from "@/lib/stores/project-store";
import { AgentWorkspaceSidebar } from "./agent-workspace-sidebar";
import { AgentWorkspaceOverview } from "./agent-workspace-overview";
import { AgentWorkspaceDetail } from "./agent-workspace-detail";

interface AgentWorkspaceProps {
  agents: Agent[];
  pool: AgentPoolSummary | null;
  runtimeCatalog: CodingAgentCatalog | null;
  bridgeHealth: BridgeHealthSummary | null;
  dispatchStats: DispatchStatsSummary | null;
  loading: boolean;
  requestedMemberId: string | null;
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
  const [activeTab, setActiveTab] = useState<"monitor" | "dispatch">(
    "monitor",
  );

  const selectedAgentId = searchParams.get("agent");
  const visibleAgents = useMemo(
    () =>
      requestedMemberId
        ? agents.filter((agent) => agent.memberId === requestedMemberId)
        : agents,
    [agents, requestedMemberId],
  );
  const bridgeDegraded = bridgeHealth?.status === "degraded";

  useEffect(() => {
    setSidebarOpen(isDesktop);
  }, [isDesktop]);

  const setSelectedAgentId = (id: string | null) => {
    const params = new URLSearchParams(searchParams.toString());
    if (id) {
      params.set("agent", id);
    } else {
      params.delete("agent");
    }

    router.replace(`/agents?${params.toString()}`, { scroll: false });
    if (isMobile) {
      setSidebarSheetOpen(false);
    }
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
          value={activeTab}
          onValueChange={(value) => setActiveTab(value as "monitor" | "dispatch")}
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
            ) : selectedAgentId ? (
              <AgentWorkspaceDetail
                agentId={selectedAgentId}
                onBack={() => setSelectedAgentId(null)}
              />
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
                  />
                </TabsContent>
              </>
            )}
          </div>
        </Tabs>
      </div>
    </TooltipProvider>
  );
}
