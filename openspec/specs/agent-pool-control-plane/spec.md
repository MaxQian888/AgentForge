# agent-pool-control-plane Specification

## Purpose
Define the canonical AgentPool control-plane requirements for queue-backed admission, pool diagnostics, queued promotion, and operator-facing visibility across Go orchestration, Bridge runtime summaries, and dashboard consumers.
## Requirements
### Requirement: AgentPool exposes one canonical capacity and diagnostics summary
The system SHALL expose one canonical AgentPool control-plane summary per project that is authored by the Go admission service and enriched by Bridge runtime-pool diagnostics. The summary MUST include at least active slots, maximum slots, available slots, queued items, warm slots, paused-resumable runs, Bridge pool health, and recent warm-reuse versus cold-start counts.

#### Scenario: Authenticated operator reads the pool summary
- **WHEN** an authenticated operator requests the AgentPool summary for a project
- **THEN** the system returns one authoritative control-plane payload instead of requiring the client to derive pool state from agent run rows alone
- **THEN** the payload includes active, available, max, queued, warm, and paused-resumable counts
- **THEN** the payload includes Bridge runtime-pool health and recent warm-reuse or cold-start diagnostics

#### Scenario: Bridge diagnostics are degraded
- **WHEN** the Go control plane cannot refresh Bridge runtime-pool diagnostics for the current project scope
- **THEN** the summary still returns the latest known Go-owned admission state
- **THEN** the summary marks Bridge diagnostics as degraded instead of silently omitting that portion of the pool state

### Requirement: AgentPool admission queues eligible work when capacity is temporarily exhausted
The system SHALL treat temporary AgentPool exhaustion as an admission decision instead of only as a hard runtime failure. When a spawn or dispatch request targets an eligible task/member/runtime tuple but no execution slot is currently available, the system MUST persist a queue entry and return a `queued` outcome rather than reporting the request as blocked.

#### Scenario: Eligible spawn request is queued
- **WHEN** a valid spawn or dispatch request reaches AgentPool admission and there is no immediate execution slot available
- **THEN** the system persists a queue entry that captures task, member, runtime, provider, model, role, and budget admission context
- **THEN** the synchronous response reports a `queued` admission outcome instead of `blocked`
- **THEN** the system MUST NOT create a real agent run until the queued request is later admitted

#### Scenario: Invalid request is still blocked rather than queued
- **WHEN** a spawn or dispatch request targets an invalid member, a missing task, an unhealthy worktree path, or a task that already has an active agent run
- **THEN** the system returns a `blocked` or equivalent failure outcome
- **THEN** the system MUST NOT persist a queue entry for that request

### Requirement: Released capacity promotes queued work through the canonical spawn path
The system SHALL promote queued AgentPool entries through the same canonical spawn orchestration used by immediate starts. When capacity becomes available, the control plane MUST select the next eligible queued entry according to the configured queue ordering policy and start execution through the existing worktree and bridge startup flow.

#### Scenario: Terminal run completion admits the next queued task
- **WHEN** an active agent run reaches a terminal state and frees an execution slot
- **THEN** the control plane evaluates the queued entries for that project
- **THEN** the next eligible queued entry is admitted through the canonical spawn path
- **THEN** the admitted request creates a real agent run only after the control plane starts execution for that task

#### Scenario: Queued entry cannot be started during promotion
- **WHEN** a queued entry reaches the front of the admission order but startup fails because of a current worktree or bridge preflight problem
- **THEN** the system records that promotion failure against the queue entry
- **THEN** the failure is visible to operators without losing the admission history for that queued request

### Requirement: AgentPool lifecycle is visible to operator-facing APIs and realtime consumers

The agents dashboard SHALL integrate the dispatch history panel to surface dispatch attempt history for operators. The dispatch preflight dialog SHALL be accessible from the agents page so operators can run preflight checks without entering the spawn flow. These components SHALL be presented in a "Dispatch" tab alongside the existing pool metrics and agents table views.

#### Scenario: Operator views dispatch history on agents page
- **WHEN** operator navigates to the agents dashboard and selects the Dispatch tab
- **THEN** the dispatch history panel displays recent dispatch attempts with task identity, member, runtime, outcome, and timestamp

#### Scenario: Operator runs preflight check from agents page
- **WHEN** operator clicks a preflight check action on the Dispatch tab
- **THEN** the dispatch preflight dialog opens showing admission likelihood, budget status, and pool snapshot
- **AND** the dialog uses the same preflight data as the spawn flow

#### Scenario: Dispatch tab shows event count badge
- **WHEN** the agents page loads and there are recent dispatch events
- **THEN** the Dispatch tab label displays a badge with the count of recent events

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

### Requirement: Queue entries support priority-ordered admission
The system SHALL store a priority level on each agent pool queue entry and use priority as the primary sort key when selecting the next entry for promotion, with creation time as the secondary sort key within equal priority levels.

#### Scenario: ReserveNextQueuedByProject respects priority ordering
- **WHEN** the system promotes the next queued entry for a project and multiple entries are waiting
- **THEN** the entry with the highest priority value is selected
- **THEN** among entries with equal priority, the entry with the earliest `created_at` timestamp is selected
- **THEN** the selected entry transitions from `queued` to `admitted` status

#### Scenario: ListQueuedByProject returns entries in priority order
- **WHEN** an operator requests the queue roster for a project
- **THEN** entries are returned ordered by priority descending, then `created_at` ascending
- **THEN** each entry includes its priority value and semantic label

#### Scenario: Database migration adds priority column with backward-compatible default
- **WHEN** the migration runs on an existing database with queue entries
- **THEN** a `priority INT NOT NULL DEFAULT 0` column is added to `agent_pool_queue_entries`
- **THEN** existing entries receive priority 0 (PriorityLow)
- **THEN** the composite index on `(project_id, status, created_at)` is updated to `(project_id, status, priority DESC, created_at ASC)`

### Requirement: Queue admission accepts an optional priority parameter
The system SHALL accept an optional priority parameter when creating a queue entry, defaulting to 0 (PriorityLow) when not specified.

#### Scenario: Queue entry created with explicit priority
- **WHEN** a dispatch queues an entry with priority set to `high` (20)
- **THEN** the entry is created with priority 20
- **THEN** the entry is promoted before entries with priority less than 20

#### Scenario: Queue entry created without priority defaults to low
- **WHEN** a dispatch queues an entry without specifying a priority
- **THEN** the entry is created with priority 0
- **THEN** the entry follows standard FIFO ordering among other priority-0 entries

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
The system SHALL expose a dedicated queue roster endpoint that returns individual queue entries for a project, complementing the count-based pool summary. The roster MUST include all fields needed for operator decision-making: task identity, member identity, runtime tuple, priority, budget context, current lifecycle status, queue identity, latest guardrail verdict, and the metadata needed to distinguish recoverable waiting states from terminal promotion failures.

#### Scenario: Queue roster returns entries in admission order with verdict metadata
- **WHEN** an authenticated operator requests the queue roster for a project
- **THEN** entries are returned in admission order: priority descending, then creation time ascending
- **THEN** each entry includes task ID, member ID, runtime, provider, model, role ID, priority, budget USD, queue identity, lifecycle status, latest guardrail verdict, and timestamps
- **THEN** a queued, re-queued, promoted, failed, or cancelled entry can be interpreted without parsing only the human-readable reason string

