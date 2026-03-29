## 1. Platform Capability And Event Contract

- [x] 1.1 Extend `lib/platform-runtime.ts` and `hooks/use-platform-capability.ts` with normalized shell action and main-window control APIs plus explicit web fallback results
- [x] 1.2 Define the shared shell action event envelope and update desktop-event typings so runtime, notification, shell action, and plugin lifecycle projections can flow through one subscription API
- [x] 1.3 Add or update focused tests for the platform facade covering desktop success, web fallback, and shell action event normalization

## 2. Tauri Shell Interaction Wiring

- [x] 2.1 Add `src-tauri/src/lib.rs` commands and event emitters for supported main-window control actions and normalized shell action results
- [x] 2.2 Introduce a minimal tray or native menu action registry that emits supported route-first or read-only quick actions without creating Tauri-only plugin mutations
- [x] 2.3 Wire desktop notification activation into the same shell action contract, preserving notification identifiers and target context before restoring or focusing the main window
- [x] 2.4 Update `src-tauri/capabilities/*.json` and any related config needed for the new shell interaction surface

## 3. Frontend Coordination And Plugin Projection

- [x] 3.1 Add a desktop coordination layer in the authenticated shell to subscribe to normalized shell actions and perform router handoff or supported fallback behavior
- [x] 3.2 Extend the frontend realtime path to consume backend `plugin.lifecycle` events and project them into the unified desktop event stream without replacing existing backend truth
- [x] 3.3 Update desktop-aware plugin or runtime surfaces to reflect shell interaction outcomes and plugin lifecycle projection availability while preserving current backend control-plane actions

## 4. Verification And Delivery Readiness

- [x] 4.1 Add or update tests for dashboard-shell or equivalent coordination code covering notification activation, route handoff, and unsupported desktop cases
- [x] 4.2 Add or update tests for plugin lifecycle desktop-event projection and shell action result handling on affected plugin-facing surfaces
- [x] 4.3 Run focused frontend and Tauri-adjacent verification for the touched desktop capability, shell coordination, and plugin event files, then record any platform-specific gaps truthfully
