## 1. Capability Contract And Facade Baseline

- [x] 1.1 Add the `desktop-window-chrome` capability and update the `desktop-native-capabilities`, `desktop-shell-actions`, and `desktop-runtime-event-bridge` requirements for frameless main-window support.
- [x] 1.2 Extend `lib/platform-runtime.ts` and `hooks/use-platform-capability.ts` with normalized maximize or restore, close, window-state snapshot, and window-state subscription APIs plus explicit web fallback semantics.
- [x] 1.3 Add or update focused platform-runtime tests covering frameless shell actions, passive window-state synchronization, and non-desktop fallback behavior.

## 2. Tauri Frameless Window Runtime

- [x] 2.1 Update `src-tauri/tauri.conf.json` and the relevant capability permissions so the main desktop window can run frameless without regressing existing size constraints or desktop features.
- [x] 2.2 Extend `src-tauri/src/lib.rs` shell action handling for custom titlebar controls, including maximize-toggle and explicit close behavior, while keeping normalized shell action results.
- [x] 2.3 Ensure passive window-state changes triggered by native gestures can flow back into the shared desktop subscription surface used by the frameless chrome.

## 3. Shared Desktop Window Frame Integration

- [x] 3.1 Introduce a shared desktop window frame or titlebar component at the app-shell boundary so auth and dashboard routes both receive consistent drag regions and system controls in desktop mode.
- [x] 3.2 Refactor `components/layout/dashboard-shell.tsx`, `components/layout/header.tsx`, and any related route shells so existing sidebar, notifications, and page content do not overlap or duplicate the frameless chrome.
- [x] 3.3 Mark interactive titlebar controls and overlay triggers as non-drag zones and keep the layout usable across desktop, web, and narrower desktop widths.

## 4. Verification

- [x] 4.1 Add or update tests for the shared desktop frame, shell integration, and drag-safe control rendering across authenticated and unauthenticated routes.
- [x] 4.2 Run focused frontend and Tauri-adjacent verification for frameless shell flows, window-state synchronization, and non-desktop fallback behavior, then record any platform-specific gaps truthfully.
