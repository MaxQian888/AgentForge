"use client";

import { useCallback, useEffect, useEffectEvent, useMemo, useState } from "react";
import { useTranslations } from "next-intl";
import {
  BellRing,
  ChevronDown,
  ChevronUp,
  Download,
  FolderOpen,
  MonitorCog,
  Puzzle,
  RefreshCw,
  Search,
  X,
} from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { PluginCard } from "@/components/plugins/plugin-card";
import { PluginDetailSidebar } from "@/components/plugins/plugin-detail-sidebar";
import { PluginInstallDialog } from "@/components/plugins/plugin-install-dialog";
import { PluginConfigDialog } from "@/components/plugins/plugin-config-dialog";
import { PluginInvokeDialog } from "@/components/plugins/plugin-invoke-dialog";
import { PageHeader } from "@/components/shared/page-header";
import { EmptyState } from "@/components/shared/empty-state";
import { ErrorBanner } from "@/components/shared/error-banner";
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
import { useBreadcrumbs } from "@/hooks/use-breadcrumbs";

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
  if (status === "ready") return "default";
  if (status === "degraded") return "destructive";
  return "secondary";
}

/* ---------- Main page ---------- */
export default function PluginsPage() {
  useBreadcrumbs([{ label: "Configuration", href: "/" }, { label: "Plugins" }]);
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
  const [runtimeExpanded, setRuntimeExpanded] = useState(false);
  const [detailSheetOpen, setDetailSheetOpen] = useState(false);
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
    if (!isDesktop) return;
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
    if (!isDesktop) return;
    let disposed = false;
    void subscribeDesktopEvents((event) => {
      if (disposed) return;
      if (event.runtime) setDesktopRuntime(event.runtime);
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
      if (disposed) { cleanup(); return; }
      cleanupRef = cleanup;
    });
    let cleanupRef = () => {};
    return () => { disposed = true; cleanupRef(); };
  }, [fetchPlugins, isDesktop, subscribeDesktopEvents]);

  /* Desktop handlers */
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
    setDesktopMessage(result.ok ? `Notification sent through ${result.mode} mode.` : result.error);
  }, [desktopRuntime.overall, sendNotification]);

  const handleTraySync = useCallback(async () => {
    const result = await updateTray({
      title: `AgentForge · ${desktopRuntime.overall}`,
      tooltip: `Backend ${desktopRuntime.backend.status} / Bridge ${desktopRuntime.bridge.status}`,
      visible: true,
    });
    setDesktopMessage(result.ok ? `Tray state synced through ${result.mode} mode.` : result.error);
  }, [desktopRuntime.backend.status, desktopRuntime.bridge.status, desktopRuntime.overall, updateTray]);

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
    const result = await installUpdate((event) => setDesktopUpdateProgress(event));
    setDesktopUpdate(result);
    setDesktopMessage(result.ok ? `Desktop update installation completed in ${result.mode} mode.` : result.error);
  }, [installUpdate]);

  const handleRelaunchToUpdate = useCallback(async () => {
    const result = await relaunchToUpdate();
    setDesktopMessage(result.ok ? `Relaunch requested through ${result.mode} mode.` : result.error);
  }, [relaunchToUpdate]);

  const activeDesktopUpdate: DesktopUpdateInfo | null =
    desktopUpdate && desktopUpdate.ok && "update" in desktopUpdate && desktopUpdate.update
      ? desktopUpdate.update
      : null;

  /* Filtering */
  const filteredInstalled = useMemo(() => filterPluginRecords(plugins, filters), [plugins, filters]);
  const installedIds = useMemo(() => new Set(plugins.map((p) => p.metadata.id)), [plugins]);
  const filteredBuiltins = useMemo(
    () => filterPluginRecords(builtins.filter((p) => !installedIds.has(p.metadata.id)), filters),
    [builtins, filters, installedIds],
  );
  const filteredMarketplace = useMemo(
    () => filterMarketplaceEntries(marketplace, filters).filter((e) => e.sourceType !== "builtin"),
    [marketplace, filters],
  );
  const filteredRemoteMarketplace = useMemo(
    () => filterMarketplaceEntries(remoteMarketplace.entries, filters),
    [remoteMarketplace.entries, filters],
  );

  const selectedPlugin = useMemo(
    () =>
      filteredInstalled.find((p) => p.metadata.id === selectedPluginId) ??
      filteredInstalled[0] ??
      null,
    [filteredInstalled, selectedPluginId],
  );

  useEffect(() => {
    if (!selectedPluginId && filteredInstalled[0]) {
      selectPlugin(filteredInstalled[0].metadata.id);
      return;
    }
    if (selectedPluginId && filteredInstalled.length > 0 && !filteredInstalled.some((p) => p.metadata.id === selectedPluginId)) {
      selectPlugin(filteredInstalled[0].metadata.id);
    }
  }, [filteredInstalled, selectPlugin, selectedPluginId]);

  const handleSelectPlugin = useCallback(
    (plugin: PluginRecord) => {
      selectPlugin(plugin.metadata.id);
      setDetailSheetOpen(true);
    },
    [selectPlugin],
  );

  const hasActiveFilters =
    filters.kind !== "all" ||
    filters.lifecycleState !== "all" ||
    filters.runtimeHost !== "all" ||
    filters.sourceType !== "all" ||
    filters.query !== "";

  return (
    <div className="flex flex-col gap-4">
      {/* ===== Header ===== */}
      <PageHeader
        title={t("title")}
        description={t("filterPluginsDesc")}
        actions={
          <>
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
              <RefreshCw className="mr-1.5 size-3.5" />
              {t("refresh")}
            </Button>
            <Button size="sm" onClick={() => setInstallOpen(true)}>
              <FolderOpen className="mr-1.5 size-3.5" />
              {t("installLocal")}
            </Button>
          </>
        }
      />

      {error ? (
        <ErrorBanner message={error} />
      ) : null}

      {/* ===== Desktop Runtime (collapsible) ===== */}
      <div className="rounded-xl border bg-card">
        <button
          type="button"
          className="flex w-full items-center justify-between px-4 py-3 text-left"
          onClick={() => setRuntimeExpanded((v) => !v)}
        >
          <div className="flex items-center gap-2.5">
            <MonitorCog className="size-4 text-muted-foreground" />
            <span className="text-sm font-medium">{t("desktopRuntime")}</span>
            <Badge variant={renderRuntimeTone(desktopRuntime.overall)} className="text-[10px]">
              {isDesktop ? desktopRuntime.overall : t("webFallback")}
            </Badge>
          </div>
          {runtimeExpanded ? (
            <ChevronUp className="size-4 text-muted-foreground" />
          ) : (
            <ChevronDown className="size-4 text-muted-foreground" />
          )}
        </button>

        {runtimeExpanded ? (
          <div className="border-t px-4 pb-4 pt-3">
            <p className="mb-3 text-xs text-muted-foreground">{t("desktopRuntimeDesc")}</p>
            <div className="grid gap-3 lg:grid-cols-[minmax(0,2fr)_minmax(280px,1fr)]">
              <div className="grid gap-3 md:grid-cols-2">
                {[desktopRuntime.backend, desktopRuntime.bridge].map((unit) => (
                  <div key={unit.label} className="rounded-lg border border-border/60 p-3 text-sm">
                    <div className="flex items-center justify-between gap-2">
                      <p className="font-medium capitalize">{unit.label}</p>
                      <Badge variant={renderRuntimeTone(unit.status)}>{unit.status}</Badge>
                    </div>
                    <div className="mt-3 grid gap-2 text-muted-foreground">
                      <p>{t("url")}: {unit.url ?? t("urlUnavailable")}</p>
                      <p>{t("pid")}: {unit.pid ?? t("pidNotRunning")}</p>
                      <p>{t("restartCount")}: {unit.restartCount}</p>
                      <p>{t("lastStart")}: {unit.lastStartedAt ?? t("lastStartNone")}</p>
                      <p>{t("lastError")}: {unit.lastError ?? t("lastErrorNone")}</p>
                    </div>
                  </div>
                ))}
              </div>

              <div className="flex flex-col gap-3 rounded-lg border border-border/60 p-4 text-sm">
                <div className="grid gap-2">
                  <p className="font-medium">{t("helperSummary")}</p>
                  <p className="text-muted-foreground">{t("bridgePlugins")}: {pluginRuntimeSummary.bridgePluginCount}</p>
                  <p className="text-muted-foreground">{t("activeBridgeRuntimes")}: {pluginRuntimeSummary.activeRuntimeCount}</p>
                  <p className="text-muted-foreground">{t("eventBridge")}: {pluginRuntimeSummary.eventBridgeAvailable ? t("eventBridgeAvailable") : t("eventBridgeUnavailable")}</p>
                  <p className="text-muted-foreground">{t("lastDesktopEvent")}: {lastDesktopEvent ?? t("lastDesktopEventNone")}</p>
                </div>

                {pluginRuntimeSummary.warnings.length > 0 ? (
                  <div className="rounded-md border border-border/60 bg-muted/40 px-3 py-2 text-xs text-muted-foreground">
                    {pluginRuntimeSummary.warnings.join(" ")}
                  </div>
                ) : null}

                <div className="flex flex-wrap gap-2">
                  <Button variant="outline" size="sm" onClick={() => void handleTraySync()}>{t("syncTray")}</Button>
                  <Button variant="outline" size="sm" onClick={() => void handleUpdateCheck()}>{t("checkUpdate")}</Button>
                  {desktopUpdate?.ok && desktopUpdate.status === "available" ? (
                    <Button variant="outline" size="sm" onClick={() => void handleInstallUpdate()}>{t("installUpdate")}</Button>
                  ) : null}
                  {desktopUpdate?.ok && desktopUpdate.status === "ready_to_relaunch" ? (
                    <Button variant="outline" size="sm" onClick={() => void handleRelaunchToUpdate()}>{t("restartToUpdate")}</Button>
                  ) : null}
                  <Button variant="outline" size="sm" onClick={() => void handleDesktopNotification()}>
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
                    <p className="mt-1">{t("currentVersion", { version: activeDesktopUpdate.currentVersion ?? "Unknown" })}</p>
                    {activeDesktopUpdate.publishedAt ? (
                      <p className="mt-1">{t("publishedAt", { date: activeDesktopUpdate.publishedAt })}</p>
                    ) : null}
                    {activeDesktopUpdate.notes ? <p className="mt-1">{activeDesktopUpdate.notes}</p> : null}
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
            </div>
          </div>
        ) : null}
      </div>

      {/* ===== Search + Filters ===== */}
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
        <div className="relative flex-1">
          <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            aria-label={t("searchPlugins")}
            className="pl-9 pr-8"
            value={searchQuery}
            onChange={(e) => {
              setSearchQuery(e.target.value);
              setFilters({ query: e.target.value });
            }}
            placeholder={t("searchPlaceholder")}
          />
          {searchQuery ? (
            <button
              type="button"
              className="absolute right-2.5 top-1/2 -translate-y-1/2 rounded-sm p-0.5 text-muted-foreground hover:text-foreground"
              onClick={() => { setSearchQuery(""); setFilters({ query: "" }); }}
              aria-label={t("clearFilters")}
            >
              <X className="size-3.5" />
            </button>
          ) : null}
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <Select
            value={filters.kind}
            onValueChange={(v) => setFilters({ kind: v as PluginPanelFilters["kind"] })}
          >
            <SelectTrigger size="sm" className="w-[130px]">
              <SelectValue placeholder={t("kind")} />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{t("allKinds")}</SelectItem>
              <SelectItem value="ToolPlugin">Tool</SelectItem>
              <SelectItem value="RolePlugin">Role</SelectItem>
              <SelectItem value="WorkflowPlugin">Workflow</SelectItem>
              <SelectItem value="IntegrationPlugin">Integration</SelectItem>
              <SelectItem value="ReviewPlugin">Review</SelectItem>
            </SelectContent>
          </Select>

          <Select
            value={filters.lifecycleState}
            onValueChange={(v) => setFilters({ lifecycleState: v as PluginPanelFilters["lifecycleState"] })}
          >
            <SelectTrigger size="sm" className="w-[130px]">
              <SelectValue placeholder={t("lifecycle")} />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{t("allStates")}</SelectItem>
              <SelectItem value="installed">installed</SelectItem>
              <SelectItem value="enabled">enabled</SelectItem>
              <SelectItem value="activating">activating</SelectItem>
              <SelectItem value="active">active</SelectItem>
              <SelectItem value="degraded">degraded</SelectItem>
              <SelectItem value="disabled">disabled</SelectItem>
            </SelectContent>
          </Select>

          <Select
            value={filters.runtimeHost}
            onValueChange={(v) => setFilters({ runtimeHost: v as PluginPanelFilters["runtimeHost"] })}
          >
            <SelectTrigger size="sm" className="w-[150px]">
              <SelectValue placeholder={t("hostFilter")} />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{t("allHosts")}</SelectItem>
              <SelectItem value="go-orchestrator">go-orchestrator</SelectItem>
              <SelectItem value="ts-bridge">ts-bridge</SelectItem>
            </SelectContent>
          </Select>

          <Select
            value={filters.sourceType}
            onValueChange={(v) => setFilters({ sourceType: v as PluginPanelFilters["sourceType"] })}
          >
            <SelectTrigger size="sm" className="w-[130px]">
              <SelectValue placeholder={t("source")} />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{t("allSources")}</SelectItem>
              <SelectItem value="builtin">builtin</SelectItem>
              <SelectItem value="local">local</SelectItem>
              <SelectItem value="marketplace">marketplace</SelectItem>
            </SelectContent>
          </Select>

          {hasActiveFilters ? (
            <Button
              variant="ghost"
              size="sm"
              className="h-8 text-xs"
              onClick={() => { setSearchQuery(""); resetFilters(); }}
            >
              <X className="mr-1 size-3" />
              {t("clearFilters")}
            </Button>
          ) : null}
        </div>
      </div>

      {/* ===== Main content: tabs + detail sidebar ===== */}
      <div className="grid gap-6 xl:grid-cols-[minmax(0,1fr)_360px]">
        <Tabs defaultValue="installed" className="min-w-0">
          <TabsList className="w-full sm:w-auto">
            <TabsTrigger value="installed" className="gap-1.5">
              {t("tabInstalled")}
              <Badge variant="secondary" className="ml-1 h-5 min-w-5 px-1 text-[10px]">
                {filteredInstalled.length}
              </Badge>
            </TabsTrigger>
            <TabsTrigger value="builtin" className="gap-1.5">
              {t("tabBuiltIn")}
              <Badge variant="secondary" className="ml-1 h-5 min-w-5 px-1 text-[10px]">
                {filteredBuiltins.length}
              </Badge>
            </TabsTrigger>
            <TabsTrigger value="marketplace" className="gap-1.5">
              {t("tabMarketplace")}
              <Badge variant="secondary" className="ml-1 h-5 min-w-5 px-1 text-[10px]">
                {filteredMarketplace.length}
              </Badge>
            </TabsTrigger>
            <TabsTrigger value="remote" className="gap-1.5">
              {t("tabRemote")}
              <Badge variant="secondary" className="ml-1 h-5 min-w-5 px-1 text-[10px]">
                {filteredRemoteMarketplace.length}
              </Badge>
            </TabsTrigger>
          </TabsList>

          {/* --- Installed --- */}
          <TabsContent value="installed" className="mt-4">
            {filteredInstalled.length === 0 ? (
              <EmptyState
                icon={Puzzle}
                title={loading ? t("loadingPlugins") : plugins.length === 0 ? t("noInstalledPlugins") : t("noInstalledMatch")}
              />
            ) : (
              <div className="grid gap-3 sm:grid-cols-2 2xl:grid-cols-3">
                {filteredInstalled.map((plugin) => (
                  <PluginCard
                    key={plugin.metadata.id}
                    plugin={plugin}
                    onConfigure={handleConfigure}
                    onInvoke={handleInvoke}
                    onSelect={handleSelectPlugin}
                    selected={selectedPlugin?.metadata.id === plugin.metadata.id}
                  />
                ))}
              </div>
            )}
          </TabsContent>

          {/* --- Built-in --- */}
          <TabsContent value="builtin" className="mt-4">
            {filteredBuiltins.length === 0 ? (
              <EmptyState
                icon={Puzzle}
                title={loading ? t("discoveringBuiltins") : t("noBuiltinMatch")}
              />
            ) : (
              <div className="grid gap-3 sm:grid-cols-2 2xl:grid-cols-3">
                {filteredBuiltins.map((builtin) => (
                  <div
                    key={builtin.metadata.id}
                    className="flex flex-col gap-3 rounded-xl border border-border/50 bg-card p-4"
                  >
                    <div className="flex items-start justify-between gap-2">
                      <div className="min-w-0">
                        <h3 className="truncate text-sm font-semibold">{builtin.metadata.name}</h3>
                        <p className="text-xs text-muted-foreground">v{builtin.metadata.version}</p>
                      </div>
                      <div className="flex items-center gap-1.5">
                        {builtin.builtIn?.readinessStatus ?? builtin.builtIn?.availabilityStatus ? (
                          <Badge variant="secondary" className="text-[10px] px-1.5 py-0">
                            {builtin.builtIn?.readinessStatus ?? builtin.builtIn?.availabilityStatus}
                          </Badge>
                        ) : null}
                        <Badge variant="outline" className="text-[10px] px-1.5 py-0">
                          {builtin.kind}
                        </Badge>
                      </div>
                    </div>

                    {builtin.metadata.description ? (
                      <p className="text-xs leading-relaxed text-muted-foreground line-clamp-2">
                        {builtin.metadata.description}
                      </p>
                    ) : null}

                    {builtin.builtIn?.readinessMessage ?? builtin.builtIn?.availabilityMessage ? (
                      <p className="text-xs text-muted-foreground">
                        {builtin.builtIn?.readinessMessage ?? builtin.builtIn?.availabilityMessage}
                      </p>
                    ) : null}
                    {builtin.builtIn?.nextStep ? (
                      <p className="text-xs text-muted-foreground">{builtin.builtIn.nextStep}</p>
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
                      <p className="text-xs text-muted-foreground">{builtin.builtIn.docsRef}</p>
                    ) : null}

                    <div className="mt-auto flex items-center justify-between">
                      <span className="text-[11px] text-muted-foreground">{t("installableFromBuiltin")}</span>
                      <Button
                        variant="outline"
                        size="sm"
                        className="h-7 text-xs"
                        onClick={() => void installFromCatalog(builtin.metadata.id)}
                        disabled={loading || builtin.builtIn?.installable === false}
                      >
                        <Download className="mr-1 size-3" />
                        {t("install")}
                      </Button>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </TabsContent>

          {/* --- Marketplace --- */}
          <TabsContent value="marketplace" className="mt-4">
            {filteredMarketplace.length === 0 ? (
              <EmptyState
                icon={Puzzle}
                title={loading ? t("loadingMarketplace") : t("noMarketplaceMatch")}
              />
            ) : (
              <div className="grid gap-3 sm:grid-cols-2 2xl:grid-cols-3">
                {filteredMarketplace.map((entry) => (
                  <div
                    key={entry.id}
                    className="flex flex-col gap-3 rounded-xl border border-border/50 bg-card p-4"
                  >
                    <div className="flex items-start justify-between gap-2">
                      <div className="min-w-0">
                        <h3 className="truncate text-sm font-semibold">{entry.name}</h3>
                        <p className="text-xs text-muted-foreground">v{entry.version} · {entry.author}</p>
                      </div>
                      <Badge variant="outline" className="text-[10px] px-1.5 py-0 shrink-0">
                        {entry.kind}
                      </Badge>
                    </div>
                    <p className="flex-1 text-xs leading-relaxed text-muted-foreground line-clamp-2">
                      {entry.description}
                    </p>
                    <div className="mt-auto flex items-center gap-2">
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
                          className="h-7 text-xs"
                          onClick={() => void installFromCatalog(entry.id)}
                          disabled={loading}
                        >
                          <Download className="mr-1 size-3" />
                          {t("install")}
                        </Button>
                      ) : (
                        <>
                          <Badge variant="secondary">{t("browseOnly")}</Badge>
                          <span className="text-[11px] text-muted-foreground">{t("remoteInstallNotReady")}</span>
                        </>
                      )}
                    </div>
                  </div>
                ))}
              </div>
            )}
          </TabsContent>

          {/* --- Remote Registry --- */}
          <TabsContent value="remote" className="mt-4">
            {remoteMarketplace.registry ? (
              <p className="mb-3 text-xs text-muted-foreground">
                {t("remoteRegistrySource")}: {remoteMarketplace.registry}
              </p>
            ) : null}
            {!remoteMarketplace.available && remoteMarketplace.error ? (
              <div className="rounded-lg border border-border/60 bg-muted/30 px-4 py-3 text-sm text-muted-foreground">
                {remoteMarketplace.error}
              </div>
            ) : filteredRemoteMarketplace.length === 0 ? (
              <EmptyState
                icon={Puzzle}
                title={loading ? t("loadingRemoteRegistry") : t("noRemoteRegistryMatch")}
              />
            ) : (
              <div className="grid gap-3 sm:grid-cols-2 2xl:grid-cols-3">
                {filteredRemoteMarketplace.map((entry) => (
                  <div
                    key={entry.id}
                    className="flex flex-col gap-3 rounded-xl border border-border/50 bg-card p-4"
                  >
                    <div className="flex items-start justify-between gap-2">
                      <div className="min-w-0">
                        <h3 className="truncate text-sm font-semibold">{entry.name}</h3>
                        <p className="text-xs text-muted-foreground">v{entry.version} · {entry.author}</p>
                      </div>
                      <Badge variant="outline" className="text-[10px] px-1.5 py-0 shrink-0">
                        {entry.kind}
                      </Badge>
                    </div>
                    <p className="flex-1 text-xs leading-relaxed text-muted-foreground line-clamp-2">
                      {entry.description}
                    </p>
                    <div className="mt-auto flex items-center gap-2">
                      {entry.installed ? (
                        <Badge className="bg-emerald-500/15 text-emerald-700 dark:text-emerald-400">
                          {t("installed")}
                        </Badge>
                      ) : entry.installable === false ? (
                        <>
                          <Badge variant="secondary">{t("browseOnly")}</Badge>
                          {entry.blockedReason ? (
                            <span className="text-[11px] text-muted-foreground">{entry.blockedReason}</span>
                          ) : null}
                        </>
                      ) : (
                        <Button
                          variant="outline"
                          size="sm"
                          className="h-7 text-xs"
                          onClick={() => void installFromRemote(entry.id, entry.version)}
                          disabled={loading}
                        >
                          <Download className="mr-1 size-3" />
                          {t("installRemote")}
                        </Button>
                      )}
                    </div>
                  </div>
                ))}
              </div>
            )}
          </TabsContent>
        </Tabs>

        {/* Desktop detail sidebar */}
        <div className="hidden xl:block">
          <PluginDetailSidebar plugin={selectedPlugin} />
        </div>

        {/* Mobile detail sheet */}
        <Sheet open={detailSheetOpen} onOpenChange={setDetailSheetOpen}>
          <SheetContent side="right" className="w-full sm:max-w-md xl:hidden overflow-y-auto">
            <SheetHeader>
              <SheetTitle>{t("detailSidebar.title")}</SheetTitle>
            </SheetHeader>
            <div className="mt-4">
              <PluginDetailSidebar plugin={selectedPlugin} />
            </div>
          </SheetContent>
        </Sheet>
      </div>

      {/* ===== Dialogs ===== */}
      <PluginInstallDialog open={installOpen} onOpenChange={setInstallOpen} />
      <PluginConfigDialog plugin={configPlugin} open={configOpen} onOpenChange={setConfigOpen} />
      <PluginInvokeDialog plugin={invokePlugin} open={invokeOpen} onOpenChange={setInvokeOpen} />
    </div>
  );
}
