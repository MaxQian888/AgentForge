# im-platform-native-interactions Specification

## Purpose
Define the platform-native interaction contract for AgentForge IM Bridge so capability matrices, interactive callbacks, mutable reply targets, and explicit downgrade paths remain consistent across Slack, Discord, Telegram, Feishu, and DingTalk.
## Requirements
### Requirement: Platform capability matrix SHALL describe native interaction strategy, not just transport availability
The system SHALL publish a capability matrix for each active IM platform that describes the platform's native command surface, structured-message surface, callback mode, asynchronous update mode, message scope, and mutability semantics. The Bridge, control-plane registration payload, and health surfaces MUST use this matrix to choose delivery behavior instead of inferring behavior from platform names or from a single rich-message boolean.

#### Scenario: Slack declares threaded block-capable interactions
- **WHEN** the active platform is Slack
- **THEN** the Bridge publishes capability metadata that identifies Socket Mode command intake, Block Kit structured output, threaded reply scope, and response-driven follow-up behavior
- **AND** downstream delivery code can choose thread-aware block responses without hard-coding `"slack"`

#### Scenario: Discord declares deferred interaction lifecycle
- **WHEN** the active platform is Discord
- **THEN** the Bridge publishes capability metadata that identifies public interaction callback requirements, deferred acknowledgement support, and follow-up or original-response mutation behavior
- **AND** asynchronous work can be routed through the correct interaction lifecycle without treating Discord like a generic chat reply target

#### Scenario: Telegram declares mutable text-first updates
- **WHEN** the active platform is Telegram
- **THEN** the Bridge publishes capability metadata that identifies inline-keyboard callbacks, mutable message updates, and text-first rendering constraints
- **AND** progress delivery can prefer low-noise message edits over repeated new messages when the reply target supports it

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

### Requirement: Unsupported native features SHALL degrade explicitly and truthfully
If a requested structured or interactive behavior is unsupported by the active platform or by the current preserved reply target, the system SHALL degrade to a supported experience explicitly. It MUST avoid emitting unusable buttons, invalid edit attempts, or fake parity claims, and it MUST preserve enough metadata for operators to understand why fallback occurred.

#### Scenario: Telegram avoids unsupported card semantics
- **WHEN** a notification requests a card-style rich message on Telegram
- **THEN** the Bridge renders the supported Telegram text or inline-keyboard variant instead of attempting an unsupported card payload
- **AND** the delivery path records that the fallback occurred because Telegram does not advertise card-style structured output

#### Scenario: Future provider remains unavailable until matrix and adapter are complete
- **WHEN** a provider such as `wecom` is present in roadmap or model-level enums but does not yet have a live adapter and declared capability matrix
- **THEN** the system marks that provider as not yet supported for runtime activation
- **AND** operators can see that the absence is an explicit gap rather than an implicit silent fallback

### Requirement: Normalized interactive callbacks SHALL produce truthful backend outcomes
After a platform-native interactive callback is normalized into the shared action envelope, the system SHALL execute the corresponding backend action workflow or return an explicit terminal failure. It MUST preserve the originating reply target and provider metadata, and it MUST NOT claim that assignment, decomposition, approval, or change-request actions succeeded when the backend only acknowledged receipt.

#### Scenario: Slack or Discord callback returns a real action outcome
- **WHEN** a Slack Block Kit action or Discord component interaction is normalized into the shared action contract
- **THEN** the backend executes the mapped task, agent, or review workflow instead of returning a placeholder acknowledgement
- **AND** the Bridge renders the resulting started, blocked, failed, or completed outcome back through the preserved reply target

#### Scenario: Feishu or DingTalk card action preserves provider-aware completion semantics
- **WHEN** a Feishu or DingTalk card action is normalized into the shared action contract with preserved callback metadata
- **THEN** the backend returns a truthful action outcome plus the reply-target context needed for the provider-aware completion path
- **AND** the Bridge may use immediate callback response, delayed update, or explicit fallback according to the provider capability matrix

#### Scenario: Unsupported or stale callback remains explicit
- **WHEN** a normalized platform callback refers to an invalid, stale, or unsupported action transition
- **THEN** the backend returns an explicit failed or blocked outcome
- **AND** the platform response does not claim the business mutation succeeded

### Requirement: Telegram interaction completions SHALL honor markdown-aware mutable-update safety
When a Telegram callback query or command completion is rendered back to the originating reply target, the Bridge SHALL evaluate the completion through Telegram's rendering profile before choosing `editMessageText`, reply, or follow-up delivery. The Bridge MUST use a Telegram formatted-text path only when the content is safe for the provider's formatting and mutable-update rules, and it MUST otherwise fall back to a supported plain-text edit or reply path.

#### Scenario: Safe Telegram callback completion edits the original message
- **WHEN** a Telegram callback query finishes with content that is safe for the provider-selected text mode and the preserved reply target supports editing
- **THEN** the Bridge answers the callback query and updates the originating Telegram message in place through the Telegram-native mutable update path
- **AND** the user does not receive an unnecessary duplicate completion message

#### Scenario: Unsafe Telegram formatted completion falls back before edit
- **WHEN** a Telegram callback completion requests formatted text that cannot be rendered safely for the preserved reply target
- **THEN** the Bridge answers the callback query and falls back to a supported plain-text edit or reply
- **AND** it does not send malformed Markdown-aware content through `editMessageText`

#### Scenario: Oversized Telegram completion degrades to segmented follow-up
- **WHEN** a Telegram callback completion exceeds the provider's editable text limits for the originating message context
- **THEN** the Bridge abandons the single-message edit plan and uses a provider-supported segmented reply or follow-up strategy
- **AND** the completion remains tied to the originating Telegram interaction context through preserved reply-target metadata

### Requirement: DingTalk adapter SHALL support ActionCard rendering

The DingTalk live adapter SHALL implement `SendActionCard()` to deliver interactive ActionCard messages via DingTalk OpenAPI. When the rendering profile resolves to card-typed delivery, the adapter SHALL construct and send an ActionCard payload with action buttons mapped to the typed envelope's action references.

#### Scenario: Sending an ActionCard with task actions
- **WHEN** a delivery envelope contains card-typed content with actions `["approve", "reject"]` targeting a task entity
- **THEN** the DingTalk adapter sends an ActionCard with two buttons labeled per the action references, each carrying the entity ID and action type in callback data

#### Scenario: ActionCard delivery fails
- **WHEN** DingTalk OpenAPI returns an error when sending ActionCard
- **THEN** the adapter falls back to plain-text delivery with action labels listed as text, and reports `X-IM-Downgrade-Reason: actioncard_send_failed`

### Requirement: DingTalk ActionCard callbacks SHALL normalize to shared action contract

When a user clicks an ActionCard button, the DingTalk adapter SHALL normalize the callback into an `IMActionRequest` through the existing `NormalizeAction()` path, preserving the entity ID, action type, and session webhook reply target.

#### Scenario: User clicks approve button on ActionCard
- **WHEN** DingTalk streams a card callback with action data `{"action": "approve", "entityId": "task-123"}`
- **THEN** the adapter produces an `IMActionRequest` with action `"approve"`, entity `"task-123"`, and reply target containing the session webhook URL

### Requirement: Review command engine SHALL support deep/approve/request-changes subcommands

The Bridge command engine SHALL handle `/review deep <taskId>`, `/review approve <reviewId>`, and `/review request-changes <reviewId> [reason]` subcommands. Each SHALL call the corresponding backend review API endpoint.

#### Scenario: /review deep command
- **WHEN** user sends `/review deep TASK-42`
- **THEN** Bridge calls `POST /api/v1/reviews` with `{"taskId": "TASK-42", "mode": "deep"}` and replies with the review creation confirmation

#### Scenario: /review approve command
- **WHEN** user sends `/review approve REV-10`
- **THEN** Bridge calls `POST /api/v1/reviews/REV-10/decide` with `{"decision": "approve"}` and replies with the decision result

#### Scenario: /review request-changes with reason
- **WHEN** user sends `/review request-changes REV-10 missing error handling`
- **THEN** Bridge calls `POST /api/v1/reviews/REV-10/decide` with `{"decision": "request_changes", "reason": "missing error handling"}` and replies with the decision result

