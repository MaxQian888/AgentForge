## MODIFIED Requirements

### Requirement: Platform metadata exposes delivery-relevant runtime characteristics
The active IM platform runtime SHALL expose the delivery characteristics and readiness tier needed by health, registration, control-plane routing, and operator documentation. Registration and health surfaces MUST distinguish whether a platform is `full_native_lifecycle`, `native_send_with_fallback`, `text_first`, or `markdown_first`, and MUST keep that tier aligned with the provider's actual callback, mutable-update, structured-rendering, and reply-target behavior instead of implying flat parity across all supported Chinese platforms.

#### Scenario: Feishu exposes full native lifecycle metadata
- **WHEN** the active platform is Feishu
- **THEN** health and registration metadata report readiness tier `full_native_lifecycle`
- **AND** the capability matrix indicates native callback response plus delayed card update support

#### Scenario: DingTalk and WeCom expose native send without mutable card parity
- **WHEN** the active platform is DingTalk or WeCom
- **THEN** health and registration metadata report readiness tier `native_send_with_fallback`
- **AND** the capability matrix indicates provider-native send and callback semantics without claiming Feishu-style mutable card updates

#### Scenario: QQ exposes text-first runtime truth
- **WHEN** the active platform is QQ
- **THEN** health and registration metadata report readiness tier `text_first`
- **AND** the capability matrix does not advertise native payload surfaces or mutable update support

#### Scenario: QQ Bot exposes markdown-first runtime truth
- **WHEN** the active platform is QQ Bot
- **THEN** health and registration metadata report readiness tier `markdown_first`
- **AND** the capability matrix indicates markdown or keyboard send support without claiming full rich-card lifecycle parity
