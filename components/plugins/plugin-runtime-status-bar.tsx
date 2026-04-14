"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { cn } from "@/lib/utils";
import type {
  DesktopRuntimeStatus,
  DesktopUpdateInfo,
  DesktopUpdateProgress,
  PlatformUpdateResult,
  PluginRuntimeSummary,
} from "@/lib/platform-runtime";
import {
  BellRing,
  ChevronRight,
  MonitorCog,
} from "lucide-react";

function renderRuntimeTone(
  status: DesktopRuntimeStatus["overall"],
): "default" | "destructive" | "secondary" {
  if (status === "ready") return "default";
  if (status === "degraded") return "destructive";
  return "secondary";
}

const statusDotColor: Record<string, string> = {
  ready: "bg-emerald-500",
  running: "bg-emerald-500",
  degraded: "bg-orange-500",
  stopped: "bg-zinc-400",
};

interface PluginRuntimeStatusBarProps {
  isDesktop: boolean;
  desktopRuntime: DesktopRuntimeStatus;
  pluginRuntimeSummary: PluginRuntimeSummary;
  lastDesktopEvent: string | null;
  desktopMessage: string | null;
  desktopUpdate: PlatformUpdateResult | null;
  desktopUpdateProgress: DesktopUpdateProgress | null;
  onTraySync: () => void;
  onCheckUpdate: () => void;
  onInstallUpdate: () => void;
  onRelaunchToUpdate: () => void;
  onNotification: () => void;
}

export function PluginRuntimeStatusBar({
  isDesktop,
  desktopRuntime,
  pluginRuntimeSummary,
  lastDesktopEvent,
  desktopMessage,
  desktopUpdate,
  desktopUpdateProgress,
  onTraySync,
  onCheckUpdate,
  onInstallUpdate,
  onRelaunchToUpdate,
  onNotification,
}: PluginRuntimeStatusBarProps) {
  const t = useTranslations("plugins");
  const [sheetOpen, setSheetOpen] = useState(false);

  const activeDesktopUpdate: DesktopUpdateInfo | null =
    desktopUpdate &&
    desktopUpdate.ok &&
    "update" in desktopUpdate &&
    desktopUpdate.update
      ? desktopUpdate.update
      : null;

  const units = [
    { label: "Backend", status: desktopRuntime.backend.status },
    { label: "Bridge", status: desktopRuntime.bridge.status },
    { label: "IM", status: desktopRuntime.imBridge.status },
  ];

  return (
    <>
      <div className="flex h-9 items-center gap-3 rounded-lg border border-border/60 bg-card px-3">
        <MonitorCog className="size-3.5 text-muted-foreground" />
        <span className="text-xs font-medium">{t("desktopRuntime")}</span>
        <Badge
          variant={renderRuntimeTone(desktopRuntime.overall)}
          className="h-5 text-[10px]"
        >
          {isDesktop ? desktopRuntime.overall : t("webFallback")}
        </Badge>

        <div className="mx-1 h-4 w-px bg-border" />

        {units.map((u) => (
          <div key={u.label} className="flex items-center gap-1.5">
            <span
              className={cn(
                "size-1.5 rounded-full",
                statusDotColor[u.status] ?? "bg-zinc-400",
              )}
            />
            <span className="text-[11px] text-muted-foreground">{u.label}</span>
          </div>
        ))}

        <div className="ml-auto flex items-center gap-1">
          <span className="text-[11px] text-muted-foreground">
            {pluginRuntimeSummary.bridgePluginCount} plugins
          </span>
          <Button
            variant="ghost"
            size="sm"
            className="h-6 w-6 p-0"
            aria-label={t("showRuntime")}
            onClick={() => setSheetOpen(true)}
          >
            <ChevronRight className="size-3.5" />
          </Button>
        </div>
      </div>

      <Sheet open={sheetOpen} onOpenChange={setSheetOpen}>
        <SheetContent side="right" className="w-full sm:max-w-lg overflow-y-auto">
          <SheetHeader>
            <SheetTitle>{t("desktopRuntime")}</SheetTitle>
          </SheetHeader>

          <div className="mt-4 space-y-4">
            <p className="text-xs text-muted-foreground">
              {t("desktopRuntimeDesc")}
            </p>

            <div className="grid gap-3">
              {[
                desktopRuntime.backend,
                desktopRuntime.bridge,
                desktopRuntime.imBridge,
              ].map((unit) => (
                <div
                  key={unit.label}
                  className="rounded-lg border border-border/60 p-3 text-sm"
                >
                  <div className="flex items-center justify-between gap-2">
                    <p className="font-medium capitalize">{unit.label}</p>
                    <Badge variant={renderRuntimeTone(unit.status)}>
                      {unit.status}
                    </Badge>
                  </div>
                  <div className="mt-3 grid gap-2 text-xs text-muted-foreground">
                    <p>
                      {t("url")}: {unit.url ?? t("urlUnavailable")}
                    </p>
                    <p>
                      {t("pid")}: {unit.pid ?? t("pidNotRunning")}
                    </p>
                    <p>
                      {t("restartCount")}: {unit.restartCount}
                    </p>
                    <p>
                      {t("lastStart")}:{" "}
                      {unit.lastStartedAt ?? t("lastStartNone")}
                    </p>
                    <p>
                      {t("lastError")}:{" "}
                      {unit.lastError ?? t("lastErrorNone")}
                    </p>
                  </div>
                </div>
              ))}
            </div>

            <div className="rounded-lg border border-border/60 p-3 text-sm">
              <p className="font-medium">{t("helperSummary")}</p>
              <div className="mt-2 grid gap-1 text-xs text-muted-foreground">
                <p>
                  {t("bridgePlugins")}:{" "}
                  {pluginRuntimeSummary.bridgePluginCount}
                </p>
                <p>
                  {t("activeBridgeRuntimes")}:{" "}
                  {pluginRuntimeSummary.activeRuntimeCount}
                </p>
                <p>
                  {t("eventBridge")}:{" "}
                  {pluginRuntimeSummary.eventBridgeAvailable
                    ? t("eventBridgeAvailable")
                    : t("eventBridgeUnavailable")}
                </p>
                <p>
                  {t("lastDesktopEvent")}:{" "}
                  {lastDesktopEvent ?? t("lastDesktopEventNone")}
                </p>
              </div>

              {pluginRuntimeSummary.warnings.length > 0 ? (
                <div className="mt-2 rounded-md border border-border/60 bg-muted/40 px-3 py-2 text-xs text-muted-foreground">
                  {pluginRuntimeSummary.warnings.join(" ")}
                </div>
              ) : null}
            </div>

            <div className="flex flex-wrap gap-2">
              <Button variant="outline" size="sm" onClick={onTraySync}>
                {t("syncTray")}
              </Button>
              <Button variant="outline" size="sm" onClick={onCheckUpdate}>
                {t("checkUpdate")}
              </Button>
              {desktopUpdate?.ok && desktopUpdate.status === "available" ? (
                <Button variant="outline" size="sm" onClick={onInstallUpdate}>
                  {t("installUpdate")}
                </Button>
              ) : null}
              {desktopUpdate?.ok &&
              desktopUpdate.status === "ready_to_relaunch" ? (
                <Button
                  variant="outline"
                  size="sm"
                  onClick={onRelaunchToUpdate}
                >
                  {t("restartToUpdate")}
                </Button>
              ) : null}
              <Button variant="outline" size="sm" onClick={onNotification}>
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
                  {desktopUpdate?.ok &&
                  desktopUpdate.status === "ready_to_relaunch"
                    ? t("updateInstalled", {
                        version: activeDesktopUpdate.version,
                      })
                    : t("updateReady", {
                        version: activeDesktopUpdate.version,
                      })}
                </p>
                <p className="mt-1">
                  {t("currentVersion", {
                    version:
                      activeDesktopUpdate.currentVersion ?? "Unknown",
                  })}
                </p>
                {activeDesktopUpdate.publishedAt ? (
                  <p className="mt-1">
                    {t("publishedAt", {
                      date: activeDesktopUpdate.publishedAt,
                    })}
                  </p>
                ) : null}
                {activeDesktopUpdate.notes ? (
                  <p className="mt-1">{activeDesktopUpdate.notes}</p>
                ) : null}
                {desktopUpdateProgress ? (
                  <p className="mt-1">
                    {desktopUpdateProgress.phase === "downloading"
                      ? t("downloading", {
                          downloaded:
                            desktopUpdateProgress.downloadedBytes,
                          total:
                            desktopUpdateProgress.totalBytes ?? "unknown",
                        })
                      : t("installing")}
                  </p>
                ) : null}
              </div>
            ) : null}
          </div>
        </SheetContent>
      </Sheet>
    </>
  );
}
