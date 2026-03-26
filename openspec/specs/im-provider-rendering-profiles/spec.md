# im-provider-rendering-profiles Specification

## Purpose
Define the provider-owned rendering profile contract for AgentForge IM Bridge so each active IM provider can declare how typed outbound delivery should be formatted, segmented, downgraded, and turned into final provider payloads.

## Requirements
### Requirement: Active IM provider SHALL publish a rendering profile
The IM Bridge SHALL define a provider-owned rendering profile for each runnable IM provider. That profile MUST declare the provider's supported text formatting modes, structured rendering preferences, message length limits, mutable-update constraints, and any optional provider-owned builders used to turn typed outbound delivery into final provider payloads.

#### Scenario: Telegram publishes a text-first rendering profile
- **WHEN** the active provider is Telegram
- **THEN** the Bridge exposes a rendering profile that identifies plain text and Markdown-capable delivery modes, inline-keyboard rendering preferences, text length constraints, and editable-message limits
- **AND** upper layers do not infer those rules from the string `"telegram"` alone

#### Scenario: Feishu publishes richer builder surfaces
- **WHEN** the active provider is Feishu
- **THEN** the Bridge exposes a rendering profile that identifies text, `lark_md`, JSON-card, and template-card construction surfaces plus delayed-update constraints
- **AND** upper layers can request provider-native rendering without assembling raw Feishu payloads themselves

### Requirement: Typed outbound delivery SHALL resolve through a provider rendering plan
Before a typed outbound IM delivery is executed, the Bridge SHALL resolve the delivery through the active provider's rendering profile and produce a provider-aware rendering plan. The rendering plan MUST decide whether the delivery uses native payloads, structured output, formatted text, segmented text, editable updates, or explicit downgrade to plain text.

#### Scenario: Control-plane replay preserves provider-aware rendering
- **WHEN** a queued delivery is replayed for the active provider
- **THEN** the Bridge resolves the same provider-aware rendering plan that direct `/im/send` or `/im/notify` would use for the same envelope
- **AND** replay does not silently flatten the delivery into a different provider representation

#### Scenario: Unsupported rich request degrades through the rendering plan
- **WHEN** a typed delivery requests rendering behavior the active provider profile or reply target cannot honor
- **THEN** the Bridge resolves an explicit downgrade plan instead of attempting an invalid native or mutable update
- **AND** the resulting delivery metadata preserves the fallback reason

### Requirement: Provider rendering profiles SHALL enforce provider-safe formatting
If a provider supports formatted text modes such as Markdown, HTML, or provider-specific rich text, its rendering profile MUST enforce the provider's escaping, segmentation, and update-safety constraints before transport execution. If the requested formatted output cannot be rendered safely, the Bridge MUST fall back to a supported plain-text representation.

#### Scenario: Unsafe formatted text falls back cleanly
- **WHEN** the Bridge receives a formatted-text intent that cannot be rendered safely under the active provider's escaping or update rules
- **THEN** the provider rendering profile degrades the delivery to a supported plain-text representation
- **AND** the transport executor does not send malformed formatted text
