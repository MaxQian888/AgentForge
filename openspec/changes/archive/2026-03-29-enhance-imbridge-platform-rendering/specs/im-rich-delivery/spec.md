## MODIFIED Requirements

### Requirement: Canonical rich delivery SHALL be rendered through the active provider profile
The canonical typed outbound IM envelope SHALL be resolved through the active provider's rendering profile before the Bridge executes transport delivery. That rendering step MUST choose the final provider-facing representation for native payloads (including new platform-specific native payloads beyond Feishu), structured payloads (including section-based structured messages), formatted text (including new platform markdown formats), segmented text, or explicit downgrade so the same typed envelope produces the same provider-aware outcome across direct notify, compatibility HTTP, queueing, and replay.

#### Scenario: Telegram structured delivery becomes text plus inline keyboard
- **WHEN** a typed delivery containing structured content targets Telegram
- **THEN** the rendering step resolves the delivery into Telegram-supported text plus inline-keyboard output instead of an unsupported card payload
- **AND** the resulting delivery receipt records that Telegram-native structured rendering was chosen through the provider profile

#### Scenario: WeCom typed delivery becomes supported app-message content
- **WHEN** a typed delivery containing structured or richer card intent targets WeCom
- **THEN** the rendering step resolves the delivery into a WeCom-supported text, news-style, or template-card-compatible representation according to the active WeCom provider profile
- **AND** transport execution does not require shared layers to special-case WeCom outside the provider contract

#### Scenario: Slack native Block Kit delivery dispatched through rendering plan
- **WHEN** a typed delivery containing a Slack Block Kit native payload targets Slack
- **THEN** the rendering step selects the native delivery path and dispatches through the Slack adapter's `NativeMessageSender`
- **AND** the delivery receipt records `type: "native"` without fallback

#### Scenario: Discord native embed delivery dispatched through rendering plan
- **WHEN** a typed delivery containing a Discord embed native payload targets Discord
- **THEN** the rendering step selects the native delivery path and dispatches through the Discord adapter's `NativeMessageSender`
- **AND** the delivery receipt records `type: "native"` without fallback

#### Scenario: DingTalk native card delivery dispatched through rendering plan
- **WHEN** a typed delivery containing a DingTalk card native payload targets DingTalk
- **THEN** the rendering step selects the native delivery path and dispatches through the DingTalk adapter's `NativeMessageSender`

#### Scenario: Native payload targeting wrong platform uses fallback text
- **WHEN** a typed delivery containing a Slack Block Kit native payload targets a Telegram bridge
- **THEN** the rendering step falls back to text delivery using the payload's `FallbackText()`
- **AND** the delivery receipt records `fallback_reason: "native_platform_mismatch"`

#### Scenario: Section-based structured message renders richer platform output
- **WHEN** a typed delivery containing a `StructuredMessage` with non-empty `Sections` targets Slack
- **THEN** the rendering step converts sections to Slack Block Kit blocks (section, image, divider, context, actions)
- **AND** the result is richer than the legacy title+body+fields rendering

#### Scenario: Feishu typed delivery becomes builder-owned native content
- **WHEN** a typed delivery containing richer card intent targets Feishu
- **THEN** the rendering step resolves the delivery through Feishu's provider-owned builders into JSON-card, template-card, or `lark_md`-backed output as appropriate
- **AND** transport execution does not require shared layers to assemble raw Feishu payload fragments directly

#### Scenario: QQ typed delivery becomes provider-supported QQ content
- **WHEN** a typed delivery containing structured or richer intent targets QQ
- **THEN** the rendering step resolves the delivery into a QQ-supported text, link, or provider-supported structured representation according to the active QQ provider profile
- **AND** transport execution does not require shared layers to special-case QQ outside the provider contract

#### Scenario: QQ Bot typed delivery becomes provider-supported QQ Bot content
- **WHEN** a typed delivery containing structured or richer intent targets QQ Bot
- **THEN** the rendering step resolves the delivery into a QQ Bot-supported text, link, or provider-supported structured representation according to the active QQ Bot provider profile
- **AND** transport execution does not require shared layers to special-case QQ Bot outside the provider contract
