## ADDED Requirements

### Requirement: Platform adapters preserve deferred reply targets for later updates
The system SHALL preserve enough platform-specific reply target data when normalizing an inbound message or interaction so later progress and terminal updates can be delivered back to the same user-visible context. This preserved target MUST be serializable, MUST survive handoff to the backend, and MUST distinguish between plain chat replies, threaded replies, and provider-specific deferred follow-up contexts where applicable.

#### Scenario: Slack command preserves threaded reply target
- **WHEN** a Slack command or mention is normalized into a `core.Message`
- **THEN** the Bridge captures the channel and thread-aware reply target needed for later updates
- **AND** that target can be serialized and reused for asynchronous progress delivery

#### Scenario: Discord interaction preserves follow-up context
- **WHEN** a Discord interaction is normalized for shared command handling
- **THEN** the Bridge captures the interaction follow-up context required after the initial acknowledgement
- **AND** that context remains available for later completion messages

#### Scenario: Feishu or Telegram command preserves conversation target
- **WHEN** a Feishu or Telegram message starts a long-running action
- **THEN** the Bridge captures the conversation-specific reply target for that action
- **AND** later updates use the preserved target instead of inferring a destination only from task metadata

### Requirement: Platform metadata exposes delivery-relevant runtime characteristics
The active IM platform runtime SHALL expose the delivery characteristics needed by the control plane, including whether it requires a public callback endpoint, whether it supports deferred follow-up delivery, and whether it supports rich or editable messages for progress updates. Registration and health surfaces MUST reflect those characteristics so the backend can route deliveries and choose the correct update strategy.

#### Scenario: Health and registration reflect callback requirements
- **WHEN** a platform such as Discord requires a public interactions endpoint for live delivery
- **THEN** the Bridge health or registration payload identifies that callback exposure requirement
- **AND** the backend can distinguish it from platforms that do not require a public callback

#### Scenario: Health and registration reflect deferred update capabilities
- **WHEN** a platform supports deferred replies or editable progress updates
- **THEN** the Bridge health or registration payload reports those capabilities
- **AND** the backend can choose a compatible progress delivery strategy for that platform
