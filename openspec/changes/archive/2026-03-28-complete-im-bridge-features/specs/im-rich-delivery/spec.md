## MODIFIED Requirements

### Requirement: Delivery fallback metadata SHALL reflect rendering-profile decisions

When the rendering profile changes the delivery method (e.g., card → text), the delivery record SHALL preserve provider-aware fallback metadata. The metadata SHALL include: original intended format, actual delivered format, reason for fallback, and provider name.

The delivery record persisted by the backend SHALL include a `downgrade_reason` field populated from the bridge ack. The `ListDeliveries` API response SHALL expose this field. The bridge control-plane ack message SHALL accept an optional `downgrade_reason` string.

#### Scenario: Unsafe markdown falls back to plain text with reason
- **WHEN** rendering profile determines markdown is unsafe for the target provider
- **THEN** delivery executes as plain text and fallback metadata records `"markdown_unsafe → plain_text"`

#### Scenario: Unsupported card delivery falls back with explicit reason
- **WHEN** a card-typed delivery targets a provider with `card: false`
- **THEN** delivery executes as structured text and fallback metadata records `"card_unsupported → structured_text"`

#### Scenario: Bridge ack carries downgrade reason to backend
- **WHEN** bridge sends delivery ack with `downgradeReason: "actioncard_send_failed"`
- **THEN** backend persists `downgrade_reason` on the delivery record and returns it in subsequent `ListDeliveries` responses

## ADDED Requirements

### Requirement: Backend SHALL expose event types endpoint

`GET /im/event-types` SHALL return the canonical list of subscribable event types. This endpoint SHALL be used by the frontend to dynamically render event subscription checkboxes instead of hardcoding.

#### Scenario: Fetching event types
- **WHEN** frontend calls `GET /api/v1/im/event-types`
- **THEN** the response includes `["task.created", "task.completed", "review.completed", "agent.started", "agent.completed", "budget.warning", "sprint.started", "sprint.completed", "review.requested", "workflow.failed"]`
