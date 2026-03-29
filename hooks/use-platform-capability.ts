"use client";

import { platformRuntime } from "@/lib/platform-runtime";

export function usePlatformCapability() {
  return {
    isDesktop: platformRuntime.isDesktop,
    selectFiles: platformRuntime.selectFiles,
    sendNotification: platformRuntime.sendNotification.bind(platformRuntime),
    syncNotificationTraySummary: platformRuntime.syncNotificationTraySummary,
    updateTray: platformRuntime.updateTray,
    registerShortcut: platformRuntime.registerShortcut,
    performShellAction: platformRuntime.performShellAction,
    closeMainWindow: platformRuntime.closeMainWindow.bind(platformRuntime),
    focusMainWindow: platformRuntime.focusMainWindow.bind(platformRuntime),
    getWindowChromeState: platformRuntime.getWindowChromeState,
    minimizeMainWindow: platformRuntime.minimizeMainWindow.bind(platformRuntime),
    restoreMainWindow: platformRuntime.restoreMainWindow.bind(platformRuntime),
    showMainWindow: platformRuntime.showMainWindow.bind(platformRuntime),
    subscribeWindowChromeState:
      platformRuntime.subscribeWindowChromeState.bind(platformRuntime),
    toggleMaximizeMainWindow:
      platformRuntime.toggleMaximizeMainWindow.bind(platformRuntime),
    checkForUpdate: platformRuntime.checkForUpdate,
    installUpdate: platformRuntime.installUpdate,
    relaunchToUpdate: platformRuntime.relaunchToUpdate,
    getDesktopRuntimeStatus: platformRuntime.getDesktopRuntimeStatus,
    getPluginRuntimeSummary: platformRuntime.getPluginRuntimeSummary,
    subscribeDesktopEvents: platformRuntime.subscribeDesktopEvents,
  };
}
