"use client";

import { useCallback, useEffect, useEffectEvent, useMemo, useState } from "react";
import {
  BellRing,
  Download,
  FolderOpen,
  MonitorCog,
  Puzzle,
  RefreshCw,
} from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Separator } from "@/components/ui/separator";
import { PluginCard } from "@/components/plugins/plugin-card";
import { PluginInstallDialog } from "@/components/plugins/plugin-install-dialog";
import { PluginConfigDialog } from "@/components/plugins/plugin-config-dialog";
import { usePlatformCapability } from "@/hooks/use-platform-capability";
import type {
  DesktopUpdateInfo,
  DesktopUpdateProgress,
  DesktopRuntimeStatus,
  PlatformUpdateResult,
  PluginRuntimeSummary,
} from "@/lib/platform-runtime";
import {
  filterMarketplaceEntries,
  filterPluginRecords,
  usePluginStore,
  type PluginPanelFilters,
  type PluginRecord,
} from "@/lib/stores/plugin-store";

const EMPTY_RUNTIME_STATUS: DesktopRuntimeStatus = {
  overall: "stopped",
  backend: {
    label: "backend",
    status: "stopped",
    url: null,
    pid: null,
    restartCount: 0,
    lastError: null,
    lastStartedAt: null,
  },
  bridge: {
    label: "bridge",
    status: "stopped",
    url: null,
    pid: null,
    restartCount: 0,
    lastError: null,
    lastStartedAt: null,
  },
};

const EMPTY_PLUGIN_RUNTIME_SUMMARY: PluginRuntimeSummary = {
  activeRuntimeCount: 0,
  backendHealthy: false,
  bridgeHealthy: false,
  bridgePluginCount: 0,
  eventBridgeAvailable: false,
  lastUpdatedAt: null,
  warnings: [],
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

  return permissions.length > 0 ? permissions.join(" · ") : "No declared permissions";
}

function renderRuntimeTone(status: DesktopRuntimeStatus["overall"]): "default" | "destructive" | "secondary" {
  if (status === "ready") {
    return "default";
  }

  if (status === "degraded") {
    return "destructive";
  }

  return "secondary";
}

export default function PluginsPage() {
  const plugins = usePluginStore((s) => s.plugins);
  const builtins = usePluginStore((s) => s.builtins);
  const marketplace = usePluginStore((s) => s.marketplace);
  const filters = usePluginStore((s) => s.filters);
  const selectedPluginId = usePluginStore((s) => s.selectedPluginId);
  const loading = usePluginStore((s) => s.loading);
  const error = usePluginStore((s) => s.error);
  const fetchPlugins = usePluginStore((s) => s.fetchPlugins);
  const discoverBuiltins = usePluginStore((s) => s.discoverBuiltins);
  const fetchMarketplace = usePluginStore((s) => s.fetchMarketplace);
  const installLocal = usePluginStore((s) => s.installLocal);
  const setFilters = usePluginStore((s) => s.setFilters);
  const resetFilters = usePluginStore((s) => s.resetFilters);
  const selectPlugin = usePluginStore((s) => s.selectPlugin);
  const {
    checkForUpdate,
    getDesktopRuntimeStatus,
    getPluginRuntimeSummary,
    installUpdate,
    isDesktop,
    relaunchToUpdate,
    sendNotification,
    subscribeDesktopEvents,
    updateTray,
  } = usePlatformCapability();

  const [installOpen, setInstallOpen] = useState(false);
  const [configPlugin, setConfigPlugin] = useState<PluginRecord | null>(null);
  const [configOpen, setConfigOpen] = useState(false);
  const [searchQuery, setSearchQuery] = useState(filters.query);
  const [desktopRuntime, setDesktopRuntime] =
    useState<DesktopRuntimeStatus>(EMPTY_RUNTIME_STATUS);
  const [pluginRuntimeSummary, setPluginRuntimeSummary] =
    useState<PluginRuntimeSummary>(EMPTY_PLUGIN_RUNTIME_SUMMARY);
  const [desktopMessage, setDesktopMessage] = useState<string | null>(null);
  const [lastDesktopEvent, setLastDesktopEvent] = useState<string | null>(null);
  const [desktopUpdate, setDesktopUpdate] =
    useState<PlatformUpdateResult | null>(null);
  const [desktopUpdateProgress, setDesktopUpdateProgress] =
    useState<DesktopUpdateProgress | null>(null);

  useEffect(() => {
    void fetchPlugins();
    void discoverBuiltins();
    void fetchMarketplace();
  }, [fetchPlugins, discoverBuiltins, fetchMarketplace]);

  const handleConfigure = useCallback((plugin: PluginRecord) => {
    setConfigPlugin(plugin);
    setConfigOpen(true);
  }, []);

  const loadDesktopState = useEffectEvent(async () => {
    if (!isDesktop) {
      return;
    }

    const [runtimeStatus, summary] = await Promise.all([
      getDesktopRuntimeStatus(),
      getPluginRuntimeSummary(),
    ]);
    setDesktopRuntime(runtimeStatus);
    setPluginRuntimeSummary(summary);
  });

  useEffect(() => {
    void loadDesktopState();
  }, []);

  useEffect(() => {
    if (!isDesktop) {
      return;
    }

    let disposed = false;
    void subscribeDesktopEvents((event) => {
      if (disposed) {
        return;
      }

      if (event.runtime) {
        setDesktopRuntime(event.runtime);
      }
      setLastDesktopEvent(event.type);

      if (event.type === "runtime.updated" || event.type === "runtime.terminated") {
        void loadDesktopState();
      }
    }).then((cleanup) => {
      if (disposed) {
        cleanup();
        return;
      }

      cleanupRef = cleanup;
    });

    let cleanupRef = () => {};
    return () => {
      disposed = true;
      cleanupRef();
    };
  }, [isDesktop, subscribeDesktopEvents]);

  const handleDesktopNotification = useCallback(async () => {
    const result = await sendNotification({
      title: "AgentForge Desktop",
      body: `Desktop runtime is currently ${desktopRuntime.overall}.`,
    });

    setDesktopMessage(
      result.ok
        ? `Notification sent through ${result.mode} mode.`
        : result.error,
    );
  }, [desktopRuntime.overall, sendNotification]);

  const handleTraySync = useCallback(async () => {
    const result = await updateTray({
      title: `AgentForge · ${desktopRuntime.overall}`,
      tooltip: `Backend ${desktopRuntime.backend.status} / Bridge ${desktopRuntime.bridge.status}`,
      visible: true,
    });

    setDesktopMessage(
      result.ok
        ? `Tray state synced through ${result.mode} mode.`
        : result.error,
    );
  }, [
    desktopRuntime.backend.status,
    desktopRuntime.bridge.status,
    desktopRuntime.overall,
    updateTray,
  ]);

  const handleUpdateCheck = useCallback(async () => {
    const result = await checkForUpdate();
    setDesktopUpdate(result);
    setDesktopUpdateProgress(null);
    setDesktopMessage(
      !result.ok
        ? result.error
        : result.status === "available"
          ? `Desktop update metadata refreshed in ${result.mode} mode.`
          : "Desktop app is already up to date.",
    );
  }, [checkForUpdate]);

  const handleInstallUpdate = useCallback(async () => {
    const result = await installUpdate((event) => {
      setDesktopUpdateProgress(event);
    });

    setDesktopUpdate(result);
    setDesktopMessage(
      result.ok
        ? `Desktop update installation completed in ${result.mode} mode.`
        : result.error,
    );
  }, [installUpdate]);

  const handleRelaunchToUpdate = useCallback(async () => {
    const result = await relaunchToUpdate();
    setDesktopMessage(
      result.ok
        ? `Relaunch requested through ${result.mode} mode.`
        : result.error,
    );
  }, [relaunchToUpdate]);

  const activeDesktopUpdate: DesktopUpdateInfo | null =
    desktopUpdate &&
    desktopUpdate.ok &&
    "update" in desktopUpdate &&
    desktopUpdate.update
      ? desktopUpdate.update
      : null;

  const filteredInstalled = useMemo(
    () => filterPluginRecords(plugins, filters),
    [plugins, filters],
  );

  const installedIds = useMemo(
    () => new Set(plugins.map((plugin) => plugin.metadata.id)),
    [plugins],
  );

  const filteredBuiltins = useMemo(
    () =>
      filterPluginRecords(
        builtins.filter((plugin) => !installedIds.has(plugin.metadata.id)),
        filters,
      ),
    [builtins, filters, installedIds],
  );

  const filteredMarketplace = useMemo(
    () => filterMarketplaceEntries(marketplace, filters),
    [marketplace, filters],
  );

  const selectedPlugin = useMemo(
    () =>
      filteredInstalled.find((plugin) => plugin.metadata.id === selectedPluginId) ??
      filteredInstalled[0] ??
      null,
    [filteredInstalled, selectedPluginId],
  );

  useEffect(() => {
    if (!selectedPluginId && filteredInstalled[0]) {
      selectPlugin(filteredInstalled[0].metadata.id);
      return;
    }

    if (
      selectedPluginId &&
      filteredInstalled.length > 0 &&
      !filteredInstalled.some((plugin) => plugin.metadata.id === selectedPluginId)
    ) {
      selectPlugin(filteredInstalled[0].metadata.id);
    }
  }, [filteredInstalled, selectPlugin, selectedPluginId]);

  return (
    <div className="flex flex-col gap-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Plugins</h1>
        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => {
              void fetchPlugins();
              void discoverBuiltins();
              void fetchMarketplace();
            }}
            disabled={loading}
          >
            <RefreshCw className="mr-1 size-3.5" />
            Refresh
          </Button>
          <Button size="sm" onClick={() => setInstallOpen(true)}>
            <FolderOpen className="mr-1 size-3.5" />
            Install Local
          </Button>
        </div>
      </div>

      {error ? (
        <div className="rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {error}
        </div>
      ) : null}

      <Card>
        <CardHeader>
          <div className="flex flex-col gap-2 md:flex-row md:items-start md:justify-between">
            <div>
              <CardTitle className="flex items-center gap-2">
                <MonitorCog className="size-4" />
                Desktop runtime
              </CardTitle>
              <CardDescription>
                Desktop telemetry is additive only. Plugin data below still comes from
                the backend API as the authoritative source.
              </CardDescription>
            </div>
            <Badge variant={renderRuntimeTone(desktopRuntime.overall)}>
              {isDesktop ? desktopRuntime.overall : "web-fallback"}
            </Badge>
          </div>
        </CardHeader>
        <CardContent className="grid gap-4 lg:grid-cols-[minmax(0,2fr)_minmax(280px,1fr)]">
          <div className="grid gap-3 md:grid-cols-2">
            {[desktopRuntime.backend, desktopRuntime.bridge].map((runtimeUnit) => (
              <div
                key={runtimeUnit.label}
                className="rounded-lg border border-border/60 p-3 text-sm"
              >
                <div className="flex items-center justify-between gap-2">
                  <p className="font-medium capitalize">{runtimeUnit.label}</p>
                  <Badge variant={renderRuntimeTone(runtimeUnit.status)}>
                    {runtimeUnit.status}
                  </Badge>
                </div>
                <div className="mt-3 grid gap-2 text-muted-foreground">
                  <p>URL: {runtimeUnit.url ?? "Unavailable"}</p>
                  <p>PID: {runtimeUnit.pid ?? "Not running"}</p>
                  <p>Restart count: {runtimeUnit.restartCount}</p>
                  <p>
                    Last start: {runtimeUnit.lastStartedAt ?? "Not started in this session"}
                  </p>
                  <p>Last error: {runtimeUnit.lastError ?? "No recent runtime errors"}</p>
                </div>
              </div>
            ))}
          </div>

          <div className="flex flex-col gap-3 rounded-lg border border-border/60 p-4 text-sm">
            <div className="grid gap-2">
              <p className="font-medium">Read-only desktop helper summary</p>
              <p className="text-muted-foreground">
                Bridge plugins: {pluginRuntimeSummary.bridgePluginCount}
              </p>
              <p className="text-muted-foreground">
                Active bridge runtimes: {pluginRuntimeSummary.activeRuntimeCount}
              </p>
              <p className="text-muted-foreground">
                Event bridge: {pluginRuntimeSummary.eventBridgeAvailable ? "available" : "unavailable"}
              </p>
              <p className="text-muted-foreground">
                Last desktop event: {lastDesktopEvent ?? "No desktop events yet"}
              </p>
            </div>

            {pluginRuntimeSummary.warnings.length > 0 ? (
              <div className="rounded-md border border-border/60 bg-muted/40 px-3 py-2 text-xs text-muted-foreground">
                {pluginRuntimeSummary.warnings.join(" ")}
              </div>
            ) : null}

            <div className="flex flex-wrap gap-2">
              <Button
                variant="outline"
                size="sm"
                onClick={() => void handleTraySync()}
              >
                Sync tray
              </Button>
              <Button
                variant="outline"
                size="sm"
                onClick={() => void handleUpdateCheck()}
              >
                Check update
              </Button>
              {desktopUpdate?.ok && desktopUpdate.status === "available" ? (
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => void handleInstallUpdate()}
                >
                  Install update
                </Button>
              ) : null}
              {desktopUpdate?.ok &&
              desktopUpdate.status === "ready_to_relaunch" ? (
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => void handleRelaunchToUpdate()}
                >
                  Restart to update
                </Button>
              ) : null}
              <Button
                variant="outline"
                size="sm"
                onClick={() => void handleDesktopNotification()}
              >
                <BellRing className="mr-1 size-3.5" />
                Notify
              </Button>
            </div>

            {desktopMessage ? (
              <div className="rounded-md border border-border/60 bg-muted/30 px-3 py-2 text-xs text-muted-foreground">
                {desktopMessage}
              </div>
            ) : null}

            {activeDesktopUpdate ? (
              <div className="rounded-md border border-border/60 bg-muted/20 px-3 py-3 text-xs text-muted-foreground">
                <p className="font-medium text-foreground">
                  {desktopUpdate?.ok && desktopUpdate.status === "ready_to_relaunch"
                    ? `Update ${activeDesktopUpdate.version} installed and waiting for restart.`
                    : `Update ${activeDesktopUpdate.version} is ready to install.`}
                </p>
                <p className="mt-1">
                  Current version: {activeDesktopUpdate.currentVersion ?? "Unknown"}
                </p>
                {activeDesktopUpdate.publishedAt ? (
                  <p className="mt-1">
                    Published at: {activeDesktopUpdate.publishedAt}
                  </p>
                ) : null}
                {activeDesktopUpdate.notes ? (
                  <p className="mt-1">{activeDesktopUpdate.notes}</p>
                ) : null}
                {desktopUpdateProgress ? (
                  <p className="mt-1">
                    {desktopUpdateProgress.phase === "downloading"
                      ? `Downloading ${desktopUpdateProgress.downloadedBytes} / ${desktopUpdateProgress.totalBytes ?? "unknown"} bytes`
                      : "Installing downloaded update"}
                  </p>
                ) : null}
              </div>
            ) : null}
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Filter plugins</CardTitle>
          <CardDescription>
            Search across installed, built-in, and marketplace entries without
            leaving the panel.
          </CardDescription>
        </CardHeader>
        <CardContent className="grid gap-3 md:grid-cols-2 xl:grid-cols-6">
          <div className="flex flex-col gap-1.5 xl:col-span-2">
            <label htmlFor="plugin-search" className="text-sm font-medium">
              Search plugins
            </label>
            <input
              id="plugin-search"
              aria-label="Search plugins"
              className="h-10 rounded-md border bg-background px-3 text-sm"
              value={searchQuery}
              onChange={(event) => {
                setSearchQuery(event.target.value);
                setFilters({ query: event.target.value });
              }}
              placeholder="Search by name, tag, runtime, or source"
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <label htmlFor="plugin-kind" className="text-sm font-medium">
              Kind
            </label>
            <select
              id="plugin-kind"
              className="h-10 rounded-md border bg-background px-3 text-sm"
              value={filters.kind}
              onChange={(event) =>
                setFilters({
                  kind: event.target.value as PluginPanelFilters["kind"],
                })
              }
            >
              <option value="all">All kinds</option>
              <option value="ToolPlugin">Tool Plugin</option>
              <option value="RolePlugin">Role Plugin</option>
              <option value="WorkflowPlugin">Workflow Plugin</option>
              <option value="IntegrationPlugin">Integration Plugin</option>
              <option value="ReviewPlugin">Review Plugin</option>
            </select>
          </div>
          <div className="flex flex-col gap-1.5">
            <label htmlFor="plugin-lifecycle" className="text-sm font-medium">
              Lifecycle
            </label>
            <select
              id="plugin-lifecycle"
              className="h-10 rounded-md border bg-background px-3 text-sm"
              value={filters.lifecycleState}
              onChange={(event) =>
                setFilters({
                  lifecycleState:
                    event.target.value as PluginPanelFilters["lifecycleState"],
                })
              }
            >
              <option value="all">All states</option>
              <option value="installed">installed</option>
              <option value="enabled">enabled</option>
              <option value="activating">activating</option>
              <option value="active">active</option>
              <option value="degraded">degraded</option>
              <option value="disabled">disabled</option>
            </select>
          </div>
          <div className="flex flex-col gap-1.5">
            <label htmlFor="plugin-host" className="text-sm font-medium">
              Host filter
            </label>
            <select
              id="plugin-host"
              className="h-10 rounded-md border bg-background px-3 text-sm"
              value={filters.runtimeHost}
              onChange={(event) =>
                setFilters({
                  runtimeHost:
                    event.target.value as PluginPanelFilters["runtimeHost"],
                })
              }
            >
              <option value="all">All hosts</option>
              <option value="go-orchestrator">go-orchestrator</option>
              <option value="ts-bridge">ts-bridge</option>
            </select>
          </div>
          <div className="flex flex-col gap-1.5">
            <label htmlFor="plugin-source" className="text-sm font-medium">
              Source
            </label>
            <select
              id="plugin-source"
              className="h-10 rounded-md border bg-background px-3 text-sm"
              value={filters.sourceType}
              onChange={(event) =>
                setFilters({
                  sourceType:
                    event.target.value as PluginPanelFilters["sourceType"],
                })
              }
            >
              <option value="all">All sources</option>
              <option value="builtin">builtin</option>
              <option value="local">local</option>
              <option value="marketplace">marketplace</option>
            </select>
          </div>
          <div className="flex items-end">
            <Button
              variant="outline"
              className="w-full"
              onClick={() => {
                setSearchQuery("");
                resetFilters();
              }}
            >
              Clear filters
            </Button>
          </div>
        </CardContent>
      </Card>

      <div className="grid gap-6 xl:grid-cols-[minmax(0,2fr)_minmax(320px,1fr)]">
        <div className="flex flex-col gap-6">
          <section>
            <div className="mb-3 flex items-center gap-2">
              <h2 className="text-lg font-semibold">Installed plugins</h2>
              <Badge variant="secondary">{filteredInstalled.length}</Badge>
            </div>
            {filteredInstalled.length === 0 ? (
              <div className="flex h-[120px] items-center justify-center rounded-md border border-dashed text-sm text-muted-foreground">
                {loading
                  ? "Loading plugins..."
                  : "No installed plugins match the current filters."}
              </div>
            ) : (
              <div className="grid gap-4 sm:grid-cols-2">
                {filteredInstalled.map((plugin) => (
                  <PluginCard
                    key={plugin.metadata.id}
                    plugin={plugin}
                    onConfigure={handleConfigure}
                    onSelect={(entry) => selectPlugin(entry.metadata.id)}
                    selected={selectedPlugin?.metadata.id === plugin.metadata.id}
                  />
                ))}
              </div>
            )}
          </section>

          <Separator />

          <section>
            <div className="mb-3 flex items-center gap-2">
              <h2 className="text-lg font-semibold">
                <Puzzle className="mr-1.5 inline-block size-4" />
                Built-in plugins
              </h2>
              <Badge variant="secondary">{filteredBuiltins.length}</Badge>
            </div>
            {filteredBuiltins.length === 0 ? (
              <div className="flex h-[120px] items-center justify-center rounded-md border border-dashed text-sm text-muted-foreground">
                {loading
                  ? "Discovering built-in plugins..."
                  : "No built-in plugins match the current filters."}
              </div>
            ) : (
              <div className="grid gap-4 sm:grid-cols-2">
                {filteredBuiltins.map((builtin) => (
                  <Card key={builtin.metadata.id}>
                    <CardHeader className="pb-3">
                      <div className="flex items-center justify-between gap-2">
                        <CardTitle className="text-base">
                          {builtin.metadata.name}
                        </CardTitle>
                        <Badge variant="outline" className="text-xs">
                          {builtin.kind}
                        </Badge>
                      </div>
                      <CardDescription className="text-xs">
                        v{builtin.metadata.version}
                      </CardDescription>
                    </CardHeader>
                    <CardContent className="flex flex-col gap-3">
                      {builtin.metadata.description ? (
                        <p className="text-sm text-muted-foreground line-clamp-2">
                          {builtin.metadata.description}
                        </p>
                      ) : null}
                      <p className="text-xs text-muted-foreground">
                        Installable from the built-in discovery registry.
                      </p>
                      <Button
                        variant="outline"
                        size="sm"
                        className="w-fit"
                        onClick={() =>
                          void installLocal(
                            builtin.source.path ?? builtin.metadata.id,
                          )
                        }
                        disabled={loading}
                      >
                        <Download className="mr-1 size-3.5" />
                        Install
                      </Button>
                    </CardContent>
                  </Card>
                ))}
              </div>
            )}
          </section>

          <Separator />

          <section>
            <div className="mb-3 flex items-center gap-2">
              <h2 className="text-lg font-semibold">Marketplace</h2>
              <Badge variant="secondary">{filteredMarketplace.length}</Badge>
            </div>
            {filteredMarketplace.length === 0 ? (
              <div className="flex h-[120px] items-center justify-center rounded-md border border-dashed text-sm text-muted-foreground">
                {loading
                  ? "Loading marketplace..."
                  : "No marketplace plugins match the current filters."}
              </div>
            ) : (
              <div className="grid gap-4 sm:grid-cols-2">
                {filteredMarketplace.map((entry) => (
                  <Card key={entry.id}>
                    <CardHeader className="pb-3">
                      <div className="flex items-center justify-between gap-2">
                        <CardTitle className="text-base">{entry.name}</CardTitle>
                        <Badge variant="outline" className="text-xs">
                          {entry.kind}
                        </Badge>
                      </div>
                      <CardDescription className="text-xs">
                        v{entry.version} · {entry.author}
                      </CardDescription>
                    </CardHeader>
                    <CardContent className="flex flex-col gap-3">
                      <p className="text-sm text-muted-foreground">
                        {entry.description}
                      </p>
                      <div className="flex flex-wrap items-center gap-2">
                        <Badge variant="secondary">Browse only</Badge>
                        <span className="text-xs text-muted-foreground">
                          Remote marketplace installation is not wired into the
                          current platform contract yet.
                        </span>
                      </div>
                    </CardContent>
                  </Card>
                ))}
              </div>
            )}
          </section>
        </div>

        <Card className="h-fit">
          <CardHeader>
            <CardTitle>Plugin details</CardTitle>
            <CardDescription>
              Inspect runtime, permissions, health, and source metadata for the
              selected installed plugin.
            </CardDescription>
          </CardHeader>
          <CardContent className="flex flex-col gap-4">
            {selectedPlugin ? (
              <>
                <div className="flex items-start justify-between gap-3">
                  <div>
                    <h3 className="text-lg font-semibold">
                      {selectedPlugin.metadata.name}
                    </h3>
                    <p className="text-sm text-muted-foreground">
                      {selectedPlugin.metadata.description ||
                        "No plugin description provided."}
                    </p>
                  </div>
                  <Badge variant="secondary">
                    {selectedPlugin.lifecycle_state}
                  </Badge>
                </div>

                <div className="grid gap-3 text-sm">
                  <div className="rounded-lg border border-border/60 p-3">
                    <p className="font-medium">Runtime host</p>
                    <p className="text-muted-foreground">
                      {selectedPlugin.runtime_host ?? "Not executable"}
                    </p>
                  </div>
                  <div className="rounded-lg border border-border/60 p-3">
                    <p className="font-medium">Runtime declaration</p>
                    <p className="text-muted-foreground">
                      {selectedPlugin.spec.runtime}
                    </p>
                  </div>
                  <div className="rounded-lg border border-border/60 p-3">
                    <p className="font-medium">Permissions</p>
                    <p className="text-muted-foreground">
                      {renderPermissions(selectedPlugin)}
                    </p>
                  </div>
                  <div className="rounded-lg border border-border/60 p-3">
                    <p className="font-medium">Resolved source path</p>
                    <p className="text-muted-foreground">
                      {selectedPlugin.resolved_source_path ??
                        selectedPlugin.source.path ??
                        "No resolved source path"}
                    </p>
                  </div>
                  <div className="rounded-lg border border-border/60 p-3">
                    <p className="font-medium">Runtime metadata</p>
                    <p className="text-muted-foreground">
                      ABI {selectedPlugin.runtime_metadata?.abi_version ?? "n/a"} · Compatible{" "}
                      {selectedPlugin.runtime_metadata?.compatible ? "yes" : "no"}
                    </p>
                  </div>
                  <div className="rounded-lg border border-border/60 p-3">
                    <p className="font-medium">Restart count</p>
                    <p className="text-muted-foreground">
                      {selectedPlugin.restart_count}
                    </p>
                  </div>
                  <div className="rounded-lg border border-border/60 p-3">
                    <p className="font-medium">Last health</p>
                    <p className="text-muted-foreground">
                      {selectedPlugin.last_health_at ?? "Not recorded yet"}
                    </p>
                  </div>
                  <div className="rounded-lg border border-border/60 p-3">
                    <p className="font-medium">Last error</p>
                    <p className="text-muted-foreground">
                      {selectedPlugin.last_error || "No recent runtime errors"}
                    </p>
                  </div>
                </div>
              </>
            ) : (
              <div className="rounded-md border border-dashed px-4 py-8 text-sm text-muted-foreground">
                Select an installed plugin to inspect operational details.
              </div>
            )}
          </CardContent>
        </Card>
      </div>

      <PluginInstallDialog open={installOpen} onOpenChange={setInstallOpen} />
      <PluginConfigDialog
        plugin={configPlugin}
        open={configOpen}
        onOpenChange={setConfigOpen}
      />
    </div>
  );
}
