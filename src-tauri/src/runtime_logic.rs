use serde::{Deserialize, Serialize};
use serde_json::{json, Value};
use std::time::{SystemTime, UNIX_EPOCH};

#[derive(Clone, Copy, Debug, Default, Deserialize, PartialEq, Eq, Serialize)]
#[serde(rename_all = "lowercase")]
pub(crate) enum RuntimeStatus {
    Degraded,
    Ready,
    Starting,
    #[default]
    Stopped,
}

#[derive(Clone, Debug, PartialEq, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct DesktopRuntimeUnit {
    pub(crate) label: String,
    pub(crate) status: RuntimeStatus,
    pub(crate) url: Option<String>,
    pub(crate) pid: Option<u32>,
    pub(crate) restart_count: u32,
    pub(crate) last_error: Option<String>,
    pub(crate) last_started_at: Option<String>,
}

#[derive(Clone, Debug, PartialEq, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct DesktopRuntimeSnapshot {
    pub(crate) overall: RuntimeStatus,
    pub(crate) backend: DesktopRuntimeUnit,
    pub(crate) bridge: DesktopRuntimeUnit,
}

#[derive(Clone, Debug, PartialEq, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct DesktopEventPayload {
    #[serde(rename = "type")]
    pub(crate) event_type: String,
    pub(crate) source: String,
    pub(crate) action_id: Option<String>,
    pub(crate) href: Option<String>,
    pub(crate) status: Option<String>,
    pub(crate) runtime: Option<DesktopRuntimeSnapshot>,
    pub(crate) shortcut: Option<String>,
    pub(crate) payload: Option<Value>,
    pub(crate) timestamp: String,
}

#[derive(Clone, Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct DesktopWindowChromeState {
    pub(crate) focused: bool,
    pub(crate) maximized: bool,
    pub(crate) minimized: bool,
    pub(crate) visible: bool,
}

#[derive(Clone, Debug, Default, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct PluginRuntimeSummary {
    pub(crate) active_runtime_count: usize,
    pub(crate) backend_healthy: bool,
    pub(crate) bridge_healthy: bool,
    pub(crate) bridge_plugin_count: usize,
    pub(crate) event_bridge_available: bool,
    pub(crate) last_updated_at: Option<String>,
    pub(crate) warnings: Vec<String>,
}

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct DesktopNotificationRequest {
    pub(crate) notification_id: String,
    pub(crate) notification_type: String,
    pub(crate) title: String,
    pub(crate) body: String,
    pub(crate) href: Option<String>,
    pub(crate) created_at: String,
    pub(crate) delivery_policy: Option<String>,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct DesktopNotificationResult {
    pub(crate) notification_id: String,
    pub(crate) status: String,
}

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub(crate) enum ShellActionKind {
    FocusMainWindow,
    ToggleMaximizeMainWindow,
    MinimizeMainWindow,
    CloseMainWindow,
    FocusRoute,
    Unsupported,
}

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub(crate) enum SelectFilesMode {
    File,
    Files,
    Folder,
    Folders,
}

#[derive(Clone, Debug, PartialEq, Eq)]
pub(crate) struct TerminationOutcome {
    pub(crate) should_restart: Option<bool>,
    pub(crate) next_status: RuntimeStatus,
    pub(crate) next_restart_count: u32,
    pub(crate) last_error: Option<String>,
}

pub(crate) fn compute_overall_status(
    backend_status: RuntimeStatus,
    bridge_status: RuntimeStatus,
) -> RuntimeStatus {
    match (backend_status, bridge_status) {
        (RuntimeStatus::Ready, RuntimeStatus::Ready) => RuntimeStatus::Ready,
        (RuntimeStatus::Stopped, RuntimeStatus::Stopped) => RuntimeStatus::Stopped,
        (RuntimeStatus::Degraded, _)
        | (_, RuntimeStatus::Degraded)
        | (RuntimeStatus::Ready, RuntimeStatus::Stopped)
        | (RuntimeStatus::Stopped, RuntimeStatus::Ready) => RuntimeStatus::Degraded,
        _ => RuntimeStatus::Starting,
    }
}

pub(crate) fn notification_outcome_event_type(status: &str) -> &'static str {
    match status {
        "suppressed" => "notification.suppressed",
        "failed" => "notification.failed",
        _ => "notification.delivered",
    }
}

pub(crate) fn notification_outcome_payload(
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
        request
            .href
            .clone()
            .map(Value::String)
            .unwrap_or(Value::Null),
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

pub(crate) fn shell_action_event_payload(
    action_id: &str,
    href: Option<String>,
    payload: Option<Value>,
    status: &str,
) -> Value {
    let mut event_payload = serde_json::Map::new();
    event_payload.insert("actionId".to_string(), Value::String(action_id.to_string()));
    event_payload.insert("status".to_string(), Value::String(status.to_string()));

    if let Some(href) = href {
        event_payload.insert("href".to_string(), Value::String(href));
    }

    if let Some(payload) = payload {
        event_payload.insert("payload".to_string(), payload);
    }

    Value::Object(event_payload)
}

pub(crate) fn build_shell_action_event(
    source: impl Into<String>,
    action_id: impl Into<String>,
    href: Option<String>,
    payload: Option<Value>,
    status: impl Into<String>,
    timestamp: String,
) -> DesktopEventPayload {
    let action_id = action_id.into();
    let status = status.into();

    DesktopEventPayload {
        event_type: "shell.action".to_string(),
        source: source.into(),
        action_id: Some(action_id.clone()),
        href: href.clone(),
        status: Some(status.clone()),
        runtime: None,
        shortcut: None,
        payload: Some(shell_action_event_payload(
            &action_id,
            href,
            payload,
            &status,
        )),
        timestamp,
    }
}

pub(crate) fn window_state_payload(state: &DesktopWindowChromeState) -> Value {
    json!({
        "focused": state.focused,
        "maximized": state.maximized,
        "minimized": state.minimized,
        "visible": state.visible,
    })
}

pub(crate) fn now_string() -> String {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|duration| duration.as_secs().to_string())
        .unwrap_or_else(|_| "0".to_string())
}

pub(crate) fn should_suppress_notification(
    delivery_policy: Option<&str>,
    main_window_focused: bool,
) -> bool {
    matches!(delivery_policy, Some("suppress_if_focused")) && main_window_focused
}

pub(crate) fn classify_shell_action(action_id: &str) -> ShellActionKind {
    match action_id {
        "focus_main_window" | "show_main_window" | "restore_main_window" => {
            ShellActionKind::FocusMainWindow
        }
        "toggle_maximize_main_window" => ShellActionKind::ToggleMaximizeMainWindow,
        "minimize_main_window" => ShellActionKind::MinimizeMainWindow,
        "close_main_window" => ShellActionKind::CloseMainWindow,
        "open_plugins" | "open_reviews" | "open_notification_target" | "refresh_plugin_runtime" => {
            ShellActionKind::FocusRoute
        }
        _ => ShellActionKind::Unsupported,
    }
}

pub(crate) fn menu_action_href(action_id: &str) -> Option<String> {
    match action_id {
        "open_plugins" => Some("/plugins".to_string()),
        "open_reviews" => Some("/reviews".to_string()),
        _ => None,
    }
}

pub(crate) fn select_files_mode(directory: bool, multiple: bool) -> SelectFilesMode {
    match (directory, multiple) {
        (true, true) => SelectFilesMode::Folders,
        (true, false) => SelectFilesMode::Folder,
        (false, true) => SelectFilesMode::Files,
        (false, false) => SelectFilesMode::File,
    }
}

pub(crate) fn compute_termination_outcome(
    current_status: RuntimeStatus,
    restart_count: u32,
    max_restart_attempts: u32,
    label: &str,
    code: Option<i32>,
    signal: Option<i32>,
) -> TerminationOutcome {
    if current_status == RuntimeStatus::Stopped {
        return TerminationOutcome {
            should_restart: None,
            next_status: RuntimeStatus::Stopped,
            next_restart_count: restart_count,
            last_error: None,
        };
    }

    let message = format!(
        "{label} sidecar terminated (code: {:?}, signal: {:?})",
        code, signal
    );

    if restart_count < max_restart_attempts {
        let next_restart_count = restart_count + 1;
        TerminationOutcome {
            should_restart: Some(true),
            next_status: RuntimeStatus::Starting,
            next_restart_count,
            last_error: Some(format!(
                "{message}; restart attempt {}/{}",
                next_restart_count, max_restart_attempts
            )),
        }
    } else {
        TerminationOutcome {
            should_restart: Some(false),
            next_status: RuntimeStatus::Degraded,
            next_restart_count: restart_count,
            last_error: Some(format!("{message}; restart limit reached")),
        }
    }
}

pub(crate) fn build_plugin_runtime_summary(
    snapshot: &DesktopRuntimeSnapshot,
    timestamp: String,
) -> PluginRuntimeSummary {
    PluginRuntimeSummary {
        backend_healthy: snapshot.backend.status == RuntimeStatus::Ready,
        bridge_healthy: snapshot.bridge.status == RuntimeStatus::Ready,
        event_bridge_available: snapshot.overall != RuntimeStatus::Stopped,
        last_updated_at: Some(timestamp),
        ..PluginRuntimeSummary::default()
    }
}

pub(crate) fn bridge_plugin_count(payload: &Value) -> usize {
    payload["plugins"]
        .as_array()
        .map(|entries| entries.len())
        .unwrap_or_default()
}

pub(crate) fn active_runtime_count(payload: &Value) -> usize {
    payload
        .as_array()
        .map(|entries| entries.len())
        .unwrap_or_default()
}

pub(crate) fn bridge_url_unavailable_warning() -> String {
    "Bridge URL is not available in the current desktop snapshot.".to_string()
}

pub(crate) fn bridge_plugin_summary_unavailable_warning() -> String {
    "Bridge plugin summary is temporarily unavailable.".to_string()
}

pub(crate) fn active_runtime_summary_unavailable_warning() -> String {
    "Bridge active-runtime summary is temporarily unavailable.".to_string()
}

pub(crate) fn resolve_updater_pubkey(
    tauri_updater_pubkey: Option<String>,
    agentforge_updater_pubkey: Option<String>,
) -> Option<String> {
    tauri_updater_pubkey
        .or(agentforge_updater_pubkey)
        .filter(|value| !value.trim().is_empty())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn compute_overall_status_covers_ready_degraded_and_starting_states() {
        assert_eq!(
            compute_overall_status(RuntimeStatus::Ready, RuntimeStatus::Ready),
            RuntimeStatus::Ready
        );
        assert_eq!(
            compute_overall_status(RuntimeStatus::Stopped, RuntimeStatus::Stopped),
            RuntimeStatus::Stopped
        );
        assert_eq!(
            compute_overall_status(RuntimeStatus::Ready, RuntimeStatus::Stopped),
            RuntimeStatus::Degraded
        );
        assert_eq!(
            compute_overall_status(RuntimeStatus::Starting, RuntimeStatus::Ready),
            RuntimeStatus::Starting
        );
    }

    #[test]
    fn notification_helpers_preserve_optional_fields_and_error_context() {
        let request = DesktopNotificationRequest {
            notification_id: "notification-1".to_string(),
            notification_type: "task".to_string(),
            title: "Task update".to_string(),
            body: "Task body".to_string(),
            href: None,
            created_at: "2026-03-30T10:00:00.000Z".to_string(),
            delivery_policy: None,
        };

        assert_eq!(
            notification_outcome_event_type("suppressed"),
            "notification.suppressed"
        );
        assert_eq!(
            notification_outcome_payload(&request, "failed", Some("boom".to_string())),
            json!({
                "notificationId": "notification-1",
                "notificationType": "task",
                "title": "Task update",
                "body": "Task body",
                "href": Value::Null,
                "createdAt": "2026-03-30T10:00:00.000Z",
                "deliveryPolicy": Value::Null,
                "status": "failed",
                "error": "boom",
            })
        );
    }

    #[test]
    fn shell_action_helpers_cover_routing_and_payload_shape() {
        assert_eq!(
            classify_shell_action("toggle_maximize_main_window"),
            ShellActionKind::ToggleMaximizeMainWindow
        );
        assert_eq!(
            classify_shell_action("refresh_plugin_runtime"),
            ShellActionKind::FocusRoute
        );
        assert_eq!(
            classify_shell_action("unknown"),
            ShellActionKind::Unsupported
        );
        assert_eq!(
            menu_action_href("open_plugins"),
            Some("/plugins".to_string())
        );
        assert_eq!(menu_action_href("unknown"), None);
        assert_eq!(
            shell_action_event_payload("refresh_plugin_runtime", None, None, "completed"),
            json!({
                "actionId": "refresh_plugin_runtime",
                "status": "completed",
            })
        );
        assert_eq!(
            build_shell_action_event(
                "notification",
                "open_notification_target",
                Some("/reviews?id=review-1".to_string()),
                Some(json!({ "notificationId": "notification-7" })),
                "completed",
                "1711800000".to_string(),
            ),
            DesktopEventPayload {
                event_type: "shell.action".to_string(),
                source: "notification".to_string(),
                action_id: Some("open_notification_target".to_string()),
                href: Some("/reviews?id=review-1".to_string()),
                status: Some("completed".to_string()),
                runtime: None,
                shortcut: None,
                payload: Some(json!({
                    "actionId": "open_notification_target",
                    "href": "/reviews?id=review-1",
                    "payload": {
                        "notificationId": "notification-7"
                    },
                    "status": "completed",
                })),
                timestamp: "1711800000".to_string(),
            }
        );
    }

    #[test]
    fn select_files_mode_covers_all_path_shapes() {
        assert_eq!(select_files_mode(false, false), SelectFilesMode::File);
        assert_eq!(select_files_mode(false, true), SelectFilesMode::Files);
        assert_eq!(select_files_mode(true, false), SelectFilesMode::Folder);
        assert_eq!(select_files_mode(true, true), SelectFilesMode::Folders);
    }

    #[test]
    fn termination_outcome_tracks_restart_budget() {
        assert_eq!(
            compute_termination_outcome(RuntimeStatus::Stopped, 1, 2, "backend", Some(0), None),
            TerminationOutcome {
                should_restart: None,
                next_status: RuntimeStatus::Stopped,
                next_restart_count: 1,
                last_error: None,
            }
        );
        assert_eq!(
            compute_termination_outcome(
                RuntimeStatus::Ready,
                0,
                2,
                "backend",
                Some(1),
                Some(9)
            ),
            TerminationOutcome {
                should_restart: Some(true),
                next_status: RuntimeStatus::Starting,
                next_restart_count: 1,
                last_error: Some(
                    "backend sidecar terminated (code: Some(1), signal: Some(9)); restart attempt 1/2"
                        .to_string(),
                ),
            }
        );
        assert_eq!(
            compute_termination_outcome(RuntimeStatus::Ready, 2, 2, "bridge", None, None),
            TerminationOutcome {
                should_restart: Some(false),
                next_status: RuntimeStatus::Degraded,
                next_restart_count: 2,
                last_error: Some(
                    "bridge sidecar terminated (code: None, signal: None); restart limit reached"
                        .to_string(),
                ),
            }
        );
    }

    #[test]
    fn plugin_summary_helpers_normalize_snapshot_payloads_and_warnings() {
        let snapshot = DesktopRuntimeSnapshot {
            overall: RuntimeStatus::Ready,
            backend: DesktopRuntimeUnit {
                label: "backend".to_string(),
                status: RuntimeStatus::Ready,
                url: Some("http://127.0.0.1:7777".to_string()),
                pid: Some(1),
                restart_count: 0,
                last_error: None,
                last_started_at: Some("1".to_string()),
            },
            bridge: DesktopRuntimeUnit {
                label: "bridge".to_string(),
                status: RuntimeStatus::Starting,
                url: Some("http://127.0.0.1:7778".to_string()),
                pid: Some(2),
                restart_count: 1,
                last_error: Some("warming".to_string()),
                last_started_at: Some("2".to_string()),
            },
        };

        let summary = build_plugin_runtime_summary(&snapshot, "123".to_string());

        assert!(summary.backend_healthy);
        assert!(!summary.bridge_healthy);
        assert!(summary.event_bridge_available);
        assert_eq!(summary.last_updated_at.as_deref(), Some("123"));
        assert_eq!(bridge_plugin_count(&json!({ "plugins": [1, 2, 3] })), 3);
        assert_eq!(active_runtime_count(&json!([1, 2])), 2);
        assert_eq!(
            bridge_url_unavailable_warning(),
            "Bridge URL is not available in the current desktop snapshot."
        );
        assert_eq!(
            bridge_plugin_summary_unavailable_warning(),
            "Bridge plugin summary is temporarily unavailable."
        );
        assert_eq!(
            active_runtime_summary_unavailable_warning(),
            "Bridge active-runtime summary is temporarily unavailable."
        );
    }

    #[test]
    fn misc_helpers_keep_expected_contracts() {
        assert!(should_suppress_notification(
            Some("suppress_if_focused"),
            true
        ));
        assert!(!should_suppress_notification(
            Some("suppress_if_focused"),
            false
        ));
        assert_eq!(
            window_state_payload(&DesktopWindowChromeState {
                focused: true,
                maximized: false,
                minimized: false,
                visible: true,
            }),
            json!({
                "focused": true,
                "maximized": false,
                "minimized": false,
                "visible": true,
            })
        );
        assert!(now_string().parse::<u64>().is_ok());
        assert_eq!(
            resolve_updater_pubkey(None, Some("fallback-key".to_string())),
            Some("fallback-key".to_string())
        );
        assert_eq!(
            resolve_updater_pubkey(Some("   ".to_string()), Some("fallback-key".to_string())),
            None
        );
    }
}
