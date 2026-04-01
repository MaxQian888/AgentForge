## ADDED Requirements

### Requirement: Delivery history SHALL support operator filters

The delivery history surface SHALL support filtering by delivery status, platform, event type, delivery kind, and time window while preserving payload preview and retry affordances for matching records. `GET /api/v1/im/deliveries` MUST accept these filters and return only matching history rows.

#### Scenario: Failures-only filter returns matching rows
- **WHEN** the operator requests delivery history with `status=failed`, `platform=slack`, and a recent time window
- **THEN** `GET /api/v1/im/deliveries` returns only Slack failed rows within that window
- **THEN** the frontend shows the filtered count and keeps payload preview and retry actions available for those rows

#### Scenario: Empty filter result remains truthful
- **WHEN** the operator applies filters that match no rows
- **THEN** the history surface displays an empty-state message for the active filter set
- **THEN** the operator can clear filters without losing the surrounding console state

### Requirement: Delivery diagnostics SHALL support batch retry for explicit retryable rows

The system SHALL provide a batch retry action for an explicit set of failed or timeout deliveries. The backend MUST re-enqueue only retryable rows, leave non-retryable rows unchanged, and return per-delivery outcomes so the operator can see partial success without guessing.

#### Scenario: Batch retry re-enqueues failed rows
- **WHEN** the operator submits a batch retry request containing failed deliveries `d1` and `d2`
- **THEN** the backend re-enqueues each retryable delivery through the canonical control-plane retry path
- **THEN** the response reports `pending` outcomes for `d1` and `d2`

#### Scenario: Batch retry rejects non-retryable rows
- **WHEN** the operator submits a batch retry request containing delivered delivery `d3`
- **THEN** the backend reports `d3` as rejected without re-enqueueing it
- **THEN** the existing history record for `d3` remains unchanged

### Requirement: Delivery detail SHALL expose settlement metadata

The delivery detail drawer SHALL show operator-relevant settlement metadata in addition to the typed payload. The detail view MUST include queued timestamp, processed timestamp when available, latency, terminal status, failure reason, downgrade reason, and rendered-outcome metadata for the selected delivery.

#### Scenario: Delivered row shows settlement details
- **WHEN** the operator opens the detail drawer for a delivered delivery
- **THEN** the drawer shows queued time, processed time, computed latency, terminal status, and any fallback metadata associated with the rendered outcome

#### Scenario: Pending row omits unavailable settlement fields truthfully
- **WHEN** the operator opens the detail drawer for a pending delivery that has not settled yet
- **THEN** the drawer shows queued time and current `pending` status
- **THEN** processed timestamp and latency remain unavailable instead of being fabricated
