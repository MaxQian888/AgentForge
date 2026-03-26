## ADDED Requirements

### Requirement: Feishu provider SHALL own message and card construction semantics
The Feishu provider SHALL expose provider-owned builders that turn typed outbound delivery intent into Feishu-supported text, `lark_md` content blocks, JSON cards, or template cards without requiring shared Bridge layers to assemble raw Feishu payloads. Those builders MUST preserve the provider-aware downgrade path between plain text, richer card content, and delayed native update flows.

#### Scenario: Shared completion text becomes Feishu-native rich text
- **WHEN** a shared action completion targets Feishu and the provider profile selects a richer text-capable output path
- **THEN** the Feishu provider builds the final message using Feishu-supported text or `lark_md` semantics
- **AND** shared delivery layers do not need to embed Feishu-specific rich-text tags or raw card fragments directly

#### Scenario: Template-card request is constructed through provider-owned inputs
- **WHEN** a Feishu-targeted delivery requests a template-based card with template identifier and variables
- **THEN** the Feishu provider builds the template-card payload through its provider-owned builder surface
- **AND** the resulting payload preserves template metadata for later update, diagnostics, or fallback decisions

### Requirement: Feishu richer builders SHALL remain aligned with reply-target update policy
If a Feishu-targeted delivery is tied to a preserved reply target that supports immediate callback response, delayed card update, or plain-text fallback, the Feishu builder and update policy SHALL produce outputs that remain compatible with that lifecycle. The Bridge MUST NOT choose a richer builder output that cannot be delivered through the preserved Feishu reply target and current token state.

#### Scenario: Delayed-update target selects update-compatible card output
- **WHEN** a Feishu reply target preserves delayed-update context for the originating interaction
- **THEN** the Feishu provider chooses a builder output that can be used with the delayed-update path when the requested richer content is eligible
- **AND** the Bridge prefers that in-place update path before sending an unrelated new message

#### Scenario: Incompatible richer output falls back explicitly
- **WHEN** the requested Feishu richer output cannot be delivered through the preserved reply target or current delayed-update context
- **THEN** the Feishu provider falls back to a supported reply or send representation
- **AND** the delivery metadata records the provider-aware fallback instead of silently pretending the richer path succeeded
