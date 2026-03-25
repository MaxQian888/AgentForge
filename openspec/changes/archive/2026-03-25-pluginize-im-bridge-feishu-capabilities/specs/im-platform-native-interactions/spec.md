## MODIFIED Requirements

### Requirement: Asynchronous progress and completion updates SHALL use the platform-native update path before fallback
For IM-initiated long-running actions, the system SHALL prefer the update path that is native to the originating platform and reply target. If a platform supports message edits, follow-ups, thread replies, session webhooks, or delayed card updates, the Bridge MUST use that strategy first. Only when the originating reply target or capability matrix does not support the preferred strategy MAY the system fall back to a new plain-text message. For Feishu, the preferred path MUST distinguish between immediate callback responses and delayed card mutation so the Bridge does not collapse all card interactions into generic reply semantics.

#### Scenario: Slack progress stays inside the originating thread
- **WHEN** a Slack command or mention starts a long-running action with a preserved `thread_ts` or `response_url`
- **THEN** progress and terminal updates are delivered inside the same thread or response context
- **AND** the Bridge does not create a new top-level channel message unless the preserved target is unavailable

#### Scenario: Discord deferred response becomes follow-up or edit
- **WHEN** a Discord interaction triggers work that cannot complete in the initial response window
- **THEN** the Bridge first satisfies the required deferred acknowledgement
- **AND** later progress or completion uses the interaction follow-up or original-response edit path associated with the preserved interaction token

#### Scenario: Feishu card action prefers immediate or delayed card mutation
- **WHEN** a Feishu card interaction starts a long-running action and the preserved reply target includes card update context
- **THEN** the Bridge first chooses an immediate callback response or a delayed card update within the provider-supported window before considering a plain-text fallback
- **AND** users remain in the same card conversation context rather than receiving an unrelated duplicate notification

#### Scenario: DingTalk uses session webhook when conversational reply is available
- **WHEN** a DingTalk action or command preserves a session webhook or conversation-scoped reply target
- **THEN** the Bridge uses that session-aware outbound path for progress or completion
- **AND** it falls back to direct-send text only when the session-aware path is unavailable

### Requirement: Platform-native interactive callbacks SHALL normalize into one backend action contract
The system SHALL normalize platform-native interactive inputs into one backend action contract that preserves action identity, entity identity, reply target, bridge identity, and provider-specific metadata. Buttons, modal submissions, select menus, inline keyboard callbacks, and card actions MUST all be convertible into the same action envelope so backend workflows can respond consistently regardless of provider. For Feishu, the normalized envelope MUST preserve the current `card.action.trigger` interaction value, originating card or message identity, operator identity, and callback-response context required for immediate acknowledgement or delayed update without leaking raw Feishu callback parsing responsibilities upstream.

#### Scenario: Slack block action preserves response context
- **WHEN** a Slack Block Kit button or modal submission triggers an action
- **THEN** the Bridge normalizes it into the shared action contract with preserved `response_url`, channel, thread, and user context
- **AND** the backend can issue a follow-up update without receiving a Slack-specific payload shape

#### Scenario: Telegram callback query becomes a shared action
- **WHEN** a Telegram inline-keyboard callback query is received
- **THEN** the Bridge normalizes the callback data, chat identity, message identity, and user context into the shared action envelope
- **AND** later completion can edit or reply to the same Telegram message using the preserved target

#### Scenario: Feishu or DingTalk card action carries provider metadata without leaking provider-specific APIs upstream
- **WHEN** a Feishu or DingTalk interactive card action is received
- **THEN** the Bridge normalizes it into the shared backend action contract while preserving provider-specific metadata in the reply target or metadata bag
- **AND** upstream handlers do not need to parse Feishu- or DingTalk-specific callback payloads directly
