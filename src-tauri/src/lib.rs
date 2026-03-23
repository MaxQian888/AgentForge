use std::sync::Mutex;
use tauri::State;
use tauri_plugin_shell::ShellExt;

/// Shared state to store the Go backend URL.
struct BackendState {
    url: Mutex<String>,
}

/// Tauri command: returns the Go backend base URL to the frontend.
#[tauri::command]
fn get_backend_url(state: State<BackendState>) -> String {
    state.url.lock().unwrap().clone()
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    const BACKEND_PORT: u16 = 7777;

    tauri::Builder::default()
        .plugin(tauri_plugin_log::Builder::new().build())
        .plugin(tauri_plugin_shell::init())
        .manage(BackendState {
            url: Mutex::new(format!("http://localhost:{}", BACKEND_PORT)),
        })
        .setup(|app| {
            let shell = app.shell();

            // Only spawn the sidecar when running in desktop mode.
            // In web-only mode (pnpm dev without tauri), there is no sidecar.
            let port_arg = BACKEND_PORT.to_string();
            let sidecar_cmd = match shell.sidecar("server") {
                Ok(cmd) => cmd,
                Err(e) => {
                    log::warn!("server sidecar not configured: {}. Running without backend.", e);
                    return Ok(());
                }
            };
            match sidecar_cmd.args(["--port", &port_arg]).spawn()
            {
                Ok((mut rx, _child)) => {
                    // Log sidecar stdout/stderr in background
                    tauri::async_runtime::spawn(async move {
                        use tauri_plugin_shell::process::CommandEvent;
                        while let Some(event) = rx.recv().await {
                            match event {
                                CommandEvent::Stdout(line) => {
                                    log::info!("[server] {}", String::from_utf8_lossy(&line));
                                }
                                CommandEvent::Stderr(line) => {
                                    log::warn!("[server] {}", String::from_utf8_lossy(&line));
                                }
                                CommandEvent::Error(err) => {
                                    log::error!("[server] error: {}", err);
                                }
                                CommandEvent::Terminated(status) => {
                                    log::info!("[server] terminated: {:?}", status);
                                    break;
                                }
                                _ => {}
                            }
                        }
                    });
                }
                Err(e) => {
                    log::warn!("Could not spawn Go sidecar: {}. Running without backend.", e);
                }
            }

            Ok(())
        })
        .invoke_handler(tauri::generate_handler![get_backend_url])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
