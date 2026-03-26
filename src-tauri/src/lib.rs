use reqwest::Client;
use serde::{Deserialize, Serialize};
use serde_json::{json, Value};
use std::sync::{Arc, Mutex};
use std::thread;
use std::time::{Duration, SystemTime, UNIX_EPOCH};
use tauri::tray::{MouseButton, MouseButtonState, TrayIconBuilder, TrayIconEvent};
use tauri::{AppHandle, Emitter, Manager, Runtime, State};
use tauri_plugin_dialog::{DialogExt, FilePath};
use tauri_plugin_global_shortcut::GlobalShortcutExt;
use tauri_plugin_notification::NotificationExt;
use tauri_plugin_shell::{
    process::{CommandChild, CommandEvent, TerminatedPayload},
    ShellExt,
};

const BACKEND_LABEL: &str = "backend";
const BACKEND_PORT: u16 = 7777;
const BRIDGE_LABEL: &str = "bridge";
const BRIDGE_PORT: u16 = 7778;
const DESKTOP_EVENT_NAME: &str = "agentforge://desktop-event";
const MAX_RESTART_ATTEMPTS: u32 = 2;
const TRAY_ID: &str = "agentforge-main-tray";

#[derive(Clone, Copy, Debug, Default, Deserialize, PartialEq, Eq, Serialize)]
#[serde(rename_all = "lowercase")]
enum RuntimeStatus {
    Degraded,
    Ready,
    Starting,
    #[default]
    Stopped,
}

#[derive(Clone, Debug, Serialize)]
#[serde(rename_all = "camelCase")]
struct DesktopRuntimeUnit {
    label: String,
    status: RuntimeStatus,
    url: Option<String>,
    pid: Option<u32>,
    restart_count: u32,
    last_error: Option<String>,
    last_started_at: Option<String>,
}

#[derive(Clone, Debug, Serialize)]
#[serde(rename_all = "camelCase")]
struct DesktopRuntimeSnapshot {
    overall: RuntimeStatus,
    backend: DesktopRuntimeUnit,
    bridge: DesktopRuntimeUnit,
}

#[derive(Clone, Debug, Serialize)]
#[serde(rename_all = "camelCase")]
struct DesktopEventPayload {
    #[serde(rename = "type")]
    event_type: String,
    source: String,
    runtime: Option<DesktopRuntimeSnapshot>,
    shortcut: Option<String>,
    payload: Option<Value>,
    timestamp: String,
}

#[derive(Clone, Debug, Default, Serialize)]
#[serde(rename_all = "camelCase")]
struct PluginRuntimeSummary {
    active_runtime_count: usize,
    backend_healthy: bool,
    bridge_healthy: bool,
    bridge_plugin_count: usize,
    event_bridge_available: bool,
    last_updated_at: Option<String>,
    warnings: Vec<String>,
}

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
    overall: RuntimeStatus,
}

impl DesktopRuntimeState {
    fn new() -> Self {
        Self {
            backend: ManagedRuntimeState::new(format!("http://127.0.0.1:{BACKEND_PORT}")),
            bridge: ManagedRuntimeState::new(format!("http://127.0.0.1:{BRIDGE_PORT}")),
            overall: RuntimeStatus::Stopped,
        }
    }

    fn runtime_mut(&mut self, label: &str) -> &mut ManagedRuntimeState {
        match label {
            BACKEND_LABEL => &mut self.backend,
            BRIDGE_LABEL => &mut self.bridge,
            _ => &mut self.backend,
        }
    }

    fn recalculate_overall(&mut self) {
        self.overall = match (self.backend.status, self.bridge.status) {
            (RuntimeStatus::Ready, RuntimeStatus::Ready) => RuntimeStatus::Ready,
            (RuntimeStatus::Stopped, RuntimeStatus::Stopped) => RuntimeStatus::Stopped,
            (RuntimeStatus::Degraded, _)
            | (_, RuntimeStatus::Degraded)
            | (RuntimeStatus::Ready, RuntimeStatus::Stopped)
            | (RuntimeStatus::Stopped, RuntimeStatus::Ready) => RuntimeStatus::Degraded,
            _ => RuntimeStatus::Starting,
        };
    }

    fn snapshot(&self) -> DesktopRuntimeSnapshot {
        DesktopRuntimeSnapshot {
            overall: self.overall,
            backend: self.backend.snapshot(BACKEND_LABEL),
            bridge: self.bridge.snapshot(BRIDGE_LABEL),
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
                    let _ = window.set_focus();
                }
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

            if runtime.status == RuntimeStatus::Stopped {
                state.recalculate_overall();
                return;
            }

            let message = format!(
                "{label} sidecar terminated (code: {:?}, signal: {:?})",
                payload.code, payload.signal
            );

            if runtime.restart_count < self.max_restart_attempts {
                runtime.restart_count += 1;
                runtime.status = RuntimeStatus::Starting;
                runtime.last_error = Some(format!(
                    "{message}; restart attempt {}/{}",
                    runtime.restart_count, self.max_restart_attempts
                ));
                state.recalculate_overall();
                true
            } else {
                runtime.status = RuntimeStatus::Degraded;
                runtime.last_error = Some(format!("{message}; restart limit reached"));
                state.recalculate_overall();
                false
            }
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

            if self.start_backend(app.clone(), true).await {
                let _ = self.start_bridge(app.clone(), true).await;
            }
            return;
        }

        let _ = self.start_bridge(app, true).await;
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
        let mut summary = PluginRuntimeSummary {
            backend_healthy: snapshot.backend.status == RuntimeStatus::Ready,
            bridge_healthy: snapshot.bridge.status == RuntimeStatus::Ready,
            event_bridge_available: snapshot.overall != RuntimeStatus::Stopped,
            last_updated_at: Some(now_string()),
            ..PluginRuntimeSummary::default()
        };

        let Some(bridge_url) = snapshot.bridge.url else {
            summary
                .warnings
                .push("Bridge URL is not available in the current desktop snapshot.".to_string());
            return summary;
        };

        if let Ok(response) = self
            .client
            .get(format!("{bridge_url}/bridge/plugins"))
            .send()
            .await
        {
            if let Ok(payload) = response.json::<Value>().await {
                summary.bridge_plugin_count = payload["plugins"]
                    .as_array()
                    .map(|entries| entries.len())
                    .unwrap_or_default();
            }
        } else {
            summary
                .warnings
                .push("Bridge plugin summary is temporarily unavailable.".to_string());
        }

        if let Ok(response) = self.client.get(format!("{bridge_url}/active")).send().await {
            if let Ok(payload) = response.json::<Value>().await {
                summary.active_runtime_count = payload
                    .as_array()
                    .map(|entries| entries.len())
                    .unwrap_or_default();
            }
        } else {
            summary
                .warnings
                .push("Bridge active-runtime summary is temporarily unavailable.".to_string());
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
struct DesktopNotificationRequest {
    notification_id: String,
    notification_type: String,
    title: String,
    body: String,
    href: Option<String>,
    created_at: String,
    delivery_policy: Option<String>,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
struct DesktopNotificationResult {
    notification_id: String,
    status: String,
}

fn notification_outcome_event_type(status: &str) -> &'static str {
    match status {
        "suppressed" => "notification.suppressed",
        "failed" => "notification.failed",
        _ => "notification.delivered",
    }
}

fn notification_outcome_payload(
    request: &DesktopNotificationRequest,
    status: &str,
    error: Option<String>,
) -> Value {
    let mut payload = serde_json::Map::new();
    payload.insert(
        "notificationId".to_string(),
        Value::String(request.notification_id.clone()),
    );
    payload.insert(
        "notificationType".to_string(),
        Value::String(request.notification_type.clone()),
    );
    payload.insert("title".to_string(), Value::String(request.title.clone()));
    payload.insert("body".to_string(), Value::String(request.body.clone()));
    payload.insert(
        "href".to_string(),
        request.href.clone().map(Value::String).unwrap_or(Value::Null),
    );
    payload.insert(
        "createdAt".to_string(),
        Value::String(request.created_at.clone()),
    );
    payload.insert(
        "deliveryPolicy".to_string(),
        request
            .delivery_policy
            .clone()
            .map(Value::String)
            .unwrap_or(Value::Null),
    );
    payload.insert("status".to_string(), Value::String(status.to_string()));

    if let Some(error) = error {
        payload.insert("error".to_string(), Value::String(error));
    }

    Value::Object(payload)
}

fn now_string() -> String {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|duration| duration.as_secs().to_string())
        .unwrap_or_else(|_| "0".to_string())
}

fn file_path_to_string(path: FilePath) -> Option<String> {
    path.into_path()
        .ok()
        .map(|resolved| resolved.to_string_lossy().into_owned())
}

#[cfg(not(any(target_os = "android", target_os = "ios")))]
fn updater_plugin_builder() -> tauri_plugin_updater::Builder {
    let mut builder = tauri_plugin_updater::Builder::new();
    if let Some(pubkey) = std::env::var("TAURI_UPDATER_PUBKEY")
        .ok()
        .or_else(|| std::env::var("AGENTFORGE_TAURI_UPDATER_PUBKEY").ok())
        .filter(|value| !value.trim().is_empty())
    {
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

    let multiple = options.multiple.unwrap_or(false);
    let directory = options.directory.unwrap_or(false);

    let paths = if directory && multiple {
        dialog
            .blocking_pick_folders()
            .unwrap_or_default()
            .into_iter()
            .filter_map(file_path_to_string)
            .collect::<Vec<_>>()
    } else if directory {
        dialog
            .blocking_pick_folder()
            .and_then(file_path_to_string)
            .map(|path| vec![path])
            .unwrap_or_default()
    } else if multiple {
        dialog
            .blocking_pick_files()
            .unwrap_or_default()
            .into_iter()
            .filter_map(file_path_to_string)
            .collect::<Vec<_>>()
    } else {
        dialog
            .blocking_pick_file()
            .and_then(file_path_to_string)
            .map(|path| vec![path])
            .unwrap_or_default()
    };

    Ok(paths)
}

#[tauri::command]
fn send_notification<R: Runtime>(
    app: AppHandle<R>,
    state: State<'_, DesktopRuntimeManager>,
    request: DesktopNotificationRequest,
) -> Result<DesktopNotificationResult, String> {
    let should_suppress = matches!(
        request.delivery_policy.as_deref(),
        Some("suppress_if_focused")
    ) && app
        .get_webview_window("main")
        .and_then(|window| window.is_focused().ok())
        .unwrap_or(false);

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

    if let Err(error) = app
        .notification()
        .builder()
        .title(request.title.clone())
        .body(request.body.clone())
        .show()
    {
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
        .invoke_handler(tauri::generate_handler![
            get_backend_url,
            get_desktop_runtime_status,
            get_plugin_runtime_summary,
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
}
