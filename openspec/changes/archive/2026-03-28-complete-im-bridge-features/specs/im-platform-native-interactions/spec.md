## ADDED Requirements

### Requirement: DingTalk adapter SHALL support ActionCard rendering

The DingTalk live adapter SHALL implement `SendActionCard()` to deliver interactive ActionCard messages via DingTalk OpenAPI. When the rendering profile resolves to card-typed delivery, the adapter SHALL construct and send an ActionCard payload with action buttons mapped to the typed envelope's action references.

#### Scenario: Sending an ActionCard with task actions
- **WHEN** a delivery envelope contains card-typed content with actions `["approve", "reject"]` targeting a task entity
- **THEN** the DingTalk adapter sends an ActionCard with two buttons labeled per the action references, each carrying the entity ID and action type in callback data

#### Scenario: ActionCard delivery fails
- **WHEN** DingTalk OpenAPI returns an error when sending ActionCard
- **THEN** the adapter falls back to plain-text delivery with action labels listed as text, and reports `X-IM-Downgrade-Reason: actioncard_send_failed`

### Requirement: DingTalk ActionCard callbacks SHALL normalize to shared action contract

When a user clicks an ActionCard button, the DingTalk adapter SHALL normalize the callback into an `IMActionRequest` through the existing `NormalizeAction()` path, preserving the entity ID, action type, and session webhook reply target.

#### Scenario: User clicks approve button on ActionCard
- **WHEN** DingTalk streams a card callback with action data `{"action": "approve", "entityId": "task-123"}`
- **THEN** the adapter produces an `IMActionRequest` with action `"approve"`, entity `"task-123"`, and reply target containing the session webhook URL

### Requirement: Review command engine SHALL support deep/approve/request-changes subcommands

The Bridge command engine SHALL handle `/review deep <taskId>`, `/review approve <reviewId>`, and `/review request-changes <reviewId> [reason]` subcommands. Each SHALL call the corresponding backend review API endpoint.

#### Scenario: /review deep command
- **WHEN** user sends `/review deep TASK-42`
- **THEN** Bridge calls `POST /api/v1/reviews` with `{"taskId": "TASK-42", "mode": "deep"}` and replies with the review creation confirmation

#### Scenario: /review approve command
- **WHEN** user sends `/review approve REV-10`
- **THEN** Bridge calls `POST /api/v1/reviews/REV-10/decide` with `{"decision": "approve"}` and replies with the decision result

#### Scenario: /review request-changes with reason
- **WHEN** user sends `/review request-changes REV-10 missing error handling`
- **THEN** Bridge calls `POST /api/v1/reviews/REV-10/decide` with `{"decision": "request_changes", "reason": "missing error handling"}` and replies with the decision result
