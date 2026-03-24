## 1. Progress Signal Data Model

- [x] 1.1 Add backend models, persistence, and migration support for task progress snapshots, including last activity, risk state, stall metadata, and alert bookkeeping.
- [x] 1.2 Extend task DTO and task read paths so task list/detail responses include the normalized progress-health fields alongside workflow status.
- [x] 1.3 Define configurable inactivity thresholds, cooldown windows, and exempt states for progress evaluation without changing the existing workflow transition rules.

## 2. Progress Detection Pipeline

- [x] 2.1 Implement a centralized task progress projector/manager that refreshes activity signals from task create/update/assign/transition events and from agent/review lifecycle hooks.
- [x] 2.2 Add a periodic detector that evaluates non-terminal tasks, marks tasks as at-risk or stalled, and clears those signals when qualifying recovery activity arrives.
- [x] 2.3 Introduce dedicated realtime event payloads for progress state changes so project-scoped subscribers can observe risk, stall, and recovery transitions in place.

## 3. Alert Delivery And UI Consumption

- [x] 3.1 Add progress alert notification types and delivery logic that deduplicates unchanged conditions while allowing escalation and recovery notifications when the signal changes.
- [x] 3.2 Wire optional IM progress alert fan-out through the existing notify receiver with best-effort failure handling that never blocks persisted in-product alerts.
- [x] 3.3 Update frontend task, notification, and WebSocket stores plus the relevant task/dashboard surfaces to render progress health, stall reasons, and live alert updates from the shared task signal shape.

## 4. Verification

- [x] 4.1 Add or update backend tests covering progress snapshot refresh, inactivity detection, deduplicated alert creation, and recovery handling.
- [x] 4.2 Add or update frontend tests covering task progress rendering, websocket-driven signal updates, and notification ingestion for progress alerts.
- [x] 4.3 Run the scoped validation commands for the touched Go, frontend, and IM notification paths and confirm the new change remains apply-ready.
