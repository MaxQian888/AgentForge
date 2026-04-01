/** @jest-environment jsdom */

import { createPlatformRuntime } from "./platform-runtime";

describe("platform-runtime", () => {
  it("falls back to the default backend URL outside desktop mode", async () => {
    const runtime = createPlatformRuntime({
      defaultBackendUrl: "http://localhost:7777",
      isDesktopEnv: () => false,
    });

    await expect(runtime.resolveBackendUrl()).resolves.toBe(
      "http://localhost:7777",
    );
  });

  it("resolves backend URL through the desktop command when Tauri is available", async () => {
    const invoke = jest
      .fn<Promise<unknown>, [string, Record<string, unknown>?]>()
      .mockResolvedValue("http://127.0.0.1:7779");
    const runtime = createPlatformRuntime({
      defaultBackendUrl: "http://localhost:7777",
      isDesktopEnv: () => true,
      invoke,
    });

    await expect(runtime.resolveBackendUrl()).resolves.toBe(
      "http://127.0.0.1:7779",
    );
    expect(invoke).toHaveBeenCalledWith("get_backend_url");
  });

  it("returns an empty plugin runtime summary outside desktop mode", async () => {
    const runtime = createPlatformRuntime({
      defaultBackendUrl: "http://localhost:7777",
      isDesktopEnv: () => false,
    });

    await expect(runtime.getPluginRuntimeSummary()).resolves.toEqual({
      activeRuntimeCount: 0,
      backendHealthy: false,
      bridgeHealthy: false,
      bridgePluginCount: 0,
      eventBridgeAvailable: false,
      lastUpdatedAt: null,
      warnings: [],
    });
  });

  it("falls back to default desktop runtime snapshots when desktop commands fail", async () => {
    const invoke = jest
      .fn<Promise<unknown>, [string, Record<string, unknown>?]>()
      .mockRejectedValue(new Error("bridge unavailable"));
    const runtime = createPlatformRuntime({
      defaultBackendUrl: "http://localhost:7777",
      isDesktopEnv: () => true,
      invoke,
    });

    await expect(runtime.getDesktopRuntimeStatus()).resolves.toEqual({
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
      imBridge: {
        label: "im-bridge",
        status: "stopped",
        url: null,
        pid: null,
        restartCount: 0,
        lastError: null,
        lastStartedAt: null,
      },
    });
    await expect(runtime.getPluginRuntimeSummary()).resolves.toEqual({
      activeRuntimeCount: 0,
      backendHealthy: false,
      bridgeHealthy: false,
      bridgePluginCount: 0,
      eventBridgeAvailable: false,
      lastUpdatedAt: null,
      warnings: ["Desktop plugin runtime summary is unavailable."],
    });
  });

  it("uses the web notification fallback when desktop APIs are unavailable", async () => {
    class MockNotification {
      static permission: NotificationPermission = "default";

      constructor(
        public readonly title: string,
        public readonly options?: NotificationOptions,
      ) {}

      static async requestPermission(): Promise<NotificationPermission> {
        return "granted";
      }
    }

    Object.defineProperty(globalThis, "Notification", {
      configurable: true,
      value: MockNotification,
    });

    const requestPermission = jest
      .fn<Promise<NotificationPermission>, []>()
      .mockResolvedValue("granted");
    const notifyWeb = jest.fn<void, [string, NotificationOptions?]>();
    const runtime = createPlatformRuntime({
      defaultBackendUrl: "http://localhost:7777",
      isDesktopEnv: () => false,
      requestNotificationPermission: requestPermission,
      notifyWeb,
    });

    await expect(
      runtime.sendNotification({
        createdAt: "2026-03-26T08:00:00.000Z",
        notificationId: "notif-1",
        notificationType: "task.completed",
        title: "AgentForge",
        body: "Desktop fallback works",
        href: "/project?id=project-1#task-task-1",
        deliveryPolicy: "always",
      }),
    ).resolves.toEqual({
      mode: "web",
      notificationId: "notif-1",
      ok: true,
      status: "delivered",
    });
    expect(requestPermission).toHaveBeenCalled();
    expect(notifyWeb).toHaveBeenCalledWith("AgentForge", {
      body: "Desktop fallback works",
    });
  });

  it("returns a permission error when browser notifications are denied", async () => {
    class MockNotification {
      static permission: NotificationPermission = "default";
    }

    Object.defineProperty(globalThis, "Notification", {
      configurable: true,
      value: MockNotification,
    });

    const runtime = createPlatformRuntime({
      defaultBackendUrl: "http://localhost:7777",
      isDesktopEnv: () => false,
      requestNotificationPermission: jest.fn().mockResolvedValue("denied"),
    });

    await expect(
      runtime.sendNotification({
        createdAt: "2026-03-30T10:00:00.000Z",
        notificationId: "notif-denied",
        notificationType: "task.completed",
        title: "Denied",
        body: "Permission denied",
      }),
    ).resolves.toEqual({
      ok: false,
      reason: "permission_denied",
      error: "Notification permission was not granted.",
    });
  });

  it("normalizes structured desktop notification delivery results", async () => {
    Object.defineProperty(globalThis, "Notification", {
      configurable: true,
      value: undefined,
    });

    const invoke = jest
      .fn<Promise<unknown>, [string, Record<string, unknown>?]>()
      .mockResolvedValue({
        notificationId: "notif-2",
        status: "delivered",
      });
    const runtime = createPlatformRuntime({
      defaultBackendUrl: "http://localhost:7777",
      isDesktopEnv: () => true,
      invoke,
    });

    await expect(
      runtime.sendNotification({
        createdAt: "2026-03-26T08:05:00.000Z",
        notificationId: "notif-2",
        notificationType: "review.completed",
        title: "Review finished",
        body: "All comments were resolved.",
        href: "/reviews?id=review-1",
        deliveryPolicy: "always",
      }),
    ).resolves.toEqual({
      mode: "desktop",
      notificationId: "notif-2",
      ok: true,
      status: "delivered",
    });
    expect(invoke).toHaveBeenCalledWith("send_notification", {
      request: {
        createdAt: "2026-03-26T08:05:00.000Z",
        notificationId: "notif-2",
        notificationType: "review.completed",
        title: "Review finished",
        body: "All comments were resolved.",
        href: "/reviews?id=review-1",
        deliveryPolicy: "always",
      },
    });
  });

  it("bridges desktop notification clicks into the shell action contract when Notification is available", async () => {
    class MockNotification {
      static permission: NotificationPermission = "granted";
      static lastInstance: MockNotification | null = null;
      onclick: ((this: Notification, ev: Event) => unknown) | null = null;

      constructor(
        public readonly title: string,
        public readonly options?: NotificationOptions,
      ) {
        MockNotification.lastInstance = this;
      }
    }

    Object.defineProperty(globalThis, "Notification", {
      configurable: true,
      value: MockNotification,
    });

    const invoke = jest
      .fn<Promise<unknown>, [string, Record<string, unknown>?]>()
      .mockResolvedValue({
        actionId: "open_notification_target",
        status: "completed",
      });
    const notifyWeb = jest.fn((title: string, options?: NotificationOptions) => {
      return new MockNotification(title, options);
    });
    const runtime = createPlatformRuntime({
      defaultBackendUrl: "http://localhost:7777",
      isDesktopEnv: () => true,
      invoke,
      notifyWeb,
    });

    await expect(
      runtime.sendNotification({
        createdAt: "2026-03-28T10:00:00.000Z",
        notificationId: "notif-click-1",
        notificationType: "review.completed",
        title: "Review finished",
        body: "Open the review backlog.",
        href: "/reviews?id=review-1",
        deliveryPolicy: "always",
      }),
    ).resolves.toEqual({
      mode: "desktop",
      notificationId: "notif-click-1",
      ok: true,
      status: "delivered",
    });

    expect(notifyWeb).toHaveBeenCalledWith("Review finished", {
      body: "Open the review backlog.",
      data: {
        createdAt: "2026-03-28T10:00:00.000Z",
        href: "/reviews?id=review-1",
        notificationId: "notif-click-1",
        type: "review.completed",
      },
    });
    expect(invoke).not.toHaveBeenCalledWith(
      "send_notification",
      expect.anything(),
    );

    MockNotification.lastInstance?.onclick?.call(
      MockNotification.lastInstance as unknown as Notification,
      new Event("click"),
    );

    await expect(Promise.resolve()).resolves.toBeUndefined();
    expect(invoke).toHaveBeenCalledWith("perform_shell_action", {
      request: {
        actionId: "open_notification_target",
        href: "/reviews?id=review-1",
        payload: {
          notificationId: "notif-click-1",
          notificationType: "review.completed",
        },
        source: "notification",
      },
    });
  });

  it("uses the native desktop notification path for focused-window suppression even when Notification exists", async () => {
    class MockNotification {
      static permission: NotificationPermission = "granted";
    }

    Object.defineProperty(globalThis, "Notification", {
      configurable: true,
      value: MockNotification,
    });

    const invoke = jest
      .fn<Promise<unknown>, [string, Record<string, unknown>?]>()
      .mockResolvedValue({
        notificationId: "notif-suppressed-1",
        status: "suppressed",
      });
    const notifyWeb = jest.fn();
    const runtime = createPlatformRuntime({
      defaultBackendUrl: "http://localhost:7777",
      isDesktopEnv: () => true,
      invoke,
      notifyWeb,
    });

    await expect(
      runtime.sendNotification({
        createdAt: "2026-03-29T08:00:00.000Z",
        notificationId: "notif-suppressed-1",
        notificationType: "review.completed",
        title: "Review finished",
        body: "Focused windows should suppress the popup.",
        href: "/reviews?id=review-1",
        deliveryPolicy: "suppress_if_focused",
      }),
    ).resolves.toEqual({
      mode: "desktop",
      notificationId: "notif-suppressed-1",
      ok: true,
      status: "suppressed",
    });
    expect(invoke).toHaveBeenCalledWith("send_notification", {
      request: {
        createdAt: "2026-03-29T08:00:00.000Z",
        notificationId: "notif-suppressed-1",
        notificationType: "review.completed",
        title: "Review finished",
        body: "Focused windows should suppress the popup.",
        href: "/reviews?id=review-1",
        deliveryPolicy: "suppress_if_focused",
      },
    });
    expect(notifyWeb).not.toHaveBeenCalled();
  });

  it("syncs notification tray summaries through the tray facade", async () => {
    const invoke = jest
      .fn<Promise<unknown>, [string, Record<string, unknown>?]>()
      .mockResolvedValue(undefined);
    const setDocumentTitle = jest.fn<void, [string]>();
    const desktopRuntime = createPlatformRuntime({
      defaultBackendUrl: "http://localhost:7777",
      isDesktopEnv: () => true,
      invoke,
    });
    const webRuntime = createPlatformRuntime({
      defaultBackendUrl: "http://localhost:7777",
      isDesktopEnv: () => false,
      setDocumentTitle,
    });

    await expect(
      desktopRuntime.syncNotificationTraySummary({
        latestTitle: "Build failed",
        unreadCount: 3,
      }),
    ).resolves.toEqual({
      mode: "desktop",
      ok: true,
    });
    expect(invoke).toHaveBeenCalledWith("update_tray", {
      title: "AgentForge · 3 unread",
      tooltip: "Build failed",
      visible: true,
    });

    await expect(
      webRuntime.syncNotificationTraySummary({
        latestTitle: "Build failed",
        unreadCount: 3,
      }),
    ).resolves.toEqual({
      mode: "web",
      ok: true,
    });
    expect(setDocumentTitle).toHaveBeenCalledWith("AgentForge · 3 unread");
  });

  it("surfaces desktop tray update failures", async () => {
    const runtime = createPlatformRuntime({
      defaultBackendUrl: "http://localhost:7777",
      isDesktopEnv: () => true,
      invoke: jest.fn().mockRejectedValue(new Error("tray unavailable")),
    });

    await expect(
      runtime.updateTray({
        title: "AgentForge",
      }),
    ).resolves.toEqual({
      ok: false,
      reason: "failed",
      error: "tray unavailable",
    });
  });

  it("returns unsupported for global shortcuts on web", async () => {
    const runtime = createPlatformRuntime({
      defaultBackendUrl: "http://localhost:7777",
      isDesktopEnv: () => false,
    });

    await expect(
      runtime.registerShortcut({
        accelerator: "Ctrl+Shift+K",
        event: "open-command-palette",
      }),
    ).resolves.toEqual({
      error: "Global shortcuts require the desktop shell.",
      ok: false,
      reason: "unsupported",
    });
  });

  it("wraps desktop shortcut requests under the Tauri command request envelope", async () => {
    const invoke = jest
      .fn<Promise<unknown>, [string, Record<string, unknown>?]>()
      .mockResolvedValue(undefined);
    const runtime = createPlatformRuntime({
      defaultBackendUrl: "http://localhost:7777",
      isDesktopEnv: () => true,
      invoke,
    });

    await expect(
      runtime.registerShortcut({
        accelerator: "Ctrl+Shift+K",
        event: "open-command-palette",
      }),
    ).resolves.toEqual({
      mode: "desktop",
      ok: true,
    });
    expect(invoke).toHaveBeenCalledWith("register_shortcut", {
      request: {
        accelerator: "Ctrl+Shift+K",
        event: "open-command-palette",
      },
    });
  });

  it("surfaces desktop shortcut registration failures", async () => {
    const runtime = createPlatformRuntime({
      defaultBackendUrl: "http://localhost:7777",
      isDesktopEnv: () => true,
      invoke: jest.fn().mockRejectedValue(new Error("shortcut denied")),
    });

    await expect(
      runtime.registerShortcut({
        accelerator: "Ctrl+Shift+K",
        event: "open-command-palette",
      }),
    ).resolves.toEqual({
      ok: false,
      reason: "failed",
      error: "shortcut denied",
    });
  });

  it("routes desktop shell actions through the shared desktop command", async () => {
    const invoke = jest
      .fn<Promise<unknown>, [string, Record<string, unknown>?]>()
      .mockResolvedValue({
        actionId: "focus_main_window",
        ok: true,
        status: "completed",
      });
    const runtime = createPlatformRuntime({
      defaultBackendUrl: "http://localhost:7777",
      isDesktopEnv: () => true,
      invoke,
    });

    await expect(
      (runtime as unknown as {
        performShellAction: (input: {
          actionId: string;
          source: string;
        }) => Promise<unknown>;
      }).performShellAction({
        actionId: "focus_main_window",
        source: "window",
      }),
    ).resolves.toEqual({
      actionId: "focus_main_window",
      mode: "desktop",
      ok: true,
      status: "completed",
    });
    expect(invoke).toHaveBeenCalledWith("perform_shell_action", {
      request: {
        actionId: "focus_main_window",
        source: "window",
      },
    });
  });

  it("surfaces shell action invocation failures on desktop", async () => {
    const runtime = createPlatformRuntime({
      defaultBackendUrl: "http://localhost:7777",
      isDesktopEnv: () => true,
      invoke: jest.fn().mockRejectedValue(new Error("shell failed")),
    });

    await expect(
      (runtime as unknown as {
        performShellAction: (input: {
          actionId: string;
          source: string;
        }) => Promise<unknown>;
      }).performShellAction({
        actionId: "focus_main_window",
        source: "window",
      }),
    ).resolves.toEqual({
      ok: false,
      actionId: "focus_main_window",
      reason: "failed",
      error: "shell failed",
      status: "failed",
    });
  });

  it("normalizes maximize-toggle and close actions through the desktop shell facade", async () => {
    const invoke = jest
      .fn<Promise<unknown>, [string, Record<string, unknown>?]>()
      .mockResolvedValueOnce({
        actionId: "toggle_maximize_main_window",
        ok: true,
        status: "completed",
      })
      .mockResolvedValueOnce({
        actionId: "close_main_window",
        ok: true,
        status: "completed",
      });
    const runtime = createPlatformRuntime({
      defaultBackendUrl: "http://localhost:7777",
      isDesktopEnv: () => true,
      invoke,
    });

    await expect(runtime.toggleMaximizeMainWindow()).resolves.toEqual({
      actionId: "toggle_maximize_main_window",
      mode: "desktop",
      ok: true,
      status: "completed",
    });
    await expect(runtime.closeMainWindow()).resolves.toEqual({
      actionId: "close_main_window",
      mode: "desktop",
      ok: true,
      status: "completed",
    });

    expect(invoke).toHaveBeenNthCalledWith(1, "perform_shell_action", {
      request: {
        actionId: "toggle_maximize_main_window",
        source: "window",
      },
    });
    expect(invoke).toHaveBeenNthCalledWith(2, "perform_shell_action", {
      request: {
        actionId: "close_main_window",
        source: "window",
      },
    });
  });

  it("returns a default window chrome snapshot outside desktop mode", async () => {
    const runtime = createPlatformRuntime({
      defaultBackendUrl: "http://localhost:7777",
      isDesktopEnv: () => false,
    });

    await expect(runtime.getWindowChromeState()).resolves.toEqual({
      focused: true,
      maximized: false,
      minimized: false,
      visible: true,
    });
  });

  it("falls back to the current window when the desktop chrome snapshot is invalid", async () => {
    const runtime = createPlatformRuntime({
      defaultBackendUrl: "http://localhost:7777",
      isDesktopEnv: () => true,
      invoke: jest.fn().mockResolvedValue({ focused: true }),
      currentWindow: () => ({
        isFocused: async () => false,
        isMaximized: async () => true,
        isMinimized: async () => false,
        isVisible: async () => true,
      }),
    });

    await expect(runtime.getWindowChromeState()).resolves.toEqual({
      focused: false,
      maximized: true,
      minimized: false,
      visible: true,
    });
  });

  it("returns the default chrome state when reading the desktop window fails", async () => {
    const runtime = createPlatformRuntime({
      defaultBackendUrl: "http://localhost:7777",
      isDesktopEnv: () => true,
      invoke: jest.fn().mockRejectedValue(new Error("no state")),
      currentWindow: () => ({
        isFocused: async () => {
          throw new Error("focus failed");
        },
        isMaximized: async () => false,
        isMinimized: async () => false,
        isVisible: async () => true,
      }),
    });

    await expect(runtime.getWindowChromeState()).resolves.toEqual({
      focused: true,
      maximized: false,
      minimized: false,
      visible: true,
    });
  });

  it("projects window chrome state updates from desktop events", async () => {
    let desktopHandler: ((event: { payload: unknown }) => void) | undefined;
    const listen = jest.fn(async (_event: string, handler: (event: { payload: unknown }) => void) => {
      desktopHandler = handler;
      return jest.fn();
    });
    const runtime = createPlatformRuntime({
      defaultBackendUrl: "http://localhost:7777",
      isDesktopEnv: () => true,
      invoke: jest.fn().mockResolvedValue({
        focused: false,
        maximized: false,
        minimized: false,
        visible: true,
      }),
      listen,
    });

    const received: Array<{
      focused: boolean;
      maximized: boolean;
      minimized: boolean;
      visible: boolean;
    }> = [];
    const cleanup = await runtime.subscribeWindowChromeState((state) => {
      received.push(state);
    });

    const currentDesktopHandler = desktopHandler;
    if (currentDesktopHandler) {
      currentDesktopHandler({
        payload: {
          type: "window.state",
          payload: {
            focused: true,
            maximized: true,
            minimized: false,
            visible: true,
          },
        },
      });
    }

    expect(received).toEqual([
      {
        focused: true,
        maximized: true,
        minimized: false,
        visible: true,
      },
    ]);

    cleanup();
  });

  it("returns unsupported for shell actions on web", async () => {
    const runtime = createPlatformRuntime({
      defaultBackendUrl: "http://localhost:7777",
      isDesktopEnv: () => false,
    });

    await expect(
      (runtime as unknown as {
        performShellAction: (input: {
          actionId: string;
          source: string;
        }) => Promise<unknown>;
      }).performShellAction({
        actionId: "open_plugins",
        source: "menu",
      }),
    ).resolves.toEqual({
      actionId: "open_plugins",
      error: "Shell actions require the desktop shell.",
      ok: false,
      reason: "unsupported",
      status: "unsupported",
    });
  });

  it("provides convenience helpers for focusing the main window", async () => {
    const invoke = jest
      .fn<Promise<unknown>, [string, Record<string, unknown>?]>()
      .mockResolvedValue({
        actionId: "focus_main_window",
        ok: true,
        status: "completed",
      });
    const runtime = createPlatformRuntime({
      defaultBackendUrl: "http://localhost:7777",
      isDesktopEnv: () => true,
      invoke,
    });

    await expect(
      (runtime as unknown as { focusMainWindow: () => Promise<unknown> }).focusMainWindow(),
    ).resolves.toEqual({
      actionId: "focus_main_window",
      mode: "desktop",
      ok: true,
      status: "completed",
    });
    expect(invoke).toHaveBeenCalledWith("perform_shell_action", {
      request: {
        actionId: "focus_main_window",
        source: "window",
      },
    });
  });

  it("returns not_applicable for update checks on web", async () => {
    const runtime = createPlatformRuntime({
      defaultBackendUrl: "http://localhost:7777",
      isDesktopEnv: () => false,
    });

    await expect(runtime.checkForUpdate()).resolves.toEqual({
      ok: false,
      reason: "not_applicable",
      error: "Update checks only run inside the desktop shell.",
    });
  });

  it("returns up_to_date when no desktop update is available", async () => {
    const runtime = createPlatformRuntime({
      defaultBackendUrl: "http://localhost:7777",
      isDesktopEnv: () => true,
      checkForDesktopUpdate: jest.fn().mockResolvedValue(null),
    });

    await expect(runtime.checkForUpdate()).resolves.toEqual({
      ok: true,
      mode: "desktop",
      status: "up_to_date",
    });
  });

  it("surfaces update check failures", async () => {
    const runtime = createPlatformRuntime({
      defaultBackendUrl: "http://localhost:7777",
      isDesktopEnv: () => true,
      checkForDesktopUpdate: jest
        .fn()
        .mockRejectedValue(new Error("update unavailable")),
    });

    await expect(runtime.checkForUpdate()).resolves.toEqual({
      ok: false,
      reason: "failed",
      error: "update unavailable",
    });
  });

  it("returns normalized update metadata when a desktop update is available", async () => {
    const runtime = createPlatformRuntime({
      defaultBackendUrl: "http://localhost:7777",
      isDesktopEnv: () => true,
      checkForDesktopUpdate: jest.fn().mockResolvedValue({
        body: "Important fixes",
        currentVersion: "0.1.0",
        date: "2026-03-25T04:00:00.000Z",
        downloadAndInstall: jest.fn(),
        version: "0.2.0",
      }),
    });

    await expect(runtime.checkForUpdate()).resolves.toEqual({
      mode: "desktop",
      ok: true,
      status: "available",
      update: {
        currentVersion: "0.1.0",
        notes: "Important fixes",
        publishedAt: "2026-03-25T04:00:00.000Z",
        version: "0.2.0",
      },
    });
  });

  it("downloads and installs a cached desktop update with normalized progress", async () => {
    const downloadAndInstall = jest
      .fn()
      .mockImplementation(async (onEvent?: (event: unknown) => void) => {
        onEvent?.({
          event: "Started",
          data: { contentLength: 2048 },
        });
        onEvent?.({
          event: "Progress",
          data: { chunkLength: 512 },
        });
        onEvent?.({
          event: "Finished",
        });
      });

    const runtime = createPlatformRuntime({
      defaultBackendUrl: "http://localhost:7777",
      isDesktopEnv: () => true,
      checkForDesktopUpdate: jest.fn().mockResolvedValue({
        body: "Important fixes",
        currentVersion: "0.1.0",
        date: "2026-03-25T04:00:00.000Z",
        downloadAndInstall,
        version: "0.2.0",
      }),
    });

    await runtime.checkForUpdate();

    const progressEvents: unknown[] = [];
    await expect(
      runtime.installUpdate((event) => {
        progressEvents.push(event);
      }),
    ).resolves.toEqual({
      mode: "desktop",
      ok: true,
      status: "ready_to_relaunch",
      update: {
        currentVersion: "0.1.0",
        notes: "Important fixes",
        publishedAt: "2026-03-25T04:00:00.000Z",
        version: "0.2.0",
      },
    });

    expect(progressEvents).toEqual([
      {
        downloadedBytes: 0,
        phase: "downloading",
        totalBytes: 2048,
      },
      {
        downloadedBytes: 512,
        phase: "downloading",
        totalBytes: 2048,
      },
      {
        downloadedBytes: 512,
        phase: "installing",
        totalBytes: 2048,
      },
    ]);
  });

  it("returns a failure when installUpdate is called without a pending desktop update", async () => {
    const runtime = createPlatformRuntime({
      defaultBackendUrl: "http://localhost:7777",
      isDesktopEnv: () => true,
    });

    await expect(runtime.installUpdate()).resolves.toEqual({
      ok: false,
      reason: "failed",
      error: "No desktop update is ready to install.",
    });
  });

  it("surfaces desktop update installation failures", async () => {
    const runtime = createPlatformRuntime({
      defaultBackendUrl: "http://localhost:7777",
      isDesktopEnv: () => true,
      checkForDesktopUpdate: jest.fn().mockResolvedValue({
        body: "Important fixes",
        currentVersion: "0.1.0",
        date: "2026-03-25T04:00:00.000Z",
        downloadAndInstall: jest
          .fn()
          .mockRejectedValue(new Error("install failed")),
        version: "0.2.0",
      }),
    });

    await runtime.checkForUpdate();

    await expect(runtime.installUpdate()).resolves.toEqual({
      ok: false,
      reason: "failed",
      error: "install failed",
    });
  });

  it("relaunches the desktop app after an installed update is ready", async () => {
    const relaunchDesktopApp = jest.fn().mockResolvedValue(undefined);
    const runtime = createPlatformRuntime({
      defaultBackendUrl: "http://localhost:7777",
      isDesktopEnv: () => true,
      checkForDesktopUpdate: jest.fn().mockResolvedValue({
        body: "Important fixes",
        currentVersion: "0.1.0",
        date: "2026-03-25T04:00:00.000Z",
        downloadAndInstall: jest.fn().mockResolvedValue(undefined),
        version: "0.2.0",
      }),
      relaunchDesktopApp,
    });

    await runtime.checkForUpdate();
    await runtime.installUpdate();

    await expect(runtime.relaunchToUpdate()).resolves.toEqual({
      mode: "desktop",
      ok: true,
    });
    expect(relaunchDesktopApp).toHaveBeenCalled();
  });

  it("returns a failure when relaunch is requested without an installed update", async () => {
    const runtime = createPlatformRuntime({
      defaultBackendUrl: "http://localhost:7777",
      isDesktopEnv: () => true,
    });

    await expect(runtime.relaunchToUpdate()).resolves.toEqual({
      ok: false,
      reason: "failed",
      error: "No installed desktop update is waiting to relaunch.",
    });
  });

  it("surfaces desktop relaunch failures", async () => {
    const runtime = createPlatformRuntime({
      defaultBackendUrl: "http://localhost:7777",
      isDesktopEnv: () => true,
      checkForDesktopUpdate: jest.fn().mockResolvedValue({
        body: "Important fixes",
        currentVersion: "0.1.0",
        date: "2026-03-25T04:00:00.000Z",
        downloadAndInstall: jest.fn().mockResolvedValue(undefined),
        version: "0.2.0",
      }),
      relaunchDesktopApp: jest
        .fn()
        .mockRejectedValue(new Error("relaunch failed")),
    });

    await runtime.checkForUpdate();
    await runtime.installUpdate();

    await expect(runtime.relaunchToUpdate()).resolves.toEqual({
      ok: false,
      reason: "failed",
      error: "relaunch failed",
    });
  });

  it("normalizes desktop runtime event subscriptions", async () => {
    const unlisten = jest.fn();
    const listen = jest
      .fn<
        Promise<() => void>,
        [string, (event: { payload: unknown }) => void]
      >()
      .mockImplementation(async (_event, handler) => {
        handler({
          payload: {
            type: "runtime.updated",
            runtime: {
              overall: "ready",
              backend: {
                label: "backend",
                status: "ready",
                url: "http://127.0.0.1:7777",
                pid: 1001,
                restartCount: 0,
                lastError: null,
                lastStartedAt: "2026-03-25T03:00:00.000Z",
              },
              bridge: {
                label: "bridge",
                status: "ready",
                url: "http://127.0.0.1:7778",
                pid: 1002,
                restartCount: 0,
                lastError: null,
                lastStartedAt: "2026-03-25T03:00:02.000Z",
              },
              imBridge: {
                label: "im-bridge",
                status: "ready",
                url: "http://127.0.0.1:7779",
                pid: 1003,
                restartCount: 0,
                lastError: null,
                lastStartedAt: "2026-03-25T03:00:03.000Z",
              },
            },
          },
        });

        return unlisten;
      });
    const runtime = createPlatformRuntime({
      defaultBackendUrl: "http://localhost:7777",
      isDesktopEnv: () => true,
      listen,
    });

    const received: unknown[] = [];
    const cleanup = await runtime.subscribeDesktopEvents((event) => {
      received.push(event);
    });

    expect(listen).toHaveBeenCalledWith(
      "agentforge://desktop-event",
      expect.any(Function),
    );
    expect(received).toEqual([
      {
        type: "runtime.updated",
        runtime: {
          overall: "ready",
          backend: {
            label: "backend",
            status: "ready",
            url: "http://127.0.0.1:7777",
            pid: 1001,
            restartCount: 0,
            lastError: null,
            lastStartedAt: "2026-03-25T03:00:00.000Z",
          },
          bridge: {
            label: "bridge",
            status: "ready",
            url: "http://127.0.0.1:7778",
            pid: 1002,
            restartCount: 0,
            lastError: null,
            lastStartedAt: "2026-03-25T03:00:02.000Z",
          },
          imBridge: {
            label: "im-bridge",
            status: "ready",
            url: "http://127.0.0.1:7779",
            pid: 1003,
            restartCount: 0,
            lastError: null,
            lastStartedAt: "2026-03-25T03:00:03.000Z",
          },
        },
      },
    ]);
    cleanup();
    expect(unlisten).toHaveBeenCalled();
  });

  it("preserves source and timestamp for notification outcome desktop events", async () => {
    const unlisten = jest.fn();
    const listen = jest
      .fn<
        Promise<() => void>,
        [string, (event: { payload: unknown }) => void]
      >()
      .mockImplementation(async (_event, handler) => {
        handler({
          payload: {
            type: "notification.failed",
            source: "notification",
            timestamp: "2026-03-26T10:30:00.000Z",
            payload: {
              notificationId: "notification-1",
              notificationType: "task_progress_stalled",
              title: "Task stalled: Implement detector",
              status: "failed",
              error: "notification backend unavailable",
            },
          },
        });

        return unlisten;
      });
    const runtime = createPlatformRuntime({
      defaultBackendUrl: "http://localhost:7777",
      isDesktopEnv: () => true,
      listen,
    });

    const received: unknown[] = [];
    const cleanup = await runtime.subscribeDesktopEvents((event) => {
      received.push(event);
    });

    expect(received).toEqual([
      {
        type: "notification.failed",
        source: "notification",
        timestamp: "2026-03-26T10:30:00.000Z",
        payload: {
          notificationId: "notification-1",
          notificationType: "task_progress_stalled",
          title: "Task stalled: Implement detector",
          status: "failed",
          error: "notification backend unavailable",
        },
      },
    ]);

    cleanup();
    expect(unlisten).toHaveBeenCalled();
  });

  it("normalizes shell action desktop events with route context", async () => {
    const unlisten = jest.fn();
    const listen = jest
      .fn<
        Promise<() => void>,
        [string, (event: { payload: unknown }) => void]
      >()
      .mockImplementation(async (_event, handler) => {
        handler({
          payload: {
            type: "shell.action",
            source: "notification",
            actionId: "open_notification_target",
            status: "triggered",
            href: "/reviews?id=review-1",
            timestamp: "2026-03-28T09:10:00.000Z",
            payload: {
              notificationId: "notification-7",
            },
          },
        });

        return unlisten;
      });
    const runtime = createPlatformRuntime({
      defaultBackendUrl: "http://localhost:7777",
      isDesktopEnv: () => true,
      listen,
    });

    const received: unknown[] = [];
    const cleanup = await runtime.subscribeDesktopEvents((event) => {
      received.push(event);
    });

    expect(received).toEqual([
      {
        type: "shell.action",
        source: "notification",
        actionId: "open_notification_target",
        status: "triggered",
        href: "/reviews?id=review-1",
        timestamp: "2026-03-28T09:10:00.000Z",
        payload: {
          notificationId: "notification-7",
        },
      },
    ]);

    cleanup();
    expect(unlisten).toHaveBeenCalled();
  });
});
