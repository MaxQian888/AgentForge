# im-bridge-progress-streaming Specification

## Purpose
Define the long-running IM interaction contract for AgentForge so commands acknowledged in chat can continue streaming progress and terminal updates back to the original reply target without duplication across reconnects or restarts.
## Requirements
### Requirement: Long-running IM commands acknowledge acceptance immediately
The system SHALL return an immediate user-visible acknowledgement when an IM command triggers work that cannot complete within the provider's normal request window. The acknowledgement MUST confirm that the request was accepted, identify the relevant task or run when available, and establish that further progress will arrive asynchronously instead of leaving the user waiting on a silent command.

#### Scenario: Agent run is accepted from IM
- **WHEN** a user triggers a long-running command such as `/agent run` or task assignment that starts an agent
- **THEN** the Bridge sends an immediate acknowledgement back to the originating IM conversation
- **AND** that acknowledgement indicates the work has started or been queued even if the final outcome is not yet known

#### Scenario: Task decomposition is accepted from IM
- **WHEN** a user triggers `/task decompose <id>` and decomposition begins asynchronously
- **THEN** the Bridge sends an immediate acknowledgement to the same IM conversation
- **AND** the user does not need to infer from silence whether the command was received

### Requirement: Progress updates remain attached to the originating reply target
The system SHALL preserve a serializable reply target for each IM-initiated long-running action so that progress and completion updates can be delivered back to the same conversation, thread, or interaction context. The backend and Bridge MUST use that preserved reply target rather than guessing a new destination from task metadata alone.

#### Scenario: Threaded platform keeps progress in the same thread
- **WHEN** a Slack or similar threaded platform command starts a long-running action
- **THEN** the Bridge preserves the thread-aware reply target for that action
- **AND** later progress updates are posted back into the same thread instead of a new unrelated conversation

#### Scenario: Deferred interaction keeps follow-up context
- **WHEN** a Discord interaction or other deferred-response platform starts a long-running action
- **THEN** the Bridge preserves the provider context needed for follow-up delivery
- **AND** later progress or completion messages use that preserved context instead of failing due to a missing interaction handle

### Requirement: Completion and recovery updates remain user-visible without duplication
The system SHALL emit a terminal update for every accepted long-running IM action that reaches a completed, failed, blocked, or cancelled state. If the Bridge reconnects or restarts while an action is still in flight, it SHALL recover the latest known delivery state and continue with progress or terminal updates without re-sending the initial acceptance message as a fresh duplicate.

#### Scenario: Terminal update summarizes the finished action
- **WHEN** a long-running IM-triggered action finishes with success, failure, blocked, or cancelled status
- **THEN** the Bridge sends a terminal summary to the originating IM reply target
- **AND** that summary includes the final status plus the most relevant task, run, review, or PR identifier available

#### Scenario: Bridge restart resumes updates without duplicating acceptance
- **WHEN** a Bridge reconnects after a restart while a previously accepted action is still running
- **THEN** it restores the latest known reply target and delivery state for that action
- **AND** it resumes progress or terminal updates without posting a second "accepted" message for the same command

### Requirement: Progress and terminal updates SHALL use the canonical typed delivery contract
The system SHALL route IM-bound progress and terminal updates through the same canonical typed outbound delivery contract used by direct notifications. Bound progress delivery MUST preserve reply-target preferences, rich payload shape, and provider-native update options so asynchronous updates can continue to use edit, follow-up, thread, session-webhook, card-update, or structured reply paths after queueing and replay.

#### Scenario: Discord terminal update keeps original-response edit semantics after queueing
- **WHEN** a Discord-originated long-running action reaches a terminal state after its update has been queued through the control plane
- **THEN** the queued terminal delivery preserves the interaction-scoped reply target and typed payload needed for original-response edit or follow-up
- **AND** the Bridge does not degrade that terminal update to a new unrelated plain-text send unless the preserved target is unusable

#### Scenario: Feishu progress replay preserves native update choice and fallback metadata
- **WHEN** a Feishu long-running action uses delayed card update or another native progress path and the Bridge must replay pending deliveries after reconnect
- **THEN** the replayed progress or terminal delivery retains the native payload or structured fallback choice encoded in the canonical delivery envelope
- **AND** any fallback reason remains visible instead of being lost during replay

#### Scenario: Text-only platform or target degrades explicitly through the same contract
- **WHEN** a progress or terminal update is queued for a platform or reply target that cannot honor the preferred rich or mutable path
- **THEN** the canonical delivery contract falls back to the supported text path
- **AND** the fallback remains explicit and consistent with direct notification delivery semantics

### Requirement: Document event streaming to IM
The IM bridge progress streaming system SHALL forward document-related events through authoritative channel/event routing instead of a hardcoded single IM target. When a configured channel subscribes to a document event, the backend MUST deliver the event through the canonical IM pipeline to that channel. When no configured route exists, the system MAY use an explicit compatibility fallback if configured, and MUST keep that fallback visible in delivery metadata or diagnostics.

#### Scenario: Page created event uses subscribed channel routing
- **WHEN** a wiki page is created in a project
- **AND** an active IM channel subscribes to `wiki.page.updated`
- **THEN** the backend sends the document notification to that subscribed channel with the page title and link
- **AND** the delivery uses the canonical IM notify/send path rather than a special-case wiki-only transport

#### Scenario: Mention event degrades truthfully when no direct IM mapping exists
- **WHEN** a user is @-mentioned in a wiki comment
- **AND** the system lacks a direct user-to-IM identity mapping for one-to-one delivery
- **THEN** the backend keeps the in-app mention notification behavior
- **AND** any IM forwarding uses configured channel routing or explicit compatibility fallback without fabricating a fake direct-message target

### Requirement: Automation-triggered IM messages
The IM bridge progress streaming system SHALL deliver automation-triggered IM messages through the canonical IM send pipeline using an explicit routing target. Automation-triggered IM delivery MUST fail explicitly when the action cannot resolve a usable routing target instead of silently choosing an unrelated global channel.

#### Scenario: Automation sends IM message to a configured channel
- **WHEN** an automation rule executes `send_im_message`
- **AND** the action resolves a configured Slack channel target for the current project
- **THEN** the backend renders the template with event context and sends the message through the canonical IM send pipeline
- **AND** the resulting delivery is visible in IM delivery history

#### Scenario: Automation message without a usable route fails explicitly
- **WHEN** an automation rule executes `send_im_message`
- **AND** it does not resolve a usable channel target and no compatibility fallback is configured
- **THEN** the automation action returns an explicit failure
- **AND** the system does not silently send the message to an unrelated default IM channel

### Requirement: IM actions for message-to-doc and message-to-task conversion
The IM bridge SHALL expose user-facing actions that convert a source IM message into a wiki page or a task, and those actions SHALL preserve source message context and reply-target lineage through the backend action contract.

#### Scenario: Save as Doc action returns the created page in the same conversation
- **WHEN** a user triggers the `Save as Doc` action on a message-backed IM card or interaction
- **THEN** the backend creates a wiki page using the source message content and metadata
- **AND** the action result returns a link to the created page back into the originating IM conversation

#### Scenario: Create Task action preserves source message context
- **WHEN** a user triggers the `Create Task` action on a message-backed IM card or interaction
- **THEN** the backend creates a project task whose title or description is derived from the source message metadata
- **AND** the IM action result returns the created task identity and link without losing the original reply-target context

### Requirement: China-platform progress replay SHALL preserve completion-mode preference and downgrade truth
For DingTalk, WeCom, QQ Bot, and QQ long-running actions, the control-plane replay contract SHALL preserve enough provider completion metadata to let the Bridge retry the originally preferred reply or update path after reconnect. If replay can no longer honor that provider-specific path, the Bridge MUST emit the documented fallback and preserve the resulting downgrade reason instead of silently switching to an unrelated send path.

#### Scenario: DingTalk replay preserves session-aware completion mode
- **WHEN** a DingTalk progress or terminal update is replayed after reconnect
- **THEN** the replayed delivery preserves whether the original preferred path was session webhook reply or conversation-scoped reply
- **AND** fallback remains explicit if neither path is still usable

#### Scenario: WeCom replay preserves callback-versus-direct-send preference
- **WHEN** a WeCom progress or terminal update is replayed after reconnect
- **THEN** the replayed delivery preserves whether the original preferred path was `response_url` reply or direct app send
- **AND** the final delivery metadata exposes any fallback taken during replay

#### Scenario: QQ Bot or QQ replay preserves conversation-scoped completion truth
- **WHEN** a QQ Bot or QQ progress or terminal update is replayed after reconnect
- **THEN** the replayed delivery preserves the original conversation-scoped reply target data needed for provider-supported follow-up delivery
- **AND** the Bridge does not re-emit the initial acceptance message as a duplicate fallback

