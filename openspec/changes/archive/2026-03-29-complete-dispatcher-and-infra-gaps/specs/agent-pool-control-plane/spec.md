## ADDED Requirements

### Requirement: Operators can cancel queued entries through the pool control plane
The system SHALL allow operators to cancel individual queued entries via the pool control plane API. Cancellation MUST transition the queue entry to `cancelled` status, emit a realtime pool lifecycle event, and update the pool summary's queued count. This requirement extends the existing operator visibility requirement to include write operations on queued entries.

#### Scenario: Cancel queued entry updates pool lifecycle events
- **WHEN** an operator cancels a queued entry through the pool control plane
- **THEN** the system emits an `agent.queue.cancelled` realtime event scoped to the project
- **THEN** the subsequent pool summary reflects one fewer queued entry
- **THEN** the cancellation is visible in the queue roster with the `cancelled` status and operator-provided or system-generated reason

#### Scenario: Cancelled entry does not participate in future promotions
- **WHEN** a queue entry has been cancelled
- **THEN** the `ReserveNextQueuedByProject` promotion logic skips cancelled entries
- **THEN** the next eligible non-cancelled entry is promoted instead

### Requirement: Queue roster endpoint exposes individual entries
The system SHALL expose a dedicated queue roster endpoint that returns individual queue entries for a project, complementing the count-based pool summary. The roster MUST include all fields needed for operator decision-making: task identity, member identity, runtime tuple, priority, budget context, and current guardrail verdict.

#### Scenario: Queue roster returns entries in admission order
- **WHEN** an authenticated operator requests the queue roster for a project
- **THEN** entries are returned in admission order: priority descending, then creation time ascending
- **THEN** each entry includes task ID, member ID, runtime, provider, model, role ID, priority, budget USD, reason, guardrail verdict, and timestamps
