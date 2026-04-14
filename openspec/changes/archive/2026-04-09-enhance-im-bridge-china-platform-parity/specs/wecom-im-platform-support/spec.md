## MODIFIED Requirements

### Requirement: WeCom outbound delivery SHALL support explicit structured downgrade semantics
The system SHALL resolve WeCom-targeted typed deliveries through a WeCom rendering profile that can choose a WeCom-supported template-card, markdown, or text representation according to the active reply target and payload shape. The Bridge MUST preserve the distinction between WeCom's richer send-time surfaces and Feishu-style mutable card lifecycle: WeCom MAY send richer payloads when the provider contract supports them, but it MUST fall back explicitly when the request requires unsupported in-place update or callback-dependent richer behavior.

#### Scenario: Template-card-capable WeCom notification uses the provider rendering profile
- **WHEN** the backend submits a WeCom-targeted delivery with template-card-compatible content
- **THEN** the Bridge resolves that delivery through the active WeCom rendering profile and sends a supported WeCom richer payload
- **AND** the transport layer does not require shared code to assemble provider-specific WeCom payloads directly

#### Scenario: WeCom richer update request falls back truthfully
- **WHEN** a WeCom-targeted delivery requests mutable richer update behavior that the current reply target or provider contract cannot honor
- **THEN** the Bridge sends a supported WeCom markdown or text fallback instead
- **AND** the resulting delivery metadata records that the richer update path was unavailable

#### Scenario: WeCom reply-first path remains explicit
- **WHEN** a WeCom-originated action has a preserved response context that supports a direct reply
- **THEN** the Bridge prefers that WeCom reply path first
- **AND** any fallback to direct app-message send remains explicit in the delivery metadata rather than being treated as invisible parity
