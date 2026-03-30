# agent-task-dispatch Specification

## Purpose
Define the backend requirements for task-centered agent assignment dispatch, dispatch preflight validation, layered synchronous dispatch outcomes, and truthful IM feedback for assignment-triggered starts.
## Requirements
### Requirement: Agent assignment can trigger a real task dispatch workflow
The system SHALL treat assignment of a task to an active agent member as a task-centered dispatch request instead of only updating assignee fields. When an authenticated caller assigns a task to an agent member, the system MUST persist the assignment, keep task workflow state aligned with the assignment, evaluate AgentPool admission readiness for that task/member/project combination, and return a structured dispatch outcome for the same request.

#### Scenario: Assigning a task to an agent starts dispatch immediately
- **WHEN** an authenticated caller assigns an existing task to an active agent member in the same project, the task has no active agent run, and AgentPool admission has an immediate slot
- **THEN** the system persists the task assignee as that agent member
- **THEN** the system returns the updated task together with a dispatch outcome of `started`
- **THEN** the dispatch outcome includes the started agent run or an equivalent runtime reference for that task

#### Scenario: Assigning a task to an agent queues dispatch
- **WHEN** an authenticated caller assigns an existing task to an active agent member in the same project, the task has no active agent run, and AgentPool admission has no immediate slot
- **THEN** the system persists the task assignee as that agent member
- **THEN** the system returns a dispatch outcome of `queued`
- **THEN** the dispatch outcome includes an admission or queue reference that identifies the queued request for that task

#### Scenario: Assigning a task to a human skips dispatch
- **WHEN** an authenticated caller assigns an existing task to a human member
- **THEN** the system persists the assignment normally
- **THEN** the system returns a dispatch outcome of `skipped`
- **THEN** the system MUST NOT create or start an agent run for that assignment

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

### Requirement: Assignment and dispatch outcomes are visible to synchronous and realtime consumers

The agent detail page SHALL display dispatch context including the dispatch outcome (started, queued, blocked, skipped), preflight summary, and budget metadata. This context SHALL be shown as a dedicated section on the agent detail page so operators can understand how the agent was dispatched without navigating away.

#### Scenario: Agent detail shows dispatch outcome
- **WHEN** operator views an agent's detail page
- **THEN** a Dispatch Context section displays the dispatch outcome (started/queued/blocked/skipped)
- **AND** shows the preflight summary including budget status and pool state at dispatch time

#### Scenario: Agent detail shows dispatch history for the task
- **WHEN** operator views an agent's detail page for a task with multiple dispatch attempts
- **THEN** the Dispatch Context section shows the dispatch history for that task
- **AND** each attempt shows outcome, timestamp, and reason if blocked

#### Scenario: Agent without dispatch context shows minimal info
- **WHEN** operator views an agent that was spawned manually without dispatch
- **THEN** the Dispatch Context section shows "Manual spawn" with the spawn timestamp
- **AND** no preflight or guardrail data is displayed

### Requirement: IM task assignment commands reflect dispatch outcomes truthfully
The system MUST make IM-triggered task assignment reuse the same task dispatch workflow and outcome semantics as the canonical backend API.

#### Scenario: IM task assignment starts an agent
- **WHEN** an IM command assigns a task to an eligible agent target and dispatch starts successfully
- **THEN** the IM command path uses the same backend dispatch workflow as the HTTP assignment path
- **THEN** the user receives a success message that confirms both assignment and agent startup

#### Scenario: IM task assignment is blocked before startup
- **WHEN** an IM command assignment succeeds but dispatch is blocked by validation or runtime preflight
- **THEN** the user receives a result that confirms the task assignment outcome
- **THEN** the same result explains why agent startup did not begin
- **THEN** the command path MUST NOT claim that an agent was started

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

