## MODIFIED Requirements

### Requirement: Notifications respect platform matching and capability-aware rich-message fallback
The notification receiver SHALL only deliver a notification through the active platform instance when the notification platform matches the running bridge platform. If a notification includes structured content, update context, or interaction affordances, the Bridge MUST first choose the native renderer and update path declared by the active platform's capability matrix. When the active platform and preserved reply target support that native path, the Bridge SHALL send the structured or mutable response in the same platform-native context. Otherwise, it SHALL fall back to the supported plain-text or text-plus-link variant instead of emitting unsupported controls or invalid message mutations.

#### Scenario: Matching Slack delivery uses native threaded blocks
- **WHEN** the notification receiver receives a notification whose platform matches the active Slack bridge
- **AND** the notification contains structured content with a preserved thread-aware reply target
- **AND** the Slack capability matrix declares Block Kit rendering and threaded follow-up support
- **THEN** the Bridge sends the structured notification back into the same Slack thread using the native Slack renderer

#### Scenario: Matching Discord delivery uses interaction-aware update semantics
- **WHEN** the notification receiver receives a notification whose platform matches the active Discord bridge
- **AND** the notification contains a preserved interaction target that supports deferred follow-up or original-response editing
- **THEN** the Bridge delivers the update through the native Discord interaction path
- **AND** it does not fall back to an unrelated plain chat send unless the preserved target is unusable

#### Scenario: Matching platform without the required native capability falls back cleanly
- **WHEN** the notification receiver receives a notification whose platform matches the active bridge platform
- **AND** the notification requests structured or mutable behavior that the active platform or preserved reply target does not support
- **THEN** the Bridge sends the supported plain-text or minimally interactive fallback instead
- **AND** it does not emit buttons, cards, or edit attempts that the active platform cannot honor

#### Scenario: Mismatched platform notification is rejected
- **WHEN** the notification receiver receives a notification whose platform does not match the active bridge platform
- **THEN** the bridge rejects the delivery request with an explicit error
- **AND** the notification is not sent to the wrong IM platform
