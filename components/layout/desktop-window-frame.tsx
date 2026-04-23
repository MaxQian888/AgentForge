"use client";

import { useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { Copy, Minus, Square, X } from "lucide-react";
import { cn } from "@/lib/utils";
import {
  type DesktopWindowChromeState,
} from "@/lib/platform-runtime";
import { usePlatformCapability } from "@/hooks/use-platform-capability";

function defaultWindowState(): DesktopWindowChromeState {
  return {
    focused: true,
    maximized: false,
    minimized: false,
    visible: true,
  };
}

export function DesktopWindowFrame({
  children,
}: {
  children: React.ReactNode;
}) {
  const t = useTranslations("common");
  const {
    isDesktop,
    closeMainWindow,
    getWindowChromeState,
    minimizeMainWindow,
    subscribeWindowChromeState,
    toggleMaximizeMainWindow,
  } = usePlatformCapability();
  const [windowState, setWindowState] = useState<DesktopWindowChromeState>(
    defaultWindowState,
  );
  const effectiveWindowState = isDesktop ? windowState : defaultWindowState();

  useEffect(() => {
    if (!isDesktop) {
      return;
    }

    let disposed = false;
    let cleanupRef = () => {};

    void getWindowChromeState().then((state) => {
      if (!disposed) {
        setWindowState(state);
      }
    });

    void subscribeWindowChromeState((state) => {
      if (!disposed) {
        setWindowState(state);
      }
    }).then((cleanup) => {
      if (disposed) {
        cleanup();
        return;
      }

      cleanupRef = cleanup;
    });

    return () => {
      disposed = true;
      cleanupRef();
    };
  }, [getWindowChromeState, isDesktop, subscribeWindowChromeState]);

  return (
    <div
      data-slot="desktop-window-frame"
      className={cn(
        "flex min-h-screen flex-col bg-background",
        isDesktop && "h-screen overflow-hidden",
      )}
    >
      {isDesktop ? (
        <div
          data-slot="desktop-window-titlebar"
          className="flex h-10 items-center border-b bg-background/95 pl-3 pr-1 backdrop-blur"
        >
          <div
            data-tauri-drag-region
            className="flex min-w-0 flex-1 items-center gap-3 overflow-hidden"
          >
            <div className="truncate text-sm font-semibold">AgentForge</div>
            <div className="truncate text-xs text-muted-foreground">
              {t("desktop.workspace")}
            </div>
          </div>
          <div
            data-desktop-no-drag="true"
            className="flex items-center gap-1 pl-3"
          >
            <button
              type="button"
              aria-label={t("desktop.minimizeWindow")}
              title={t("desktop.minimizeWindow")}
              className="inline-flex h-8 w-8 items-center justify-center rounded-md text-muted-foreground transition hover:bg-accent hover:text-accent-foreground"
              onClick={() => {
                void minimizeMainWindow();
              }}
            >
              <Minus className="size-4" />
            </button>
            <button
              type="button"
              aria-label={
                effectiveWindowState.maximized
                  ? t("desktop.restoreWindow")
                  : t("desktop.maximizeWindow")
              }
              title={
                effectiveWindowState.maximized
                  ? t("desktop.restoreWindow")
                  : t("desktop.maximizeWindow")
              }
              className="inline-flex h-8 w-8 items-center justify-center rounded-md text-muted-foreground transition hover:bg-accent hover:text-accent-foreground"
              onClick={() => {
                void toggleMaximizeMainWindow();
              }}
            >
              {effectiveWindowState.maximized ? (
                <Copy className="size-3.5" />
              ) : (
                <Square className="size-3.5" />
              )}
            </button>
            <button
              type="button"
              aria-label={t("desktop.closeWindow")}
              title={t("desktop.closeWindow")}
              className="inline-flex h-8 w-8 items-center justify-center rounded-md text-muted-foreground transition hover:bg-destructive/15 hover:text-destructive"
              onClick={() => {
                void closeMainWindow();
              }}
            >
              <X className="size-4" />
            </button>
          </div>
        </div>
      ) : null}
      <div
        data-slot="desktop-window-content"
        className={cn("flex-1", isDesktop && "min-h-0 overflow-hidden")}
      >
        {children}
      </div>
    </div>
  );
}
