"use client";

import { platformRuntime } from "@/lib/platform-runtime";

export function usePlatformCapability() {
  return {
    isDesktop: platformRuntime.isDesktop,
    selectFiles: platformRuntime.selectFiles,
    sendNotification: platformRuntime.sendNotification,
    syncNotificationTraySummary: platformRuntime.syncNotificationTraySummary,
    updateTray: platformRuntime.updateTray,
    registerShortcut: platformRuntime.registerShortcut,
    checkForUpdate: platformRuntime.checkForUpdate,
    installUpdate: platformRuntime.installUpdate,
    relaunchToUpdate: platformRuntime.relaunchToUpdate,
    getDesktopRuntimeStatus: platformRuntime.getDesktopRuntimeStatus,
    getPluginRuntimeSummary: platformRuntime.getPluginRuntimeSummary,
    subscribeDesktopEvents: platformRuntime.subscribeDesktopEvents,
  };
}
