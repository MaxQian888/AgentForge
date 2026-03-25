export type CapabilityFailureReason =
  | "cancelled"
  | "failed"
  | "not_applicable"
  | "permission_denied"
  | "unsupported";

export interface DesktopRuntimeUnit {
  label: string;
  status: "degraded" | "ready" | "starting" | "stopped";
  url: string | null;
  pid: number | null;
  restartCount: number;
  lastError: string | null;
  lastStartedAt: string | null;
}

export interface DesktopRuntimeStatus {
  overall: "degraded" | "ready" | "starting" | "stopped";
  backend: DesktopRuntimeUnit;
  bridge: DesktopRuntimeUnit;
}

export interface DesktopRuntimeEvent {
  type: string;
  runtime?: DesktopRuntimeStatus;
  shortcut?: string;
  payload?: unknown;
}

export interface PluginRuntimeSummary {
  activeRuntimeCount: number;
  backendHealthy: boolean;
  bridgeHealthy: boolean;
  bridgePluginCount: number;
  eventBridgeAvailable: boolean;
  lastUpdatedAt: string | null;
  warnings: string[];
}

export interface SelectFilesOptions {
  directory?: boolean;
  filters?: Array<{
    extensions?: string[];
    name: string;
  }>;
  multiple?: boolean;
  title?: string;
}

export type SelectFilesResult =
  | {
      ok: true;
      mode: "desktop" | "web";
      paths: string[];
      files?: File[];
    }
  | {
      ok: false;
      reason: CapabilityFailureReason;
      error: string;
    };

export type PlatformResult =
  | {
      ok: true;
      mode: "desktop" | "web";
    }
  | {
      ok: false;
      reason: CapabilityFailureReason;
      error: string;
    };

export interface RegisterShortcutRequest {
  accelerator: string;
  event: string;
}

export interface DesktopUpdateInfo {
  currentVersion: string | null;
  notes: string | null;
  publishedAt: string | null;
  version: string;
}

export interface DesktopUpdateProgress {
  downloadedBytes: number;
  phase: "downloading" | "installing";
  totalBytes: number | null;
}

export type PlatformUpdateResult =
  | {
      ok: true;
      mode: "desktop";
      status: "available" | "ready_to_relaunch";
      update: DesktopUpdateInfo;
    }
  | {
      ok: true;
      mode: "desktop";
      status: "up_to_date";
      update?: undefined;
    }
  | {
      ok: false;
      reason: CapabilityFailureReason;
      error: string;
    };

interface DesktopUpdateHandle {
  body?: string;
  currentVersion?: string;
  date?: string | null;
  downloadAndInstall: (
    onEvent?: (event: unknown) => void,
  ) => Promise<void>;
  version: string;
}

interface PlatformRuntimeDeps {
  checkForDesktopUpdate?: () => Promise<DesktopUpdateHandle | null>;
  defaultBackendUrl?: string;
  inputFactory?: () => HTMLInputElement;
  invoke?: (
    command: string,
    args?: Record<string, unknown>,
  ) => Promise<unknown>;
  isDesktopEnv?: () => boolean;
  listen?: (
    event: string,
    handler: (event: { payload: unknown }) => void,
  ) => Promise<() => void>;
  notifyWeb?: (title: string, options?: NotificationOptions) => void;
  relaunchDesktopApp?: () => Promise<void>;
  requestNotificationPermission?: () => Promise<NotificationPermission>;
  setDocumentTitle?: (title: string) => void;
}

export const DESKTOP_EVENT_NAME = "agentforge://desktop-event";

const DEFAULT_BACKEND_URL =
  process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";

function defaultDesktopRuntimeStatus(): DesktopRuntimeStatus {
  return {
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
  };
}

function defaultPluginRuntimeSummary(): PluginRuntimeSummary {
  return {
    activeRuntimeCount: 0,
    backendHealthy: false,
    bridgeHealthy: false,
    bridgePluginCount: 0,
    eventBridgeAvailable: false,
    lastUpdatedAt: null,
    warnings: [],
  };
}

function isBrowserNotificationAvailable(): boolean {
  return typeof Notification !== "undefined";
}

function createBrowserInput(): HTMLInputElement {
  return document.createElement("input");
}

async function importInvoke() {
  const { invoke } = await import("@tauri-apps/api/core");
  return invoke;
}

async function importListen() {
  const { listen } = await import("@tauri-apps/api/event");
  return listen;
}

async function importUpdaterCheck() {
  const { check } = await import("@tauri-apps/plugin-updater");
  return check;
}

async function importRelaunch() {
  const { relaunch } = await import("@tauri-apps/plugin-process");
  return relaunch;
}

function normalizeDesktopEvent(payload: unknown): DesktopRuntimeEvent | null {
  if (!payload || typeof payload !== "object") {
    return null;
  }

  const typedPayload = payload as Record<string, unknown>;
  if (typeof typedPayload.type !== "string") {
    return null;
  }

  return {
    type: typedPayload.type,
    runtime: typedPayload.runtime as DesktopRuntimeStatus | undefined,
    shortcut:
      typeof typedPayload.shortcut === "string"
        ? typedPayload.shortcut
        : undefined,
    payload: typedPayload.payload,
  };
}

function normalizeDesktopUpdateInfo(
  update: DesktopUpdateHandle,
): DesktopUpdateInfo {
  return {
    currentVersion: update.currentVersion ?? null,
    notes: update.body ?? null,
    publishedAt: update.date ?? null,
    version: update.version,
  };
}

function normalizeUpdateError(error: unknown, fallback: string): string {
  return error instanceof Error ? error.message : fallback;
}

function pickFilesFromBrowser(
  options: SelectFilesOptions,
  inputFactory: () => HTMLInputElement,
): Promise<SelectFilesResult> {
  if (typeof document === "undefined") {
    return Promise.resolve({
      ok: false,
      reason: "unsupported",
      error: "Browser file selection is unavailable in this environment.",
    });
  }

  return new Promise((resolve) => {
    const input = inputFactory();
    input.type = "file";
    input.multiple = Boolean(options.multiple);

    if (options.directory) {
      input.setAttribute("webkitdirectory", "");
    }

    const extensions =
      options.filters?.flatMap((filter) =>
        filter.extensions?.map((extension) => `.${extension}`) ?? [],
      ) ?? [];

    if (extensions.length > 0) {
      input.accept = extensions.join(",");
    }

    input.onchange = () => {
      const selectedFiles = Array.from(input.files ?? []);
      if (selectedFiles.length === 0) {
        resolve({
          ok: false,
          reason: "cancelled",
          error: "No files were selected.",
        });
        return;
      }

      resolve({
        ok: true,
        mode: "web",
        files: selectedFiles,
        paths: selectedFiles.map((file) => file.name),
      });
    };

    input.click();
  });
}

export function createPlatformRuntime(deps: PlatformRuntimeDeps = {}) {
  const defaultBackendUrl = deps.defaultBackendUrl ?? DEFAULT_BACKEND_URL;
  const getIsDesktopEnv =
    deps.isDesktopEnv ??
    (() => typeof window !== "undefined" && "__TAURI_INTERNALS__" in window);
  const getInvoke =
    deps.invoke ??
    (async (command: string, args?: Record<string, unknown>) => {
      const invoke = await importInvoke();
      return invoke(command, args);
    });
  const getListen =
    deps.listen ??
    (async (event: string, handler: (event: { payload: unknown }) => void) => {
      const listen = await importListen();
      return listen(event, handler);
    });
  const requestNotificationPermission =
    deps.requestNotificationPermission ??
    (async () => Notification.requestPermission());
  const notifyWeb =
    deps.notifyWeb ??
    ((title: string, options?: NotificationOptions) => {
      new Notification(title, options);
    });
  const setDocumentTitle =
    deps.setDocumentTitle ??
    ((title: string) => {
      if (typeof document !== "undefined") {
        document.title = title;
      }
    });
  const inputFactory = deps.inputFactory ?? createBrowserInput;
  const checkForDesktopUpdate =
    deps.checkForDesktopUpdate ??
    (async () => {
      const check = await importUpdaterCheck();
      return (await check()) as DesktopUpdateHandle | null;
    });
  const relaunchDesktopApp =
    deps.relaunchDesktopApp ??
    (async () => {
      const relaunch = await importRelaunch();
      await relaunch();
    });
  let pendingDesktopUpdate: DesktopUpdateHandle | null = null;
  let installedDesktopUpdate: DesktopUpdateHandle | null = null;

  return {
    defaultBackendUrl,
    get isDesktop() {
      return getIsDesktopEnv();
    },
    async resolveBackendUrl(): Promise<string> {
      if (!getIsDesktopEnv()) {
        return defaultBackendUrl;
      }

      try {
        return (await getInvoke("get_backend_url")) as string;
      } catch {
        return defaultBackendUrl;
      }
    },
    async getDesktopRuntimeStatus(): Promise<DesktopRuntimeStatus> {
      if (!getIsDesktopEnv()) {
        return defaultDesktopRuntimeStatus();
      }

      try {
        return (await getInvoke(
          "get_desktop_runtime_status",
        )) as DesktopRuntimeStatus;
      } catch {
        return defaultDesktopRuntimeStatus();
      }
    },
    async getPluginRuntimeSummary(): Promise<PluginRuntimeSummary> {
      if (!getIsDesktopEnv()) {
        return defaultPluginRuntimeSummary();
      }

      try {
        return (await getInvoke(
          "get_plugin_runtime_summary",
        )) as PluginRuntimeSummary;
      } catch {
        return {
          ...defaultPluginRuntimeSummary(),
          warnings: ["Desktop plugin runtime summary is unavailable."],
        };
      }
    },
    async subscribeDesktopEvents(
      handler: (event: DesktopRuntimeEvent) => void,
    ): Promise<() => void> {
      if (!getIsDesktopEnv()) {
        return () => {};
      }

      const unlisten = await getListen(DESKTOP_EVENT_NAME, (event) => {
        const normalized = normalizeDesktopEvent(event.payload);
        if (normalized) {
          handler(normalized);
        }
      });

      return () => {
        unlisten();
      };
    },
    async selectFiles(options: SelectFilesOptions): Promise<SelectFilesResult> {
      if (getIsDesktopEnv()) {
        try {
          const paths = (await getInvoke("select_files", {
            options,
          })) as string[];

          if (!paths || paths.length === 0) {
            return {
              ok: false,
              reason: "cancelled",
              error: "No files were selected.",
            };
          }

          return {
            ok: true,
            mode: "desktop",
            paths,
          };
        } catch (error) {
          return {
            ok: false,
            reason: "failed",
            error:
              error instanceof Error
                ? error.message
                : "Desktop file selection failed.",
          };
        }
      }

      return pickFilesFromBrowser(options, inputFactory);
    },
    async sendNotification(payload: {
      body: string;
      title: string;
    }): Promise<PlatformResult> {
      if (getIsDesktopEnv()) {
        try {
          await getInvoke(
            "send_notification",
            payload as Record<string, unknown>,
          );
          return { ok: true, mode: "desktop" };
        } catch (error) {
          return {
            ok: false,
            reason: "failed",
            error:
              error instanceof Error
                ? error.message
                : "Desktop notification failed.",
          };
        }
      }

      if (!isBrowserNotificationAvailable()) {
        return {
          ok: false,
          reason: "unsupported",
          error: "Browser notifications are unavailable.",
        };
      }

      const permission =
        Notification.permission === "default"
          ? await requestNotificationPermission()
          : Notification.permission;

      if (permission !== "granted") {
        return {
          ok: false,
          reason: "permission_denied",
          error: "Notification permission was not granted.",
        };
      }

      notifyWeb(payload.title, { body: payload.body });
      return { ok: true, mode: "web" };
    },
    async updateTray(payload: {
      title?: string | null;
      tooltip?: string | null;
      visible?: boolean;
    }): Promise<PlatformResult> {
      if (getIsDesktopEnv()) {
        try {
          await getInvoke("update_tray", payload as Record<string, unknown>);
          return { ok: true, mode: "desktop" };
        } catch (error) {
          return {
            ok: false,
            reason: "failed",
            error:
              error instanceof Error ? error.message : "Tray update failed.",
          };
        }
      }

      setDocumentTitle(payload.title ?? payload.tooltip ?? "AgentForge");
      return { ok: true, mode: "web" };
    },
    async registerShortcut(
      request: RegisterShortcutRequest,
    ): Promise<PlatformResult> {
      if (!getIsDesktopEnv()) {
        return {
          ok: false,
          reason: "unsupported",
          error: "Global shortcuts require the desktop shell.",
        };
      }

      try {
        await getInvoke(
          "register_shortcut",
          request as unknown as Record<string, unknown>,
        );
        return { ok: true, mode: "desktop" };
      } catch (error) {
        return {
          ok: false,
          reason: "failed",
          error:
            error instanceof Error
              ? error.message
              : "Global shortcut registration failed.",
        };
      }
    },
    async checkForUpdate(): Promise<PlatformUpdateResult> {
      if (!getIsDesktopEnv()) {
        return {
          ok: false,
          reason: "not_applicable",
          error: "Update checks only run inside the desktop shell.",
        };
      }

      try {
        const update = await checkForDesktopUpdate();

        if (!update) {
          pendingDesktopUpdate = null;
          installedDesktopUpdate = null;
          return { ok: true, mode: "desktop", status: "up_to_date" };
        }

        pendingDesktopUpdate = update;
        installedDesktopUpdate = null;

        return {
          ok: true,
          mode: "desktop",
          status: "available",
          update: normalizeDesktopUpdateInfo(update),
        };
      } catch (error) {
        return {
          ok: false,
          reason: "failed",
          error: normalizeUpdateError(error, "Update check failed."),
        };
      }
    },
    async installUpdate(
      onProgress?: (event: DesktopUpdateProgress) => void,
    ): Promise<PlatformUpdateResult> {
      if (!getIsDesktopEnv()) {
        return {
          ok: false,
          reason: "not_applicable",
          error: "Update installation only runs inside the desktop shell.",
        };
      }

      if (!pendingDesktopUpdate) {
        return {
          ok: false,
          reason: "failed",
          error: "No desktop update is ready to install.",
        };
      }

      const update = pendingDesktopUpdate;
      let downloadedBytes = 0;
      let totalBytes: number | null = null;

      try {
        await update.downloadAndInstall((event) => {
          if (!onProgress || !event || typeof event !== "object") {
            return;
          }

          const typedEvent = event as {
            data?: { chunkLength?: number; contentLength?: number };
            event?: string;
          };

          if (typedEvent.event === "Started") {
            totalBytes = typedEvent.data?.contentLength ?? null;
            onProgress({
              downloadedBytes: 0,
              phase: "downloading",
              totalBytes,
            });
            return;
          }

          if (typedEvent.event === "Progress") {
            downloadedBytes += typedEvent.data?.chunkLength ?? 0;
            onProgress({
              downloadedBytes,
              phase: "downloading",
              totalBytes,
            });
            return;
          }

          if (typedEvent.event === "Finished") {
            onProgress({
              downloadedBytes,
              phase: "installing",
              totalBytes,
            });
          }
        });

        pendingDesktopUpdate = null;
        installedDesktopUpdate = update;

        return {
          ok: true,
          mode: "desktop",
          status: "ready_to_relaunch",
          update: normalizeDesktopUpdateInfo(update),
        };
      } catch (error) {
        return {
          ok: false,
          reason: "failed",
          error: normalizeUpdateError(error, "Update installation failed."),
        };
      }
    },
    async relaunchToUpdate(): Promise<PlatformResult> {
      if (!getIsDesktopEnv()) {
        return {
          ok: false,
          reason: "not_applicable",
          error: "App relaunch only runs inside the desktop shell.",
        };
      }

      if (!installedDesktopUpdate) {
        return {
          ok: false,
          reason: "failed",
          error: "No installed desktop update is waiting to relaunch.",
        };
      }

      try {
        await relaunchDesktopApp();
        return { ok: true, mode: "desktop" };
      } catch (error) {
        return {
          ok: false,
          reason: "failed",
          error: normalizeUpdateError(error, "App relaunch failed."),
        };
      }
    },
  };
}

export const platformRuntime = createPlatformRuntime();
