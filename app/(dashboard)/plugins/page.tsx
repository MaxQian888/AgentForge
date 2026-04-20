"use client";

import { useCallback, useEffect, useEffectEvent, useMemo, useState } from "react";
import { useTranslations } from "next-intl";
import { Puzzle } from "lucide-react";
import {
  ResizableHandle,
  ResizablePanel,
  ResizablePanelGroup,
} from "@/components/ui/resizable";
import { ScrollArea } from "@/components/ui/scroll-area";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { PluginDetailPanel } from "@/components/plugins/plugin-detail-panel";
import { PluginInstallDialog } from "@/components/plugins/plugin-install-dialog";
import { PluginConfigDialog } from "@/components/plugins/plugin-config-dialog";
import { PluginInvokeDialog } from "@/components/plugins/plugin-invoke-dialog";
import { PluginListItem } from "@/components/plugins/plugin-list-item";
import { PluginMarketplaceListItem } from "@/components/plugins/plugin-marketplace-list-item";
import { PluginRuntimeStatusBar } from "@/components/plugins/plugin-runtime-status-bar";
import { PluginSearchBar } from "@/components/plugins/plugin-search-bar";
import { ErrorBanner } from "@/components/shared/error-banner";
import { useBreakpoint } from "@/hooks/use-breakpoint";
import { usePlatformCapability } from "@/hooks/use-platform-capability";
import type {
  DesktopUpdateProgress,
  DesktopRuntimeStatus,
  PlatformUpdateResult,
  PluginRuntimeSummary,
} from "@/lib/platform-runtime";
import {
  filterMarketplaceEntries,
  filterPluginRecords,
  usePluginStore,
  type MarketplacePluginEntry,
  type PluginRecord,
} from "@/lib/stores/plugin-store";
import { useBreadcrumbs } from "@/hooks/use-breadcrumbs";

const EMPTY_RUNTIME_STATUS: DesktopRuntimeStatus = {
  overall: "stopped",
  backend: { label: "backend", status: "stopped", url: null, pid: null, restartCount: 0, lastError: null, lastStartedAt: null },
  bridge: { label: "bridge", status: "stopped", url: null, pid: null, restartCount: 0, lastError: null, lastStartedAt: null },
  imBridge: { label: "im-bridge", status: "stopped", url: null, pid: null, restartCount: 0, lastError: null, lastStartedAt: null },
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

export default function PluginsPage() {
  useBreadcrumbs([{ label: "Configuration", href: "/" }, { label: "Plugins" }]);
  const t = useTranslations("plugins");
  const { isDesktop: isDesktopBreakpoint } = useBreakpoint();

  /* ── Store ── */
  const plugins = usePluginStore((s) => s.plugins);
  const builtins = usePluginStore((s) => s.builtins);
  const marketplace = usePluginStore((s) => s.marketplace);
  const remoteMarketplace = usePluginStore((s) => s.remoteMarketplace);
  const filters = usePluginStore((s) => s.filters);
  const viewCategory = usePluginStore((s) => s.viewCategory);
  const selectedPluginId = usePluginStore((s) => s.selectedPluginId);
  const loading = usePluginStore((s) => s.loading);
  const error = usePluginStore((s) => s.error);
  const fetchPlugins = usePluginStore((s) => s.fetchPlugins);
  const discoverBuiltins = usePluginStore((s) => s.discoverBuiltins);
  const fetchMarketplace = usePluginStore((s) => s.fetchMarketplace);
  const fetchRemoteMarketplace = usePluginStore((s) => s.fetchRemoteMarketplace);
  const installFromCatalog = usePluginStore((s) => s.installFromCatalog);
  const installFromRemote = usePluginStore((s) => s.installFromRemote);
  const selectPlugin = usePluginStore((s) => s.selectPlugin);
  const selectedMarketplaceId = usePluginStore((s) => s.selectedMarketplaceId);
  const selectMarketplaceEntry = usePluginStore((s) => s.selectMarketplaceEntry);

  /* ── Platform capability ── */
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

  /* ── Local UI state ── */
  const [installOpen, setInstallOpen] = useState(false);
  const [configPlugin, setConfigPlugin] = useState<PluginRecord | null>(null);
  const [configOpen, setConfigOpen] = useState(false);
  const [invokePlugin, setInvokePlugin] = useState<PluginRecord | null>(null);
  const [invokeOpen, setInvokeOpen] = useState(false);
  const [detailSheetOpen, setDetailSheetOpen] = useState(false);

  /* ── Desktop runtime state ── */
  const [desktopRuntime, setDesktopRuntime] = useState<DesktopRuntimeStatus>(EMPTY_RUNTIME_STATUS);
  const [pluginRuntimeSummary, setPluginRuntimeSummary] = useState<PluginRuntimeSummary>(EMPTY_PLUGIN_RUNTIME_SUMMARY);
  const [desktopMessage, setDesktopMessage] = useState<string | null>(null);
  const [lastDesktopEvent, setLastDesktopEvent] = useState<string | null>(null);
  const [desktopUpdate, setDesktopUpdate] = useState<PlatformUpdateResult | null>(null);
  const [desktopUpdateProgress, setDesktopUpdateProgress] = useState<DesktopUpdateProgress | null>(null);

  /* ── Data fetching ── */
  useEffect(() => {
    void fetchPlugins();
    void discoverBuiltins();
    void fetchMarketplace();
    void fetchRemoteMarketplace();
  }, [fetchPlugins, discoverBuiltins, fetchMarketplace, fetchRemoteMarketplace]);

  const loadDesktopState = useEffectEvent(async () => {
    if (!isDesktop) return;
    const [runtimeStatus, summary] = await Promise.all([
      getDesktopRuntimeStatus(),
      getPluginRuntimeSummary(),
    ]);
    setDesktopRuntime(runtimeStatus);
    setPluginRuntimeSummary(summary);
  });

  useEffect(() => { void loadDesktopState(); }, []);

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
        setPluginRuntimeSummary((c) => ({ ...c, eventBridgeAvailable: true, lastUpdatedAt: event.timestamp ?? c.lastUpdatedAt }));
        void fetchPlugins();
        return;
      }
      if (event.type === "shell.action") {
        setPluginRuntimeSummary((c) => ({ ...c, eventBridgeAvailable: true, lastUpdatedAt: event.timestamp ?? c.lastUpdatedAt }));
      }
    }).then((cleanup) => {
      if (disposed) { cleanup(); return; }
      cleanupRef = cleanup;
    });
    let cleanupRef = () => {};
    return () => { disposed = true; cleanupRef(); };
  }, [fetchPlugins, isDesktop, subscribeDesktopEvents]);

  /* ── Desktop handlers ── */
  const handleDesktopNotification = useCallback(async () => {
    const result = await sendNotification({
      notificationId: `plugins-desktop-runtime-${desktopRuntime.overall}`,
      notificationType: "desktop.runtime.status",
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
      tooltip: `Backend ${desktopRuntime.backend.status} / Bridge ${desktopRuntime.bridge.status} / IM Bridge ${desktopRuntime.imBridge.status}`,
      visible: true,
    });
    setDesktopMessage(result.ok ? `Tray state synced through ${result.mode} mode.` : result.error);
  }, [desktopRuntime.backend.status, desktopRuntime.bridge.status, desktopRuntime.imBridge.status, desktopRuntime.overall, updateTray]);

  const handleUpdateCheck = useCallback(async () => {
    const result = await checkForUpdate();
    setDesktopUpdate(result);
    setDesktopUpdateProgress(null);
    setDesktopMessage(!result.ok ? result.error : result.status === "available" ? `Desktop update metadata refreshed in ${result.mode} mode.` : "Desktop app is already up to date.");
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

  /* ── Callbacks ── */
  const handleConfigure = useCallback((plugin: PluginRecord) => {
    setConfigPlugin(plugin);
    setConfigOpen(true);
  }, []);

  const handleInvoke = useCallback((plugin: PluginRecord) => {
    setInvokePlugin(plugin);
    setInvokeOpen(true);
  }, []);

  const handleRefresh = useCallback(() => {
    void fetchPlugins();
    void discoverBuiltins();
    void fetchMarketplace();
    void fetchRemoteMarketplace();
  }, [fetchPlugins, discoverBuiltins, fetchMarketplace, fetchRemoteMarketplace]);

  /* ── Filtering ── */
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

  /* ── Selection ── */
  const selectedPlugin = useMemo(
    () => filteredInstalled.find((p) => p.metadata.id === selectedPluginId) ?? filteredInstalled[0] ?? null,
    [filteredInstalled, selectedPluginId],
  );

  const allMarketplaceEntries = useMemo(() => {
    const builtinEntries: MarketplacePluginEntry[] = filteredBuiltins.map((b) => ({
      id: b.metadata.id,
      name: b.metadata.name,
      description: b.metadata.description ?? "",
      version: b.metadata.version,
      author: "Built-in",
      kind: b.kind,
      installed: false,
      installable: b.builtIn?.installable !== false,
      blockedReason: b.builtIn?.installBlockedReason,
      sourceType: "builtin" as const,
      runtime: b.spec.runtime,
      builtIn: b.builtIn,
    }));
    return [...builtinEntries, ...filteredMarketplace, ...filteredRemoteMarketplace];
  }, [filteredBuiltins, filteredMarketplace, filteredRemoteMarketplace]);

  const selectedMarketplaceEntry = useMemo(
    () => allMarketplaceEntries.find((e) => e.id === selectedMarketplaceId) ?? null,
    [allMarketplaceEntries, selectedMarketplaceId],
  );

  useEffect(() => {
    if (viewCategory !== "installed") return;
    if (!selectedPluginId && filteredInstalled[0]) {
      selectPlugin(filteredInstalled[0].metadata.id);
      return;
    }
    if (selectedPluginId && filteredInstalled.length > 0 && !filteredInstalled.some((p) => p.metadata.id === selectedPluginId)) {
      selectPlugin(filteredInstalled[0].metadata.id);
    }
  }, [filteredInstalled, selectPlugin, selectedPluginId, viewCategory]);

  const handleSelectPlugin = useCallback(
    (plugin: PluginRecord) => {
      selectPlugin(plugin.metadata.id);
      if (!isDesktopBreakpoint) setDetailSheetOpen(true);
    },
    [selectPlugin, isDesktopBreakpoint],
  );

  const handleSelectMarketplaceEntry = useCallback(
    (entry: MarketplacePluginEntry) => {
      selectMarketplaceEntry(entry.id);
      if (!isDesktopBreakpoint) setDetailSheetOpen(true);
    },
    [selectMarketplaceEntry, isDesktopBreakpoint],
  );

  const handleMarketplaceInstall = useCallback(
    (entry: MarketplacePluginEntry) => {
      if (entry.sourceType === "catalog" || entry.sourceType === "builtin") {
        void installFromCatalog(entry.id);
      } else {
        void installFromRemote(entry.id, entry.version);
      }
    },
    [installFromCatalog, installFromRemote],
  );

  /* ── List rendering helpers ── */
  const renderListContent = () => {
    if (viewCategory === "installed") {
      if (filteredInstalled.length === 0) {
        return (
          <div className="flex flex-col items-center justify-center gap-2 py-12 text-muted-foreground">
            <Puzzle className="size-8 opacity-20" />
            <p className="text-xs">
              {loading ? t("loadingPlugins") : plugins.length === 0 ? t("noInstalledPlugins") : t("noInstalledMatch")}
            </p>
          </div>
        );
      }
      return filteredInstalled.map((plugin) => (
        <PluginListItem
          key={plugin.metadata.id}
          plugin={plugin}
          onConfigure={handleConfigure}
          onInvoke={handleInvoke}
          onSelect={handleSelectPlugin}
          selected={selectedPlugin?.metadata.id === plugin.metadata.id}
        />
      ));
    }

    if (viewCategory === "builtin") {
      if (filteredBuiltins.length === 0) {
        return (
          <div className="flex flex-col items-center justify-center gap-2 py-12 text-muted-foreground">
            <Puzzle className="size-8 opacity-20" />
            <p className="text-xs">{loading ? t("discoveringBuiltins") : t("noBuiltinMatch")}</p>
          </div>
        );
      }
      return filteredBuiltins.map((builtin) => {
        const entry: MarketplacePluginEntry = {
          id: builtin.metadata.id,
          name: builtin.metadata.name,
          description: builtin.metadata.description ?? "",
          version: builtin.metadata.version,
          author: "Built-in",
          kind: builtin.kind,
          installed: false,
          installable: builtin.builtIn?.installable !== false,
          blockedReason: builtin.builtIn?.installBlockedReason,
          sourceType: "builtin",
          runtime: builtin.spec.runtime,
          builtIn: builtin.builtIn,
        };
        return (
          <PluginMarketplaceListItem
            key={builtin.metadata.id}
            entry={entry}
            onInstall={() => void installFromCatalog(builtin.metadata.id)}
            onSelect={handleSelectMarketplaceEntry}
            selected={selectedMarketplaceId === builtin.metadata.id}
            loading={loading}
          />
        );
      });
    }

    if (viewCategory === "marketplace") {
      if (filteredMarketplace.length === 0) {
        return (
          <div className="flex flex-col items-center justify-center gap-2 py-12 text-muted-foreground">
            <Puzzle className="size-8 opacity-20" />
            <p className="text-xs">{loading ? t("loadingMarketplace") : t("noMarketplaceMatch")}</p>
          </div>
        );
      }
      return filteredMarketplace.map((entry) => (
        <PluginMarketplaceListItem
          key={entry.id}
          entry={entry}
          onInstall={handleMarketplaceInstall}
          onSelect={handleSelectMarketplaceEntry}
          selected={selectedMarketplaceId === entry.id}
          loading={loading}
        />
      ));
    }

    // remote
    if (!remoteMarketplace.available && remoteMarketplace.error) {
      return (
        <div className="px-3 py-4 text-xs text-muted-foreground">
          {remoteMarketplace.error}
        </div>
      );
    }
    if (filteredRemoteMarketplace.length === 0) {
      return (
        <div className="flex flex-col items-center justify-center gap-2 py-12 text-muted-foreground">
          <Puzzle className="size-8 opacity-20" />
          <p className="text-xs">{loading ? t("loadingRemoteRegistry") : t("noRemoteRegistryMatch")}</p>
        </div>
      );
    }
    return filteredRemoteMarketplace.map((entry) => (
      <PluginMarketplaceListItem
        key={entry.id}
        entry={entry}
        onInstall={(e) => void installFromRemote(e.id, e.version)}
        onSelect={handleSelectMarketplaceEntry}
        selected={selectedMarketplaceId === entry.id}
        loading={loading}
      />
    ));
  };

  return (
    <div className="flex h-[calc(100vh-var(--header-height))] flex-col gap-[var(--space-stack-sm)]">
      {error ? <ErrorBanner message={error} /> : null}

      {/* ── Runtime status bar ── */}
      <PluginRuntimeStatusBar
        isDesktop={isDesktop}
        desktopRuntime={desktopRuntime}
        pluginRuntimeSummary={pluginRuntimeSummary}
        lastDesktopEvent={lastDesktopEvent}
        desktopMessage={desktopMessage}
        desktopUpdate={desktopUpdate}
        desktopUpdateProgress={desktopUpdateProgress}
        onTraySync={() => void handleTraySync()}
        onCheckUpdate={() => void handleUpdateCheck()}
        onInstallUpdate={() => void handleInstallUpdate()}
        onRelaunchToUpdate={() => void handleRelaunchToUpdate()}
        onNotification={() => void handleDesktopNotification()}
      />

      {/* ── Search + category chips ── */}
      <PluginSearchBar
        installedCount={filteredInstalled.length}
        builtinCount={filteredBuiltins.length}
        marketplaceCount={filteredMarketplace.length}
        remoteCount={filteredRemoteMarketplace.length}
        loading={loading}
        onRefresh={handleRefresh}
        onInstallLocal={() => setInstallOpen(true)}
      />

      {/* ── Two-panel layout (desktop) / single list (mobile) ── */}
      {isDesktopBreakpoint ? (
        <div className="min-h-0 flex-1 rounded-lg border">
          <ResizablePanelGroup orientation="horizontal">
            <ResizablePanel defaultSize={35} minSize={25} maxSize={50}>
              <ScrollArea className="h-full">
                <div className="divide-y divide-border/40">
                  {renderListContent()}
                </div>
              </ScrollArea>
            </ResizablePanel>

            <ResizableHandle withHandle />

            <ResizablePanel defaultSize={65} minSize={40}>
              <PluginDetailPanel
                plugin={viewCategory === "installed" ? selectedPlugin : null}
                marketplaceEntry={viewCategory !== "installed" ? selectedMarketplaceEntry : null}
                onConfigure={handleConfigure}
                onInvoke={handleInvoke}
                onInstallEntry={handleMarketplaceInstall}
                loading={loading}
              />
            </ResizablePanel>
          </ResizablePanelGroup>
        </div>
      ) : (
        <>
          <ScrollArea className="min-h-0 flex-1 rounded-lg border">
            <div className="divide-y divide-border/40">
              {renderListContent()}
            </div>
          </ScrollArea>

          <Sheet open={detailSheetOpen} onOpenChange={setDetailSheetOpen}>
            <SheetContent side="right" className="w-full sm:max-w-lg p-0 overflow-hidden">
              <SheetHeader className="px-4 pt-4">
                <SheetTitle>{t("detailSidebar.title")}</SheetTitle>
              </SheetHeader>
              <PluginDetailPanel
                plugin={viewCategory === "installed" ? selectedPlugin : null}
                marketplaceEntry={viewCategory !== "installed" ? selectedMarketplaceEntry : null}
                onConfigure={handleConfigure}
                onInvoke={handleInvoke}
                onInstallEntry={handleMarketplaceInstall}
                loading={loading}
              />
            </SheetContent>
          </Sheet>
        </>
      )}

      {/* ── Dialogs ── */}
      <PluginInstallDialog open={installOpen} onOpenChange={setInstallOpen} />
      <PluginConfigDialog plugin={configPlugin} open={configOpen} onOpenChange={setConfigOpen} />
      <PluginInvokeDialog plugin={invokePlugin} open={invokeOpen} onOpenChange={setInvokeOpen} />
    </div>
  );
}
