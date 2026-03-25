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
