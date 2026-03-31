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
  imBridge: DesktopRuntimeUnit;
}

export interface DesktopRuntimeEvent {
  type: string;
  source?: string;
  timestamp?: string;
  actionId?: string;
  href?: string;
  runtime?: DesktopRuntimeStatus;
  status?: "completed" | "failed" | "triggered" | "unsupported";
  shortcut?: string;
  payload?: unknown;
  windowState?: DesktopWindowChromeState;
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

export type NotificationDeliveryPolicy = "always" | "suppress_if_focused";

export interface BusinessNotificationPayload {
  notificationId: string;
  type: string;
  title: string;
  body: string;
  createdAt: string;
  href?: string | null;
  deliveryPolicy?: NotificationDeliveryPolicy;
}

export type PlatformNotificationResult =
  | {
      ok: true;
      mode: "desktop" | "web";
      notificationId: string;
      status: "delivered" | "suppressed";
    }
  | {
      ok: false;
      reason: CapabilityFailureReason;
      error: string;
    };

export interface NotificationTraySummary {
  unreadCount: number;
  latestTitle?: string | null;
  visible?: boolean;
}

export interface RegisterShortcutRequest {
  accelerator: string;
  event: string;
}

export interface DesktopShellActionRequest {
  actionId: string;
  href?: string | null;
  payload?: Record<string, unknown>;
  source: string;
}

export interface DesktopWindowChromeState {
  focused: boolean;
  maximized: boolean;
  minimized: boolean;
  visible: boolean;
}

export type DesktopShellActionResult =
  | {
      ok: true;
      mode: "desktop" | "web";
      actionId: string;
      status: "completed" | "triggered";
    }
  | {
      ok: false;
      actionId: string;
      reason: CapabilityFailureReason;
      error: string;
      status: "failed" | "unsupported";
    };

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

interface DesktopWindowHandle {
  isFocused(): Promise<boolean>;
  isMaximized(): Promise<boolean>;
  isMinimized(): Promise<boolean>;
  isVisible(): Promise<boolean>;
  onFocusChanged?(
    handler: (event: { payload: boolean }) => void,
  ): Promise<() => void>;
  onMoved?(handler: (event: { payload: unknown }) => void): Promise<() => void>;
  onResized?(handler: (event: { payload: unknown }) => void): Promise<() => void>;
  onScaleChanged?(
    handler: (event: { payload: unknown }) => void,
  ): Promise<() => void>;
}

interface PlatformRuntimeDeps {
  checkForDesktopUpdate?: () => Promise<DesktopUpdateHandle | null>;
  currentWindow?: () => Promise<DesktopWindowHandle> | DesktopWindowHandle;
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
  notifyWeb?: (
    title: string,
    options?: NotificationOptions,
  ) => Notification | { onclick: ((ev: Event) => unknown) | null } | void;
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
    imBridge: {
      label: "im-bridge",
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

async function importCurrentWindow(): Promise<DesktopWindowHandle> {
  const { getCurrentWindow } = await import("@tauri-apps/api/window");
  return getCurrentWindow() as unknown as DesktopWindowHandle;
}

function defaultDesktopWindowChromeState(): DesktopWindowChromeState {
  return {
    focused: true,
    maximized: false,
    minimized: false,
    visible: true,
  };
}

function normalizeDesktopWindowChromeState(
  payload: unknown,
): DesktopWindowChromeState | null {
  if (!payload || typeof payload !== "object") {
    return null;
  }

  const typedPayload = payload as Record<string, unknown>;
  if (
    typeof typedPayload.focused !== "boolean" ||
    typeof typedPayload.maximized !== "boolean" ||
    typeof typedPayload.minimized !== "boolean" ||
    typeof typedPayload.visible !== "boolean"
  ) {
    return null;
  }

  return {
    focused: typedPayload.focused,
    maximized: typedPayload.maximized,
    minimized: typedPayload.minimized,
    visible: typedPayload.visible,
  };
}

async function readDesktopWindowChromeState(
  window: DesktopWindowHandle,
): Promise<DesktopWindowChromeState> {
  try {
    const [focused, maximized, minimized, visible] = await Promise.all([
      window.isFocused(),
      window.isMaximized(),
      window.isMinimized(),
      window.isVisible(),
    ]);

    return {
      focused,
      maximized,
      minimized,
      visible,
    };
  } catch {
    return defaultDesktopWindowChromeState();
  }
}

function normalizeDesktopEvent(payload: unknown): DesktopRuntimeEvent | null {
  if (!payload || typeof payload !== "object") {
    return null;
  }

  const typedPayload = payload as Record<string, unknown>;
  if (typeof typedPayload.type !== "string") {
    return null;
  }
  const normalized: DesktopRuntimeEvent = {
    type: typedPayload.type,
  };

  if (typeof typedPayload.source === "string") {
    normalized.source = typedPayload.source;
  }
  if (typeof typedPayload.timestamp === "string") {
    normalized.timestamp = typedPayload.timestamp;
  }
  if (typeof typedPayload.actionId === "string") {
    normalized.actionId = typedPayload.actionId;
  }
  if (typeof typedPayload.href === "string") {
    normalized.href = typedPayload.href;
  }
  if (
    typedPayload.status === "completed" ||
    typedPayload.status === "failed" ||
    typedPayload.status === "triggered" ||
    typedPayload.status === "unsupported"
  ) {
    normalized.status = typedPayload.status;
  }
  if (typedPayload.runtime && typeof typedPayload.runtime === "object") {
    normalized.runtime = typedPayload.runtime as DesktopRuntimeStatus;
  }
  if (typeof typedPayload.shortcut === "string") {
    normalized.shortcut = typedPayload.shortcut;
  }
  if (typeof typedPayload.payload !== "undefined") {
    normalized.payload = typedPayload.payload;
  }
  const windowState =
    normalizeDesktopWindowChromeState(typedPayload.windowState) ??
    (typedPayload.type === "window.state"
      ? normalizeDesktopWindowChromeState(typedPayload.payload)
      : null);
  if (windowState) {
    normalized.windowState = windowState;
  }

  return normalized;
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

function normalizeNotificationStatus(
  status: unknown,
): "delivered" | "suppressed" {
  return status === "suppressed" ? "suppressed" : "delivered";
}

function normalizeShellActionStatus(
  status: unknown,
): "completed" | "failed" | "triggered" | "unsupported" {
  if (
    status === "completed" ||
    status === "failed" ||
    status === "triggered" ||
    status === "unsupported"
  ) {
    return status;
  }

  return "completed";
}

function buildNotificationTrayState(summary: NotificationTraySummary): {
  title: string;
  tooltip: string;
  visible: boolean;
} {
  const unreadCount = Math.max(0, summary.unreadCount);
  const title =
    unreadCount > 0 ? `AgentForge · ${unreadCount} unread` : "AgentForge";
  const tooltip =
    summary.latestTitle?.trim() ||
    (unreadCount > 0
      ? `${unreadCount} unread notifications`
      : "No unread notifications");

  return {
    title,
    tooltip,
    visible: summary.visible ?? unreadCount > 0,
  };
}

function getProjectedDesktopEventTarget(): EventTarget | null {
  if (typeof window !== "undefined") {
    return window;
  }

  return null;
}

export function emitProjectedDesktopEvent(event: DesktopRuntimeEvent): void {
  const target = getProjectedDesktopEventTarget();
  if (!target || typeof CustomEvent === "undefined") {
    return;
  }

  target.dispatchEvent(
    new CustomEvent(DESKTOP_EVENT_NAME, {
      detail: event,
    }),
  );
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
      return new Notification(title, options);
    });
  const setDocumentTitle =
    deps.setDocumentTitle ??
    ((title: string) => {
      if (typeof document !== "undefined") {
        document.title = title;
      }
    });
  const inputFactory = deps.inputFactory ?? createBrowserInput;
  const resolveCurrentWindow =
    deps.currentWindow ??
    (async () => {
      return importCurrentWindow();
    });
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
  let cachedDesktopWindow: Promise<DesktopWindowHandle> | null = null;

  const getDesktopWindow = async () => {
    cachedDesktopWindow ??= Promise.resolve(resolveCurrentWindow());
    return cachedDesktopWindow;
  };

  const publishWindowChromeState = async () => {
    if (!getIsDesktopEnv()) {
      return;
    }

    const window = await getDesktopWindow();
    const nextState = await readDesktopWindowChromeState(window);
    emitProjectedDesktopEvent({
      type: "window.state",
      source: "window",
      payload: nextState,
      windowState: nextState,
    });
  };

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
      const projectedTarget = getProjectedDesktopEventTarget();
      const projectedListener =
        projectedTarget && typeof projectedTarget.addEventListener === "function"
          ? ((event: Event) => {
              const detail =
                event instanceof CustomEvent ? event.detail : undefined;
              const normalized = normalizeDesktopEvent(detail);
              if (normalized) {
                handler(normalized);
              }
            })
          : null;

      if (projectedTarget && projectedListener) {
        projectedTarget.addEventListener(
          DESKTOP_EVENT_NAME,
          projectedListener as EventListener,
        );
      }

      if (!getIsDesktopEnv()) {
        return () => {
          if (projectedTarget && projectedListener) {
            projectedTarget.removeEventListener(
              DESKTOP_EVENT_NAME,
              projectedListener as EventListener,
            );
          }
        };
      }

      const unlisten = await getListen(DESKTOP_EVENT_NAME, (event) => {
        const normalized = normalizeDesktopEvent(event.payload);
        if (normalized) {
          handler(normalized);
        }
      });

      return () => {
        unlisten();
        if (projectedTarget && projectedListener) {
          projectedTarget.removeEventListener(
            DESKTOP_EVENT_NAME,
            projectedListener as EventListener,
          );
        }
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
    async sendNotification(
      payload: BusinessNotificationPayload,
    ): Promise<PlatformNotificationResult> {
      const shouldPreferBrowserDesktopNotification =
        getIsDesktopEnv() &&
        isBrowserNotificationAvailable() &&
        payload.deliveryPolicy !== "suppress_if_focused";

      if (shouldPreferBrowserDesktopNotification) {
        const permission =
          Notification.permission === "default"
            ? await requestNotificationPermission()
            : Notification.permission;

        if (permission === "granted") {
          const notification = notifyWeb(payload.title, {
            body: payload.body,
            data: {
              createdAt: payload.createdAt,
              href: payload.href,
              notificationId: payload.notificationId,
              type: payload.type,
            },
          });

          if (
            notification &&
            typeof notification === "object" &&
            "onclick" in notification
          ) {
            notification.onclick = () => {
              void this.performShellAction({
                actionId: "open_notification_target",
                href: payload.href ?? undefined,
                payload: {
                  notificationId: payload.notificationId,
                  notificationType: payload.type,
                },
                source: "notification",
              });
            };
          }

          return {
            ok: true,
            mode: "desktop",
            notificationId: payload.notificationId,
            status: "delivered",
          };
        }
      }

      if (getIsDesktopEnv()) {
        try {
          const response = (await getInvoke(
            "send_notification",
            { request: payload } as Record<string, unknown>,
          )) as { notificationId?: string; status?: string } | undefined;
          return {
            ok: true,
            mode: "desktop",
            notificationId: response?.notificationId ?? payload.notificationId,
            status: normalizeNotificationStatus(response?.status),
          };
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
      return {
        ok: true,
        mode: "web",
        notificationId: payload.notificationId,
        status: "delivered",
      };
    },
    async syncNotificationTraySummary(
      summary: NotificationTraySummary,
    ): Promise<PlatformResult> {
      const trayState = buildNotificationTrayState(summary);

      if (getIsDesktopEnv()) {
        try {
          await getInvoke(
            "update_tray",
            trayState as Record<string, unknown>,
          );
          return { ok: true, mode: "desktop" };
        } catch (error) {
          return {
            ok: false,
            reason: "failed",
            error:
              error instanceof Error
                ? error.message
                : "Tray update failed.",
          };
        }
      }

      setDocumentTitle(trayState.title);
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
          { request } as Record<string, unknown>,
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
    async performShellAction(
      request: DesktopShellActionRequest,
    ): Promise<DesktopShellActionResult> {
      if (!getIsDesktopEnv()) {
        return {
          ok: false,
          actionId: request.actionId,
          reason: "unsupported",
          error: "Shell actions require the desktop shell.",
          status: "unsupported",
        };
      }

      try {
        const response = (await getInvoke(
          "perform_shell_action",
          { request } as Record<string, unknown>,
        )) as { actionId?: string; status?: string } | undefined;
        const status = normalizeShellActionStatus(response?.status);

        return {
          ok: true,
          mode: "desktop",
          actionId: response?.actionId ?? request.actionId,
          status: status === "failed" || status === "unsupported" ? "completed" : status,
        };
      } catch (error) {
        return {
          ok: false,
          actionId: request.actionId,
          reason: "failed",
          error:
            error instanceof Error ? error.message : "Shell action failed.",
          status: "failed",
        };
      }
    },
    async closeMainWindow(): Promise<DesktopShellActionResult> {
      return this.performShellAction({
        actionId: "close_main_window",
        source: "window",
      });
    },
    async focusMainWindow(): Promise<DesktopShellActionResult> {
      return this.performShellAction({
        actionId: "focus_main_window",
        source: "window",
      });
    },
    async getWindowChromeState(): Promise<DesktopWindowChromeState> {
      if (!getIsDesktopEnv()) {
        return defaultDesktopWindowChromeState();
      }

      try {
        const snapshot = await getInvoke("get_window_chrome_state");
        return (
          normalizeDesktopWindowChromeState(snapshot) ??
          readDesktopWindowChromeState(await getDesktopWindow())
        );
      } catch {
        try {
          return await readDesktopWindowChromeState(await getDesktopWindow());
        } catch {
          return defaultDesktopWindowChromeState();
        }
      }
    },
    async minimizeMainWindow(): Promise<DesktopShellActionResult> {
      return this.performShellAction({
        actionId: "minimize_main_window",
        source: "window",
      });
    },
    async restoreMainWindow(): Promise<DesktopShellActionResult> {
      return this.performShellAction({
        actionId: "restore_main_window",
        source: "window",
      });
    },
    async showMainWindow(): Promise<DesktopShellActionResult> {
      return this.performShellAction({
        actionId: "show_main_window",
        source: "window",
      });
    },
    async subscribeWindowChromeState(
      handler: (state: DesktopWindowChromeState) => void,
    ): Promise<() => void> {
      const cleanupDesktopEvents = await this.subscribeDesktopEvents((event) => {
        if (event.windowState) {
          handler(event.windowState);
          return;
        }

        if (event.type === "window.state") {
          const nextState = normalizeDesktopWindowChromeState(event.payload);
          if (nextState) {
            handler(nextState);
          }
          return;
        }

        if (event.type === "shell.action" && event.source === "window") {
          void publishWindowChromeState();
        }
      });

      if (!getIsDesktopEnv()) {
        return cleanupDesktopEvents;
      }

      try {
        const window = await getDesktopWindow();
        const cleanupFns = (
          await Promise.all(
            [
              window.onFocusChanged?.(() => {
                void publishWindowChromeState();
              }),
              window.onMoved?.(() => {
                void publishWindowChromeState();
              }),
              window.onResized?.(() => {
                void publishWindowChromeState();
              }),
              window.onScaleChanged?.(() => {
                void publishWindowChromeState();
              }),
            ].filter(Boolean),
          )
        ) as Array<() => void>;

        return () => {
          cleanupDesktopEvents();
          cleanupFns.forEach((cleanup) => cleanup());
        };
      } catch {
        return cleanupDesktopEvents;
      }
    },
    async toggleMaximizeMainWindow(): Promise<DesktopShellActionResult> {
      return this.performShellAction({
        actionId: "toggle_maximize_main_window",
        source: "window",
      });
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
