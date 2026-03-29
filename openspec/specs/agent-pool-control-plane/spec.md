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
The system SHALL expose AgentPool lifecycle changes through first-class APIs and realtime events so Web dashboards and other operator surfaces can render queue, warm-pool, and admission transitions truthfully.

#### Scenario: Queue admission change emits a realtime event
- **WHEN** a request is queued, promoted from the queue, or fails during promotion
- **THEN** the system emits an explicit AgentPool lifecycle event for the relevant project scope
- **THEN** realtime consumers can distinguish queued, promoted, started, and failed-promotion states without inferring from missing `agent.started` events

#### Scenario: Operator lists queued work
- **WHEN** an authenticated operator requests the current queue roster for a project
- **THEN** the system returns each queued entry with task identity, member identity, runtime tuple, enqueue time, queue reason, and current queue state
- **THEN** the returned roster matches the same queue facts used by the admission control plane

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

