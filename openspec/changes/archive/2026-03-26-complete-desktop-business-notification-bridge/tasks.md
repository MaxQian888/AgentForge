## 1. Platform notification contract

- [x] 1.1 Expand `lib/platform-runtime.ts` notification types and results to support structured business notification payloads plus notification-driven tray summary updates.
- [x] 1.2 Update `hooks/use-platform-capability.ts` and existing desktop notification callers to consume the structured facade contract without page-level raw Tauri imports.

## 2. Authenticated shell coordination

- [x] 2.1 Add a dashboard-shell-level desktop notification coordinator that observes normalized notifications from the shared store and maintains a session delivery ledger keyed by notification ID.
- [x] 2.2 Wire hydration and websocket notification flows so desktop delivery deduplicates repeated records, applies foreground suppression rules, and keeps unread business truth in the existing notification store.

## 3. Tauri notification and tray bridge

- [x] 3.1 Extend `src-tauri/src/lib.rs` notification handling so desktop notification calls accept structured metadata and emit normalized `notification.delivered` / `notification.suppressed` / `notification.failed` desktop events.
- [x] 3.2 Reuse the supported tray path to synchronize unread notification summaries from the desktop coordinator without introducing a second desktop-only notification state source.

## 4. Verification

- [x] 4.1 Add or update platform-runtime and desktop-event tests covering structured notification payloads, normalized outcome events, and tray-summary fallback semantics.
- [x] 4.2 Add or update dashboard-shell, notification-store, and websocket integration tests covering hydration plus realtime deduplication, foreground suppression, tray sync, and failure-safe desktop degradation.
