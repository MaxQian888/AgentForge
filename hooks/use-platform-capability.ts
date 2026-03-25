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
    getPluginRuntimeSummary: platformRuntime.getPluginRuntimeSummary,
    subscribeDesktopEvents: platformRuntime.subscribeDesktopEvents,
  };
}
