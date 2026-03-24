## 1. Tauri Baseline And Packaging Alignment

- [ ] 1.1 Align `src-tauri/tauri.conf.json`, Cargo metadata, window title, and bundle identifier with AgentForge instead of template defaults.
- [ ] 1.2 Update Tauri bundle and build configuration so the desktop package accounts for both required sidecars and their runtime arguments.
- [ ] 1.3 Add or confirm the minimal Rust plugins, guest packages, and capability permissions needed for runtime events, notifications, dialogs, tray behavior, shortcuts, and update checks.

## 2. Desktop Sidecar Supervision

- [ ] 2.1 Introduce a shared desktop runtime state model in `src-tauri` that tracks backend, bridge, and overall desktop status, including last error and restart count.
- [ ] 2.2 Implement ordered startup for the Go orchestrator and TS bridge sidecars, including ready-state detection and resolved endpoint storage for frontend consumption.
- [ ] 2.3 Implement unexpected-exit handling, bounded restart behavior, and degraded-state reporting for managed sidecars.

## 3. Native Capability Facade

- [ ] 3.1 Add Tauri commands for native file selection, system notifications, tray updates, global shortcut registration, and update checks.
- [ ] 3.2 Create a shared frontend platform-capability abstraction and hook that routes to Tauri commands in desktop mode and documented fallbacks or explicit unsupported results in web mode.
- [ ] 3.3 Replace direct page-level desktop capability calls with the shared abstraction so future desktop features use one contract.

## 4. Desktop Event And Plugin Runtime Bridge

- [ ] 4.1 Expose a desktop runtime status query and normalized desktop runtime event stream from Tauri to the frontend.
- [ ] 4.2 Add read-only desktop helper commands or event forwarding for plugin and runtime summaries without bypassing the existing backend control plane.
- [ ] 4.3 Update plugin or runtime-facing frontend surfaces to consume desktop status and event enhancements while preserving backend API as the authoritative source.

## 5. Verification And Delivery Readiness

- [ ] 5.1 Add focused tests or harness coverage for desktop runtime state transitions, capability fallbacks, and desktop-event normalization.
- [ ] 5.2 Validate the desktop workflow in `pnpm tauri dev`, including startup success, degraded-mode behavior, and at least one native capability path plus its web fallback path.
- [ ] 5.3 Document the implemented desktop capability contract, fallback semantics, and remaining platform-specific limitations for future follow-on work.
