## ADDED Requirements

### Requirement: Feishu provider SHALL support both JSON card and template card delivery
The Feishu provider SHALL expose a richer card delivery contract that can send or reply with either raw card JSON or a template-based card payload. Template-based delivery MUST support a template identifier and template variables, and MAY include explicit version metadata when the caller provides it. The Bridge MUST preserve a truthful fallback path when the requested payload cannot be delivered through the native Feishu card surface.

#### Scenario: JSON card is delivered through native interactive messaging
- **WHEN** a Feishu-targeted notification requests a native card using JSON card content
- **THEN** the provider sends that card through the Feishu interactive message API
- **AND** the Bridge does not first flatten the request into plain text unless the native card path is unavailable

#### Scenario: Template card is delivered with template variables
- **WHEN** a Feishu-targeted notification requests a template card with `template_id` and `template_variable` data
- **THEN** the provider sends the template-based Feishu card using those variables
- **AND** the template metadata remains available for later provider-aware updates or diagnostics

### Requirement: Feishu card callbacks SHALL preserve schema 2.0 interaction context
The Feishu provider SHALL normalize `card.action.trigger` interactions using the current callback structure while preserving the provider-specific interaction data required for later response handling. The normalized action contract MUST retain the originating message identity, update token, operator identity, card action value, and any provider metadata needed for immediate response or delayed update, without leaking the raw Feishu callback payload to shared backend handlers.

#### Scenario: Card callback becomes a shared action envelope
- **WHEN** the Bridge receives a Feishu `card.action.trigger` callback
- **THEN** it normalizes the action into the shared backend action contract with preserved message identity, update token, action value, and operator context
- **AND** upstream handlers do not need to parse Feishu callback schema details directly

#### Scenario: Callback response can return toast without losing update context
- **WHEN** a Feishu card interaction is acknowledged immediately with a toast or keep-current-card response
- **THEN** the provider returns the synchronous callback response within the provider response window
- **AND** the preserved reply target still retains the context required for later delayed update if needed

### Requirement: Feishu long-running card actions SHALL coordinate immediate acknowledgement and delayed update
When a Feishu card action triggers work that cannot complete within the synchronous callback window, the Bridge SHALL first acknowledge the callback in time and then use the preserved delayed-update context to update the originating card within the provider-supported validity window. If the update token is expired, exhausted, or invalid for the requested update mode, the Bridge MUST fall back explicitly to a supported reply path and record the reason for that fallback.

#### Scenario: Long-running card action uses delayed update after acknowledgement
- **WHEN** a Feishu card action starts work that outlives the immediate callback window
- **THEN** the Bridge acknowledges the callback first
- **AND** later updates the originating card through the preserved delayed-update token instead of sending an unrelated duplicate message

#### Scenario: Expired delayed-update token falls back explicitly
- **WHEN** a Feishu completion update is ready but the preserved delayed-update token can no longer be used
- **THEN** the Bridge falls back to a supported Feishu reply or send path
- **AND** the delivery outcome records that native card mutation was skipped because the delayed-update context was no longer valid
