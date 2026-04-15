## ADDED Requirements

### Requirement: Feishu rendering plans SHALL keep operator cards on provider-safe interactive structures
When a structured operator response targets Feishu, the rendering profile SHALL resolve it through a provider-safe interactive card plan instead of assuming that any shared markdown fragment can be embedded into any Feishu card field. The Feishu rendering plan MUST choose only documented card elements and markdown-bearing fields that are valid for the selected interactive message shape, and MUST fall back explicitly when the requested richer output cannot be rendered safely.

#### Scenario: Feishu help response resolves through a provider-safe card plan
- **WHEN** `/help` or another structured command-catalog response targets the Feishu provider
- **THEN** the rendering plan produces a documented interactive card structure that the Feishu send or reply API accepts
- **AND** the final card body uses only provider-safe markdown or text slots for the content it carries

#### Scenario: Unsafe or unsupported richer formatting downgrades explicitly
- **WHEN** a structured Feishu response would require markdown or card elements that are incompatible with the selected send, reply, or update path
- **THEN** the rendering plan downgrades to a simpler supported representation such as plain text or a reduced card body
- **AND** the delivery metadata preserves an explicit fallback reason instead of silently emitting an invalid Feishu payload

### Requirement: Feishu update rendering SHALL remain compatible with delayed-update constraints
If a Feishu reply target carries delayed-update context, the rendering profile SHALL choose an update-compatible card representation before attempting in-place mutation. The Bridge MUST NOT select a rendering mode that only works for fresh sends or replies when the preserved callback token can only be used for supported delayed card updates.

#### Scenario: Delayed-update target chooses an update-compatible card body
- **WHEN** a Feishu completion flow prefers delayed card update for the originating interaction
- **THEN** the rendering plan selects a card body that is valid for the delayed-update path and current token state
- **AND** the provider does not attempt to mutate the card with a send-only or reply-only representation
