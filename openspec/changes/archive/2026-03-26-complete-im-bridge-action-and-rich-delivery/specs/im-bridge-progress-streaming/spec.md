## ADDED Requirements

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
