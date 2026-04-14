## MODIFIED Requirements

### Requirement: Queue lifecycle preserves latest guardrail verdict and admission context
The system SHALL retain the runtime tuple, budget context, queue identity, latest dispatch guardrail verdict, and final promoted run linkage for each queue entry across queued, promoted, cancelled, and failed states. Operator-facing queue and pool views and realtime lifecycle events MUST expose finalized queue state so consumers can tell whether an entry is still queued because of a recoverable guardrail, has failed terminally, or has been promoted into a specific run.

#### Scenario: Recoverable guardrail failure keeps a queue entry visible with its latest verdict
- **WHEN** a queued entry fails promotion revalidation because of a recoverable budget or transient infrastructure guardrail
- **THEN** the queue entry remains in a visible queued state instead of being silently discarded
- **THEN** the queue entry retains the latest machine-readable guardrail verdict together with its original runtime, provider, model, role, priority, and budget admission context
- **THEN** realtime pool lifecycle events make it clear that the entry is still queued rather than started or terminally failed

#### Scenario: Terminal promotion failure preserves admission history
- **WHEN** a queued entry fails promotion revalidation because task, member, or runtime ownership context is irrecoverably invalid
- **THEN** the queue entry transitions to a terminal failed state
- **THEN** operator-facing APIs and realtime events retain the original admission context together with the terminal failure reason and latest guardrail verdict
- **THEN** consumers can distinguish this terminal failure from an entry that remains queued awaiting recovery

#### Scenario: Promoted queue event exposes finalized queue linkage
- **WHEN** a queued entry is successfully promoted into runtime startup
- **THEN** the queue entry is finalized to `promoted` state with promoted recovery disposition and linked run identity before the `agent.queue.promoted` event is emitted
- **THEN** the promotion payload includes the finalized queue record and the linked run so consumers do not receive a stale `admitted` snapshot
- **THEN** operator-facing lifecycle events can relate the promoted run back to the originating queue entry without reconstructing the tuple from free-form text
