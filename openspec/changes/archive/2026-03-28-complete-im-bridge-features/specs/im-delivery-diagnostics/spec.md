## ADDED Requirements

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
