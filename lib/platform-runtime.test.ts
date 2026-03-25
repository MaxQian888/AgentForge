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
        title: "AgentForge",
        body: "Desktop fallback works",
      }),
    ).resolves.toEqual({
      mode: "web",
      ok: true,
    });
    expect(requestPermission).toHaveBeenCalled();
    expect(notifyWeb).toHaveBeenCalledWith("AgentForge", {
      body: "Desktop fallback works",
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
        },
      },
    ]);
    cleanup();
    expect(unlisten).toHaveBeenCalled();
  });
});
