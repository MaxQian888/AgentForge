"use client";

import { useMemo, useState } from "react";
import { Bot, Search } from "lucide-react";
import { useTranslations } from "next-intl";
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from "@/components/ui/empty";
import { Input } from "@/components/ui/input";
import { ScrollArea } from "@/components/ui/scroll-area";
import type { Agent, AgentPoolSummary, AgentStatus } from "@/lib/stores/agent-store";
import { AgentSidebarItem } from "./agent-sidebar-item";

const STATUS_GROUP_ORDER: AgentStatus[] = [
  "running",
  "starting",
  "paused",
  "completed",
  "failed",
  "cancelled",
  "budget_exceeded",
];

const STATUS_GROUP_KEYS: Record<AgentStatus, string> = {
  running: "workspace.groupRunning",
  starting: "workspace.groupStarting",
  paused: "workspace.groupPaused",
  completed: "workspace.groupCompleted",
  failed: "workspace.groupFailed",
  cancelled: "workspace.groupCancelled",
  budget_exceeded: "workspace.groupBudgetExceeded",
};

interface AgentWorkspaceSidebarProps {
  agents: Agent[];
  pool: AgentPoolSummary | null;
  selectedAgentId: string | null;
  onSelectAgent: (id: string | null) => void;
  onPause: (id: string) => void;
  onResume: (id: string) => void;
  onKill: (id: string) => void;
  bridgeDegraded: boolean;
}

export function AgentWorkspaceSidebar({
  agents,
  pool,
  selectedAgentId,
  onSelectAgent,
  onPause,
  onResume,
  onKill,
  bridgeDegraded,
}: AgentWorkspaceSidebarProps) {
  const t = useTranslations("agents");
  const [searchQuery, setSearchQuery] = useState("");

  const filteredAgents = useMemo(() => {
    if (!searchQuery.trim()) {
      return agents;
    }

    const query = searchQuery.toLowerCase();
    return agents.filter(
      (agent) =>
        agent.taskTitle.toLowerCase().includes(query) ||
        agent.roleName.toLowerCase().includes(query) ||
        agent.runtime?.toLowerCase().includes(query),
    );
  }, [agents, searchQuery]);

  const grouped = useMemo(() => {
    const groups = new Map<AgentStatus, Agent[]>();
    for (const agent of filteredAgents) {
      const list = groups.get(agent.status) ?? [];
      list.push(agent);
      groups.set(agent.status, list);
    }
    return groups;
  }, [filteredAgents]);

  const handleSelect = (id: string) => {
    onSelectAgent(selectedAgentId === id ? null : id);
  };

  return (
    <div className="flex h-full flex-col">
      {pool && (
        <div className="shrink-0 border-b px-3 py-2">
          <p className="text-xs font-medium text-muted-foreground">
            {t("workspace.poolSummary", {
              active: pool.active,
              max: pool.max,
              queued: pool.queued ?? 0,
            })}
          </p>
        </div>
      )}

      <div className="shrink-0 px-3 py-2">
        <div className="relative">
          <Search className="absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
          <Input
            placeholder={t("workspace.searchPlaceholder")}
            value={searchQuery}
            onChange={(event) => setSearchQuery(event.target.value)}
            className="h-8 pl-8 text-sm"
          />
        </div>
      </div>

      <ScrollArea className="flex-1">
        <div className="flex flex-col gap-1 px-2 pb-4">
          {STATUS_GROUP_ORDER.map((status) => {
            const group = grouped.get(status);
            if (!group?.length) {
              return null;
            }

            return (
              <div key={status}>
                <div className="sticky top-0 z-10 bg-sidebar px-1 py-1.5">
                  <p className="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">
                    {t(STATUS_GROUP_KEYS[status])} ({group.length})
                  </p>
                </div>
                {group.map((agent) => (
                  <AgentSidebarItem
                    key={agent.id}
                    agent={agent}
                    selected={agent.id === selectedAgentId}
                    onSelect={handleSelect}
                    onPause={onPause}
                    onResume={onResume}
                    onKill={onKill}
                    bridgeDegraded={bridgeDegraded}
                  />
                ))}
              </div>
            );
          })}

          {filteredAgents.length === 0 && (
            <div className="px-2 py-6">
              <Empty className="gap-4 rounded-md border bg-background/60 p-4">
                <EmptyHeader className="gap-1.5">
                  <EmptyMedia variant="icon">
                    <Bot className="size-5" />
                  </EmptyMedia>
                  <EmptyTitle>
                    {searchQuery ? t("empty.noMatch") : t("empty.noAgents")}
                  </EmptyTitle>
                  {searchQuery ? (
                    <EmptyDescription>{t("workspace.searchPlaceholder")}</EmptyDescription>
                  ) : null}
                </EmptyHeader>
              </Empty>
            </div>
          )}
        </div>
      </ScrollArea>
    </div>
  );
}
