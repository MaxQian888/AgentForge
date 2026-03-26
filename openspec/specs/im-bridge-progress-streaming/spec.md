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
The IM bridge progress streaming system SHALL forward document-related events to configured IM channels.

#### Scenario: Page created event streamed to IM
- **WHEN** a wiki page is created in a project with an IM channel configured for doc events
- **THEN** the IM bridge sends a message to the channel with the page title, creator, and a link to the page

#### Scenario: Comment mention forwarded to IM
- **WHEN** a user is @-mentioned in a wiki comment and has IM notifications enabled
- **THEN** the IM bridge sends a direct message to the user with the comment context and a link to the comment

### Requirement: Automation-triggered IM messages
The IM bridge progress streaming system SHALL deliver messages triggered by automation rule actions.

#### Scenario: Automation sends IM message
- **WHEN** an automation rule executes a send_im_message action with a channel and template
- **THEN** the IM bridge renders the template with event context and sends the message to the specified channel

### Requirement: IM actions for message-to-doc and message-to-task conversion
The IM bridge SHALL support actions to convert an IM message into a wiki page or a task.

#### Scenario: Convert message to doc page
- **WHEN** user triggers the "Save as Doc" action on an IM message
- **THEN** the IM bridge creates a wiki page in the project's doc space with the message content as the body, and replies with a link to the created page

#### Scenario: Convert message to task
- **WHEN** user triggers the "Create Task" action on an IM message
- **THEN** the IM bridge creates a task in the project backlog with the message content as the description, sets origin=im, and replies with a link to the created task
