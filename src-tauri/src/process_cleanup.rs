#![allow(dead_code)]

use std::collections::HashSet;
use std::ffi::OsStr;
use std::path::Path;
use std::process::Command;

const SIDECAR_IMAGE_PREFIXES: &[&str] = &["server", "bridge", "im-bridge"];

/// Build a `Command` that does not flash a console window on Windows.
///
/// On Windows, GUI-subsystem parents spawn console children with an attached
/// console window by default. `CREATE_NO_WINDOW` (0x0800_0000) suppresses it
/// without affecting stdio redirection. No-op on non-Windows platforms.
pub(crate) fn hidden_command(program: impl AsRef<OsStr>) -> Command {
    let mut command = Command::new(program);
    #[cfg(windows)]
    {
        use std::os::windows::process::CommandExt;
        const CREATE_NO_WINDOW: u32 = 0x0800_0000;
        command.creation_flags(CREATE_NO_WINDOW);
    }
    command
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub(crate) struct PortConflict {
    pub(crate) port: u16,
    pub(crate) pid: u32,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub(crate) struct NamedProcess {
    pub(crate) image_name: String,
    pub(crate) pid: u32,
}

pub(crate) fn is_sidecar_image_name(image_name: &str) -> bool {
    let image_lower = image_name.to_ascii_lowercase();
    SIDECAR_IMAGE_PREFIXES.iter().any(|prefix| {
        image_lower == format!("{prefix}.exe") || image_lower.starts_with(&format!("{prefix}-"))
    })
}

pub(crate) fn collect_windows_named_sidecar_processes(
    tasklist_output: &str,
    current_pid: u32,
) -> Vec<NamedProcess> {
    let mut matches = Vec::new();
    let mut seen = HashSet::new();

    for line in tasklist_output.lines().skip(1) {
        // CSV format: "Image Name","PID","Session Name","Session#","Mem Usage"
        let mut fields = line.splitn(3, ',');
        let Some(image) = fields.next() else {
            continue;
        };
        let Some(pid_raw) = fields.next() else {
            continue;
        };

        let image_name = image.trim_matches('"').trim().to_string();
        let Ok(pid) = pid_raw.trim_matches('"').trim().parse::<u32>() else {
            continue;
        };
        if pid == 0 || pid == current_pid || !is_sidecar_image_name(&image_name) {
            continue;
        }

        if seen.insert(pid) {
            matches.push(NamedProcess { image_name, pid });
        }
    }

    matches
}

pub(crate) fn is_agentforge_owned_sidecar_path(repo_root: &Path, exe_path: &Path) -> bool {
    let Some(file_name) = exe_path.file_name().and_then(|name| name.to_str()) else {
        return false;
    };
    if !is_sidecar_image_name(file_name) {
        return false;
    }

    let exe_normalized = normalize_path_for_match(exe_path);
    let repo_root_normalized = normalize_path_for_match(repo_root);
    let sep = platform_separator();

    [
        format!("{repo_root_normalized}{sep}src-tauri{sep}binaries{sep}"),
        format!("{repo_root_normalized}{sep}src-tauri{sep}target{sep}"),
        format!("{repo_root_normalized}{sep}target{sep}"),
    ]
    .into_iter()
    .any(|allowed_prefix| exe_normalized.starts_with(&allowed_prefix))
}

pub(crate) fn collect_windows_port_conflicts(
    netstat_output: &str,
    ports: &[u16],
    current_pid: u32,
) -> Vec<PortConflict> {
    let tracked_ports: HashSet<u16> = ports.iter().copied().collect();
    let mut seen = HashSet::new();
    let mut conflicts = Vec::new();

    for line in netstat_output.lines() {
        let parts: Vec<&str> = line.split_whitespace().collect();
        // netstat -ano lines: Proto  LocalAddress  ForeignAddress  State  PID
        if parts.len() < 5 || parts[3] != "LISTENING" {
            continue;
        }

        let Ok(pid) = parts[4].parse::<u32>() else {
            continue;
        };
        if pid == 0 || pid == current_pid {
            continue;
        }

        let Some(port) = extract_port(parts[1]) else {
            continue;
        };
        if !tracked_ports.contains(&port) || !seen.insert((port, pid)) {
            continue;
        }

        conflicts.push(PortConflict { port, pid });
    }

    conflicts.sort_by_key(|conflict| conflict.port);
    conflicts
}

pub(crate) fn format_port_conflict_message(
    label: &str,
    port: u16,
    pid: u32,
    executable_path: Option<&str>,
) -> String {
    match executable_path.filter(|path| !path.is_empty()) {
        Some(path) => format!(
            "{label} sidecar cannot start because port {port} is already in use by PID {pid} ({path})"
        ),
        None => format!(
            "{label} sidecar cannot start because port {port} is already in use by PID {pid}"
        ),
    }
}

#[cfg(target_os = "windows")]
pub(crate) fn windows_process_executable_path(pid: u32) -> Option<String> {
    let script = format!(
        "$process = Get-CimInstance Win32_Process -Filter \\\"ProcessId = {pid}\\\" -ErrorAction SilentlyContinue; if ($null -ne $process -and $null -ne $process.ExecutablePath) {{ [Console]::Out.Write($process.ExecutablePath) }}"
    );
    let output = hidden_command("powershell")
        .args(["-NoLogo", "-NoProfile", "-Command", &script])
        .output()
        .ok()?;
    if !output.status.success() {
        return None;
    }

    let stdout = String::from_utf8_lossy(&output.stdout).trim().to_string();
    if stdout.is_empty() {
        None
    } else {
        Some(stdout)
    }
}

#[cfg(not(target_os = "windows"))]
pub(crate) fn windows_process_executable_path(_pid: u32) -> Option<String> {
    None
}

fn extract_port(local_addr: &str) -> Option<u16> {
    local_addr
        .rsplit_once(':')
        .and_then(|(_, port)| port.parse::<u16>().ok())
}

fn platform_separator() -> char {
    std::path::MAIN_SEPARATOR
}

fn normalize_path_for_match(path: &Path) -> String {
    let mut normalized = path.to_string_lossy().to_string();

    #[cfg(windows)]
    {
        normalized = normalized.replace('/', "\\");
        if let Some(stripped) = normalized.strip_prefix(r"\\?\") {
            normalized = stripped.to_string();
        }
    }
    #[cfg(not(windows))]
    {
        normalized = normalized.replace('\\', "/");
    }

    let sep = platform_separator();
    while normalized.ends_with(sep) {
        normalized.pop();
    }

    normalized.to_ascii_lowercase()
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::path::PathBuf;

    #[test]
    fn collect_windows_named_sidecar_processes_filters_by_image_name_and_pid() {
        let tasklist_output = "\
\"Image Name\",\"PID\",\"Session Name\",\"Session#\",\"Mem Usage\"\r\n\
\"server.exe\",\"1200\",\"Console\",\"1\",\"12,000 K\"\r\n\
\"bridge-x86_64-pc-windows-msvc.exe\",\"1300\",\"Console\",\"1\",\"42,000 K\"\r\n\
\"not-agentforge.exe\",\"1400\",\"Console\",\"1\",\"8,000 K\"\r\n\
\"im-bridge-x86_64-pc-windows-msvc.exe\",\"1500\",\"Console\",\"1\",\"10,000 K\"\r\n";

        assert_eq!(
            collect_windows_named_sidecar_processes(tasklist_output, 1500),
            vec![
                NamedProcess {
                    image_name: "server.exe".to_string(),
                    pid: 1200,
                },
                NamedProcess {
                    image_name: "bridge-x86_64-pc-windows-msvc.exe".to_string(),
                    pid: 1300,
                },
            ]
        );
    }

    #[test]
    fn agentforge_owned_sidecar_paths_are_scoped_to_this_checkout() {
        let repo_root = PathBuf::from(r"D:\Project\AgentForge");

        assert!(is_agentforge_owned_sidecar_path(
            &repo_root,
            Path::new(
                r"D:\Project\AgentForge\src-tauri\binaries\server-x86_64-pc-windows-msvc.exe"
            )
        ));
        assert!(is_agentforge_owned_sidecar_path(
            &repo_root,
            Path::new(
                r"D:\Project\AgentForge\src-tauri\target\debug\bridge-x86_64-pc-windows-msvc.exe"
            )
        ));

        assert!(!is_agentforge_owned_sidecar_path(
            &repo_root,
            Path::new(r"C:\tools\server.exe")
        ));
        assert!(!is_agentforge_owned_sidecar_path(
            &repo_root,
            Path::new(r"D:\Project\OtherRepo\src-tauri\binaries\server-x86_64-pc-windows-msvc.exe")
        ));
    }

    #[test]
    fn collect_windows_port_conflicts_returns_matching_listeners_without_self_pid() {
        let netstat_output = "\
  Proto  Local Address          Foreign Address        State           PID\r\n\
  TCP    127.0.0.1:7777         0.0.0.0:0              LISTENING       4242\r\n\
  TCP    127.0.0.1:7778         0.0.0.0:0              LISTENING       9001\r\n\
  TCP    127.0.0.1:3000         0.0.0.0:0              LISTENING       3333\r\n\
  TCP    127.0.0.1:7779         0.0.0.0:0              LISTENING       1000\r\n";

        assert_eq!(
            collect_windows_port_conflicts(netstat_output, &[7777, 7778, 7779], 1000),
            vec![
                PortConflict {
                    port: 7777,
                    pid: 4242
                },
                PortConflict {
                    port: 7778,
                    pid: 9001
                },
            ]
        );
    }

    #[test]
    fn format_port_conflict_message_includes_executable_path_when_known() {
        assert_eq!(
            format_port_conflict_message(
                "backend",
                7777,
                4242,
                Some(r"D:\Project\AgentForge\src-tauri\binaries\server-x86_64-pc-windows-msvc.exe"),
            ),
            "backend sidecar cannot start because port 7777 is already in use by PID 4242 (D:\\Project\\AgentForge\\src-tauri\\binaries\\server-x86_64-pc-windows-msvc.exe)"
        );
    }
}
