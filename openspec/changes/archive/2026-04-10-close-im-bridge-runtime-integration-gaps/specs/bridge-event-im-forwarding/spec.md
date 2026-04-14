## MODIFIED Requirements

### Requirement: Budget alerts forwarded to IM platforms
The system SHALL treat budget alerts as canonical forwarded runtime events whether they originate in TS Bridge or are synthesized by Go from the same agent-budget threshold transition. When a budget alert belongs to an IM-originated run with preserved reply-target lineage, the backend MUST return the alert through that bound IM context. When no bound reply target is available, the backend SHALL resolve delivery through authoritative channel/event routing or an explicit compatibility fallback. Routing filters and preference metadata MUST be able to suppress delivery without pretending the underlying alert never occurred.

#### Scenario: Bound budget alert returns to the originating IM conversation
- **WHEN** a running IM-originated agent crosses the budget warning threshold
- **AND** the run has a preserved reply target bound to its originating bridge conversation
- **THEN** the backend queues the budget alert back to that same bound IM context through the control plane
- **AND** the forwarded delivery preserves budget-related event metadata for diagnostics and preference filtering

#### Scenario: Unbound budget alert routes through configured channel subscriptions
- **WHEN** a budget alert is emitted for work that has no bound IM reply target
- **AND** an active IM channel subscribes to the corresponding budget warning event type
- **THEN** the backend routes the alert to the subscribed channel through the canonical IM delivery pipeline
- **AND** it does not fabricate watcher-specific delivery to an unrelated IM destination

### Requirement: Agent status changes forwarded to IM for operational visibility
The system SHALL forward agent status changes through Go backend mediation using authoritative routing sources instead of implicit watcher fan-out. If the affected run has a bound IM reply target, the status change MUST be delivered back to that same conversation or interaction context in order. If no bound reply target exists, the backend MUST still update orchestration state and MAY emit a channel-scoped IM delivery only when another configured routing source explicitly applies.

#### Scenario: Bound status change updates the same conversation
- **WHEN** TS Bridge emits `status_change` from `running` to `paused` for an IM-originated run
- **AND** the run has a preserved reply target bound to the originating bridge conversation
- **THEN** the backend forwards the paused update to that same IM conversation in order
- **AND** the delivery does not require the backend to rediscover a watcher list

#### Scenario: Unbound status change does not invent IM delivery
- **WHEN** TS Bridge emits `status_change` for a run that has no bound IM reply target
- **AND** no configured channel routing rule applies to that event
- **THEN** the backend still records the orchestration state transition and websocket-visible status change
- **AND** it does not fabricate a synthetic IM watcher delivery just to satisfy the forwarding path

### Requirement: Event ordering guarantees delivery
Go backend and the IM control plane SHALL preserve enqueue order for forwarded events that belong to the same bound entity or delivery stream. Permission requests, status changes, terminal updates, and replayed deliveries MUST remain causally ordered for the same task or run instead of being reordered by ad hoc batching.

#### Scenario: Permission request is delivered before a later paused status
- **WHEN** TS Bridge emits `permission_request`
- **AND** then emits a later `status_change` to `paused` for the same run
- **THEN** the backend enqueues the permission request before the paused status for that bound IM context
- **AND** IM Bridge receives those deliveries in the same order

#### Scenario: Replayed deliveries preserve cursor order after reconnect
- **WHEN** an IM Bridge reconnects and requests replay for queued forwarded events
- **THEN** the control plane replays those pending deliveries in cursor order for that bridge instance
- **AND** the bridge does not receive a later terminal event ahead of an earlier permission or progress event from the same queue

### Requirement: Event forwarding respects user notification preferences
The system SHALL honor authoritative forwarding preferences encoded in runtime routing metadata. For bound flows this includes reply-target metadata such as `bridge_event_enabled.<type>`; for channel-scoped flows this includes configured channel event subscriptions. A suppressed delivery MUST remain truthful in delivery history or diagnostics instead of being silently rerouted to an unrelated target.

#### Scenario: Bound preference disables permission request delivery
- **WHEN** a bound IM reply target contains `bridge_event_enabled.permission_request = false`
- **AND** the backend receives a forwarded permission request event for that same bound run
- **THEN** the backend suppresses IM delivery for that event type
- **AND** the resulting diagnostics remain truthful about the suppressed forwarding decision

#### Scenario: Missing channel subscription suppresses broadcast forwarding
- **WHEN** the backend emits a channel-scoped runtime event such as a budget warning
- **AND** no active configured channel subscribes to that event type
- **THEN** the event is not delivered to unrelated channels
- **AND** only an explicit compatibility fallback may send it elsewhere
