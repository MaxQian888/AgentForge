## ADDED Requirements

### Requirement: Configured IM channels SHALL be authoritative for channel-scoped event routing
The system SHALL resolve channel-scoped IM events through active `IMChannel` records and their subscribed event types before using any legacy single-target compatibility fallback. When one or more active configured channels match the event type, the backend MUST route the event to those channels through the canonical IM delivery pipeline instead of sending it to an unrelated repo-level fallback target.

#### Scenario: Wiki event routes to a subscribed configured channel
- **WHEN** a wiki page update emits `wiki.page.updated`
- **AND** an active Slack channel is configured with `wiki.page.updated` in its event subscriptions
- **THEN** the backend routes the event to that configured Slack channel through the canonical IM notify/send pipeline
- **AND** the event does not also go to an unrelated compatibility fallback target

#### Scenario: Compatibility fallback is used only when no configured channel matches
- **WHEN** a channel-scoped IM event is emitted
- **AND** no active configured channel subscribes to that event type
- **AND** a legacy compatibility fallback target is configured
- **THEN** the backend MAY deliver the event through that compatibility fallback target
- **AND** the resulting delivery metadata and diagnostics indicate that compatibility fallback was used

### Requirement: Channel subscriptions SHALL suppress unrelated IM deliveries
The system SHALL treat channel event subscriptions as a routing filter, not as decorative metadata. Inactive channels or channels that do not subscribe to a given event type MUST NOT receive that event, even when they share the same platform or project scope.

#### Scenario: Unsubscribed channel is skipped
- **WHEN** an active Feishu channel subscribes only to `review.requested`
- **AND** the backend emits `wiki.version.published`
- **THEN** that channel does not receive the wiki version event
- **AND** the backend does not fabricate delivery to the channel just because the platform matches

#### Scenario: Inactive channel is ignored
- **WHEN** a Slack channel is configured for `workflow.failed` but marked inactive
- **AND** the backend emits `workflow.failed`
- **THEN** the inactive channel is ignored for routing
- **AND** only active matching channels remain eligible targets

### Requirement: Channel-routed IM deliveries SHALL remain operator-visible and truthful
The system SHALL record settlement-truthful delivery history for channel-routed events through the same diagnostics surface used by other IM deliveries. Operators MUST be able to inspect platform, channel, event type, and settlement outcome for deliveries that were resolved through channel routing.

#### Scenario: Channel-routed automation message appears in delivery history
- **WHEN** an automation-triggered IM message is routed to a configured QQ Bot channel
- **THEN** the resulting delivery record includes the resolved platform, target channel, event type, and settlement status
- **AND** the operator can inspect that record in `/im` history without a separate diagnostics path
