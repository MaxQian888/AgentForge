## MODIFIED Requirements

### Requirement: QQ Bot outbound delivery SHALL support explicit structured downgrade semantics
The system SHALL resolve QQ Bot-targeted typed deliveries through a markdown-first QQ Bot rendering profile that may include keyboard buttons when the current scene and payload support them. QQ Bot MUST preserve the truthful boundary between markdown or keyboard send, reply-target reuse via preserved conversation metadata, and the absence of Feishu-style mutable card lifecycle. When the requested richer path cannot be honored, the Bridge SHALL fall back explicitly to supported QQ Bot text output and preserve fallback metadata.

#### Scenario: Markdown QQ Bot notification uses the provider rendering profile
- **WHEN** the notification receiver handles a QQ Bot-targeted delivery with markdown-compatible richer content
- **THEN** the Bridge resolves that delivery through the active QQ Bot rendering profile and uses the QQ Bot markdown path
- **AND** the resulting delivery metadata preserves that QQ Bot-native markdown rendering was chosen

#### Scenario: QQ Bot keyboard or mutable update request degrades explicitly when context is incompatible
- **WHEN** a QQ Bot-targeted delivery requests keyboard-assisted completion or mutable richer update behavior that the current reply target cannot honor
- **THEN** the Bridge falls back to a supported QQ Bot text follow-up path
- **AND** the resulting delivery metadata records that the original richer update plan was unavailable

#### Scenario: QQ Bot reply-target reuse remains truthful
- **WHEN** a QQ Bot-originated action preserves conversation metadata that supports replying in place to the same chat context
- **THEN** the Bridge reuses that preserved reply target for the follow-up delivery path it actually supports
- **AND** it does not claim full rich-card lifecycle parity when only markdown or keyboard send is available
