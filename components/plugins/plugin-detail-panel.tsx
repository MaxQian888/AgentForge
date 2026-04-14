"use client";

import { useCallback } from "react";
import { useTranslations } from "next-intl";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Separator } from "@/components/ui/separator";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { cn } from "@/lib/utils";
import { PluginDetailSection } from "./plugin-detail-section";
import { PluginEventTimeline } from "./plugin-event-timeline";
import { PluginIcon } from "./plugin-icon";
import { PluginKindDetail } from "./plugin-kind-detail";
import { PluginMCPPanel } from "./plugin-mcp-panel";
import { PluginTrustBadge } from "./plugin-trust-badge";
import { PluginWorkflowRuns } from "./plugin-workflow-runs";
import type {
  MarketplacePluginEntry,
  PluginKind,
  PluginRecord,
  PluginLifecycleState,
} from "@/lib/stores/plugin-store";
import { usePluginStore } from "@/lib/stores/plugin-store";
import {
  ArrowUpCircle,
  Download,
  HeartPulse,
  MoreHorizontal,
  Pause,
  Play,
  Puzzle,
  RotateCcw,
  Settings,
  Square,
  Terminal,
  Trash2,
  Zap,
} from "lucide-react";

const stateColors: Record<PluginLifecycleState, string> = {
  installed: "bg-blue-500/15 text-blue-700 dark:text-blue-400",
  enabled: "bg-green-500/15 text-green-700 dark:text-green-400",
  activating: "bg-cyan-500/15 text-cyan-700 dark:text-cyan-400",
  active: "bg-emerald-500/15 text-emerald-700 dark:text-emerald-400",
  degraded: "bg-orange-500/15 text-orange-700 dark:text-orange-400",
  disabled: "bg-zinc-500/15 text-zinc-600 dark:text-zinc-400",
};

function renderPermissions(plugin: PluginRecord): string {
  const permissions: string[] = [];
  if (plugin.permissions.network?.required) {
    permissions.push(
      `Network${
        plugin.permissions.network.domains?.length
          ? ` (${plugin.permissions.network.domains.join(", ")})`
          : ""
      }`,
    );
  }
  if (plugin.permissions.filesystem?.required) {
    permissions.push(
      `Filesystem${
        plugin.permissions.filesystem.allowed_paths?.length
          ? ` (${plugin.permissions.filesystem.allowed_paths.join(", ")})`
          : ""
      }`,
    );
  }
  return permissions.length > 0 ? permissions.join(" · ") : "None";
}

function hasKindSpecificData(plugin: PluginRecord): boolean {
  if (plugin.kind === "WorkflowPlugin" && plugin.spec.workflow) return true;
  if (plugin.kind === "ReviewPlugin" && plugin.spec.review) return true;
  if (plugin.kind === "IntegrationPlugin" && plugin.spec.capabilities?.length)
    return true;
  if (plugin.kind === "RolePlugin") return true;
  if (plugin.kind === "ToolPlugin" && plugin.runtime_metadata?.mcp) return true;
  return false;
}

/* ── Marketplace entry detail (lighter view) ── */

function MarketplaceEntryDetail({
  entry,
  onInstall,
  loading,
}: {
  entry: MarketplacePluginEntry;
  onInstall?: (entry: MarketplacePluginEntry) => void;
  loading?: boolean;
}) {
  const t = useTranslations("plugins");
  const kind = (entry.kind ?? "ToolPlugin") as PluginKind;

  return (
    <ScrollArea className="h-full">
      <div className="p-4">
        {/* Header */}
        <div className="flex items-start gap-4">
          <PluginIcon name={entry.name} kind={kind} size="lg" />
          <div className="min-w-0 flex-1">
            <h2 className="text-xl font-bold leading-tight">{entry.name}</h2>
            <div className="mt-1 flex flex-wrap items-center gap-2 text-sm text-muted-foreground">
              <span>v{entry.version}</span>
              {entry.author ? (
                <>
                  <span className="text-border">·</span>
                  <span>{entry.author}</span>
                </>
              ) : null}
              <span className="text-border">·</span>
              <Badge variant="outline" className="text-[11px] px-1.5 py-0">
                {kind.replace("Plugin", "")}
              </Badge>
            </div>
          </div>
        </div>

        {/* Install action */}
        <div className="mt-4 flex items-center gap-2">
          {entry.installed ? (
            <Badge className="bg-emerald-500/15 text-emerald-700 dark:text-emerald-400">
              {t("installed")}
            </Badge>
          ) : entry.installable === false ? (
            <>
              <Badge variant="secondary">{t("browseOnly")}</Badge>
              {entry.blockedReason ? (
                <span className="text-xs text-muted-foreground">
                  {entry.blockedReason}
                </span>
              ) : null}
            </>
          ) : (
            <Button
              size="sm"
              onClick={() => onInstall?.(entry)}
              disabled={loading}
            >
              <Download className="mr-1.5 size-3.5" />
              {t("install")}
            </Button>
          )}
        </div>

        {/* Description */}
        {entry.description ? (
          <p className="mt-4 text-sm leading-relaxed text-muted-foreground">
            {entry.description}
          </p>
        ) : null}

        <Separator className="my-4" />

        {/* Info grid */}
        <div className="grid gap-3 sm:grid-cols-2">
          <PluginDetailSection title="Kind">
            {kind}
          </PluginDetailSection>

          {entry.runtime ? (
            <PluginDetailSection title="Runtime">
              {entry.runtime}
            </PluginDetailSection>
          ) : null}

          {entry.sourceType ? (
            <PluginDetailSection title="Source">
              {entry.sourceType}
            </PluginDetailSection>
          ) : null}

          {entry.registry ? (
            <PluginDetailSection title="Registry">
              {entry.registry}
            </PluginDetailSection>
          ) : null}

          {entry.trustStatus ? (
            <PluginDetailSection title="Trust">
              <Badge
                variant={
                  entry.trustStatus === "verified"
                    ? "default"
                    : entry.trustStatus === "untrusted"
                      ? "destructive"
                      : "secondary"
                }
                className="text-[10px]"
              >
                {entry.trustStatus}
              </Badge>
              {entry.approvalState && entry.approvalState !== "not-required" ? (
                <Badge variant="outline" className="ml-1 text-[10px]">
                  {entry.approvalState}
                </Badge>
              ) : null}
            </PluginDetailSection>
          ) : null}

          {entry.release?.version ? (
            <PluginDetailSection title="Release">
              <div className="grid gap-1">
                <p>Version: {entry.release.version}</p>
                {entry.release.channel ? (
                  <p>Channel: {entry.release.channel}</p>
                ) : null}
                {entry.release.publishedAt ? (
                  <p>Published: {entry.release.publishedAt}</p>
                ) : null}
              </div>
            </PluginDetailSection>
          ) : null}

          {entry.builtIn ? (
            <PluginDetailSection title="Readiness">
              <div className="grid gap-1">
                <p>
                  {entry.builtIn.readinessStatus ??
                    entry.builtIn.availabilityStatus ??
                    "unknown"}
                </p>
                {entry.builtIn.readinessMessage ??
                entry.builtIn.availabilityMessage ? (
                  <p>
                    {entry.builtIn.readinessMessage ??
                      entry.builtIn.availabilityMessage}
                  </p>
                ) : null}
                {entry.builtIn.nextStep ? (
                  <p>Next: {entry.builtIn.nextStep}</p>
                ) : null}
                {entry.builtIn.starterFamily ? (
                  <p>Starter family: {entry.builtIn.starterFamily}</p>
                ) : null}
                {entry.builtIn.coreFlows?.length ? (
                  <p>Core flows: {entry.builtIn.coreFlows.join(", ")}</p>
                ) : null}
                {entry.builtIn.dependencyRefs?.length ? (
                  <p>Dependencies: {entry.builtIn.dependencyRefs.join(", ")}</p>
                ) : null}
                {entry.builtIn.workspaceRefs?.length ? (
                  <p>Workspaces: {entry.builtIn.workspaceRefs.join(", ")}</p>
                ) : null}
                {entry.builtIn.missingPrerequisites?.length ? (
                  <p>
                    Missing: {entry.builtIn.missingPrerequisites.join(", ")}
                  </p>
                ) : null}
                {entry.builtIn.missingConfiguration?.length ? (
                  <p>
                    Config needed:{" "}
                    {entry.builtIn.missingConfiguration.join(", ")}
                  </p>
                ) : null}
                {entry.builtIn.docsRef ? (
                  <p>Docs: {entry.builtIn.docsRef}</p>
                ) : null}
              </div>
            </PluginDetailSection>
          ) : null}
        </div>
      </div>
    </ScrollArea>
  );
}

/* ── Main detail panel ── */

interface PluginDetailPanelProps {
  plugin: PluginRecord | null;
  marketplaceEntry?: MarketplacePluginEntry | null;
  onConfigure?: (plugin: PluginRecord) => void;
  onInvoke?: (plugin: PluginRecord) => void;
  onInstallEntry?: (entry: MarketplacePluginEntry) => void;
  loading?: boolean;
}

export function PluginDetailPanel({
  plugin,
  marketplaceEntry,
  onConfigure,
  onInvoke,
  onInstallEntry,
  loading,
}: PluginDetailPanelProps) {
  const t = useTranslations("plugins");
  const enablePlugin = usePluginStore((s) => s.enablePlugin);
  const disablePlugin = usePluginStore((s) => s.disablePlugin);
  const activatePlugin = usePluginStore((s) => s.activatePlugin);
  const deactivatePlugin = usePluginStore((s) => s.deactivatePlugin);
  const updatePlugin = usePluginStore((s) => s.updatePlugin);
  const uninstallPlugin = usePluginStore((s) => s.uninstallPlugin);
  const checkHealth = usePluginStore((s) => s.checkHealth);
  const restartPlugin = usePluginStore((s) => s.restartPlugin);

  const handleConfigure = useCallback(() => {
    if (plugin) onConfigure?.(plugin);
  }, [plugin, onConfigure]);

  const handleInvoke = useCallback(() => {
    if (plugin) onInvoke?.(plugin);
  }, [plugin, onInvoke]);

  /* Show marketplace entry detail when no installed plugin is selected */
  if (!plugin && marketplaceEntry) {
    return (
      <MarketplaceEntryDetail
        entry={marketplaceEntry}
        onInstall={onInstallEntry}
        loading={loading}
      />
    );
  }

  if (!plugin) {
    return (
      <div className="flex h-full flex-col items-center justify-center gap-3 text-muted-foreground">
        <Puzzle className="size-12 opacity-20" />
        <p className="text-sm">{t("detailSidebar.selectPrompt")}</p>
      </div>
    );
  }

  const id = plugin.metadata.id;
  const state = plugin.lifecycle_state;
  const isExecutable =
    plugin.spec.runtime !== "declarative" && Boolean(plugin.runtime_host);

  const canEnable = state === "installed" || state === "disabled";
  const canDisable =
    state === "enabled" ||
    state === "active" ||
    state === "activating" ||
    state === "degraded";
  const canActivate = state === "enabled" && isExecutable;
  const canRestart =
    isExecutable && (state === "active" || state === "degraded");
  const canDeactivate = state === "active" && isExecutable;
  const canInvoke = state === "active" && isExecutable;
  const hasUpdate =
    Boolean(plugin.source.release?.availableVersion) &&
    plugin.source.release?.availableVersion !== plugin.metadata.version &&
    Boolean(plugin.source.path ?? plugin.resolved_source_path);
  const canCheckHealth =
    isExecutable && (state === "active" || state === "degraded");

  const showKindTab = hasKindSpecificData(plugin);
  const showMCPTab =
    plugin.kind === "ToolPlugin" && plugin.spec.runtime === "mcp";
  const showWorkflowTab = plugin.kind === "WorkflowPlugin";

  const roleConsumers = plugin.roleConsumers ?? [];

  return (
    <ScrollArea className="h-full">
      <div className="p-4">
        {/* Header */}
        <div className="flex items-start gap-4">
          <PluginIcon name={plugin.metadata.name} kind={plugin.kind} size="lg" />
          <div className="min-w-0 flex-1">
            <h2 className="text-xl font-bold leading-tight">
              {plugin.metadata.name}
            </h2>
            <div className="mt-1 flex flex-wrap items-center gap-2 text-sm text-muted-foreground">
              <span>v{plugin.metadata.version}</span>
              <span className="text-border">·</span>
              <Badge variant="outline" className="text-[11px] px-1.5 py-0">
                {plugin.kind.replace("Plugin", "")}
              </Badge>
              <Badge
                variant="secondary"
                className={cn("text-[11px] px-1.5 py-0", stateColors[state])}
              >
                {state}
              </Badge>
              {plugin.source.type !== "local" ? (
                <>
                  <span className="text-border">·</span>
                  <span>{plugin.source.type}</span>
                </>
              ) : null}
            </div>
          </div>
        </div>

        {/* Action buttons */}
        <div className="mt-4 flex flex-wrap items-center gap-2">
          {canEnable ? (
            <Button size="sm" onClick={() => void enablePlugin(id)}>
              <Play className="mr-1.5 size-3.5" />
              {t("pluginCard.enable")}
            </Button>
          ) : canActivate ? (
            <Button size="sm" onClick={() => void activatePlugin(id)}>
              <Zap className="mr-1.5 size-3.5" />
              {t("pluginCard.activate")}
            </Button>
          ) : canDisable ? (
            <Button
              variant="outline"
              size="sm"
              onClick={() => void disablePlugin(id)}
            >
              <Pause className="mr-1.5 size-3.5" />
              {t("pluginCard.disable")}
            </Button>
          ) : null}

          {hasUpdate ? (
            <Button
              variant="outline"
              size="sm"
              className="text-blue-600 border-blue-200 hover:bg-blue-50 dark:text-blue-400 dark:border-blue-800 dark:hover:bg-blue-950"
              onClick={() => void updatePlugin(plugin)}
            >
              <ArrowUpCircle className="mr-1.5 size-3.5" />
              {t("pluginCard.update")}
            </Button>
          ) : null}

          <Button variant="outline" size="sm" onClick={handleConfigure}>
            <Settings className="mr-1.5 size-3.5" />
            {t("pluginCard.configure")}
          </Button>

          {canInvoke ? (
            <Button variant="outline" size="sm" onClick={handleInvoke}>
              <Terminal className="mr-1.5 size-3.5" />
              {t("pluginCard.invoke")}
            </Button>
          ) : null}

          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" size="sm" className="h-8 w-8 p-0">
                <MoreHorizontal className="size-4" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-44">
              {canCheckHealth ? (
                <DropdownMenuItem onClick={() => void checkHealth(id)}>
                  <HeartPulse className="mr-2 size-3.5" />
                  {t("pluginCard.health")}
                </DropdownMenuItem>
              ) : null}
              {canRestart ? (
                <DropdownMenuItem onClick={() => void restartPlugin(id)}>
                  <RotateCcw className="mr-2 size-3.5" />
                  {t("pluginCard.restart")}
                </DropdownMenuItem>
              ) : null}
              {canDeactivate ? (
                <DropdownMenuItem onClick={() => void deactivatePlugin(id)}>
                  <Square className="mr-2 size-3.5" />
                  {t("pluginCard.deactivate")}
                </DropdownMenuItem>
              ) : null}
              <DropdownMenuSeparator />
              <DropdownMenuItem
                className="text-destructive focus:text-destructive"
                onClick={() => void uninstallPlugin(id)}
              >
                <Trash2 className="mr-2 size-3.5" />
                {t("pluginCard.uninstall")}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>

        {/* Description */}
        {plugin.metadata.description ? (
          <p className="mt-4 text-sm leading-relaxed text-muted-foreground">
            {plugin.metadata.description}
          </p>
        ) : null}

        {plugin.metadata.tags?.length ? (
          <div className="mt-3 flex flex-wrap gap-1">
            {plugin.metadata.tags.map((tag) => (
              <Badge key={tag} variant="secondary" className="text-[10px]">
                {tag}
              </Badge>
            ))}
          </div>
        ) : null}

        {plugin.last_error ? (
          <div className="mt-3 rounded-md bg-destructive/10 px-3 py-2 text-xs text-destructive">
            {plugin.last_error}
          </div>
        ) : null}

        <PluginTrustBadge source={plugin.source} />

        <Separator className="my-4" />

        {/* Tabs */}
        <Tabs defaultValue="details">
          <TabsList className="w-full flex-wrap justify-start">
            <TabsTrigger value="details">
              {t("detailSidebar.tabOverview")}
            </TabsTrigger>
            <TabsTrigger value="events">
              {t("detailSidebar.tabEvents")}
            </TabsTrigger>
            {showKindTab ? (
              <TabsTrigger value="contributions">
                {t("detailSidebar.tabKind")}
              </TabsTrigger>
            ) : null}
            {showMCPTab ? (
              <TabsTrigger value="mcp">
                {t("detailSidebar.tabMcp")}
              </TabsTrigger>
            ) : null}
            {showWorkflowTab ? (
              <TabsTrigger value="workflow">
                {t("detailSidebar.tabWorkflow")}
              </TabsTrigger>
            ) : null}
          </TabsList>

          <TabsContent value="details" className="mt-4">
            <div className="grid gap-3">
              <div className="grid gap-3 sm:grid-cols-2">
                <PluginDetailSection title="Runtime Host">
                  {plugin.runtime_host ?? "Not executable"}
                </PluginDetailSection>

                <PluginDetailSection title="Runtime Type">
                  {plugin.spec.runtime}
                  {plugin.spec.transport
                    ? ` (${plugin.spec.transport})`
                    : ""}
                </PluginDetailSection>

                <PluginDetailSection title="Permissions">
                  {renderPermissions(plugin)}
                </PluginDetailSection>

                <PluginDetailSection title="ABI Compatibility">
                  {plugin.runtime_metadata?.abi_version ?? "n/a"} ·{" "}
                  {plugin.runtime_metadata?.compatible
                    ? "Compatible"
                    : "Incompatible"}
                </PluginDetailSection>

                <PluginDetailSection title="Restart Count">
                  {plugin.restart_count}
                </PluginDetailSection>

                <PluginDetailSection title="Last Health Check">
                  {plugin.last_health_at ?? "Not recorded"}
                </PluginDetailSection>
              </div>

              <PluginDetailSection title="Source Path">
                <span className="break-all font-mono text-xs">
                  {plugin.resolved_source_path ??
                    plugin.source.path ??
                    "No resolved path"}
                </span>
              </PluginDetailSection>

              {plugin.source.registry ||
              plugin.source.entry ||
              plugin.source.version ? (
                <PluginDetailSection title="Remote Provenance">
                  <div className="grid gap-1">
                    {plugin.source.registry ? (
                      <p>Registry: {plugin.source.registry}</p>
                    ) : null}
                    {plugin.source.entry ? (
                      <p>Entry: {plugin.source.entry}</p>
                    ) : null}
                    {plugin.source.version ? (
                      <p>Version: {plugin.source.version}</p>
                    ) : null}
                  </div>
                </PluginDetailSection>
              ) : null}

              {plugin.source.release ? (
                <PluginDetailSection title="Release Info">
                  <div className="grid gap-1">
                    {plugin.source.release.version ? (
                      <p>Version: {plugin.source.release.version}</p>
                    ) : null}
                    {plugin.source.release.channel ? (
                      <p>Channel: {plugin.source.release.channel}</p>
                    ) : null}
                    {plugin.source.release.publishedAt ? (
                      <p>Published: {plugin.source.release.publishedAt}</p>
                    ) : null}
                    {plugin.source.release.notesUrl ? (
                      <p>
                        Notes:{" "}
                        <a
                          href={plugin.source.release.notesUrl}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="underline"
                        >
                          Release Notes
                        </a>
                      </p>
                    ) : null}
                    {plugin.source.release.availableVersion ? (
                      <p className="font-medium text-foreground">
                        Update available: v
                        {plugin.source.release.availableVersion}
                      </p>
                    ) : null}
                  </div>
                </PluginDetailSection>
              ) : null}

              {plugin.builtIn ? (
                <PluginDetailSection title="Built-in Readiness">
                  <div className="grid gap-1">
                    <p>
                      {plugin.builtIn.readinessStatus ??
                        plugin.builtIn.availabilityStatus ??
                        "unknown"}
                    </p>
                    {plugin.builtIn.readinessMessage ??
                    plugin.builtIn.availabilityMessage ? (
                      <p>
                        {plugin.builtIn.readinessMessage ??
                          plugin.builtIn.availabilityMessage}
                      </p>
                    ) : null}
                    {plugin.builtIn.nextStep ? (
                      <p>Next: {plugin.builtIn.nextStep}</p>
                    ) : null}
                    {plugin.builtIn.starterFamily ? (
                      <p>Starter family: {plugin.builtIn.starterFamily}</p>
                    ) : null}
                    {plugin.builtIn.coreFlows?.length ? (
                      <p>Core flows: {plugin.builtIn.coreFlows.join(", ")}</p>
                    ) : null}
                    {plugin.builtIn.dependencyRefs?.length ? (
                      <p>Dependencies: {plugin.builtIn.dependencyRefs.join(", ")}</p>
                    ) : null}
                    {plugin.builtIn.workspaceRefs?.length ? (
                      <p>Workspaces: {plugin.builtIn.workspaceRefs.join(", ")}</p>
                    ) : null}
                    {plugin.builtIn.missingPrerequisites?.length ? (
                      <p>
                        Missing prerequisites:{" "}
                        {plugin.builtIn.missingPrerequisites.join(", ")}
                      </p>
                    ) : null}
                    {plugin.builtIn.missingConfiguration?.length ? (
                      <p>
                        Missing config:{" "}
                        {plugin.builtIn.missingConfiguration.join(", ")}
                      </p>
                    ) : null}
                  </div>
                </PluginDetailSection>
              ) : null}

              {roleConsumers.length > 0 ? (
                <PluginDetailSection title="Role Consumers">
                  <div className="grid gap-1">
                    {roleConsumers.map((c) => (
                      <p
                        key={`${c.roleId}:${c.referenceType}`}
                      >
                        {c.roleName
                          ? `${c.roleName} (${c.roleId})`
                          : c.roleId}{" "}
                        · {c.referenceType} · {c.status}
                      </p>
                    ))}
                    <a href="/roles" className="underline text-xs">
                      Open roles workspace
                    </a>
                  </div>
                </PluginDetailSection>
              ) : null}
            </div>
          </TabsContent>

          <TabsContent value="events" className="mt-4">
            <PluginEventTimeline pluginId={plugin.metadata.id} />
          </TabsContent>

          {showKindTab ? (
            <TabsContent value="contributions" className="mt-4">
              <PluginKindDetail plugin={plugin} />
            </TabsContent>
          ) : null}

          {showMCPTab ? (
            <TabsContent value="mcp" className="mt-4">
              <PluginMCPPanel plugin={plugin} />
            </TabsContent>
          ) : null}

          {showWorkflowTab ? (
            <TabsContent value="workflow" className="mt-4">
              <PluginWorkflowRuns plugin={plugin} />
            </TabsContent>
          ) : null}
        </Tabs>
      </div>
    </ScrollArea>
  );
}
