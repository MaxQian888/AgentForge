## MODIFIED Requirements

### Requirement: Canonical rich delivery SHALL be rendered through the active provider profile
The canonical typed outbound IM envelope SHALL be resolved through the active provider's rendering profile before the Bridge executes transport delivery. That rendering step MUST choose the final provider-facing representation for native payloads, structured payloads, formatted text, segmented text, or explicit downgrade so the same typed envelope produces the same provider-aware outcome across direct notify, compatibility HTTP, queueing, and replay.

#### Scenario: Telegram structured delivery becomes text plus inline keyboard
- **WHEN** a typed delivery containing structured content targets Telegram
- **THEN** the rendering step resolves the delivery into Telegram-supported text plus inline-keyboard output instead of an unsupported card payload
- **AND** the resulting delivery receipt records that Telegram-native structured rendering was chosen through the provider profile

#### Scenario: WeCom typed delivery becomes supported app-message content
- **WHEN** a typed delivery containing structured or richer card intent targets WeCom
- **THEN** the rendering step resolves the delivery into a WeCom-supported text, news-style, or template-card-compatible representation according to the active WeCom provider profile
- **AND** transport execution does not require shared layers to special-case WeCom outside the provider contract

#### Scenario: Feishu typed delivery becomes builder-owned native content
- **WHEN** a typed delivery containing richer card intent targets Feishu
- **THEN** the rendering step resolves the delivery through Feishu's provider-owned builders into JSON-card, template-card, or `lark_md`-backed output as appropriate
- **AND** transport execution does not require shared layers to assemble raw Feishu payload fragments directly

### Requirement: Delivery fallback metadata SHALL reflect rendering-profile decisions
If the active provider profile changes the final delivery method by downgrading formatted text, splitting oversized text, avoiding an unsafe edit, or abandoning a native update path, the delivery result SHALL preserve provider-aware fallback metadata that explains the rendering decision.

#### Scenario: Unsafe Telegram markdown falls back to plain text
- **WHEN** a Telegram-targeted delivery requests formatted text but the provider renderer cannot produce safe Markdown-aware output
- **THEN** the Bridge falls back to Telegram plain text before sending the message
- **AND** the delivery result records that the formatted path was skipped because the renderer selected a safe fallback

#### Scenario: Unsupported WeCom richer update falls back to text with explicit reason
- **WHEN** a WeCom-targeted delivery requests a richer card update or mutable path that the active WeCom reply target cannot honor
- **THEN** the Bridge falls back to a WeCom-supported text or follow-up delivery before sending the message
- **AND** the fallback metadata explains that the richer WeCom update plan was not usable

#### Scenario: Incompatible mutable update becomes provider-aware follow-up
- **WHEN** a reply target requests an in-place update that the active provider profile considers invalid for the current content or target
- **THEN** the Bridge chooses a supported provider-aware follow-up delivery path instead of forcing the invalid update
- **AND** the fallback metadata explains that the original mutable update plan was not usable
