## MODIFIED Requirements

### Requirement: Canonical rich delivery SHALL be rendered through the active provider profile
The canonical typed outbound IM envelope SHALL be resolved through the active provider profile and readiness tier before the Bridge executes transport delivery. Chinese platform deliveries MUST preserve identical provider-aware outcomes and fallback metadata across direct notify, compatibility HTTP, control-plane replay, and action completion. The rendered outcome MUST honor provider-specific limits: Feishu may use native card send or delayed update, DingTalk may use ActionCard send or truthful text fallback, WeCom may use template-card or markdown or text without mutable-update pretence, QQ SHALL resolve to text or link output, and QQ Bot SHALL resolve to markdown or keyboard-first output or explicit text fallback.

#### Scenario: Feishu delivery keeps native card or delayed-update semantics across transports
- **WHEN** a Feishu-targeted typed delivery crosses direct notify, replay, or action-completion paths
- **THEN** the rendering step preserves the same Feishu-native card or delayed-update plan whenever the preserved reply target permits it
- **AND** fallback metadata is emitted only when that native lifecycle cannot be honored

#### Scenario: DingTalk delivery uses ActionCard or explicit fallback consistently
- **WHEN** a DingTalk-targeted typed delivery requests card-like richer output
- **THEN** the rendering step chooses ActionCard delivery when the provider profile and reply target allow it
- **AND** otherwise falls back to text with the same machine-readable downgrade reason regardless of transport path

#### Scenario: WeCom delivery resolves through template-card-aware profile
- **WHEN** a WeCom-targeted typed delivery requests structured or richer content
- **THEN** the rendering step chooses a WeCom-supported template-card, markdown, or text representation according to the active WeCom profile
- **AND** it does not pretend that mutable richer updates are available when only send-time richer payloads are supported

#### Scenario: QQ delivery remains text-first with explicit richer fallback
- **WHEN** a QQ-targeted typed delivery requests structured, native, or mutable-update behavior
- **THEN** the rendering step resolves the delivery into QQ-supported text or link output
- **AND** the delivery receipt records that the richer request degraded because QQ is text-first

#### Scenario: QQ Bot delivery remains markdown-first with explicit update limits
- **WHEN** a QQ Bot-targeted typed delivery requests markdown, keyboard, or richer completion output
- **THEN** the rendering step uses the QQ Bot markdown or keyboard path when the current reply target supports it
- **AND** falls back explicitly when the request requires unsupported mutable-update behavior
