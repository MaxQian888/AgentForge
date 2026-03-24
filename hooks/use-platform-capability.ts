"use client";

import { platformRuntime } from "@/lib/platform-runtime";

export function usePlatformCapability() {
  return {
    isDesktop: platformRuntime.isDesktop,
    selectFiles: platformRuntime.selectFiles,
    sendNotification: platformRuntime.sendNotification,
    updateTray: platformRuntime.updateTray,
    registerShortcut: platformRuntime.registerShortcut,
    checkForUpdate: platformRuntime.checkForUpdate,
    getDesktopRuntimeStatus: platformRuntime.getDesktopRuntimeStatus,
    subscribeDesktopEvents: platformRuntime.subscribeDesktopEvents,
  };
}
