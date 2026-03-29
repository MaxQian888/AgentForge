"use client";

import { useCallback, useEffect, useEffectEvent, useMemo, useState } from "react";
import { useTranslations } from "next-intl";
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
import { PluginDetailSidebar } from "@/components/plugins/plugin-detail-sidebar";
import { PluginInstallDialog } from "@/components/plugins/plugin-install-dialog";
import { PluginConfigDialog } from "@/components/plugins/plugin-config-dialog";
import { PluginInvokeDialog } from "@/components/plugins/plugin-invoke-dialog";
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
  const t = useTranslations("plugins");
  const plugins = usePluginStore((s) => s.plugins);
  const builtins = usePluginStore((s) => s.builtins);
  const marketplace = usePluginStore((s) => s.marketplace);
  const remoteMarketplace = usePluginStore((s) => s.remoteMarketplace);
  const filters = usePluginStore((s) => s.filters);
  const selectedPluginId = usePluginStore((s) => s.selectedPluginId);
  const loading = usePluginStore((s) => s.loading);
  const error = usePluginStore((s) => s.error);
  const fetchPlugins = usePluginStore((s) => s.fetchPlugins);
  const discoverBuiltins = usePluginStore((s) => s.discoverBuiltins);
  const fetchMarketplace = usePluginStore((s) => s.fetchMarketplace);
  const fetchRemoteMarketplace = usePluginStore((s) => s.fetchRemoteMarketplace);
  const installFromCatalog = usePluginStore((s) => s.installFromCatalog);
  const installFromRemote = usePluginStore((s) => s.installFromRemote);
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
  const [invokePlugin, setInvokePlugin] = useState<PluginRecord | null>(null);
  const [invokeOpen, setInvokeOpen] = useState(false);
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
    void fetchRemoteMarketplace();
  }, [fetchPlugins, discoverBuiltins, fetchMarketplace, fetchRemoteMarketplace]);

  const handleConfigure = useCallback((plugin: PluginRecord) => {
    setConfigPlugin(plugin);
    setConfigOpen(true);
  }, []);

  const handleInvoke = useCallback((plugin: PluginRecord) => {
    setInvokePlugin(plugin);
    setInvokeOpen(true);
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
        return;
      }

      if (event.type === "plugin.lifecycle") {
        setPluginRuntimeSummary((current) => ({
          ...current,
          eventBridgeAvailable: true,
          lastUpdatedAt: event.timestamp ?? current.lastUpdatedAt,
        }));
        void fetchPlugins();
        return;
      }

      if (event.type === "shell.action") {
        setPluginRuntimeSummary((current) => ({
          ...current,
          eventBridgeAvailable: true,
          lastUpdatedAt: event.timestamp ?? current.lastUpdatedAt,
        }));
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
  }, [fetchPlugins, isDesktop, subscribeDesktopEvents]);

  const handleDesktopNotification = useCallback(async () => {
    const result = await sendNotification({
      notificationId: `plugins-desktop-runtime-${desktopRuntime.overall}`,
      type: "desktop.runtime.status",
      title: "AgentForge Desktop",
      body: `Desktop runtime is currently ${desktopRuntime.overall}.`,
      href: "/plugins",
      createdAt: new Date().toISOString(),
      deliveryPolicy: "always",
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
    () =>
      filterMarketplaceEntries(marketplace, filters).filter(
        (entry) => entry.sourceType !== "builtin",
      ),
    [marketplace, filters],
  );

  const filteredRemoteMarketplace = useMemo(
    () => filterMarketplaceEntries(remoteMarketplace.entries, filters),
    [remoteMarketplace.entries, filters],
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
        <h1 className="text-2xl font-bold">{t("title")}</h1>
        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => {
              void fetchPlugins();
              void discoverBuiltins();
              void fetchMarketplace();
              void fetchRemoteMarketplace();
            }}
            disabled={loading}
          >
            <RefreshCw className="mr-1 size-3.5" />
            {t("refresh")}
          </Button>
          <Button size="sm" onClick={() => setInstallOpen(true)}>
            <FolderOpen className="mr-1 size-3.5" />
            {t("installLocal")}
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
                {t("desktopRuntime")}
              </CardTitle>
              <CardDescription>
                {t("desktopRuntimeDesc")}
              </CardDescription>
            </div>
            <Badge variant={renderRuntimeTone(desktopRuntime.overall)}>
              {isDesktop ? desktopRuntime.overall : t("webFallback")}
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
                  <p>{t("url")}: {runtimeUnit.url ?? t("urlUnavailable")}</p>
                  <p>{t("pid")}: {runtimeUnit.pid ?? t("pidNotRunning")}</p>
                  <p>{t("restartCount")}: {runtimeUnit.restartCount}</p>
                  <p>
                    {t("lastStart")}: {runtimeUnit.lastStartedAt ?? t("lastStartNone")}
                  </p>
                  <p>{t("lastError")}: {runtimeUnit.lastError ?? t("lastErrorNone")}</p>
                </div>
              </div>
            ))}
          </div>

          <div className="flex flex-col gap-3 rounded-lg border border-border/60 p-4 text-sm">
            <div className="grid gap-2">
              <p className="font-medium">{t("helperSummary")}</p>
              <p className="text-muted-foreground">
                {t("bridgePlugins")}: {pluginRuntimeSummary.bridgePluginCount}
              </p>
              <p className="text-muted-foreground">
                {t("activeBridgeRuntimes")}: {pluginRuntimeSummary.activeRuntimeCount}
              </p>
              <p className="text-muted-foreground">
                {t("eventBridge")}: {pluginRuntimeSummary.eventBridgeAvailable ? t("eventBridgeAvailable") : t("eventBridgeUnavailable")}
              </p>
              <p className="text-muted-foreground">
                {t("lastDesktopEvent")}: {lastDesktopEvent ?? t("lastDesktopEventNone")}
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
                {t("syncTray")}
              </Button>
              <Button
                variant="outline"
                size="sm"
                onClick={() => void handleUpdateCheck()}
              >
                {t("checkUpdate")}
              </Button>
              {desktopUpdate?.ok && desktopUpdate.status === "available" ? (
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => void handleInstallUpdate()}
                >
                  {t("installUpdate")}
                </Button>
              ) : null}
              {desktopUpdate?.ok &&
              desktopUpdate.status === "ready_to_relaunch" ? (
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => void handleRelaunchToUpdate()}
                >
                  {t("restartToUpdate")}
                </Button>
              ) : null}
              <Button
                variant="outline"
                size="sm"
                onClick={() => void handleDesktopNotification()}
              >
                <BellRing className="mr-1 size-3.5" />
                {t("notify")}
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
                    ? t("updateInstalled", { version: activeDesktopUpdate.version })
                    : t("updateReady", { version: activeDesktopUpdate.version })}
                </p>
                <p className="mt-1">
                  {t("currentVersion", { version: activeDesktopUpdate.currentVersion ?? "Unknown" })}
                </p>
                {activeDesktopUpdate.publishedAt ? (
                  <p className="mt-1">
                    {t("publishedAt", { date: activeDesktopUpdate.publishedAt })}
                  </p>
                ) : null}
                {activeDesktopUpdate.notes ? (
                  <p className="mt-1">{activeDesktopUpdate.notes}</p>
                ) : null}
                {desktopUpdateProgress ? (
                  <p className="mt-1">
                    {desktopUpdateProgress.phase === "downloading"
                      ? t("downloading", { downloaded: desktopUpdateProgress.downloadedBytes, total: desktopUpdateProgress.totalBytes ?? "unknown" })
                      : t("installing")}
                  </p>
                ) : null}
              </div>
            ) : null}
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t("filterPlugins")}</CardTitle>
          <CardDescription>
            {t("filterPluginsDesc")}
          </CardDescription>
        </CardHeader>
        <CardContent className="grid gap-3 md:grid-cols-2 xl:grid-cols-6">
          <div className="flex flex-col gap-1.5 xl:col-span-2">
            <label htmlFor="plugin-search" className="text-sm font-medium">
              {t("searchPlugins")}
            </label>
            <input
              id="plugin-search"
              aria-label={t("searchPlugins")}
              className="h-10 rounded-md border bg-background px-3 text-sm"
              value={searchQuery}
              onChange={(event) => {
                setSearchQuery(event.target.value);
                setFilters({ query: event.target.value });
              }}
              placeholder={t("searchPlaceholder")}
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <label htmlFor="plugin-kind" className="text-sm font-medium">
              {t("kind")}
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
              <option value="all">{t("allKinds")}</option>
              <option value="ToolPlugin">Tool Plugin</option>
              <option value="RolePlugin">Role Plugin</option>
              <option value="WorkflowPlugin">Workflow Plugin</option>
              <option value="IntegrationPlugin">Integration Plugin</option>
              <option value="ReviewPlugin">Review Plugin</option>
            </select>
          </div>
          <div className="flex flex-col gap-1.5">
            <label htmlFor="plugin-lifecycle" className="text-sm font-medium">
              {t("lifecycle")}
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
              <option value="all">{t("allStates")}</option>
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
              {t("hostFilter")}
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
              <option value="all">{t("allHosts")}</option>
              <option value="go-orchestrator">go-orchestrator</option>
              <option value="ts-bridge">ts-bridge</option>
            </select>
          </div>
          <div className="flex flex-col gap-1.5">
            <label htmlFor="plugin-source" className="text-sm font-medium">
              {t("source")}
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
              <option value="all">{t("allSources")}</option>
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
              {t("clearFilters")}
            </Button>
          </div>
        </CardContent>
      </Card>

      <div className="grid gap-6 xl:grid-cols-[minmax(0,2fr)_minmax(320px,1fr)]">
        <div className="flex flex-col gap-6">
          <section>
            <div className="mb-3 flex items-center gap-2">
              <h2 className="text-lg font-semibold">{t("installedPlugins")}</h2>
              <Badge variant="secondary">{filteredInstalled.length}</Badge>
            </div>
            {filteredInstalled.length === 0 ? (
              <div className="flex h-[120px] items-center justify-center rounded-md border border-dashed text-sm text-muted-foreground">
                {loading
                  ? t("loadingPlugins")
                  : t("noInstalledMatch")}
              </div>
            ) : (
              <div className="grid gap-4 sm:grid-cols-2">
                {filteredInstalled.map((plugin) => (
                  <PluginCard
                    key={plugin.metadata.id}
                    plugin={plugin}
                    onConfigure={handleConfigure}
                    onInvoke={handleInvoke}
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
                {t("builtInPlugins")}
              </h2>
              <Badge variant="secondary">{filteredBuiltins.length}</Badge>
            </div>
            {filteredBuiltins.length === 0 ? (
              <div className="flex h-[120px] items-center justify-center rounded-md border border-dashed text-sm text-muted-foreground">
                {loading
                  ? t("discoveringBuiltins")
                  : t("noBuiltinMatch")}
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
                        <div className="flex items-center gap-2">
                          {builtin.builtIn?.readinessStatus ?? builtin.builtIn?.availabilityStatus ? (
                            <Badge variant="secondary" className="text-xs">
                              {builtin.builtIn?.readinessStatus ?? builtin.builtIn?.availabilityStatus}
                            </Badge>
                          ) : null}
                          <Badge variant="outline" className="text-xs">
                            {builtin.kind}
                          </Badge>
                        </div>
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
                      {builtin.builtIn?.readinessMessage ?? builtin.builtIn?.availabilityMessage ? (
                        <p className="text-xs text-muted-foreground">
                          {builtin.builtIn?.readinessMessage ?? builtin.builtIn?.availabilityMessage}
                        </p>
                      ) : null}
                      {builtin.builtIn?.nextStep ? (
                        <p className="text-xs text-muted-foreground">
                          {builtin.builtIn.nextStep}
                        </p>
                      ) : null}
                      {builtin.builtIn?.missingPrerequisites?.length ? (
                        <p className="text-xs text-muted-foreground">
                          Missing prerequisites: {builtin.builtIn.missingPrerequisites.join(", ")}
                        </p>
                      ) : null}
                      {builtin.builtIn?.missingConfiguration?.length ? (
                        <p className="text-xs text-muted-foreground">
                          Missing configuration: {builtin.builtIn.missingConfiguration.join(", ")}
                        </p>
                      ) : null}
                      {builtin.builtIn?.docsRef ? (
                        <p className="text-xs text-muted-foreground">
                          {builtin.builtIn.docsRef}
                        </p>
                      ) : null}
                      <p className="text-xs text-muted-foreground">
                        {t("installableFromBuiltin")}
                      </p>
                      <Button
                        variant="outline"
                        size="sm"
                        className="w-fit"
                        onClick={() => void installFromCatalog(builtin.metadata.id)}
                        disabled={loading || builtin.builtIn?.installable === false}
                      >
                        <Download className="mr-1 size-3.5" />
                        {t("install")}
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
              <h2 className="text-lg font-semibold">{t("marketplace")}</h2>
              <Badge variant="secondary">{filteredMarketplace.length}</Badge>
            </div>
            {filteredMarketplace.length === 0 ? (
              <div className="flex h-[120px] items-center justify-center rounded-md border border-dashed text-sm text-muted-foreground">
                {loading
                  ? t("loadingMarketplace")
                  : t("noMarketplaceMatch")}
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
                        {entry.installed ? (
                          <Badge className="bg-emerald-500/15 text-emerald-700 dark:text-emerald-400">
                            {t("installed")}
                          </Badge>
                        ) : entry.sourceType === "builtin" ? (
                          <Badge variant="secondary">{t("builtIn")}</Badge>
                        ) : entry.sourceType === "catalog" ? (
                          <Button
                            variant="outline"
                            size="sm"
                            onClick={() => void installFromCatalog(entry.id)}
                            disabled={loading}
                          >
                            <Download className="mr-1 size-3.5" />
                            {t("install")}
                          </Button>
                        ) : (
                          <>
                            <Badge variant="secondary">{t("browseOnly")}</Badge>
                            <span className="text-xs text-muted-foreground">
                              {t("remoteInstallNotReady")}
                            </span>
                          </>
                        )}
                      </div>
                    </CardContent>
                  </Card>
                ))}
              </div>
            )}
          </section>

          <Separator />

          <section>
            <div className="mb-3 flex items-center gap-2">
              <h2 className="text-lg font-semibold">{t("remoteRegistry")}</h2>
              <Badge variant="secondary">{filteredRemoteMarketplace.length}</Badge>
            </div>
            {remoteMarketplace.registry ? (
              <p className="mb-3 text-xs text-muted-foreground">
                {t("remoteRegistrySource")}: {remoteMarketplace.registry}
              </p>
            ) : null}
            {!remoteMarketplace.available && remoteMarketplace.error ? (
              <div className="rounded-md border border-border/60 bg-muted/30 px-3 py-2 text-sm text-muted-foreground">
                {remoteMarketplace.error}
              </div>
            ) : filteredRemoteMarketplace.length === 0 ? (
              <div className="flex h-[120px] items-center justify-center rounded-md border border-dashed text-sm text-muted-foreground">
                {loading ? t("loadingRemoteRegistry") : t("noRemoteRegistryMatch")}
              </div>
            ) : (
              <div className="grid gap-4 sm:grid-cols-2">
                {filteredRemoteMarketplace.map((entry) => (
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
                            variant="outline"
                            size="sm"
                            onClick={() => void installFromRemote(entry.id, entry.version)}
                            disabled={loading}
                          >
                            <Download className="mr-1 size-3.5" />
                            {t("installRemote")}
                          </Button>
                        )}
                      </div>
                    </CardContent>
                  </Card>
                ))}
              </div>
            )}
          </section>
        </div>

        <PluginDetailSidebar plugin={selectedPlugin} />
      </div>

      <PluginInstallDialog open={installOpen} onOpenChange={setInstallOpen} />
      <PluginConfigDialog
        plugin={configPlugin}
        open={configOpen}
        onOpenChange={setConfigOpen}
      />
      <PluginInvokeDialog
        plugin={invokePlugin}
        open={invokeOpen}
        onOpenChange={setInvokeOpen}
      />
    </div>
  );
}
