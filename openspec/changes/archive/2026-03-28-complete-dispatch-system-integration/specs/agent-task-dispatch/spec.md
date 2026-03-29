## MODIFIED Requirements

### Requirement: Dispatch preflight validates the target before runtime startup
The system MUST validate task dispatch targets before attempting to start agent runtime execution. The dispatch preflight MUST reject or block startup when the task does not exist, the assignee is not an active agent member for the task's project, the task already has an active agent run, or any governing budget scope has exhausted its allowance.

#### Scenario: Assignment targets a non-agent or inactive member
- **WHEN** a caller attempts to trigger agent dispatch for a member that is inactive, not part of the task's project, or not typed as `agent`
- **THEN** the system MUST NOT start an agent run
- **THEN** the system returns a dispatch outcome of `blocked`
- **THEN** the outcome explains why the target cannot be dispatched

#### Scenario: Task already has an active agent run
- **WHEN** a caller assigns or dispatches a task that already has an agent run in `starting`, `running`, or `paused` state
- **THEN** the system MUST NOT create a duplicate run
- **THEN** the system returns a dispatch outcome of `blocked`
- **THEN** the outcome explains that the task already has an active agent run

#### Scenario: Budget exhaustion blocks dispatch before runtime startup
- **WHEN** a caller triggers dispatch for a task whose governing task, sprint, or project budget has already exceeded its allowance
- **THEN** the system MUST NOT create a new agent run or queue entry
- **THEN** the system returns a dispatch outcome of `blocked`
- **THEN** the outcome includes a machine-readable budget guardrail classification identifying the exhausted scope

### Requirement: Assignment-triggered dispatch outcomes remain canonical across consumer contracts
The system SHALL expose assignment-triggered dispatch through one canonical outcome contract for synchronous API callers, realtime consumers, and IM bridge clients. The contract MUST preserve explicit `started`, `queued`, `blocked`, and `skipped` branches, together with any queue references, budget guardrail metadata, and machine-readable guardrail details required to explain non-started outcomes.

#### Scenario: Queued assignment preserves queue metadata across surfaces
- **WHEN** assigning a task to an eligible agent member results in a queued dispatch outcome
- **THEN** the synchronous assignment response includes the authoritative queue reference, priority level, and dispatch metadata for that request
- **THEN** the corresponding realtime and IM consumer contracts expose the same `queued` branch instead of collapsing it to generic success or idle text
- **THEN** consumers can identify that assignment succeeded even though runtime startup has not begun yet

#### Scenario: Human assignment remains an explicit skipped outcome
- **WHEN** a task assignment targets a human member and therefore does not attempt agent startup
- **THEN** the synchronous response returns a `skipped` dispatch outcome
- **THEN** IM and other consumer contracts preserve that `skipped` branch explicitly
- **THEN** consumers MUST NOT infer a human assignment indirectly from the absence of run or queue data

#### Scenario: Blocked assignment preserves a machine-readable dispatch reason
- **WHEN** task assignment succeeds but dispatch preflight blocks runtime startup
- **THEN** the synchronous response and realtime dispatch payload include the blocked outcome together with a machine-readable reason classification
- **THEN** the reason classification distinguishes budget, pool, worktree, and target-validation blocks
- **THEN** IM consumer output identifies that assignment completed while startup did not begin
- **THEN** consumers can distinguish budget, pool, worktree, and target-validation blocks without relying on free-form reason strings alone

#### Scenario: Budget-blocked dispatch carries the governing scope in metadata
- **WHEN** a dispatch is blocked because a budget limit was exceeded
- **THEN** the dispatch outcome includes a `guardrailScope` field identifying which scope (task, sprint, or project) caused the block
- **THEN** the dispatch outcome includes a `guardrailType` field with value `budget`
- **THEN** consumers can render the budget-blocked state with the specific scope without parsing the reason string
