## MODIFIED Requirements

### Requirement: Platform capability matrix SHALL describe native interaction strategy, not just transport availability
The system SHALL publish a capability matrix for each active IM platform that describes native command intake, structured surface, callback mode, asynchronous completion strategy, mutable-update truth, and readiness tier. The matrix MUST describe Chinese platforms according to their real provider model: Feishu as full card lifecycle with delayed update, DingTalk and WeCom as callback-capable but fallback-aware native-send platforms, QQ Bot as markdown or keyboard-first, and QQ as text-first with no native interaction surface. The Bridge, control-plane registration payload, and health surfaces MUST use this matrix instead of inferring behavior from platform names or from a single rich-message boolean.

#### Scenario: Feishu declares delayed-update-native interaction lifecycle
- **WHEN** the active platform is Feishu
- **THEN** the capability matrix identifies callback-token preservation, immediate callback acknowledgement, and delayed card update as the preferred asynchronous completion path
- **AND** downstream delivery code can distinguish that lifecycle from ordinary reply-only providers

#### Scenario: DingTalk declares ActionCard callbacks without mutable-card parity
- **WHEN** the active platform is DingTalk
- **THEN** the capability matrix identifies Stream callback intake, ActionCard send or callback semantics, and session-webhook follow-up delivery
- **AND** it does not claim in-place card mutation parity with Feishu

#### Scenario: WeCom declares callback-driven richer send with explicit update limits
- **WHEN** the active platform is WeCom
- **THEN** the capability matrix identifies callback-driven command intake plus template-card or markdown-capable outbound delivery
- **AND** it marks richer updates as fallback-aware instead of implying guaranteed mutable-card support

#### Scenario: QQ Bot and QQ publish truthful non-Feishu interaction tiers
- **WHEN** the active platform is QQ Bot or QQ
- **THEN** the capability matrix distinguishes QQ Bot markdown or keyboard send from QQ text-first delivery
- **AND** neither platform claims Feishu-style card callback or delayed-update semantics unless the adapter truly implements them
