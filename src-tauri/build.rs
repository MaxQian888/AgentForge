use std::collections::HashSet;
use std::path::Path;
use std::process::Command;

#[path = "src/process_cleanup.rs"]
mod process_cleanup;

/// Kill any running AgentForge-owned sidecar processes so that tauri-build can
/// overwrite the binaries inside the current checkout without touching
/// unrelated local processes.
fn kill_stale_sidecars() {
    #[cfg(target_os = "windows")]
    {
        let output = match Command::new("tasklist").arg("/FO").arg("CSV").output() {
            Ok(output) => output,
            Err(_) => return,
        };
        let stdout = String::from_utf8_lossy(&output.stdout);
        let repo_root = Path::new(env!("CARGO_MANIFEST_DIR"))
            .parent()
            .unwrap_or_else(|| Path::new(env!("CARGO_MANIFEST_DIR")));
        let mut killed = HashSet::new();

        for process in
            process_cleanup::collect_windows_named_sidecar_processes(&stdout, std::process::id())
        {
            let Some(executable_path) =
                process_cleanup::windows_process_executable_path(process.pid)
            else {
                continue;
            };
            if !process_cleanup::is_agentforge_owned_sidecar_path(
                repo_root,
                Path::new(&executable_path),
            ) {
                continue;
            }

            if killed.insert(process.pid) {
                println!(
                    "cargo:warning=killing stale AgentForge sidecar process: {} (PID {})",
                    process.image_name, process.pid
                );
                let _ = Command::new("taskkill")
                    .args(["/F", "/PID", &process.pid.to_string()])
                    .output();
            }
        }
    }

    #[cfg(not(target_os = "windows"))]
    {
        let repo_root = Path::new(env!("CARGO_MANIFEST_DIR"))
            .parent()
            .unwrap_or_else(|| Path::new(env!("CARGO_MANIFEST_DIR")));
        let binary_dirs: Vec<std::path::PathBuf> = [
            "src-tauri/binaries",
            "src-tauri/target/debug",
            "src-tauri/target/release",
            "target/debug",
            "target/release",
        ]
        .iter()
        .map(|d| repo_root.join(d))
        .collect();
        let mut killed = HashSet::new();

        for name in ["server", "bridge", "im-bridge"] {
            let output = match Command::new("pgrep").args(["-x", name]).output() {
                Ok(output) => output,
                Err(_) => continue,
            };
            let stdout = String::from_utf8_lossy(&output.stdout);
            for pid_str in stdout.split_whitespace() {
                let Ok(pid) = pid_str.parse::<u32>() else {
                    continue;
                };
                if pid == 0 || pid == std::process::id() {
                    continue;
                }

                // Resolve /proc/<pid>/exe to verify this is an AgentForge binary
                let exe_path = match std::fs::read_link(format!("/proc/{pid}/exe")) {
                    Ok(path) => path,
                    Err(_) => continue,
                };
                let is_owned = binary_dirs.iter().any(|dir| exe_path.starts_with(dir));
                if is_owned && killed.insert(pid) {
                    println!(
                        "cargo:warning=killing stale AgentForge sidecar: {name} (PID {pid})"
                    );
                    let _ = Command::new("kill").args(["-9", pid_str]).output();
                }
            }
        }
    }
}

fn main() {
    kill_stale_sidecars();
    tauri_build::build()
}
