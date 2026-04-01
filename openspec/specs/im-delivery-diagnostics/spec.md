# im-delivery-diagnostics Specification

## Purpose
Define the operator-facing IM delivery diagnostics contract so delivery history, settlement detail, and retry workflows stay truthful across fallback, replay, and manual operator actions.
## Requirements
### Requirement: Delivery history table SHALL display downgrade reason

The `IMMessageHistory` component SHALL render a `downgrade_reason` column when a delivery record has a non-empty downgrade reason. The column SHALL display the reason text (e.g., "actioncard_unsupported → text_fallback").

#### Scenario: Delivery with downgrade
- **WHEN** a delivery record has `downgradeReason: "markdown_unsafe → plain_text"`
- **THEN** the table row displays the downgrade reason in a warning-styled cell

#### Scenario: Delivery without downgrade
- **WHEN** a delivery record has no `downgradeReason`
- **THEN** the downgrade column is empty for that row

### Requirement: Delivery history SHALL support payload preview

Each delivery row SHALL have an expandable detail drawer that shows the original payload (structured + native), the final rendered output, and the delivery metadata.

#### Scenario: Opening payload preview
- **WHEN** operator clicks the expand button on a delivery row
- **THEN** a side drawer opens showing the original typed envelope JSON, the rendered output, and delivery timestamps

### Requirement: Delivery history SHALL support manual retry

Failed deliveries SHALL show a retry button that triggers `POST /im/deliveries/:id/retry` to re-enqueue the delivery through the control plane.

#### Scenario: Retrying a failed delivery
- **WHEN** operator clicks "Retry" on a delivery with status "failed"
- **THEN** the system calls the retry endpoint and updates the row status to "pending"

#### Scenario: Retry on successful delivery
- **WHEN** a delivery has status "delivered"
- **THEN** the retry button is not displayed

### Requirement: Backend SHALL persist downgrade_reason on delivery records

The `AckDelivery` endpoint SHALL accept an optional `downgrade_reason` field from the bridge and persist it on the delivery record. The `ListDeliveries` response SHALL include `downgrade_reason` for each record.

#### Scenario: Bridge reports downgrade on ack
- **WHEN** bridge sends `POST /im/bridge/ack` with `{"deliveryId": "d1", "cursor": 42, "downgradeReason": "card_unsupported → text"}`
- **THEN** the delivery record `d1` is updated with `downgrade_reason = "card_unsupported → text"`

### Requirement: Backend SHALL expose delivery retry endpoint

`POST /im/deliveries/:id/retry` SHALL re-enqueue a failed delivery through the control plane with the same typed envelope. Only deliveries with status "failed" or "timeout" SHALL be retryable.

#### Scenario: Retry a failed delivery
- **WHEN** operator calls `POST /im/deliveries/d1/retry` and delivery `d1` has status "failed"
- **THEN** the delivery is re-enqueued and status changes to "pending"

#### Scenario: Retry a delivered message
- **WHEN** operator calls `POST /im/deliveries/d1/retry` and delivery `d1` has status "delivered"
- **THEN** the endpoint returns 409 Conflict

### Requirement: All platform adapters SHALL report downgrade reason via header

When a platform adapter falls back from structured/native delivery to plain text, it SHALL set `X-IM-Downgrade-Reason` header on the ack request with a machine-readable reason string (e.g., `"actioncard_unsupported"`, `"markdown_unsafe"`, `"card_update_unavailable"`).

#### Scenario: DingTalk falls back from ActionCard to text
- **WHEN** DingTalk adapter receives a card-typed delivery but ActionCard send fails
- **THEN** the adapter delivers as plain text and includes `X-IM-Downgrade-Reason: actioncard_send_failed` in the ack

#### Scenario: QQ falls back from structured to text
- **WHEN** QQ adapter receives a structured delivery with unsupported components
- **THEN** the adapter delivers as plain text and includes `X-IM-Downgrade-Reason: structured_unsupported` in the ack

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

