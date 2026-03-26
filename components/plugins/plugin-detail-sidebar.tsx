"use client";

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { PluginDetailOverview } from "@/components/plugins/plugin-detail-overview";
import { PluginEventTimeline } from "@/components/plugins/plugin-event-timeline";
import { PluginKindDetail } from "@/components/plugins/plugin-kind-detail";
import { PluginMCPPanel } from "@/components/plugins/plugin-mcp-panel";
import { PluginWorkflowRuns } from "@/components/plugins/plugin-workflow-runs";
import type { PluginRecord } from "@/lib/stores/plugin-store";

interface PluginDetailSidebarProps {
  plugin: PluginRecord | null;
}

export function PluginDetailSection({
  title,
  children,
  action,
}: {
  title: string;
  children: React.ReactNode;
  action?: React.ReactNode;
}) {
  return (
    <div className="rounded-lg border border-border/60 p-3">
      <div className="flex items-center justify-between">
        <p className="font-medium text-sm">{title}</p>
        {action}
      </div>
      <div className="mt-2 text-sm text-muted-foreground">{children}</div>
    </div>
  );
}

function hasKindSpecificData(plugin: PluginRecord): boolean {
  if (plugin.kind === "WorkflowPlugin" && plugin.spec.workflow) return true;
  if (plugin.kind === "ReviewPlugin" && plugin.spec.review) return true;
  if (plugin.kind === "IntegrationPlugin" && plugin.spec.capabilities?.length) return true;
  if (plugin.kind === "RolePlugin") return true;
  if (plugin.kind === "ToolPlugin" && plugin.runtime_metadata?.mcp) return true;
  return false;
}

export function PluginDetailSidebar({ plugin }: PluginDetailSidebarProps) {
  if (!plugin) {
    return (
      <Card className="h-fit">
        <CardHeader>
          <CardTitle>Plugin details</CardTitle>
          <CardDescription>
            Inspect runtime, permissions, health, and source metadata for the
            selected installed plugin.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="rounded-md border border-dashed px-4 py-8 text-sm text-muted-foreground">
            Select an installed plugin to inspect operational details.
          </div>
        </CardContent>
      </Card>
    );
  }

  const showKindTab = hasKindSpecificData(plugin);

  return (
    <Card className="h-fit">
      <CardHeader>
        <CardTitle>Plugin details</CardTitle>
        <CardDescription>
          Inspect runtime, permissions, health, and source metadata for the
          selected installed plugin.
        </CardDescription>
      </CardHeader>
      <CardContent>
        <Tabs defaultValue="overview">
          <TabsList className="w-full flex-wrap">
            <TabsTrigger value="overview">Overview</TabsTrigger>
            <TabsTrigger value="events">Events</TabsTrigger>
            {showKindTab ? (
              <TabsTrigger value="kind">Kind</TabsTrigger>
            ) : null}
            {plugin.kind === "ToolPlugin" && plugin.spec.runtime === "mcp" ? (
              <TabsTrigger value="mcp">MCP</TabsTrigger>
            ) : null}
            {plugin.kind === "WorkflowPlugin" ? (
              <TabsTrigger value="workflow">Workflow</TabsTrigger>
            ) : null}
          </TabsList>

          <TabsContent value="overview" className="mt-4">
            <PluginDetailOverview plugin={plugin} />
          </TabsContent>

          <TabsContent value="events" className="mt-4">
            <PluginEventTimeline pluginId={plugin.metadata.id} />
          </TabsContent>

          {showKindTab ? (
            <TabsContent value="kind" className="mt-4">
              <PluginKindDetail plugin={plugin} />
            </TabsContent>
          ) : null}

          {plugin.kind === "ToolPlugin" && plugin.spec.runtime === "mcp" ? (
            <TabsContent value="mcp" className="mt-4">
              <PluginMCPPanel plugin={plugin} />
            </TabsContent>
          ) : null}

          {plugin.kind === "WorkflowPlugin" ? (
            <TabsContent value="workflow" className="mt-4">
              <PluginWorkflowRuns plugin={plugin} />
            </TabsContent>
          ) : null}
        </Tabs>
      </CardContent>
    </Card>
  );
}
