## 1. Shared Desktop Notification Contract

- [ ] 1.1 Extend `lib/platform-runtime.ts` and `hooks/use-platform-capability.ts` to support structured business-notification payloads plus normalized desktop delivery outcomes instead of title/body-only requests.
- [ ] 1.2 Define the first-phase desktop notification policy in shared frontend code, including eligible notification types, foreground suppression defaults, and the metadata needed to preserve `href`/context without introducing a second notification DTO.

## 2. Dashboard Shell Notification Bridge

- [ ] 2.1 Add a single desktop notification bridge/coordinator under `components/layout/dashboard-shell.tsx` that observes the authenticated notification store, evaluates hydration and websocket notifications for native delivery, and avoids page-level duplicate bridges.
- [ ] 2.2 Implement notification-ID-based deduplication, unread-truth preservation, and foreground suppression bookkeeping so fetch hydration, websocket replay, and store re-renders do not create duplicate native popups or accidental read mutations.

## 3. Tauri Delivery Outcomes And Tray Summary

- [ ] 3.1 Update `src-tauri/src/lib.rs` and the relevant Tauri capability configuration so the desktop shell accepts structured notification delivery requests and emits normalized `notification.delivered`, `notification.suppressed`, and `notification.failed` outcome events.
- [ ] 3.2 Reuse the existing tray update path to reflect unread notification summary changes during desktop sessions, including cases where native popups are suppressed or delivery fails.

## 4. Notification UI Consistency And Verification

- [ ] 4.1 Keep `lib/stores/notification-store.ts`, `lib/stores/ws-store.ts`, and `components/layout/header.tsx` aligned so in-app notifications remain the authoritative read-handling surface even when desktop delivery is active, suppressed, or unavailable.
- [ ] 4.2 Add or update focused tests for platform runtime notification delivery, dashboard-shell bridge behavior, notification-store/websocket replay, and tray or desktop-event outcomes, then run scoped verification covering the touched notification and desktop files.
