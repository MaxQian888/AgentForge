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
      .fn<Promise<string>, [string, Record<string, unknown>?]>()
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
