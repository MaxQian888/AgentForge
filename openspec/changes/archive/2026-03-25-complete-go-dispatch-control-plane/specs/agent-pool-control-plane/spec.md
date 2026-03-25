## ADDED Requirements

### Requirement: Queue lifecycle preserves latest guardrail verdict and admission context
The system SHALL retain the runtime tuple, budget context, and latest dispatch guardrail verdict for each queue entry across queued, promoted, and failed states. Operator-facing queue and pool views MUST be able to tell whether an entry is still waiting because of recoverable guardrails or has failed promotion terminally.

#### Scenario: Recoverable guardrail failure keeps a queue entry visible
- **WHEN** a queued entry fails promotion revalidation because of a recoverable budget or transient infrastructure guardrail
- **THEN** the queue entry remains in a visible queued state instead of being silently discarded
- **THEN** the latest guardrail verdict is reflected in the queue entry's operator-facing data
- **THEN** realtime pool lifecycle events make it clear that the entry is still queued rather than started or terminally failed

#### Scenario: Terminal promotion failure preserves admission history
- **WHEN** a queued entry fails promotion revalidation because task, member, or runtime ownership context is irrecoverably invalid
- **THEN** the queue entry transitions to a terminal failed state
- **THEN** operator-facing APIs and realtime events retain the original admission context together with the terminal failure reason
- **THEN** consumers can distinguish this terminal failure from an entry that remains queued awaiting recovery

#### Scenario: Promoted queue entry preserves its original dispatch tuple
- **WHEN** a queued entry is successfully promoted into runtime startup
- **THEN** the promotion uses the authoritative runtime, provider, model, role, and budget context stored for that admission
- **THEN** the queue history remains traceable to the original queued request
- **THEN** operator-facing lifecycle events can relate the promoted run back to the originating queue entry
