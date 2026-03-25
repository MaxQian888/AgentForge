## ADDED Requirements

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
