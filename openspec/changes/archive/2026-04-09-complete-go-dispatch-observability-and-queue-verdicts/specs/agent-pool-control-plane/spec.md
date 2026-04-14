## MODIFIED Requirements

### Requirement: Queue lifecycle preserves latest guardrail verdict and admission context
The system SHALL retain the runtime tuple, budget context, queue identity, and latest dispatch guardrail verdict for each queue entry across queued, promoted, cancelled, and failed states. Operator-facing queue and pool views MUST be able to tell whether an entry is still waiting because of recoverable guardrails, has been re-queued after promotion revalidation, or has failed promotion terminally.

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

#### Scenario: Promoted queue entry preserves its original dispatch tuple
- **WHEN** a queued entry is successfully promoted into runtime startup
- **THEN** the promotion uses the authoritative runtime, provider, model, role, and budget context stored for that admission
- **THEN** the queue history remains traceable to the original queued request through the queue identity and promoted run linkage
- **THEN** operator-facing lifecycle events can relate the promoted run back to the originating queue entry without reconstructing the tuple from free-form text

### Requirement: Queue roster endpoint exposes individual entries
The system SHALL expose a dedicated queue roster endpoint that returns individual queue entries for a project, complementing the count-based pool summary. The roster MUST include all fields needed for operator decision-making: task identity, member identity, runtime tuple, priority, budget context, current lifecycle status, queue identity, latest guardrail verdict, and the metadata needed to distinguish recoverable waiting states from terminal promotion failures.

#### Scenario: Queue roster returns entries in admission order with verdict metadata
- **WHEN** an authenticated operator requests the queue roster for a project
- **THEN** entries are returned in admission order: priority descending, then creation time ascending
- **THEN** each entry includes task ID, member ID, runtime, provider, model, role ID, priority, budget USD, queue identity, lifecycle status, latest guardrail verdict, and timestamps
- **THEN** a queued, re-queued, promoted, failed, or cancelled entry can be interpreted without parsing only the human-readable reason string
