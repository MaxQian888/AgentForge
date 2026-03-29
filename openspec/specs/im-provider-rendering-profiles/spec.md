# im-provider-rendering-profiles Specification

## Purpose
Define the provider-owned rendering profile contract for AgentForge IM Bridge so each active IM provider can declare how typed outbound delivery should be formatted, segmented, downgraded, and turned into final provider payloads.
## Requirements
### Requirement: Active IM provider SHALL publish a rendering profile

Each runnable IM provider SHALL define supported text formatting modes, structured rendering preferences, message length limits, mutable-update constraints, and card capability declarations. Optional provider-owned builders SHALL turn typed delivery into final provider payloads.

The DingTalk provider SHALL declare `card: true` and `cardUpdate: false` in its rendering profile, indicating support for sending ActionCard messages but no support for in-place card updates. The profile SHALL include an ActionCard builder that maps typed envelope actions to DingTalk ActionCard button payloads.

#### Scenario: DingTalk provider publishes rendering profile with card support
- **WHEN** DingTalk provider initializes
- **THEN** its rendering profile declares `{text: true, markdown: false, card: true, cardUpdate: false, callback: "stream"}` and includes an ActionCard payload builder

#### Scenario: Feishu provider declares full card lifecycle
- **WHEN** Feishu provider initializes
- **THEN** its rendering profile declares `{text: true, markdown: true, card: true, cardUpdate: true, callback: "longConnection"}` with delayed-update constraints

#### Scenario: QQ provider declares text-only profile
- **WHEN** QQ provider initializes
- **THEN** its rendering profile declares `{text: true, markdown: false, card: false, cardUpdate: false, callback: "websocket"}` with reply-segment awareness

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

