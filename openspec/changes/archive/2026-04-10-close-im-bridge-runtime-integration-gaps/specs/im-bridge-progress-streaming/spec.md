## MODIFIED Requirements

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
