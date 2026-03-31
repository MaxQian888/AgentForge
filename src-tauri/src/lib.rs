use reqwest::Client;
use serde::{Deserialize, Serialize};
use serde_json::{json, Value};
use std::sync::{Arc, Mutex};
use std::thread;
use std::time::Duration;
use tauri::menu::{Menu, MenuItem};
use tauri::tray::{MouseButton, MouseButtonState, TrayIconBuilder, TrayIconEvent};
use tauri::{AppHandle, Emitter, Manager, Runtime, State};
use tauri_plugin_dialog::{DialogExt, FilePath};
use tauri_plugin_global_shortcut::GlobalShortcutExt;
use tauri_plugin_notification::NotificationExt;
use tauri_plugin_shell::{
    process::{CommandChild, CommandEvent, TerminatedPayload},
    ShellExt,
};

mod runtime_logic;

use crate::runtime_logic::{
    active_runtime_count, active_runtime_summary_unavailable_warning, bridge_plugin_count,
    bridge_plugin_summary_unavailable_warning, bridge_url_unavailable_warning,
    build_plugin_runtime_summary, build_shell_action_event, classify_shell_action,
    compute_overall_status, compute_termination_outcome, menu_action_href,
    notification_outcome_event_type, notification_outcome_payload, now_string,
    resolve_updater_pubkey, select_files_mode, should_suppress_notification, window_state_payload,
    DesktopEventPayload, DesktopNotificationRequest, DesktopNotificationResult,
    DesktopRuntimeSnapshot, DesktopRuntimeUnit, DesktopWindowChromeState, PluginRuntimeSummary,
    RuntimeStatus, SelectFilesMode, ShellActionKind,
};

#[cfg(test)]
use crate::runtime_logic::shell_action_event_payload;

const BACKEND_LABEL: &str = "backend";
const BACKEND_PORT: u16 = 7777;
const BRIDGE_LABEL: &str = "bridge";
const BRIDGE_PORT: u16 = 7778;
const IM_BRIDGE_LABEL: &str = "im-bridge";
const IM_BRIDGE_PORT: u16 = 7779;
const IM_BRIDGE_TEST_PORT: u16 = 7780;
const DESKTOP_EVENT_NAME: &str = "agentforge://desktop-event";
const MAX_RESTART_ATTEMPTS: u32 = 2;
const TRAY_ID: &str = "agentforge-main-tray";

#[derive(Debug)]
struct ManagedRuntimeState {
    child: Option<CommandChild>,
    last_error: Option<String>,
    last_started_at: Option<String>,
    pid: Option<u32>,
    restart_count: u32,
    status: RuntimeStatus,
    url: Option<String>,
}

impl ManagedRuntimeState {
    fn new(url: String) -> Self {
        Self {
            child: None,
            last_error: None,
            last_started_at: None,
            pid: None,
            restart_count: 0,
            status: RuntimeStatus::Stopped,
            url: Some(url),
        }
    }

    fn snapshot(&self, label: &str) -> DesktopRuntimeUnit {
        DesktopRuntimeUnit {
            label: label.to_string(),
            status: self.status,
            url: self.url.clone(),
            pid: self.pid,
            restart_count: self.restart_count,
            last_error: self.last_error.clone(),
            last_started_at: self.last_started_at.clone(),
        }
    }
}

#[derive(Debug)]
struct DesktopRuntimeState {
    backend: ManagedRuntimeState,
    bridge: ManagedRuntimeState,
    im_bridge: ManagedRuntimeState,
    overall: RuntimeStatus,
}

impl DesktopRuntimeState {
    fn new() -> Self {
        Self {
            backend: ManagedRuntimeState::new(format!("http://127.0.0.1:{BACKEND_PORT}")),
            bridge: ManagedRuntimeState::new(format!("http://127.0.0.1:{BRIDGE_PORT}")),
            im_bridge: ManagedRuntimeState::new(format!("http://127.0.0.1:{IM_BRIDGE_PORT}")),
            overall: RuntimeStatus::Stopped,
        }
    }

    fn runtime_mut(&mut self, label: &str) -> &mut ManagedRuntimeState {
        match label {
            BACKEND_LABEL => &mut self.backend,
            BRIDGE_LABEL => &mut self.bridge,
            IM_BRIDGE_LABEL => &mut self.im_bridge,
            _ => &mut self.backend,
        }
    }

    fn recalculate_overall(&mut self) {
        self.overall = compute_overall_status(
            self.backend.status,
            self.bridge.status,
            self.im_bridge.status,
        );
    }

    fn snapshot(&self) -> DesktopRuntimeSnapshot {
        DesktopRuntimeSnapshot {
            overall: self.overall,
            backend: self.backend.snapshot(BACKEND_LABEL),
            bridge: self.bridge.snapshot(BRIDGE_LABEL),
            im_bridge: self.im_bridge.snapshot(IM_BRIDGE_LABEL),
        }
    }
}

#[derive(Clone)]
struct DesktopRuntimeManager {
    client: Client,
    inner: Arc<Mutex<DesktopRuntimeState>>,
    max_restart_attempts: u32,
}

impl DesktopRuntimeManager {
    fn new() -> Self {
        Self {
            client: Client::builder()
                .timeout(Duration::from_secs(2))
                .build()
                .unwrap_or_else(|_| Client::new()),
            inner: Arc::new(Mutex::new(DesktopRuntimeState::new())),
            max_restart_attempts: MAX_RESTART_ATTEMPTS,
        }
    }

    fn snapshot(&self) -> DesktopRuntimeSnapshot {
        self.inner.lock().unwrap().snapshot()
    }

    fn backend_url(&self) -> String {
        self.snapshot()
            .backend
            .url
            .unwrap_or_else(|| format!("http://localhost:{BACKEND_PORT}"))
    }

    fn mutate<F>(&self, mutator: F) -> DesktopRuntimeSnapshot
    where
        F: FnOnce(&mut DesktopRuntimeState),
    {
        let mut state = self.inner.lock().unwrap();
        mutator(&mut state);
        state.recalculate_overall();
        state.snapshot()
    }

    fn emit_event<R: Runtime>(
        &self,
        app: &AppHandle<R>,
        event_type: impl Into<String>,
        source: impl Into<String>,
        shortcut: Option<String>,
        payload: Option<Value>,
        runtime: Option<DesktopRuntimeSnapshot>,
    ) {
        let event = DesktopEventPayload {
            event_type: event_type.into(),
            source: source.into(),
            action_id: None,
            href: None,
            status: None,
            runtime,
            shortcut,
            payload,
            timestamp: now_string(),
        };

        if let Err(error) = app.emit(DESKTOP_EVENT_NAME, event) {
            log::warn!("failed to emit desktop event: {error}");
        }
    }

    fn emit_runtime_event<R: Runtime>(
        &self,
        app: &AppHandle<R>,
        event_type: impl Into<String>,
        source: impl Into<String>,
        payload: Option<Value>,
    ) {
        self.emit_event(
            app,
            event_type,
            source,
            None,
            payload,
            Some(self.snapshot()),
        );
    }

    fn emit_system_event<R: Runtime>(
        &self,
        app: &AppHandle<R>,
        event_type: impl Into<String>,
        source: impl Into<String>,
        shortcut: Option<String>,
        payload: Option<Value>,
    ) {
        self.emit_event(app, event_type, source, shortcut, payload, None);
    }

    fn emit_shell_action_event<R: Runtime>(
        &self,
        app: &AppHandle<R>,
        source: impl Into<String>,
        action_id: impl Into<String>,
        href: Option<String>,
        payload: Option<Value>,
        status: impl Into<String>,
    ) {
        let event =
            build_shell_action_event(source, action_id, href, payload, status, now_string());

        if let Err(error) = app.emit(DESKTOP_EVENT_NAME, event) {
            log::warn!("failed to emit desktop shell action event: {error}");
        }
    }

    fn mark_starting<R: Runtime>(&self, app: &AppHandle<R>, label: &str, pid: u32) {
        let snapshot = self.mutate(|state| {
            let runtime = state.runtime_mut(label);
            runtime.pid = Some(pid);
            runtime.status = RuntimeStatus::Starting;
            runtime.last_error = None;
            runtime.last_started_at = Some(now_string());
        });

        self.emit_event(
            app,
            "runtime.updated",
            label,
            None,
            Some(json!({
                "message": format!("{label} sidecar started"),
                "pid": pid,
                "status": "starting",
            })),
            Some(snapshot),
        );
    }

    fn mark_ready<R: Runtime>(&self, app: &AppHandle<R>, label: &str) {
        let snapshot = self.mutate(|state| {
            let runtime = state.runtime_mut(label);
            runtime.status = RuntimeStatus::Ready;
            runtime.last_error = None;
        });

        self.emit_event(
            app,
            "runtime.updated",
            label,
            None,
            Some(json!({
                "message": format!("{label} sidecar is healthy"),
                "status": "ready",
            })),
            Some(snapshot),
        );
    }

    fn mark_degraded<R: Runtime>(&self, app: &AppHandle<R>, label: &str, error: impl Into<String>) {
        let message = error.into();
        let snapshot = self.mutate(|state| {
            let runtime = state.runtime_mut(label);
            runtime.status = RuntimeStatus::Degraded;
            runtime.last_error = Some(message.clone());
        });

        self.emit_event(
            app,
            "runtime.updated",
            label,
            None,
            Some(json!({
                "message": message,
                "status": "degraded",
            })),
            Some(snapshot),
        );
    }

    fn stop_runtime(&self, label: &str, reason: Option<String>) -> Option<CommandChild> {
        let mut state = self.inner.lock().unwrap();
        let runtime = state.runtime_mut(label);
        let child = runtime.child.take();
        runtime.pid = None;
        runtime.status = RuntimeStatus::Stopped;
        runtime.last_error = reason;
        state.recalculate_overall();
        child
    }

    fn ensure_tray<R: Runtime>(&self, app: &AppHandle<R>) -> Result<(), String> {
        if app.tray_by_id(TRAY_ID).is_some() {
            return Ok(());
        }

        let mut builder = TrayIconBuilder::with_id(TRAY_ID)
            .tooltip("AgentForge desktop runtime")
            .title("AgentForge")
            .show_menu_on_left_click(false);
        if let Some(icon) = app.default_window_icon().cloned() {
            builder = builder.icon(icon);
        }

        let manager = self.clone();
        builder = builder.on_tray_icon_event(move |tray, event| {
            if let TrayIconEvent::Click {
                button: MouseButton::Left,
                button_state: MouseButtonState::Up,
                ..
            } = event
            {
                if let Some(window) = tray.app_handle().get_webview_window("main") {
                    let _ = window.show();
                    let _ = window.unminimize();
                    let _ = window.set_focus();
                }
                manager.emit_shell_action_event(
                    tray.app_handle(),
                    "tray",
                    "focus_main_window",
                    None,
                    Some(json!({ "trayId": TRAY_ID })),
                    "completed",
                );
                manager.emit_system_event(
                    tray.app_handle(),
                    "tray.clicked",
                    "tray",
                    None,
                    Some(json!({ "trayId": TRAY_ID })),
                );
            }
        });

        builder
            .build(app)
            .map_err(|error: tauri::Error| error.to_string())?;
        Ok(())
    }

    async fn bootstrap<R: Runtime>(&self, app: AppHandle<R>) {
        self.emit_runtime_event(
            &app,
            "runtime.updated",
            "desktop",
            Some(json!({ "message": "Starting desktop runtimes" })),
        );

        let backend_ready = self.start_backend(app.clone(), false).await;
        if backend_ready {
            let _ = self.start_bridge(app.clone(), false).await;
            let _ = self.start_im_bridge(app.clone(), false).await;
        }
    }

    async fn start_backend<R: Runtime>(&self, app: AppHandle<R>, is_restart: bool) -> bool {
        let port_arg = BACKEND_PORT.to_string();
        let command = match app.shell().sidecar("server") {
            Ok(command) => command
                .env("SCHEDULER_EXECUTION_MODE", "os_registered")
                .args(["--port", &port_arg]),
            Err(error) => {
                self.mark_degraded(
                    &app,
                    BACKEND_LABEL,
                    format!("backend sidecar not configured: {error}"),
                );
                return false;
            }
        };

        match command.spawn() {
            Ok((rx, child)) => {
                let pid = child.pid();
                self.mutate(|state| {
                    let runtime = state.runtime_mut(BACKEND_LABEL);
                    runtime.child = Some(child);
                });
                self.mark_starting(&app, BACKEND_LABEL, pid);
                self.watch_sidecar_events(app.clone(), BACKEND_LABEL, rx);

                let health_urls = [
                    format!("http://127.0.0.1:{BACKEND_PORT}/health"),
                    format!("http://127.0.0.1:{BACKEND_PORT}/api/v1/health"),
                ];
                if self.wait_for_health(&health_urls).await {
                    self.mark_ready(&app, BACKEND_LABEL);
                    true
                } else {
                    self.mark_degraded(
                        &app,
                        BACKEND_LABEL,
                        if is_restart {
                            "backend health check timed out after restart"
                        } else {
                            "backend health check timed out during startup"
                        },
                    );
                    false
                }
            }
            Err(error) => {
                self.mark_degraded(
                    &app,
                    BACKEND_LABEL,
                    format!("failed to spawn backend sidecar: {error}"),
                );
                false
            }
        }
    }

    async fn start_bridge<R: Runtime>(&self, app: AppHandle<R>, is_restart: bool) -> bool {
        let backend_url = self.backend_url();
        let backend_ws_url = format!("ws://127.0.0.1:{BACKEND_PORT}/ws/bridge");
        let bridge_port = BRIDGE_PORT.to_string();
        let command = match app.shell().sidecar("bridge") {
            Ok(command) => command
                .env("GO_API_URL", &backend_url)
                .env("GO_WS_URL", &backend_ws_url)
                .env("BRIDGE_SCHEDULER_MODE", "desktop")
                .env("PORT", &bridge_port),
            Err(error) => {
                self.mark_degraded(
                    &app,
                    BRIDGE_LABEL,
                    format!("bridge sidecar not configured: {error}"),
                );
                return false;
            }
        };

        match command.spawn() {
            Ok((rx, child)) => {
                let pid = child.pid();
                self.mutate(|state| {
                    let runtime = state.runtime_mut(BRIDGE_LABEL);
                    runtime.child = Some(child);
                });
                self.mark_starting(&app, BRIDGE_LABEL, pid);
                self.watch_sidecar_events(app.clone(), BRIDGE_LABEL, rx);

                let health_urls = [
                    format!("http://127.0.0.1:{BRIDGE_PORT}/health"),
                    format!("http://127.0.0.1:{BRIDGE_PORT}/bridge/health"),
                ];
                if self.wait_for_health(&health_urls).await {
                    self.mark_ready(&app, BRIDGE_LABEL);
                    true
                } else {
                    self.mark_degraded(
                        &app,
                        BRIDGE_LABEL,
                        if is_restart {
                            "bridge health check timed out after restart"
                        } else {
                            "bridge health check timed out during startup"
                        },
                    );
                    false
                }
            }
            Err(error) => {
                self.mark_degraded(
                    &app,
                    BRIDGE_LABEL,
                    format!("failed to spawn bridge sidecar: {error}"),
                );
                false
            }
        }
    }

    fn im_bridge_id_file<R: Runtime>(&self, app: &AppHandle<R>) -> String {
        let primary_runtime_dir = app
            .path()
            .app_data_dir()
            .unwrap_or_else(|_| std::env::temp_dir().join("agentforge-desktop"))
            .join("runtime");
        let fallback_runtime_dir = std::env::temp_dir().join("agentforge-desktop-runtime");

        for candidate in [primary_runtime_dir, fallback_runtime_dir] {
            if std::fs::create_dir_all(&candidate).is_ok() {
                return candidate
                    .join("im-bridge-id")
                    .to_string_lossy()
                    .into_owned();
            }
        }

        std::env::temp_dir()
            .join("agentforge-desktop-runtime")
            .join("im-bridge-id")
            .to_string_lossy()
            .into_owned()
    }

    async fn start_im_bridge<R: Runtime>(&self, app: AppHandle<R>, is_restart: bool) -> bool {
        let backend_url = self.backend_url();
        let notify_port = IM_BRIDGE_PORT.to_string();
        let test_port = IM_BRIDGE_TEST_PORT.to_string();
        let im_bridge_id_file = self.im_bridge_id_file(&app);
        let project_scope = std::env::var("AGENTFORGE_PROJECT_ID")
            .ok()
            .map(|value| value.trim().to_string())
            .filter(|value| !value.is_empty());
        let command = match app.shell().sidecar(IM_BRIDGE_LABEL) {
            Ok(command) => {
                let command = command
                    .env("AGENTFORGE_API_BASE", &backend_url)
                    .env("IM_BRIDGE_ID_FILE", &im_bridge_id_file)
                    .env("IM_PLATFORM", "feishu")
                    .env("IM_TRANSPORT_MODE", "stub")
                    .env("NOTIFY_PORT", &notify_port)
                    .env("TEST_PORT", &test_port);
                if let Some(project_scope) = project_scope.as_deref() {
                    command.env("AGENTFORGE_PROJECT_ID", project_scope)
                } else {
                    command
                }
            }
            Err(error) => {
                self.mark_degraded(
                    &app,
                    IM_BRIDGE_LABEL,
                    format!("IM bridge sidecar not configured: {error}"),
                );
                return false;
            }
        };

        match command.spawn() {
            Ok((rx, child)) => {
                let pid = child.pid();
                self.mutate(|state| {
                    let runtime = state.runtime_mut(IM_BRIDGE_LABEL);
                    runtime.child = Some(child);
                });
                self.mark_starting(&app, IM_BRIDGE_LABEL, pid);
                self.watch_sidecar_events(app.clone(), IM_BRIDGE_LABEL, rx);

                let health_urls = [format!("http://127.0.0.1:{IM_BRIDGE_PORT}/im/health")];
                if self.wait_for_health(&health_urls).await {
                    self.mark_ready(&app, IM_BRIDGE_LABEL);
                    true
                } else {
                    self.mark_degraded(
                        &app,
                        IM_BRIDGE_LABEL,
                        if is_restart {
                            "IM bridge health check timed out after restart"
                        } else {
                            "IM bridge health check timed out during startup"
                        },
                    );
                    false
                }
            }
            Err(error) => {
                self.mark_degraded(
                    &app,
                    IM_BRIDGE_LABEL,
                    format!("failed to spawn IM bridge sidecar: {error}"),
                );
                false
            }
        }
    }

    fn watch_sidecar_events<R: Runtime>(
        &self,
        app: AppHandle<R>,
        label: &'static str,
        mut receiver: tauri::async_runtime::Receiver<CommandEvent>,
    ) {
        let manager = self.clone();
        tauri::async_runtime::spawn(async move {
            while let Some(event) = receiver.recv().await {
                match event {
                    CommandEvent::Stdout(line) => {
                        log::info!("[{label}] {}", String::from_utf8_lossy(&line));
                    }
                    CommandEvent::Stderr(line) => {
                        log::warn!("[{label}] {}", String::from_utf8_lossy(&line));
                    }
                    CommandEvent::Error(error) => {
                        manager.mark_degraded(&app, label, error);
                    }
                    CommandEvent::Terminated(payload) => {
                        manager
                            .handle_termination(app.clone(), label, payload)
                            .await;
                        break;
                    }
                    _ => {}
                }
            }
        });
    }

    async fn handle_termination<R: Runtime>(
        &self,
        app: AppHandle<R>,
        label: &'static str,
        payload: TerminatedPayload,
    ) {
        let should_restart = {
            let mut state = self.inner.lock().unwrap();
            let runtime = state.runtime_mut(label);
            runtime.child = None;
            runtime.pid = None;
            let outcome = compute_termination_outcome(
                runtime.status,
                runtime.restart_count,
                self.max_restart_attempts,
                label,
                payload.code,
                payload.signal,
            );

            runtime.status = outcome.next_status;
            runtime.restart_count = outcome.next_restart_count;
            runtime.last_error = outcome.last_error;
            state.recalculate_overall();

            let Some(should_restart) = outcome.should_restart else {
                return;
            };

            should_restart
        };

        self.emit_runtime_event(
            &app,
            "runtime.terminated",
            label,
            Some(json!({
                "code": payload.code,
                "signal": payload.signal,
                "willRestart": should_restart,
            })),
        );

        if !should_restart {
            return;
        }

        if label == BACKEND_LABEL {
            if let Some(child) = self.stop_runtime(
                BRIDGE_LABEL,
                Some("bridge stopped while backend restarts".to_string()),
            ) {
                let _ = child.kill();
            }
            if let Some(child) = self.stop_runtime(
                IM_BRIDGE_LABEL,
                Some("IM bridge stopped while backend restarts".to_string()),
            ) {
                let _ = child.kill();
            }

            if self.start_backend(app.clone(), true).await {
                let _ = self.start_bridge(app.clone(), true).await;
                let _ = self.start_im_bridge(app.clone(), true).await;
            }
            return;
        }

        if label == BRIDGE_LABEL {
            let _ = self.start_bridge(app, true).await;
            return;
        }

        if label == IM_BRIDGE_LABEL {
            let _ = self.start_im_bridge(app, true).await;
        }
    }

    async fn wait_for_health(&self, urls: &[String]) -> bool {
        for _ in 0..20 {
            for url in urls {
                match self.client.get(url).send().await {
                    Ok(response) if response.status().is_success() => return true,
                    Ok(_) | Err(_) => {}
                }
            }

            thread::sleep(Duration::from_millis(500));
        }

        false
    }

    async fn plugin_runtime_summary(&self) -> PluginRuntimeSummary {
        let snapshot = self.snapshot();
        let mut summary = build_plugin_runtime_summary(&snapshot, now_string());

        let Some(bridge_url) = snapshot.bridge.url else {
            summary.warnings.push(bridge_url_unavailable_warning());
            return summary;
        };

        if let Ok(response) = self
            .client
            .get(format!("{bridge_url}/bridge/plugins"))
            .send()
            .await
        {
            if let Ok(payload) = response.json::<Value>().await {
                summary.bridge_plugin_count = bridge_plugin_count(&payload);
            }
        } else {
            summary
                .warnings
                .push(bridge_plugin_summary_unavailable_warning());
        }

        if let Ok(response) = self.client.get(format!("{bridge_url}/active")).send().await {
            if let Ok(payload) = response.json::<Value>().await {
                summary.active_runtime_count = active_runtime_count(&payload);
            }
        } else {
            summary
                .warnings
                .push(active_runtime_summary_unavailable_warning());
        }

        summary
    }
}

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
struct SelectFilesOptions {
    directory: Option<bool>,
    filters: Option<Vec<SelectFilesFilter>>,
    multiple: Option<bool>,
    title: Option<String>,
}

#[derive(Debug, Deserialize)]
struct SelectFilesFilter {
    extensions: Option<Vec<String>>,
    name: String,
}

#[derive(Debug, Deserialize)]
struct ShortcutRequest {
    accelerator: String,
    event: String,
}

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
struct DesktopShellActionRequest {
    action_id: String,
    href: Option<String>,
    payload: Option<Value>,
    source: String,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
struct DesktopShellActionResult {
    action_id: String,
    status: String,
}

fn file_path_to_string(path: FilePath) -> Option<String> {
    path.into_path()
        .ok()
        .map(|resolved| resolved.to_string_lossy().into_owned())
}

#[cfg(not(any(target_os = "android", target_os = "ios")))]
fn updater_plugin_builder() -> tauri_plugin_updater::Builder {
    let mut builder = tauri_plugin_updater::Builder::new();
    if let Some(pubkey) = resolve_updater_pubkey(
        std::env::var("TAURI_UPDATER_PUBKEY").ok(),
        std::env::var("AGENTFORGE_TAURI_UPDATER_PUBKEY").ok(),
    ) {
        builder = builder.pubkey(pubkey);
    }

    builder
}

#[tauri::command]
fn get_backend_url(state: State<'_, DesktopRuntimeManager>) -> String {
    state.backend_url()
}

#[tauri::command]
fn get_desktop_runtime_status(state: State<'_, DesktopRuntimeManager>) -> DesktopRuntimeSnapshot {
    state.snapshot()
}

#[tauri::command]
async fn get_plugin_runtime_summary(
    state: State<'_, DesktopRuntimeManager>,
) -> Result<PluginRuntimeSummary, String> {
    Ok(state.plugin_runtime_summary().await)
}

#[tauri::command]
fn select_files<R: Runtime>(
    app: AppHandle<R>,
    options: SelectFilesOptions,
) -> Result<Vec<String>, String> {
    let mut dialog = app.dialog().file();
    if let Some(title) = options.title {
        dialog = dialog.set_title(title);
    }

    if let Some(filters) = options.filters {
        for filter in filters {
            let extensions = filter.extensions.unwrap_or_default();
            let extension_refs = extensions.iter().map(String::as_str).collect::<Vec<_>>();
            dialog = dialog.add_filter(filter.name, &extension_refs);
        }
    }

    let paths = match select_files_mode(
        options.directory.unwrap_or(false),
        options.multiple.unwrap_or(false),
    ) {
        SelectFilesMode::Folders => dialog
            .blocking_pick_folders()
            .unwrap_or_default()
            .into_iter()
            .filter_map(file_path_to_string)
            .collect::<Vec<_>>(),
        SelectFilesMode::Folder => dialog
            .blocking_pick_folder()
            .and_then(file_path_to_string)
            .map(|path| vec![path])
            .unwrap_or_default(),
        SelectFilesMode::Files => dialog
            .blocking_pick_files()
            .unwrap_or_default()
            .into_iter()
            .filter_map(file_path_to_string)
            .collect::<Vec<_>>(),
        SelectFilesMode::File => dialog
            .blocking_pick_file()
            .and_then(file_path_to_string)
            .map(|path| vec![path])
            .unwrap_or_default(),
    };

    Ok(paths)
}

#[tauri::command]
fn send_notification<R: Runtime>(
    app: AppHandle<R>,
    state: State<'_, DesktopRuntimeManager>,
    request: DesktopNotificationRequest,
) -> Result<DesktopNotificationResult, String> {
    let should_suppress = should_suppress_notification(
        request.delivery_policy.as_deref(),
        app.get_webview_window("main")
            .and_then(|window| window.is_focused().ok())
            .unwrap_or(false),
    );

    if should_suppress {
        let status = "suppressed";
        state.emit_system_event(
            &app,
            notification_outcome_event_type(status),
            "notification",
            None,
            Some(notification_outcome_payload(&request, status, None)),
        );
        return Ok(DesktopNotificationResult {
            notification_id: request.notification_id,
            status: status.to_string(),
        });
    }

    let mut notification_builder = app
        .notification()
        .builder()
        .title(request.title.clone())
        .body(request.body.clone())
        .auto_cancel()
        .extra("notificationId", request.notification_id.clone())
        .extra("notificationType", request.notification_type.clone())
        .extra("createdAt", request.created_at.clone());

    if let Some(href) = request.href.clone() {
        notification_builder = notification_builder.extra("href", href);
    }

    if let Err(error) = notification_builder.show() {
        let message = error.to_string();
        state.emit_system_event(
            &app,
            notification_outcome_event_type("failed"),
            "notification",
            None,
            Some(notification_outcome_payload(
                &request,
                "failed",
                Some(message.clone()),
            )),
        );
        return Err(message);
    }

    let status = "delivered";
    state.emit_system_event(
        &app,
        notification_outcome_event_type(status),
        "notification",
        None,
        Some(notification_outcome_payload(&request, status, None)),
    );
    Ok(DesktopNotificationResult {
        notification_id: request.notification_id,
        status: status.to_string(),
    })
}

#[tauri::command]
fn update_tray<R: Runtime>(
    app: AppHandle<R>,
    state: State<'_, DesktopRuntimeManager>,
    title: Option<String>,
    tooltip: Option<String>,
    visible: Option<bool>,
) -> Result<(), String> {
    state.ensure_tray(&app)?;

    let tray = app
        .tray_by_id(TRAY_ID)
        .ok_or_else(|| "AgentForge tray is not available.".to_string())?;
    if let Some(next_title) = title.as_deref() {
        tray.set_title(Some(next_title))
            .map_err(|error: tauri::Error| error.to_string())?;
    }
    if let Some(next_tooltip) = tooltip.as_deref() {
        tray.set_tooltip(Some(next_tooltip))
            .map_err(|error: tauri::Error| error.to_string())?;
    }
    if let Some(next_visible) = visible {
        tray.set_visible(next_visible)
            .map_err(|error: tauri::Error| error.to_string())?;
    }

    state.emit_system_event(
        &app,
        "tray.updated",
        "tray",
        None,
        Some(json!({
            "title": title,
            "tooltip": tooltip,
            "visible": visible,
        })),
    );
    Ok(())
}

#[tauri::command]
fn register_shortcut<R: Runtime>(
    app: AppHandle<R>,
    state: State<'_, DesktopRuntimeManager>,
    request: ShortcutRequest,
) -> Result<(), String> {
    if app
        .global_shortcut()
        .is_registered(request.accelerator.as_str())
    {
        app.global_shortcut()
            .unregister(request.accelerator.as_str())
            .map_err(|error| error.to_string())?;
    }

    let manager = state.inner().clone();
    let accelerator = request.accelerator.clone();
    let event_name = request.event.clone();
    let closure_accelerator = accelerator.clone();
    let closure_event_name = event_name.clone();
    let shortcut_key = request.accelerator;
    app.global_shortcut()
        .on_shortcut(
            shortcut_key.as_str(),
            move |app_handle, _shortcut, _event| {
                manager.emit_system_event(
                    app_handle,
                    "shortcut.triggered",
                    "shortcut",
                    Some(closure_accelerator.clone()),
                    Some(json!({ "event": closure_event_name })),
                );
            },
        )
        .map_err(|error| error.to_string())?;

    state.emit_system_event(
        &app,
        "shortcut.registered",
        "shortcut",
        Some(accelerator.clone()),
        Some(json!({ "event": event_name })),
    );
    Ok(())
}

#[tauri::command]
fn unregister_shortcut<R: Runtime>(
    app: AppHandle<R>,
    state: State<'_, DesktopRuntimeManager>,
    accelerator: String,
) -> Result<(), String> {
    if app.global_shortcut().is_registered(accelerator.as_str()) {
        app.global_shortcut()
            .unregister(accelerator.as_str())
            .map_err(|error| error.to_string())?;
    }

    state.emit_system_event(
        &app,
        "shortcut.unregistered",
        "shortcut",
        Some(accelerator),
        None,
    );
    Ok(())
}

fn with_main_window<R: Runtime>(app: &AppHandle<R>) -> Result<tauri::WebviewWindow<R>, String> {
    app.get_webview_window("main")
        .ok_or_else(|| "AgentForge main window is not available.".to_string())
}

fn read_main_window_chrome_state<R: Runtime>(
    app: &AppHandle<R>,
) -> Result<DesktopWindowChromeState, String> {
    let window = with_main_window(app)?;

    Ok(DesktopWindowChromeState {
        focused: window.is_focused().map_err(|error| error.to_string())?,
        maximized: window.is_maximized().map_err(|error| error.to_string())?,
        minimized: window.is_minimized().map_err(|error| error.to_string())?,
        visible: window.is_visible().map_err(|error| error.to_string())?,
    })
}

fn emit_window_state_event<R: Runtime>(
    app: &AppHandle<R>,
    state: &DesktopRuntimeManager,
) -> Result<(), String> {
    let window_state = read_main_window_chrome_state(app)?;
    state.emit_system_event(
        app,
        "window.state",
        "window",
        None,
        Some(window_state_payload(&window_state)),
    );
    Ok(())
}

#[tauri::command]
fn get_window_chrome_state<R: Runtime>(
    app: AppHandle<R>,
) -> Result<DesktopWindowChromeState, String> {
    read_main_window_chrome_state(&app)
}

#[tauri::command]
fn perform_shell_action<R: Runtime>(
    app: AppHandle<R>,
    state: State<'_, DesktopRuntimeManager>,
    request: DesktopShellActionRequest,
) -> Result<DesktopShellActionResult, String> {
    let window = with_main_window(&app)?;

    match classify_shell_action(request.action_id.as_str()) {
        ShellActionKind::FocusMainWindow => {
            let _ = window.show();
            let _ = window.unminimize();
            window.set_focus().map_err(|error| error.to_string())?;
        }
        ShellActionKind::ToggleMaximizeMainWindow => {
            let _ = window.show();
            let _ = window.unminimize();
            if window.is_maximized().map_err(|error| error.to_string())? {
                window.unmaximize().map_err(|error| error.to_string())?;
            } else {
                window.maximize().map_err(|error| error.to_string())?;
            }
            let _ = window.set_focus();
        }
        ShellActionKind::MinimizeMainWindow => {
            window.minimize().map_err(|error| error.to_string())?;
        }
        ShellActionKind::CloseMainWindow => {
            state.emit_shell_action_event(
                &app,
                request.source.clone(),
                request.action_id.clone(),
                request.href.clone(),
                request.payload.clone(),
                "completed",
            );
            window.close().map_err(|error| error.to_string())?;
            return Ok(DesktopShellActionResult {
                action_id: request.action_id,
                status: "completed".to_string(),
            });
        }
        ShellActionKind::FocusRoute => {
            let _ = window.show();
            let _ = window.unminimize();
            let _ = window.set_focus();
        }
        ShellActionKind::Unsupported => {
            return Err(format!(
                "Unsupported AgentForge desktop shell action: {}",
                request.action_id
            ));
        }
    }

    state.emit_shell_action_event(
        &app,
        request.source,
        request.action_id.clone(),
        request.href.clone(),
        request.payload.clone(),
        "completed",
    );
    let _ = emit_window_state_event(&app, state.inner());

    Ok(DesktopShellActionResult {
        action_id: request.action_id,
        status: "completed".to_string(),
    })
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    let runtime_manager = DesktopRuntimeManager::new();
    let setup_manager = runtime_manager.clone();

    tauri::Builder::default()
        .plugin(tauri_plugin_process::init())
        .plugin(tauri_plugin_notification::init())
        .plugin(tauri_plugin_dialog::init())
        .plugin(tauri_plugin_log::Builder::new().build())
        .plugin(tauri_plugin_shell::init())
        .plugin(tauri_plugin_global_shortcut::Builder::new().build())
        .plugin(updater_plugin_builder().build())
        .manage(runtime_manager)
        .setup(move |app| {
            let plugins_item =
                MenuItem::with_id(app, "open_plugins", "Open Plugins", true, None::<&str>)
                    .map_err(|error| std::io::Error::other(error.to_string()))?;
            let reviews_item =
                MenuItem::with_id(app, "open_reviews", "Open Reviews", true, None::<&str>)
                    .map_err(|error| std::io::Error::other(error.to_string()))?;
            let menu = Menu::with_items(app, &[&plugins_item, &reviews_item])
                .map_err(|error| std::io::Error::other(error.to_string()))?;
            app.set_menu(menu)
                .map_err(|error| std::io::Error::other(error.to_string()))?;

            if let Err(error) = setup_manager.ensure_tray(app.handle()) {
                return Err(std::io::Error::other(error).into());
            }

            let manager = setup_manager.clone();
            let app_handle = app.handle().clone();
            tauri::async_runtime::spawn(async move {
                manager.bootstrap(app_handle).await;
            });

            Ok(())
        })
        .on_menu_event(|app, event| {
            let action_id = event.id().0.to_string();
            let href = menu_action_href(action_id.as_str());

            if let Some(window) = app.get_webview_window("main") {
                let _ = window.show();
                let _ = window.unminimize();
                let _ = window.set_focus();
            }

            if let Some(manager) = app.try_state::<DesktopRuntimeManager>() {
                manager.emit_shell_action_event(app, "menu", action_id, href, None, "completed");
            }
        })
        .invoke_handler(tauri::generate_handler![
            get_backend_url,
            get_desktop_runtime_status,
            get_window_chrome_state,
            get_plugin_runtime_summary,
            perform_shell_action,
            register_shortcut,
            select_files,
            send_notification,
            unregister_shortcut,
            update_tray,
        ])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn notification_outcome_payload_preserves_business_metadata() {
        let request = DesktopNotificationRequest {
            notification_id: "notification-1".to_string(),
            notification_type: "task_progress_stalled".to_string(),
            title: "Task stalled: Implement detector".to_string(),
            body: "Task Implement detector is stalled.".to_string(),
            href: Some("/project?id=project-1#task-task-1".to_string()),
            created_at: "2026-03-26T10:00:00.000Z".to_string(),
            delivery_policy: Some("always".to_string()),
        };

        assert_eq!(
            notification_outcome_payload(&request, "delivered", None),
            json!({
                "notificationId": "notification-1",
                "notificationType": "task_progress_stalled",
                "title": "Task stalled: Implement detector",
                "body": "Task Implement detector is stalled.",
                "href": "/project?id=project-1#task-task-1",
                "createdAt": "2026-03-26T10:00:00.000Z",
                "deliveryPolicy": "always",
                "status": "delivered",
            })
        );
    }

    #[test]
    fn notification_outcome_event_type_maps_statuses() {
        assert_eq!(
            notification_outcome_event_type("delivered"),
            "notification.delivered"
        );
        assert_eq!(
            notification_outcome_event_type("suppressed"),
            "notification.suppressed"
        );
        assert_eq!(
            notification_outcome_event_type("failed"),
            "notification.failed"
        );
    }

    #[test]
    fn notification_outcome_payload_includes_error_and_null_optionals_when_missing() {
        let request = DesktopNotificationRequest {
            notification_id: "notification-2".to_string(),
            notification_type: "review_ready".to_string(),
            title: "Review ready".to_string(),
            body: "A review is ready.".to_string(),
            href: None,
            created_at: "2026-03-30T10:00:00.000Z".to_string(),
            delivery_policy: None,
        };

        assert_eq!(
            notification_outcome_payload(
                &request,
                "failed",
                Some("notification backend unavailable".to_string()),
            ),
            json!({
                "notificationId": "notification-2",
                "notificationType": "review_ready",
                "title": "Review ready",
                "body": "A review is ready.",
                "href": Value::Null,
                "createdAt": "2026-03-30T10:00:00.000Z",
                "deliveryPolicy": Value::Null,
                "status": "failed",
                "error": "notification backend unavailable",
            })
        );
    }

    #[test]
    fn runtime_status_serializes_using_lowercase_contract() {
        assert_eq!(
            serde_json::to_value(RuntimeStatus::Starting).unwrap(),
            json!("starting")
        );
        assert_eq!(
            serde_json::to_value(RuntimeStatus::Degraded).unwrap(),
            json!("degraded")
        );
    }

    #[test]
    fn runtime_state_recalculate_overall_covers_primary_status_combinations() {
        let cases = [
            (
                RuntimeStatus::Ready,
                RuntimeStatus::Ready,
                RuntimeStatus::Ready,
                RuntimeStatus::Ready,
            ),
            (
                RuntimeStatus::Stopped,
                RuntimeStatus::Stopped,
                RuntimeStatus::Stopped,
                RuntimeStatus::Stopped,
            ),
            (
                RuntimeStatus::Degraded,
                RuntimeStatus::Ready,
                RuntimeStatus::Ready,
                RuntimeStatus::Degraded,
            ),
            (
                RuntimeStatus::Ready,
                RuntimeStatus::Ready,
                RuntimeStatus::Stopped,
                RuntimeStatus::Degraded,
            ),
            (
                RuntimeStatus::Ready,
                RuntimeStatus::Ready,
                RuntimeStatus::Stopped,
                RuntimeStatus::Degraded,
            ),
            (
                RuntimeStatus::Starting,
                RuntimeStatus::Ready,
                RuntimeStatus::Ready,
                RuntimeStatus::Starting,
            ),
        ];

        for (backend_status, bridge_status, im_bridge_status, expected) in cases {
            let mut state = DesktopRuntimeState::new();
            state.backend.status = backend_status;
            state.bridge.status = bridge_status;
            state.im_bridge.status = im_bridge_status;

            state.recalculate_overall();

            assert_eq!(
                state.overall, expected,
                "expected overall status {expected:?} for backend={backend_status:?}, bridge={bridge_status:?}, im_bridge={im_bridge_status:?}"
            );
        }
    }

    #[test]
    fn runtime_mut_routes_bridge_and_im_bridge_labels_and_defaults_unknown_to_backend() {
        let mut state = DesktopRuntimeState::new();

        state.runtime_mut(BRIDGE_LABEL).status = RuntimeStatus::Ready;
        state.runtime_mut(IM_BRIDGE_LABEL).status = RuntimeStatus::Ready;
        state.runtime_mut("unknown-runtime").status = RuntimeStatus::Degraded;

        assert_eq!(state.bridge.status, RuntimeStatus::Ready);
        assert_eq!(state.im_bridge.status, RuntimeStatus::Ready);
        assert_eq!(state.backend.status, RuntimeStatus::Degraded);
    }

    #[test]
    fn runtime_manager_mutate_updates_snapshot_metadata_and_labels() {
        let manager = DesktopRuntimeManager::new();

        let snapshot = manager.mutate(|state| {
            state.backend.status = RuntimeStatus::Ready;
            state.backend.pid = Some(42);
            state.backend.restart_count = 1;
            state.backend.last_error = Some("recovered from bootstrap failure".to_string());
            state.backend.last_started_at = Some("1234567890".to_string());
            state.bridge.status = RuntimeStatus::Ready;
            state.im_bridge.status = RuntimeStatus::Ready;
        });

        assert_eq!(snapshot.overall, RuntimeStatus::Ready);
        assert_eq!(snapshot.backend.label, BACKEND_LABEL);
        assert_eq!(snapshot.backend.status, RuntimeStatus::Ready);
        assert_eq!(
            snapshot.backend.url.as_deref(),
            Some("http://127.0.0.1:7777")
        );
        assert_eq!(snapshot.backend.pid, Some(42));
        assert_eq!(snapshot.backend.restart_count, 1);
        assert_eq!(
            snapshot.backend.last_error.as_deref(),
            Some("recovered from bootstrap failure")
        );
        assert_eq!(
            snapshot.backend.last_started_at.as_deref(),
            Some("1234567890")
        );
        assert_eq!(snapshot.bridge.label, BRIDGE_LABEL);
        assert_eq!(
            snapshot.bridge.url.as_deref(),
            Some("http://127.0.0.1:7778")
        );
        assert_eq!(snapshot.im_bridge.label, IM_BRIDGE_LABEL);
        assert_eq!(
            snapshot.im_bridge.url.as_deref(),
            Some("http://127.0.0.1:7779")
        );
    }

    #[test]
    fn default_capability_allows_im_bridge_sidecar() {
        let capability: Value =
            serde_json::from_str(include_str!("../capabilities/default.json")).unwrap();
        let allow_list = capability["permissions"]
            .as_array()
            .and_then(|permissions| {
                permissions.iter().find_map(|permission| {
                    let object = permission.as_object()?;
                    let identifier = object.get("identifier")?.as_str()?;
                    if identifier == "shell:allow-execute" {
                        object.get("allow")?.as_array().cloned()
                    } else {
                        None
                    }
                })
            })
            .expect("shell:allow-execute allow list should exist");

        assert!(allow_list.iter().any(|entry| {
            entry["name"] == Value::String(IM_BRIDGE_LABEL.to_string())
                && entry["sidecar"] == Value::Bool(true)
        }));
    }

    #[test]
    fn backend_url_falls_back_to_localhost_when_runtime_url_is_missing() {
        let manager = DesktopRuntimeManager::new();

        manager.mutate(|state| {
            state.backend.url = None;
        });

        assert_eq!(
            manager.backend_url(),
            format!("http://localhost:{BACKEND_PORT}")
        );
    }

    #[test]
    fn stop_runtime_resets_runtime_state_and_preserves_reason() {
        let manager = DesktopRuntimeManager::new();

        manager.mutate(|state| {
            state.backend.status = RuntimeStatus::Ready;
            state.backend.pid = Some(7777);
            state.bridge.status = RuntimeStatus::Ready;
            state.im_bridge.status = RuntimeStatus::Ready;
        });

        let stopped = manager.stop_runtime(BACKEND_LABEL, Some("manual shutdown".to_string()));
        let snapshot = manager.snapshot();

        assert!(stopped.is_none());
        assert_eq!(snapshot.backend.status, RuntimeStatus::Stopped);
        assert_eq!(snapshot.backend.pid, None);
        assert_eq!(
            snapshot.backend.last_error.as_deref(),
            Some("manual shutdown")
        );
        assert_eq!(snapshot.overall, RuntimeStatus::Degraded);
    }

    #[test]
    fn shell_action_event_payload_preserves_route_and_context() {
        assert_eq!(
            shell_action_event_payload(
                "open_notification_target",
                Some("/reviews?id=review-1".to_string()),
                Some(json!({ "notificationId": "notification-1" })),
                "completed",
            ),
            json!({
                "actionId": "open_notification_target",
                "href": "/reviews?id=review-1",
                "payload": {
                    "notificationId": "notification-1"
                },
                "status": "completed",
            })
        );
    }

    #[test]
    fn shell_action_event_payload_omits_optional_fields_when_absent() {
        assert_eq!(
            shell_action_event_payload("refresh_plugin_runtime", None, None, "completed"),
            json!({
                "actionId": "refresh_plugin_runtime",
                "status": "completed",
            })
        );
    }

    #[test]
    fn window_state_payload_preserves_window_flags() {
        assert_eq!(
            window_state_payload(&DesktopWindowChromeState {
                focused: true,
                maximized: true,
                minimized: false,
                visible: true,
            }),
            json!({
                "focused": true,
                "maximized": true,
                "minimized": false,
                "visible": true,
            })
        );
    }

    #[test]
    fn now_string_returns_numeric_timestamp() {
        assert!(now_string().parse::<u64>().is_ok());
    }

    #[test]
    fn file_path_to_string_returns_lossy_owned_path() {
        let path = std::env::temp_dir().join("agentforge-desktop-test.txt");

        assert_eq!(
            file_path_to_string(FilePath::Path(path.clone())),
            Some(path.to_string_lossy().into_owned())
        );
    }
}
