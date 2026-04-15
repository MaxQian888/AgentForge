## ADDED Requirements

### Requirement: IM-bound task lifecycle outcomes SHALL return through preserved reply targets
The system SHALL route task lifecycle follow-up produced by IM-originated task actions through the same preserved reply-target-aware delivery contract used by other long-running IM interactions. When an IM-originated task transition or adjacent task lifecycle action produces a later task or workflow verdict, the backend and Bridge MUST deliver that follow-up to the same bound conversation or thread when the preserved task binding is still valid.

#### Scenario: IM-originated task transition returns workflow verdict in the same thread
- **WHEN** a user transitions a task from IM and that transition later starts, blocks, skips, or fails a task-triggered workflow outcome
- **THEN** the backend queues a bound task follow-up delivery using the preserved reply target for that task action
- **AND** the Bridge renders the resulting task or workflow verdict in the same conversation or thread where the transition originated

#### Scenario: Missing live bound instance keeps task follow-up truthful
- **WHEN** a task lifecycle follow-up is ready but the bridge instance bound to the originating task action is no longer live
- **THEN** the control plane records the delivery as blocked, stale, or retryable according to the failure reason
- **AND** the system does not silently reroute that task lifecycle result to another unrelated bridge instance

#### Scenario: Replay preserves structured task follow-up after reconnect
- **WHEN** a task lifecycle follow-up for an IM-originated task action is queued while the Bridge is offline and later replayed after reconnect
- **THEN** the replayed delivery preserves the same task identity, structured payload, reply target, and fallback metadata chosen for that follow-up
- **AND** the Bridge does not re-emit the initial acceptance message as a duplicate fallback
